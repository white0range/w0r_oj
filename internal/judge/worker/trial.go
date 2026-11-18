package worker

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	chatModel "gojo/internal/chat/model"
	chatRepo "gojo/internal/chat/repository"
	"gojo/internal/judge/service"
	"gojo/internal/judge/trial"
)

func (w *JudgeWorker) runTrials(acceptCtx, taskCtx context.Context) {
	ticker := time.NewTicker(queuePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-acceptCtx.Done():
			return
		case <-ticker.C:
			task, err := trial.Pop(acceptCtx)
			if errors.Is(err, redis.Nil) || acceptCtx.Err() != nil {
				continue
			}
			if err != nil {
				log.Printf("judge trial dequeue failed: %v", err)
				continue
			}
			if !task.Deadline.After(time.Now()) {
				continue
			}
			ctx, cancel := context.WithDeadline(taskCtx, task.Deadline)
			result := runTrial(ctx, task)
			if ctx.Err() == nil {
				if err := trial.Publish(ctx, task.TurnID, result); err != nil {
					log.Printf("[chat] turn_id=%d stage=trial_publish_failed", task.TurnID)
				}
			}
			log.Printf("[chat] turn_id=%d stage=trial_done status=%s", task.TurnID, result.Status)
			cancel()
		}
	}
}

func runTrial(ctx context.Context, task trial.Task) trial.Result {
	if task.TurnID == 0 || task.SubmissionID == 0 || len(task.Input) > trial.MaxInputBytes {
		return trial.Result{Status: "system_error"}
	}
	turns := chatRepo.NewChatRepository()
	turn, err := turns.GetTurnByID(ctx, task.TurnID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !currentTrialAttempt(turn, task.AttemptHash, time.Now())) {
		return trial.Result{Status: "stale"}
	}
	if err != nil {
		return trial.Result{Status: "system_error"}
	}
	session, err := turns.GetSessionByID(ctx, turn.SessionID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && (session.Status != chatModel.ChatSessionStatusActive || session.UserID != turn.UserID)) {
		return trial.Result{Status: "stale"}
	}
	if err != nil {
		return trial.Result{Status: "system_error"}
	}
	submission, err := trial.LoadSubmission(ctx, turn.UserID, task.SubmissionID)
	if err != nil {
		return trial.Result{Status: "system_error"}
	}
	log.Printf("[chat] turn_id=%d stage=trial_start submission_id=%d", task.TurnID, task.SubmissionID)
	result := service.RunTrial(ctx, submission, task.Input)
	turn, err = turns.GetTurnByID(ctx, task.TurnID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !currentTrialAttempt(turn, task.AttemptHash, time.Now())) {
		return trial.Result{Status: "stale"}
	}
	if err != nil {
		return trial.Result{Status: "system_error"}
	}
	return result
}

func currentTrialAttempt(turn *chatModel.ChatTurn, attemptHash string, now time.Time) bool {
	return turn != nil && turn.UserID != 0 && turn.Status == chatModel.TaskStatusRunning && turn.ProcessingToken != "" &&
		turn.LeaseExpiresAt != nil && turn.LeaseExpiresAt.After(now) && trial.HashAttempt(turn.ProcessingToken) == attemptHash
}
