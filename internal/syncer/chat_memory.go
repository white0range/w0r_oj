package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"

	"gojo/config"
	"gojo/infrastructure/mysql"
	chatModel "gojo/internal/chat/model"
)

type chatMemoryPayload struct {
	TurnID     uint            `json:"turn_id"`
	UserID     uint            `json:"user_id"`
	QueryText  string          `json:"query_text"`
	Result     json.RawMessage `json:"result"`
	FinishedAt time.Time       `json:"finished_at"`
}

type chatMemoryStore interface {
	LoadConfirmed(context.Context, uint) (*chatMemoryPayload, error)
	MarkSynced(context.Context, uint) error
	ListPending(context.Context, uint, int) ([]uint, error)
}

type mysqlChatMemoryStore struct{}

func (s *mysqlChatMemoryStore) LoadConfirmed(ctx context.Context, turnID uint) (*chatMemoryPayload, error) {
	var turn chatModel.ChatTurn
	err := mysql.DB.WithContext(ctx).Where("id = ? AND status = ? AND memory_sync_pending = ?", turnID, chatModel.TaskStatusSucceeded, true).First(&turn).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if turn.FinishedAt == nil || !json.Valid([]byte(turn.Result)) {
		return nil, errors.New("confirmed chat turn has invalid memory data")
	}
	var session chatModel.ChatSession
	if err := mysql.DB.WithContext(ctx).First(&session, turn.SessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if session.Status != chatModel.ChatSessionStatusActive {
		return nil, nil
	}
	if session.UserID != turn.UserID {
		return nil, errors.New("chat memory session owner mismatch")
	}

	var message chatModel.ChatMessage
	if err := mysql.DB.WithContext(ctx).First(&message, turn.UserMessageID).Error; err != nil {
		return nil, err
	}
	if message.SessionID != turn.SessionID || message.Role != chatModel.ChatMessageRoleUser || message.TurnID == nil || *message.TurnID != turn.ID {
		return nil, errors.New("chat memory source message mismatch")
	}
	return &chatMemoryPayload{TurnID: turn.ID, UserID: turn.UserID, QueryText: message.Content, Result: json.RawMessage(turn.Result), FinishedAt: turn.FinishedAt.UTC()}, nil
}

func (s *mysqlChatMemoryStore) MarkSynced(ctx context.Context, turnID uint) error {
	return mysql.DB.WithContext(ctx).Model(&chatModel.ChatTurn{}).
		Where("id = ? AND status = ?", turnID, chatModel.TaskStatusSucceeded).
		Update("memory_sync_pending", false).Error
}

func (s *mysqlChatMemoryStore) ListPending(ctx context.Context, afterID uint, limit int) ([]uint, error) {
	var ids []uint
	err := mysql.DB.WithContext(ctx).Model(&chatModel.ChatTurn{}).
		Where("id > ? AND status = ? AND memory_sync_pending = ?", afterID, chatModel.TaskStatusSucceeded, true).
		Order("id ASC").Limit(limit).Pluck("id", &ids).Error
	return ids, err
}

type chatMemoryHandler struct {
	store  chatMemoryStore
	client *http.Client
}

func (h *chatMemoryHandler) Handle(ctx context.Context, task Task) error {
	if task.Action != ActionChatMemorySync || task.EntityID == 0 {
		return errors.New("invalid chat memory task")
	}
	started := time.Now()
	log.Printf("[chat] turn_id=%d stage=memory_sync_start task_id=%s", task.EntityID, task.ID)
	payload, err := h.store.LoadConfirmed(ctx, task.EntityID)
	if err != nil {
		return err
	}
	if payload == nil {
		// Already synced, no successful result, or the session was archived.
		log.Printf("[chat] turn_id=%d stage=memory_sync_skipped", task.EntityID)
		return h.store.MarkSynced(ctx, task.EntityID)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(config.GlobalConfig.Chat.AgentBaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/memory/sync", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Service-Token", config.GlobalConfig.Chat.AgentServiceToken)
	client := h.client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("chat memory sync returned status=%d", resp.StatusCode)
	}
	if err := h.store.MarkSynced(ctx, task.EntityID); err != nil {
		return err
	}
	log.Printf("[chat] turn_id=%d stage=memory_sync_done duration_ms=%d", task.EntityID, time.Since(started).Milliseconds())
	return nil
}

func (m *Manager) EnqueueChatMemorySync(ctx context.Context, turnID uint) error {
	return m.queue.enqueue(ctx, NewTask(TargetChatMemory, ActionChatMemorySync, turnID))
}

func (m *Manager) runChatMemoryDispatcher(ctx context.Context) {
	ticker := time.NewTicker(recoveryPollInterval)
	defer ticker.Stop()
	for {
		if err := m.dispatchPendingChatMemories(ctx); err != nil && ctx.Err() == nil {
			log.Printf("dispatch pending chat memories failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) dispatchPendingChatMemories(ctx context.Context) error {
	const batchSize = 100
	var afterID uint
	for {
		ids, err := m.memories.ListPending(ctx, afterID, batchSize)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := m.EnqueueChatMemorySync(ctx, id); err != nil {
				return err
			}
		}
		if len(ids) < batchSize {
			return nil
		}
		afterID = ids[len(ids)-1]
	}
}
