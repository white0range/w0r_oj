package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gojo/infrastructure/cache"
	"gojo/internal/study_plan/dto"
	"gojo/internal/study_plan/model"
	"gojo/internal/study_plan/repository"
	studyPlanService "gojo/internal/study_plan/service"
)

const chatContextWindowSize = 8

func (w *StudyPlanWorker) StartTurnWorkerPool(workerCount int) {
	log.Printf("starting study plan turn worker pool, workers=%d\n", workerCount)
	for i := 1; i <= workerCount; i++ {
		go w.runTurnWorker(i)
	}
}

func (w *StudyPlanWorker) chatRepo() (repository.ChatRepository, error) {
	repo, ok := w.repo.(repository.ChatRepository)
	if !ok {
		return nil, fmt.Errorf("chat repository not configured")
	}
	return repo, nil
}

func (w *StudyPlanWorker) ProcessTurn(ctx context.Context, turnID uint) error {
	repo, err := w.chatRepo()
	if err != nil {
		return err
	}

	turn, err := repo.GetTurnByID(ctx, turnID)
	if err != nil {
		return fmt.Errorf("get study plan turn failed: %w", err)
	}

	if err := repo.UpdateTurnStatus(ctx, turn.ID, model.TaskStatusRunning); err != nil {
		return fmt.Errorf("update study plan turn status to running failed: %w", err)
	}

	session, archivedChunk, err := repo.PrepareSessionCompaction(ctx, turn.SessionID, chatContextWindowSize)
	if err != nil {
		_ = repo.UpdateTurnFailed(ctx, turn.ID, err.Error(), time.Now())
		return fmt.Errorf("prepare session compaction failed: %w", err)
	}

	if len(archivedChunk) > 0 {
		mergedSummary, summaryErr := w.summarizeSessionMessages(ctx, session.SummaryText, toAgentMessages(archivedChunk))
		if summaryErr != nil {
			log.Printf("study plan turn %d llm session summary failed, fallback to rule summary: %v", turn.ID, summaryErr)
			mergedSummary = repository.BuildCompactSessionSummary(session.SummaryText, archivedChunk)
		}

		session, err = repo.ApplySessionCompaction(ctx, turn.SessionID, chatMessageIDs(archivedChunk), mergedSummary)
		if err != nil {
			_ = repo.UpdateTurnFailed(ctx, turn.ID, err.Error(), time.Now())
			return fmt.Errorf("apply session compaction failed: %w", err)
		}
	}

	messages, err := repo.ListRecentMessagesBySessionID(ctx, turn.SessionID, chatContextWindowSize)
	if err != nil {
		_ = repo.UpdateTurnFailed(ctx, turn.ID, err.Error(), time.Now())
		return fmt.Errorf("load session messages failed: %w", err)
	}

	payload := studyPlanAgentRequest{
		UserID:         turn.UserID,
		SessionSummary: strings.TrimSpace(session.SummaryText),
		Messages:       toAgentMessages(messages),
	}
	resultJSON, err := w.callStudyPlanAgentWithPayload(ctx, payload)
	if err != nil {
		_ = repo.UpdateTurnFailed(ctx, turn.ID, err.Error(), time.Now())
		return fmt.Errorf("call study plan agent failed: %w", err)
	}

	assistantContent := studyPlanService.FormatChatAssistantContentForWorker(resultJSON)
	if _, err := repo.CompleteTurn(ctx, turn.ID, assistantContent, resultJSON, time.Now()); err != nil {
		_ = repo.UpdateTurnFailed(ctx, turn.ID, err.Error(), time.Now())
		return fmt.Errorf("complete study plan turn failed: %w", err)
	}

	return nil
}

func runRoleForAgent(role string) string {
	switch role {
	case model.ChatMessageRoleAssistant:
		return model.ChatMessageRoleAssistant
	case model.ChatMessageRoleSystem:
		return model.ChatMessageRoleSystem
	default:
		return model.ChatMessageRoleUser
	}
}

func toAgentMessages(messages []model.ChatMessage) []studyPlanAgentMessage {
	items := make([]studyPlanAgentMessage, 0, len(messages))
	for _, message := range messages {
		items = append(items, studyPlanAgentMessage{
			Role:    runRoleForAgent(message.Role),
			Content: strings.TrimSpace(message.Content),
		})
	}
	return items
}

func chatMessageIDs(messages []model.ChatMessage) []uint {
	ids := make([]uint, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return ids
}

func (w *StudyPlanWorker) runTurnWorker(id int) {
	ctx := context.Background()

	for {
		result, err := cache.Rdb.BRPop(ctx, 0, repository.ChatTurnQueueKey).Result()
		if err != nil {
			log.Printf("study plan turn worker %d pop task failed: %v\n", id, err)
			continue
		}

		if len(result) < 2 {
			continue
		}

		var task dto.ChatTurnQueueTask
		if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
			log.Printf("study plan turn worker %d unmarshal task failed: %v\n", id, err)
			continue
		}

		log.Printf("study plan turn worker %d processing turn_id=%d\n", id, task.TurnID)
		if err := w.ProcessTurn(ctx, task.TurnID); err != nil {
			log.Printf("study plan turn worker %d process turn failed: %v\n", id, err)
		}
	}
}
