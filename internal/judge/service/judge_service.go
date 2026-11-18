package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"gojo/internal/judge/dto"
	"gojo/internal/judge/model"
	"gojo/internal/judge/repository"
	"gojo/internal/judge/sandbox"
	problemlimits "gojo/internal/problem/limits"
	"gojo/internal/realtime"
	"gojo/pkg/compare"
)

type JudgeService struct {
	repo repository.JudgeRepository
}

func NewJudgeService(r repository.JudgeRepository) *JudgeService {
	return &JudgeService{repo: r}
}

// Process returns an error only when the task was not safely persisted. The
// queue worker retries those errors and acknowledges successful completion.
func (s *JudgeService) Process(ctx context.Context, task dto.JudgeTask) error {
	pending, err := s.repo.IsSubmissionPending(ctx, task.SubmissionID)
	if err != nil {
		return fmt.Errorf("check submission status: %w", err)
	}
	if !pending {
		return nil
	}

	problem, err := s.repo.GetProblemWithCases(ctx, task.ProblemID)
	if err != nil || len(problem.TestCases) == 0 {
		log.Printf("judge submission_id=%d: problem or test cases unavailable", task.SubmissionID)
		return s.complete(ctx, task, model.StatusSystemError, 0, 0)
	}

	workDir, err := os.MkdirTemp("", "judge_*")
	if err != nil {
		log.Printf("judge submission_id=%d: create work directory failed", task.SubmissionID)
		return s.complete(ctx, task, model.StatusSystemError, 0, 0)
	}
	defer os.RemoveAll(workDir)

	compiled, _, err := sandbox.CompileCode(ctx, task.Code, workDir, task.SubmissionID)
	if err != nil {
		log.Printf("judge submission_id=%d: compiler infrastructure failed", task.SubmissionID)
		return s.complete(ctx, task, model.StatusSystemError, 0, 0)
	}
	if !compiled {
		return s.complete(ctx, task, model.StatusCompileError, 0, 0)
	}

	if !problemlimits.ValidTimeMS(problem.TimeLimit) || !problemlimits.ValidMemoryMB(problem.MemoryLimit) {
		log.Printf("judge submission_id=%d: invalid problem resource limits", task.SubmissionID)
		return s.complete(ctx, task, model.StatusSystemError, 0, 0)
	}
	memoryLimitMB := problem.MemoryLimit

	containerID, err := sandbox.StartPersistentSandbox(ctx, workDir, int64(memoryLimitMB), task.SubmissionID)
	if err != nil {
		log.Printf("judge submission_id=%d: start sandbox failed", task.SubmissionID)
		return s.complete(ctx, task, model.StatusSystemError, 0, 0)
	}
	defer sandbox.RemoveSandbox(ctx, containerID)

	cpuLimitMS := problem.TimeLimit
	wallLimitMS := cpuLimitMS * 2

	finalStatus := model.StatusAccepted
	maxTimeCost := 0
	maxMemoryCost := 0

	for _, tc := range problem.TestCases {
		execDeadline := time.Duration(wallLimitMS+1000) * time.Millisecond
		execCtx, cancel := context.WithTimeout(ctx, execDeadline)
		result := sandbox.ExecTestCase(execCtx, containerID, tc.Input, cpuLimitMS, wallLimitMS, memoryLimitMB)
		cancel()

		if result.TimeCost > maxTimeCost {
			maxTimeCost = result.TimeCost
		}
		if result.MemoryCost > maxMemoryCost {
			maxMemoryCost = result.MemoryCost
		}

		if result.Error != nil {
			log.Printf("judge submission_id=%d: runner infrastructure failed", task.SubmissionID)
			finalStatus = model.StatusSystemError
			break
		}
		if result.Status != model.StatusAccepted {
			finalStatus = result.Status
			break
		}
		if !compare.CompareOutput(result.Output, tc.ExpectedOutput) {
			finalStatus = model.StatusWrongAnswer
			break
		}
	}

	return s.complete(ctx, task, finalStatus, maxTimeCost, maxMemoryCost)
}

func (s *JudgeService) complete(ctx context.Context, task dto.JudgeTask, status model.JudgeStatus, timeCost, memoryCost int) error {
	updated, err := s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, status, timeCost, memoryCost)
	if err != nil {
		return fmt.Errorf("persist judge result: %w", err)
	}
	if !updated {
		return nil
	}

	if err := realtime.PublishDistributed(ctx, task.UserID, "judge_result", map[string]any{
		"submission_id": task.SubmissionID,
		"status":        status,
	}); err != nil {
		// The result is already durable in MySQL. SSE is best effort and clients
		// can always retrieve the final state through the submission endpoint.
		log.Printf("publish judge result event submission_id=%d failed: %v", task.SubmissionID, err)
	}
	return nil
}

// MarkFailed resolves a task that has exhausted infrastructure retries so it
// does not remain Pending forever. If persistence is unavailable, the pending
// reconciler will try again when the database recovers.
func (s *JudgeService) MarkFailed(ctx context.Context, task dto.JudgeTask) error {
	return s.complete(ctx, task, model.StatusSystemError, 0, 0)
}
