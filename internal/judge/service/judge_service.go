package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"gojo/infrastructure/websocket"
	"gojo/internal/judge/dto"
	"gojo/internal/judge/model"
	"gojo/internal/judge/repository"
	"gojo/internal/judge/sandbox"
	"gojo/pkg/compare"

	"github.com/gin-gonic/gin"
)

type JudgeService struct {
	repo repository.JudgeRepository
}

func NewJudgeService(r repository.JudgeRepository) *JudgeService {
	return &JudgeService{repo: r}
}

func (s *JudgeService) Process(ctx context.Context, task dto.JudgeTask) {
	problem, err := s.repo.GetProblemWithCases(ctx, task.ProblemID)
	if err != nil || len(problem.TestCases) == 0 {
		_ = s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, "SE", "problem data is invalid", 0, 0)
		return
	}

	workDir, err := os.MkdirTemp("", "judge_*")
	if err != nil {
		_ = s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, "SE", "create workdir failed", 0, 0)
		return
	}
	defer os.RemoveAll(workDir)

	compiled, info, err := sandbox.CompileCode(ctx, task.Code, workDir)
	if err != nil {
		_ = s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, "SE", err.Error(), 0, 0)
		return
	}
	if !compiled {
		_ = s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, string(model.StatusCompileError), info, 0, 0)
		return
	}

	memoryLimitMB := problem.MemoryLimit
	if memoryLimitMB <= 0 {
		memoryLimitMB = 256
	}

	containerID, err := sandbox.StartPersistentSandbox(ctx, workDir, int64(memoryLimitMB))
	if err != nil {
		_ = s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, "SE", "start sandbox failed", 0, 0)
		return
	}
	defer sandbox.RemoveSandbox(ctx, containerID)

	cpuLimitMS := problem.TimeLimit
	if cpuLimitMS <= 0 {
		cpuLimitMS = 1000
	}
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

	_ = s.repo.UpdateJudgeResult(ctx, task.SubmissionID, task.ProblemID, task.UserID, finalStatus, finalOutput, maxTimeCost, maxMemoryCost)

	websocket.SendWsMessage(fmt.Sprintf("%d", task.UserID), gin.H{
		"type":          "JUDGE_RESULT",
		"submission_id": task.SubmissionID,
		"status":        finalStatus,
	})
}
