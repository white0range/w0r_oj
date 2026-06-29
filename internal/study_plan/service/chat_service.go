package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"gojo/config"
	"gojo/internal/app/apperror"
	"gojo/internal/study_plan/dto"
	"gojo/internal/study_plan/model"
	"gojo/internal/study_plan/repository"

	"gorm.io/gorm"
)

func (s *StudyPlanService) chatRepo() (repository.ChatRepository, error) {
	repo, ok := s.repo.(repository.ChatRepository)
	if !ok {
		return nil, errors.New("chat repository not configured")
	}
	return repo, nil
}

func (s *StudyPlanService) CreateChatSession(ctx context.Context, userID uint, title string) (*model.ChatSession, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}

	session := &model.ChatSession{
		UserID: userID,
		Title:  strings.TrimSpace(title),
		Status: model.ChatSessionStatusActive,
	}
	if err := repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *StudyPlanService) ListChatSessions(ctx context.Context, userID uint, limit int) ([]model.ChatSession, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return repo.ListSessionsByUserID(ctx, userID, limit)
}

func (s *StudyPlanService) GetChatSession(ctx context.Context, userID uint, sessionID uint) (*model.ChatSession, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	session, err := repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apperror.ErrForbidden
	}
	if session.Status != model.ChatSessionStatusActive {
		return nil, gorm.ErrRecordNotFound
	}
	return session, nil
}

func (s *StudyPlanService) ArchiveChatSession(ctx context.Context, userID uint, sessionID uint) error {
	repo, err := s.chatRepo()
	if err != nil {
		return err
	}

	session, err := repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return apperror.ErrForbidden
	}
	if session.Status != model.ChatSessionStatusActive {
		return gorm.ErrRecordNotFound
	}

	hasActiveTurn, err := repo.HasActiveTurn(ctx, sessionID)
	if err != nil {
		return err
	}
	if hasActiveTurn {
		return apperror.ErrChatSessionBusy
	}

	return repo.ArchiveSession(ctx, sessionID)
}

func (s *StudyPlanService) GetChatMessages(ctx context.Context, userID uint, sessionID uint, limit int) ([]model.ChatMessage, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	if _, err := s.GetChatSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	if limit < 0 {
		limit = 0
	}
	return repo.ListMessagesBySessionID(ctx, sessionID, limit)
}

func (s *StudyPlanService) SendChatMessage(ctx context.Context, userID uint, sessionID uint, content string) (*model.ChatTurn, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}

	session, err := s.GetChatSession(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("message content is empty")
	}

	modelName := config.GlobalConfig.AI.Model
	if strings.TrimSpace(modelName) == "" {
		modelName = "study-plan-agent"
	}

	_, turn, err := repo.CreateUserMessageTurn(ctx, session, content, modelName)
	if err != nil {
		return nil, err
	}

	queueTask := dto.ChatTurnQueueTask{TurnID: turn.ID}
	taskBytes, err := json.Marshal(queueTask)
	if err != nil {
		return nil, err
	}
	if err := repo.PushTurnToQueue(ctx, taskBytes); err != nil {
		return nil, err
	}

	return turn, nil
}

func (s *StudyPlanService) GetChatTurn(ctx context.Context, userID uint, turnID uint) (*model.ChatTurn, error) {
	repo, err := s.chatRepo()
	if err != nil {
		return nil, err
	}
	turn, err := repo.GetTurnByID(ctx, turnID)
	if err != nil {
		return nil, err
	}
	session, err := repo.GetSessionByID(ctx, turn.SessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, apperror.ErrForbidden
	}
	if session.Status != model.ChatSessionStatusActive {
		return nil, gorm.ErrRecordNotFound
	}
	return turn, nil
}

func FormatChatAssistantContentForWorker(rawResult string) string {
	type recommendedProblem struct {
		ProblemID uint   `json:"problem_id"`
		Title     string `json:"title"`
		Reason    string `json:"reason"`
	}
	type studyPlanResult struct {
		WeakTags            []string             `json:"weak_tags"`
		RecommendedProblems []recommendedProblem `json:"recommended_problems"`
		StudyPlanSummary    string               `json:"study_plan_summary"`
	}

	var parsed studyPlanResult
	if err := json.Unmarshal([]byte(rawResult), &parsed); err != nil {
		return strings.TrimSpace(rawResult)
	}

	lines := make([]string, 0, 8)
	if strings.TrimSpace(parsed.StudyPlanSummary) != "" {
		lines = append(lines, parsed.StudyPlanSummary)
	}
	if len(parsed.WeakTags) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Weak tags: "+strings.Join(parsed.WeakTags, ", "))
	}
	if len(parsed.RecommendedProblems) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Recommended problems:")
		for _, item := range parsed.RecommendedProblems {
			lines = append(lines, "- #"+uintToString(item.ProblemID)+" "+item.Title+": "+item.Reason)
		}
	}
	if len(lines) == 0 {
		return strings.TrimSpace(rawResult)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func BuildChatGoalForWorker(messages []model.ChatMessage) string {
	if len(messages) == 0 {
		return "general study plan"
	}

	lines := []string{
		"You are continuing an ongoing OJ learning conversation.",
		"Answer the latest user message directly.",
		"If the user asks for practice recommendations, you may recommend problems.",
		"If the user asks a basic algorithm or OJ question, answer it clearly in the summary field and keep recommendations empty when appropriate.",
		"Conversation history:",
	}
	for _, message := range messages {
		lines = append(lines, chatRoleLabel(message.Role)+": "+strings.TrimSpace(message.Content))
	}
	return strings.Join(lines, "\n")
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
