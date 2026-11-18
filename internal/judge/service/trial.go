package service

import (
	"context"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	judgeModel "gojo/internal/judge/model"
	"gojo/internal/judge/sandbox"
	"gojo/internal/judge/trial"
	"gojo/internal/problem/limits"
	problemRepo "gojo/internal/problem/repository"
)

// RunTrial compiles one source buffer and runs only the supplied input.
func RunTrial(ctx context.Context, problemID, submissionID uint, code, input string) trial.Result {
	failure := trial.Result{Status: "system_error"}
	problem, err := problemRepo.NewProblemRepository().GetProblemByID(ctx, strconv.FormatUint(uint64(problemID), 10))
	if err != nil || !limits.ValidTimeMS(problem.TimeLimit) || !limits.ValidMemoryMB(problem.MemoryLimit) {
		return failure
	}
	workDir, err := os.MkdirTemp("", "judge_trial_*")
	if err != nil {
		return failure
	}
	defer os.RemoveAll(workDir)

	compiled, info, err := sandbox.CompileCode(ctx, code, workDir, submissionID)
	if err != nil {
		return failure
	}
	if !compiled {
		return trialOutput(string(judgeModel.StatusCompileError), info, 0)
	}
	containerID, err := sandbox.StartPersistentSandbox(ctx, workDir, int64(problem.MemoryLimit), submissionID)
	if err != nil {
		return failure
	}
	defer sandbox.RemoveSandbox(ctx, containerID)

	result := sandbox.ExecTestCase(ctx, containerID, input, problem.TimeLimit, 2*problem.TimeLimit, problem.MemoryLimit)
	if result.Error != nil {
		return failure
	}
	status := string(result.Status)
	if result.Status == judgeModel.StatusAccepted {
		status = "ok" // Running a custom input does not establish an accepted verdict.
	}
	return trialOutput(status, result.Output, result.WallTimeCost)
}

func trialOutput(status, output string, durationMS int) trial.Result {
	const maxOutputBytes = 4 * 1024
	output = strings.ToValidUTF8(output, "\uFFFD")
	truncated := len(output) > maxOutputBytes
	if truncated {
		end := maxOutputBytes
		for !utf8.RuneStart(output[end]) {
			end--
		}
		output = output[:end]
	}
	return trial.Result{Status: status, Output: output, DurationMS: durationMS, Truncated: truncated}
}
