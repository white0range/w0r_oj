package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/internal/chat/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type chatRepoMysql struct{}

const ChatTurnQueueKey = "chat_turn_queue"
const ChatTurnProcessingQueueKey = "chat_turn_processing"
const ChatTurnDispatchPrefix = "chat_turn_dispatch:"

const (
	chatSummaryMaxChars      = 4000
	chatSummaryMessageMaxLen = 240
)

type ChatRepository interface {
	CreateSession(ctx context.Context, session *model.ChatSession) error
	ListSessionsByUserID(ctx context.Context, userID uint, limit int) ([]model.ChatSession, error)
	GetSessionByID(ctx context.Context, sessionID uint) (*model.ChatSession, error)
	ArchiveSession(ctx context.Context, sessionID uint) error
	HasActiveTurn(ctx context.Context, sessionID uint) (bool, error)
	ListMessagesBySessionID(ctx context.Context, sessionID uint, limit int) ([]model.ChatMessage, error)
	ListRecentMessagesBySessionID(ctx context.Context, sessionID uint, limit int) ([]model.ChatMessage, error)
	PrepareSessionCompaction(ctx context.Context, sessionID uint, keepRecent int) (*model.ChatSession, []model.ChatMessage, error)
	ApplySessionCompaction(ctx context.Context, sessionID uint, archivedMessageIDs []uint, mergedSummary string) (*model.ChatSession, error)
	CreateUserMessageTurn(ctx context.Context, session *model.ChatSession, content string, modelName string) (*model.ChatMessage, *model.ChatTurn, error)
	GetTurnByID(ctx context.Context, turnID uint) (*model.ChatTurn, error)
	ClaimTurn(ctx context.Context, turnID uint, processingToken string, leaseExpiresAt time.Time) (*model.ChatTurn, bool, error)
	RenewTurnLease(ctx context.Context, turnID uint, processingToken string, leaseExpiresAt time.Time) (bool, error)
	ListDispatchableTurnIDs(ctx context.Context, now time.Time, limit int) ([]uint, error)
	UpsertPlanFeedback(ctx context.Context, feedback *model.ChatPlanFeedback) error
	GetPlanFeedbackByTurnIDAndUserID(ctx context.Context, turnID uint, userID uint) (*model.ChatPlanFeedback, error)
	CompleteClaimedTurn(ctx context.Context, turnID uint, processingToken string, assistantContent string, rawResult string, finishedAt time.Time) (*model.ChatMessage, bool, error)
	FailClaimedTurn(ctx context.Context, turnID uint, processingToken string, errorMessage string, finishedAt time.Time) (bool, error)
	PushTurnToQueue(ctx context.Context, taskBytes []byte) error
}

func NewChatRepository() ChatRepository {
	return &chatRepoMysql{}
}

func (r *chatRepoMysql) CreateSession(ctx context.Context, session *model.ChatSession) error {
	return mysql.DB.WithContext(ctx).Create(session).Error
}

func (r *chatRepoMysql) ListSessionsByUserID(ctx context.Context, userID uint, limit int) ([]model.ChatSession, error) {
	var sessions []model.ChatSession
	query := mysql.DB.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, model.ChatSessionStatusActive).
		Order("COALESCE(last_message_at, created_at) DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&sessions).Error
	return sessions, err
}

func (r *chatRepoMysql) GetSessionByID(ctx context.Context, sessionID uint) (*model.ChatSession, error) {
	var session model.ChatSession
	if err := mysql.DB.WithContext(ctx).First(&session, sessionID).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *chatRepoMysql) ArchiveSession(ctx context.Context, sessionID uint) error {
	return mysql.DB.WithContext(ctx).
		Model(&model.ChatSession{}).
		Where("id = ?", sessionID).
		Update("status", model.ChatSessionStatusArchived).Error
}

func (r *chatRepoMysql) HasActiveTurn(ctx context.Context, sessionID uint) (bool, error) {
	var count int64
	err := mysql.DB.WithContext(ctx).
		Model(&model.ChatTurn{}).
		Where("session_id = ? AND status IN ?", sessionID, []string{model.TaskStatusPending, model.TaskStatusRunning}).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *chatRepoMysql) ListMessagesBySessionID(ctx context.Context, sessionID uint, limit int) ([]model.ChatMessage, error) {
	var messages []model.ChatMessage
	query := mysql.DB.WithContext(ctx).
		Where("session_id = ? AND archived = ?", sessionID, false).
		Order("id ASC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&messages).Error
	return messages, err
}

func (r *chatRepoMysql) ListRecentMessagesBySessionID(ctx context.Context, sessionID uint, limit int) ([]model.ChatMessage, error) {
	if limit <= 0 {
		return r.ListMessagesBySessionID(ctx, sessionID, 0)
	}

	var recent []model.ChatMessage
	if err := mysql.DB.WithContext(ctx).
		Where("session_id = ? AND archived = ? AND is_summary = ?", sessionID, false, false).
		Order("id DESC").
		Limit(limit).
		Find(&recent).Error; err != nil {
		return nil, err
	}

	for left, right := 0, len(recent)-1; left < right; left, right = left+1, right-1 {
		recent[left], recent[right] = recent[right], recent[left]
	}
	return recent, nil
}

func (r *chatRepoMysql) PrepareSessionCompaction(ctx context.Context, sessionID uint, keepRecent int) (*model.ChatSession, []model.ChatMessage, error) {
	if keepRecent <= 0 {
		keepRecent = 8
	}

	session, err := r.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, nil, err
	}

	var active []model.ChatMessage
	if err := mysql.DB.WithContext(ctx).
		Where("session_id = ? AND archived = ? AND is_summary = ?", sessionID, false, false).
		Order("id ASC").
		Find(&active).Error; err != nil {
		return nil, nil, err
	}

	if len(active) <= keepRecent {
		return session, nil, nil
	}

	cutoff := len(active) - keepRecent
	chunk := append([]model.ChatMessage(nil), active[:cutoff]...)
	return session, chunk, nil
}

func (r *chatRepoMysql) ApplySessionCompaction(ctx context.Context, sessionID uint, archivedMessageIDs []uint, mergedSummary string) (*model.ChatSession, error) {
	var session model.ChatSession

	err := mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&session, sessionID).Error; err != nil {
			return err
		}

		if len(archivedMessageIDs) > 0 {
			if err := tx.Model(&model.ChatMessage{}).Where("id IN ?", archivedMessageIDs).Update("archived", true).Error; err != nil {
				return err
			}
		}

		nextSummary := strings.TrimSpace(mergedSummary)
		if nextSummary != session.SummaryText {
			if err := tx.Model(&model.ChatSession{}).Where("id = ?", sessionID).Update("summary_text", nextSummary).Error; err != nil {
				return err
			}
			session.SummaryText = nextSummary
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &session, nil
}

func (r *chatRepoMysql) CreateUserMessageTurn(ctx context.Context, session *model.ChatSession, content string, modelName string) (*model.ChatMessage, *model.ChatTurn, error) {
	var message model.ChatMessage
	var turn model.ChatTurn
	now := time.Now()

	err := mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		message = model.ChatMessage{
			SessionID: session.ID,
			Role:      model.ChatMessageRoleUser,
			Content:   content,
		}
		if err := tx.Create(&message).Error; err != nil {
			return err
		}

		turn = model.ChatTurn{
			SessionID:     session.ID,
			UserID:        session.UserID,
			UserMessageID: message.ID,
			Status:        model.TaskStatusPending,
			Model:         modelName,
		}
		if err := tx.Create(&turn).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.ChatMessage{}).Where("id = ?", message.ID).Update("turn_id", turn.ID).Error; err != nil {
			return err
		}
		message.TurnID = &turn.ID

		updates := map[string]interface{}{
			"last_message_at": now,
		}
		if strings.TrimSpace(session.Title) == "" {
			updates["title"] = chatSessionTitleFromContent(content)
			session.Title = updates["title"].(string)
		}
		if err := tx.Model(&model.ChatSession{}).Where("id = ?", session.ID).Updates(updates).Error; err != nil {
			return err
		}
		session.LastMessageAt = &now
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return &message, &turn, nil
}

func (r *chatRepoMysql) GetTurnByID(ctx context.Context, turnID uint) (*model.ChatTurn, error) {
	var turn model.ChatTurn
	if err := mysql.DB.WithContext(ctx).First(&turn, turnID).Error; err != nil {
		return nil, err
	}
	return &turn, nil
}

func (r *chatRepoMysql) ClaimTurn(ctx context.Context, turnID uint, processingToken string, leaseExpiresAt time.Time) (*model.ChatTurn, bool, error) {
	now := time.Now()
	result := mysql.DB.WithContext(ctx).
		Model(&model.ChatTurn{}).
		Where("id = ? AND (status = ? OR (status = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)))", turnID, model.TaskStatusPending, model.TaskStatusRunning, now).
		Updates(map[string]interface{}{
			"status":           model.TaskStatusRunning,
			"processing_token": processingToken,
			"lease_expires_at": leaseExpiresAt,
			"error_message":    "",
			"finished_at":      nil,
		})
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}

	var turn model.ChatTurn
	if err := mysql.DB.WithContext(ctx).First(&turn, turnID).Error; err != nil {
		return nil, false, err
	}
	return &turn, true, nil
}

func (r *chatRepoMysql) RenewTurnLease(ctx context.Context, turnID uint, processingToken string, leaseExpiresAt time.Time) (bool, error) {
	result := mysql.DB.WithContext(ctx).
		Model(&model.ChatTurn{}).
		Where("id = ? AND status = ? AND processing_token = ?", turnID, model.TaskStatusRunning, processingToken).
		Update("lease_expires_at", leaseExpiresAt)
	return result.RowsAffected == 1, result.Error
}

func (r *chatRepoMysql) ListDispatchableTurnIDs(ctx context.Context, now time.Time, limit int) ([]uint, error) {
	if limit <= 0 {
		limit = 100
	}
	var turns []model.ChatTurn
	if err := mysql.DB.WithContext(ctx).
		Select("id").
		Where("status = ? OR (status = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?))", model.TaskStatusPending, model.TaskStatusRunning, now).
		Order("id ASC").
		Limit(limit).
		Find(&turns).Error; err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(turns))
	for _, turn := range turns {
		ids = append(ids, turn.ID)
	}
	return ids, nil
}

func (r *chatRepoMysql) CompleteClaimedTurn(ctx context.Context, turnID uint, processingToken string, assistantContent string, rawResult string, finishedAt time.Time) (*model.ChatMessage, bool, error) {
	var turn model.ChatTurn
	var assistantMessage model.ChatMessage

	err := mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND status = ? AND processing_token = ?", turnID, model.TaskStatusRunning, processingToken).First(&turn).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}

		assistantMessage = model.ChatMessage{
			SessionID:         turn.SessionID,
			TurnID:            &turn.ID,
			Role:              model.ChatMessageRoleAssistant,
			Content:           assistantContent,
			StructuredPayload: rawResult,
		}
		if err := tx.Create(&assistantMessage).Error; err != nil {
			return err
		}

		result := tx.Model(&model.ChatTurn{}).
			Where("id = ? AND status = ? AND processing_token = ?", turn.ID, model.TaskStatusRunning, processingToken).
			Updates(map[string]interface{}{
				"assistant_message_id": assistantMessage.ID,
				"status":               model.TaskStatusSucceeded,
				"result":               rawResult,
				"finished_at":          finishedAt,
				"processing_token":     "",
				"lease_expires_at":     nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("chat turn claim was lost during completion")
		}
		return tx.Model(&model.ChatSession{}).Where("id = ?", turn.SessionID).Update("last_message_at", finishedAt).Error
	})
	if err != nil {
		return nil, false, err
	}
	if turn.ID == 0 {
		return nil, false, nil
	}
	return &assistantMessage, true, nil
}

func (r *chatRepoMysql) FailClaimedTurn(ctx context.Context, turnID uint, processingToken string, errorMessage string, finishedAt time.Time) (bool, error) {
	result := mysql.DB.WithContext(ctx).
		Model(&model.ChatTurn{}).
		Where("id = ? AND status = ? AND processing_token = ?", turnID, model.TaskStatusRunning, processingToken).
		Updates(map[string]interface{}{
			"status":           model.TaskStatusFailed,
			"error_message":    errorMessage,
			"finished_at":      finishedAt,
			"processing_token": "",
			"lease_expires_at": nil,
		})
	return result.RowsAffected == 1, result.Error
}
func (r *chatRepoMysql) PushTurnToQueue(ctx context.Context, taskBytes []byte) error {
	return cache.Rdb.LPush(ctx, ChatTurnQueueKey, taskBytes).Err()
}

func chatSessionTitleFromContent(content string) string {
	title := strings.TrimSpace(content)
	if title == "" {
		return "New Chat"
	}
	runes := []rune(title)
	if len(runes) > 32 {
		return strings.TrimSpace(string(runes[:32])) + "..."
	}
	return title
}

func BuildCompactSessionSummary(existing string, messages []model.ChatMessage) string {
	return mergeSessionSummary(existing, summarizeArchivedChatMessages(messages))
}

func summarizeArchivedChatMessages(messages []model.ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}

	lines := make([]string, 0, len(messages))
	for _, message := range messages {
		content := collapseWhitespace(message.Content)
		if content == "" {
			continue
		}
		content = truncateRunes(content, chatSummaryMessageMaxLen)
		lines = append(lines, chatRoleLabel(message.Role)+": "+content)
	}
	return strings.Join(lines, "\n")
}

func mergeSessionSummary(existing string, fragment string) string {
	existing = strings.TrimSpace(existing)
	fragment = strings.TrimSpace(fragment)

	switch {
	case existing == "":
		return truncateRunes(fragment, chatSummaryMaxChars)
	case fragment == "":
		return truncateRunes(existing, chatSummaryMaxChars)
	default:
		return truncateRunes(existing+"\n"+fragment, chatSummaryMaxChars)
	}
}

func chatRoleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case model.ChatMessageRoleAssistant:
		return "Assistant"
	case model.ChatMessageRoleSystem:
		return "System"
	default:
		return "User"
	}
}

func collapseWhitespace(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func truncateRunes(text string, maxChars int) string {
	if maxChars <= 0 || utf8.RuneCountInString(text) <= maxChars {
		return text
	}

	runes := []rune(text)
	if maxChars <= 3 {
		return string(runes[:maxChars])
	}
	return string(runes[:maxChars-3]) + "..."
}

func (r *chatRepoMysql) UpsertPlanFeedback(ctx context.Context, feedback *model.ChatPlanFeedback) error {
	return mysql.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "chat_turn_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"helpful", "comment", "updated_at"}),
	}).Create(feedback).Error
}

func (r *chatRepoMysql) GetPlanFeedbackByTurnIDAndUserID(ctx context.Context, turnID uint, userID uint) (*model.ChatPlanFeedback, error) {
	var feedback model.ChatPlanFeedback
	if err := mysql.DB.WithContext(ctx).Where("chat_turn_id = ? AND user_id = ?", turnID, userID).First(&feedback).Error; err != nil {
		return nil, err
	}
	return &feedback, nil
}
