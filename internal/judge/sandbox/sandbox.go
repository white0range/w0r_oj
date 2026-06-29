package sandbox

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gojo/config"
	"gojo/internal/judge/docker"
	"gojo/internal/judge/model"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

//go:embed runner/runner_source.txt
var judgeRunnerSource string

type runnerExecResult struct {
	ExitCode     int    `json:"exit_code"`
	Signal       int    `json:"signal"`
	WallTimeMS   int64  `json:"wall_time_ms"`
	CPUTimeMS    int64  `json:"cpu_time_ms"`
	MaxRSSKB     int64  `json:"max_rss_kb"`
	WallTimedOut bool   `json:"wall_timed_out"`
	Stdout       string `json:"stdout"`
	Stderr       string `json:"stderr"`
	Error        string `json:"error"`
}

const (
	linuxSIGXCPU            = 24
	defaultCompileTimeout   = 45 * time.Second
	judgeRuntimeImage       = "golang:alpine"
)

func CompileCode(ctx context.Context, code string, workDir string) (bool, string, error) {
	codePath := filepath.Join(workDir, "main.go")
	if err := os.WriteFile(codePath, []byte(code), 0644); err != nil {
		return false, "", fmt.Errorf("write solution code failed: %w", err)
	}
	if err := writeJudgeRunner(workDir); err != nil {
		return false, "", fmt.Errorf("prepare judge runner failed: %w", err)
	}

	resp, err := docker.DockerClient.ContainerCreate(ctx,
		&container.Config{
			Image:      judgeRuntimeImage,
			WorkingDir: "/app",
			Cmd: []string{
				"sh",
				"-c",
				"if ! GO111MODULE=off go build -o solution main.go; then exit 11; fi; if ! GO111MODULE=off go build -o .judge/runner .judge/runner.go; then exit 12; fi",
			},
		},
		&container.HostConfig{
			Binds: []string{workDir + ":/app"},
		}, nil, nil, "")
	if err != nil {
		return false, "", err
	}
	defer docker.DockerClient.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})

	if err := docker.DockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return false, "", fmt.Errorf("start compile container failed: %w", err)
	}

	statusCh, errCh := docker.DockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	compileTimeout := getCompileTimeout()
	timeoutCh := time.After(compileTimeout)

	select {
	case err := <-errCh:
		if err != nil {
			return false, "", fmt.Errorf("compile container wait failed: %w", err)
		}
	case status := <-statusCh:
		return finalizeCompileResult(ctx, resp.ID, int64(status.StatusCode))
	case <-timeoutCh:
		if killErr := docker.DockerClient.ContainerKill(ctx, resp.ID, "SIGKILL"); killErr != nil {
			if strings.Contains(strings.ToLower(killErr.Error()), "is not running") {
				inspect, inspectErr := docker.DockerClient.ContainerInspect(ctx, resp.ID)
				if inspectErr == nil && inspect.ContainerJSONBase != nil && inspect.ContainerJSONBase.State != nil {
					return finalizeCompileResult(ctx, resp.ID, int64(inspect.ContainerJSONBase.State.ExitCode))
				}
			}
			return false, "", fmt.Errorf("kill compile container failed after timeout: %w", killErr)
		}

		logOutput, logErr := readCompileContainerOutput(ctx, resp.ID)
		if logErr != nil {
			return false, "", fmt.Errorf("read compile container logs after timeout failed: %w", logErr)
		}

		timeoutMessage := fmt.Sprintf("compile timeout exceeded after %s", compileTimeout)
		if logOutput != "" {
			timeoutMessage += "\n" + logOutput
		}
		return false, timeoutMessage, nil
	}

	return true, "", nil
}

func finalizeCompileResult(ctx context.Context, containerID string, statusCode int64) (bool, string, error) {
	logOutput, err := readCompileContainerOutput(ctx, containerID)
	if err != nil {
		return false, "", fmt.Errorf("read compile container logs failed: %w", err)
	}

	if statusCode != 0 {
		if statusCode == 11 {
			return false, logOutput, nil
		}
		if logOutput == "" {
			logOutput = fmt.Sprintf("compile container exited with code=%d", statusCode)
		}
		return false, "", fmt.Errorf("build judge helper failed: %s", logOutput)
	}

	return true, "", nil
}

func StartPersistentSandbox(ctx context.Context, workDir string, memoryLimitMB int64) (string, error) {
	if memoryLimitMB <= 0 {
		memoryLimitMB = 256
	}

	memoryLimitBytes := memoryLimitMB * 1024 * 1024

	resp, err := docker.DockerClient.ContainerCreate(ctx, &container.Config{
		Image:      judgeRuntimeImage,
		Cmd:        []string{"sleep", "3600"},
		WorkingDir: "/app",
	}, &container.HostConfig{
		NetworkMode: "none",
		Binds:       []string{workDir + ":/app"},
		Resources: container.Resources{
			Memory:     memoryLimitBytes,
			MemorySwap: memoryLimitBytes,
			NanoCPUs:   1 * 1e9,
			PidsLimit:  &[]int64{64}[0],
		},
	}, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("create sandbox failed: %w", err)
	}

	if err := docker.DockerClient.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start sandbox failed: %w", err)
	}

	return resp.ID, nil
}

func ExecTestCase(ctx context.Context, containerID string, input string, cpuLimitMS int, wallLimitMS int, memoryLimitMB int) model.JudgeResult {
	if cpuLimitMS <= 0 {
		cpuLimitMS = 1000
	}
	if wallLimitMS <= 0 {
		wallLimitMS = cpuLimitMS * 2
	}
	if memoryLimitMB <= 0 {
		memoryLimitMB = 256
	}

	memoryLimitKB := memoryLimitMB * 1024

	execCreate, err := docker.DockerClient.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		Cmd: []string{
			"/app/.judge/runner",
			"-bin", "/app/solution",
			"-cpu-limit-ms", strconv.Itoa(cpuLimitMS),
			"-wall-limit-ms", strconv.Itoa(wallLimitMS),
			"-memory-limit-kb", strconv.Itoa(memoryLimitKB),
		},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return model.JudgeResult{Status: model.StatusSystemError, Error: fmt.Errorf("create exec failed: %w", err)}
	}

	hijackedResp, err := docker.DockerClient.ContainerExecAttach(ctx, execCreate.ID, container.ExecStartOptions{})
	if err != nil {
		return model.JudgeResult{Status: model.StatusSystemError, Error: fmt.Errorf("attach exec failed: %w", err)}
	}
	defer hijackedResp.Close()

	_, _ = hijackedResp.Conn.Write([]byte(input))
	_ = hijackedResp.CloseWrite()

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, hijackedResp.Reader)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return model.JudgeResult{
				Status:       model.StatusTimeLimitExceeded,
				Output:       "judge runner deadline exceeded",
				TimeCost:     cpuLimitMS,
				WallTimeCost: wallLimitMS,
				MemoryCost:   memoryLimitKB,
			}
		}
		return model.JudgeResult{Status: model.StatusSystemError, Error: fmt.Errorf("read exec output failed: %w", err)}
	}

	var runnerResult runnerExecResult
	if err := json.Unmarshal(bytes.TrimSpace(stdoutBuf.Bytes()), &runnerResult); err != nil {
		return model.JudgeResult{
			Status: model.StatusSystemError,
			Error: fmt.Errorf(
				"decode judge runner result failed: %w, stdout=%q stderr=%q",
				err,
				stdoutBuf.String(),
				stderrBuf.String(),
			),
		}
	}

	if runnerResult.Error != "" {
		return model.JudgeResult{
			Status: model.StatusSystemError,
			Error:  fmt.Errorf("judge runner failed: %s", runnerResult.Error),
		}
	}

	result := model.JudgeResult{
		Status:       model.StatusAccepted,
		Output:       runnerResult.Stdout,
		TimeCost:     int(runnerResult.CPUTimeMS),
		WallTimeCost: int(runnerResult.WallTimeMS),
		MemoryCost:   int(runnerResult.MaxRSSKB),
		ExitCode:     runnerResult.ExitCode,
	}

	if runnerResult.WallTimedOut || runnerResult.CPUTimeMS > int64(cpuLimitMS) || runnerResult.Signal == linuxSIGXCPU {
		result.Status = model.StatusTimeLimitExceeded
		result.Output = fmt.Sprintf("time limit exceeded (cpu=%dms wall=%dms)", runnerResult.CPUTimeMS, runnerResult.WallTimeMS)
		return result
	}

	if isMemoryLimitExceeded(runnerResult, memoryLimitKB) {
		result.Status = model.StatusMemoryLimitExceeded
		result.Output = fmt.Sprintf("memory limit exceeded (peak=%dKB limit=%dKB)", runnerResult.MaxRSSKB, memoryLimitKB)
		return result
	}

	if runnerResult.ExitCode != 0 || runnerResult.Signal != 0 {
		runtimeOutput := strings.TrimSpace(runnerResult.Stderr)
		if runtimeOutput == "" {
			runtimeOutput = strings.TrimSpace(runnerResult.Stdout)
		}
		if runtimeOutput == "" {
			runtimeOutput = fmt.Sprintf("process exited with code=%d signal=%d", runnerResult.ExitCode, runnerResult.Signal)
		}
		result.Status = model.StatusRuntimeError
		result.Output = runtimeOutput
		return result
	}

	return result
}

func RemoveSandbox(ctx context.Context, containerID string) {
	if containerID == "" {
		return
	}
	if err := docker.DockerClient.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		fmt.Printf("warning: failed to remove sandbox %s: %v\n", containerID, err)
	}
}

func writeJudgeRunner(workDir string) error {
	judgeDir := filepath.Join(workDir, ".judge")
	if err := os.MkdirAll(judgeDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(judgeDir, "runner.go"), []byte(judgeRunnerSource), 0644)
}

func isMemoryLimitExceeded(result runnerExecResult, memoryLimitKB int) bool {
	if memoryLimitKB <= 0 {
		return false
	}
	if result.MaxRSSKB >= int64(memoryLimitKB) {
		return true
	}

	stderrLower := strings.ToLower(result.Stderr)
	for _, pattern := range []string{
		"out of memory",
		"cannot allocate memory",
		"runtime: failed to create new os thread",
	} {
		if strings.Contains(stderrLower, pattern) {
			return true
		}
	}

	return false
}

func getCompileTimeout() time.Duration {
	seconds := config.GlobalConfig.Judge.CompileTimeoutSeconds
	if seconds <= 0 {
		return defaultCompileTimeout
	}
	return time.Duration(seconds) * time.Second
}

func readCompileContainerOutput(ctx context.Context, containerID string) (string, error) {
	out, err := docker.DockerClient.ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", err
	}
	defer out.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	_, _ = stdcopy.StdCopy(&stdoutBuf, &stderrBuf, out)

	stderrText := strings.TrimSpace(stderrBuf.String())
	stdoutText := strings.TrimSpace(stdoutBuf.String())

	switch {
	case stderrText != "" && stdoutText != "":
		return stderrText + "\n" + stdoutText, nil
	case stderrText != "":
		return stderrText, nil
	default:
		return stdoutText, nil
	}
}
