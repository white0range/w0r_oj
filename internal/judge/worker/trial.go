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
				if err := trial.Publish(ctx, task, result); err != nil {
					log.Printf("judge trial publish failed turn_id=%d", task.TurnID)
				}
			}
			if task.CodeRun != nil {
				log.Printf("[judge] run_id=%s stage=trial_done status=%s", task.CodeRun.ID, result.Status)
			} else {
				log.Printf("[chat] turn_id=%d stage=trial_done status=%s", task.TurnID, result.Status)
			}
			cancel()
		}
	}
}

func runTrial(ctx context.Context, task trial.Task) trial.Result {
	if len(task.Input) > trial.MaxInputBytes {
		return trial.Result{Status: "system_error"}
	}
	if run := task.CodeRun; run != nil {
		if run.ID == "" || run.UserID == 0 || run.ProblemID == 0 || len(run.Code) == 0 || len(run.Code) > trial.MaxCodeBytes {
			return trial.Result{Status: "system_error"}
		}
		return service.RunTrial(ctx, run.ProblemID, 0, run.Code, task.Input)
	}
	if task.TurnID == 0 || task.SubmissionID == 0 {
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
	result := service.RunTrial(ctx, submission.ProblemID, submission.ID, submission.Code, task.Input)
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
