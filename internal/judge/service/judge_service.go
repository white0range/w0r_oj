package service

import (
	"context"
	"fmt"
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
		return s.complete(ctx, task, "SE", "problem data is invalid", 0, 0)
	}

	workDir, err := os.MkdirTemp("", "judge_*")
	if err != nil {
		return s.complete(ctx, task, "SE", "create workdir failed", 0, 0)
	}
	defer os.RemoveAll(workDir)

	compiled, info, err := sandbox.CompileCode(ctx, task.Code, workDir)
	if err != nil {
		return s.complete(ctx, task, "SE", err.Error(), 0, 0)
	}
	if !compiled {
		return s.complete(ctx, task, string(model.StatusCompileError), info, 0, 0)
	}

	if !problemlimits.ValidTimeMS(problem.TimeLimit) || !problemlimits.ValidMemoryMB(problem.MemoryLimit) {
		return s.complete(ctx, task, "SE", "problem resource limits are outside the allowed range", 0, 0)
	}
	memoryLimitMB := problem.MemoryLimit

	containerID, err := sandbox.StartPersistentSandbox(ctx, workDir, int64(memoryLimitMB))
	if err != nil {
		return s.complete(ctx, task, "SE", "start sandbox failed", 0, 0)
	}
	defer sandbox.RemoveSandbox(ctx, containerID)

	cpuLimitMS := problem.TimeLimit
	wallLimitMS := cpuLimitMS * 2

	finalStatus := string(model.StatusAccepted)
	finalOutput := "all test cases passed"
	maxTimeCost := 0
	maxMemoryCost := 0

	for i, tc := range problem.TestCases {
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
			finalStatus = string(model.StatusSystemError)
			finalOutput = result.Error.Error()
			break
		}
		if result.Status != model.StatusAccepted {
			finalStatus = string(result.Status)
			finalOutput = fmt.Sprintf("test case %d failed:\n%s", i+1, result.Output)
			break
		}
		if !compare.CompareOutput(result.Output, tc.ExpectedOutput) {
			finalStatus = string(model.StatusWrongAnswer)
			finalOutput = fmt.Sprintf("wrong answer on test case %d:\n%s", i+1, result.Output)
			break
		}
	}

	return s.complete(ctx, task, finalStatus, finalOutput, maxTimeCost, maxMemoryCost)
}

func (s *JudgeService) complete(ctx context.Context, task dto.JudgeTask, status, output string, timeCost, memoryCost int) error {
	updated, err := s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, status, output, timeCost, memoryCost)
	if err != nil {
		return fmt.Errorf("persist judge result: %w", err)
	}
	if !updated {
		return nil
	}

	realtime.Publish(task.UserID, "judge_result", map[string]any{
		"submission_id": task.SubmissionID,
		"status":        status,
	})
	return nil
}

// MarkFailed resolves a task that has exhausted infrastructure retries so it
// does not remain Pending forever. If persistence is unavailable, the pending
// reconciler will try again when the database recovers.
func (s *JudgeService) MarkFailed(ctx context.Context, task dto.JudgeTask, cause error) error {
	return s.complete(ctx, task, "SE", "judge failed after retries: "+cause.Error(), 0, 0)
}
