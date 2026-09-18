package sandbox

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
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
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/pkg/stdcopy"
)

//go:embed runner/runner_source.txt
var judgeRunnerSource string

type runnerExecResult struct {
	ExitCode       int    `json:"exit_code"`
	Signal         int    `json:"signal"`
	WallTimeMS     int64  `json:"wall_time_ms"`
	CPUTimeMS      int64  `json:"cpu_time_ms"`
	MaxRSSKB       int64  `json:"max_rss_kb"`
	WallTimedOut   bool   `json:"wall_timed_out"`
	OutputExceeded bool   `json:"output_exceeded"`
	Stdout         string `json:"stdout"`
	Stderr         string `json:"stderr"`
	Error          string `json:"error"`
}

const (
	linuxSIGXCPU          = 24
	defaultCompileTimeout = 45 * time.Second
	judgeRuntimeImage     = "golang:alpine"
	judgeContainerUser    = "65532:65532"
	judgeWorkDirPrefix    = "judge_"

	sandboxLabelKey           = "gojo.sandbox"
	sandboxTypeLabelKey       = "gojo.sandbox.type"
	sandboxSubmissionLabelKey = "gojo.submission_id"
	sandboxOwnerLabelKey      = "gojo.sandbox.owner"

	compileMemoryLimitBytes = 512 * 1024 * 1024
	compileNanoCPUs         = 1 * 1e9
	compilePidsLimit        = 128
	runtimePidsLimit        = 64
	maxProgramOutputBytes   = 1 << 20
	maxCompileOutputBytes   = 1 << 20
	maxRunnerResponseBytes  = 2 << 20
)

type OrphanCleanupSummary struct {
	ContainersRemoved int
	WorkDirsRemoved   int
}

func CompileCode(ctx context.Context, code string, workDir string, submissionID uint) (bool, string, error) {
	if err := prepareWorkDir(workDir); err != nil {
		return false, "", fmt.Errorf("prepare judge workdir permissions failed: %w", err)
	}

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
			User:       judgeContainerUser,
			WorkingDir: "/app",
			Labels:     sandboxLabels("compile", submissionID),
			Env: []string{
				"HOME=/tmp",
				"GOCACHE=/tmp/go-build",
				"GOTMPDIR=/tmp",
				"CGO_ENABLED=0",
			},
			Cmd: []string{
				"sh",
				"-c",
				"ulimit -f 65536; if ! GO111MODULE=off go build -o solution main.go; then exit 11; fi; if ! GO111MODULE=off go build -o .judge/runner .judge/runner.go; then exit 12; fi",
			},
		},
		&container.HostConfig{
			NetworkMode:    "none",
			Binds:          []string{workDir + ":/app:rw"},
			ReadonlyRootfs: true,
			CapDrop:        []string{"ALL"},
			SecurityOpt:    []string{"no-new-privileges:true"},
			Tmpfs:          map[string]string{"/tmp": "rw,nosuid,nodev,size=128m"},
			Resources: container.Resources{
				Memory:     compileMemoryLimitBytes,
				MemorySwap: compileMemoryLimitBytes,
				NanoCPUs:   compileNanoCPUs,
				PidsLimit:  int64Ptr(compilePidsLimit),
			},
		}, nil, nil, "")
	if err != nil {
		return false, "", err
	}
	defer RemoveSandbox(context.Background(), resp.ID)

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

func StartPersistentSandbox(ctx context.Context, workDir string, memoryLimitMB int64, submissionID uint) (string, error) {
	if memoryLimitMB <= 0 {
		memoryLimitMB = 256
	}

	memoryLimitBytes := memoryLimitMB * 1024 * 1024

	resp, err := docker.DockerClient.ContainerCreate(ctx, &container.Config{
		Image:      judgeRuntimeImage,
		User:       judgeContainerUser,
		Cmd:        []string{"sleep", "3600"},
		WorkingDir: "/app",
		Labels:     sandboxLabels("runtime", submissionID),
	}, &container.HostConfig{
		NetworkMode:    "none",
		Binds:          []string{workDir + ":/app:ro"},
		ReadonlyRootfs: true,
		CapDrop:        []string{"ALL"},
		SecurityOpt:    []string{"no-new-privileges:true"},
		Tmpfs:          map[string]string{"/tmp": "rw,nosuid,nodev,noexec,size=16m"},
		Resources: container.Resources{
			Memory:     memoryLimitBytes,
			MemorySwap: memoryLimitBytes,
			NanoCPUs:   compileNanoCPUs,
			PidsLimit:  int64Ptr(runtimePidsLimit),
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

	stdoutBuf := newCappedBuffer(maxRunnerResponseBytes)
	stderrBuf := newCappedBuffer(maxRunnerResponseBytes)
	_, err = stdcopy.StdCopy(stdoutBuf, stderrBuf, hijackedResp.Reader)
	if err != nil {
		if errors.Is(err, errOutputTooLarge) {
			return model.JudgeResult{Status: model.StatusSystemError, Error: fmt.Errorf("judge runner response exceeded %d bytes", maxRunnerResponseBytes)}
		}
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
	if runnerResult.OutputExceeded {
		return model.JudgeResult{
			Status:       model.StatusOutputLimitExceeded,
			Output:       fmt.Sprintf("output limit exceeded (%d bytes)", maxProgramOutputBytes),
			TimeCost:     int(runnerResult.CPUTimeMS),
			WallTimeCost: int(runnerResult.WallTimeMS),
			MemoryCost:   int(runnerResult.MaxRSSKB),
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

func RemoveSandbox(_ context.Context, containerID string) {
	if containerID == "" {
		return
	}
	// A task context may already be cancelled during forced shutdown. Cleanup
	// must use its own short-lived context so the container is not left behind.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := docker.DockerClient.ContainerRemove(cleanupCtx, containerID, container.RemoveOptions{Force: true}); err != nil {
		fmt.Printf("warning: failed to remove sandbox %s: %v\n", containerID, err)
	}
}

// CleanupOrphanedResources removes judge containers and work directories left
// behind when a previous server process did not complete its deferred cleanup.
// It must run before the judge worker starts consuming new tasks.
func CleanupOrphanedResources(ctx context.Context) (OrphanCleanupSummary, error) {
	var summary OrphanCleanupSummary
	if docker.DockerClient == nil {
		return summary, errors.New("docker client is not initialized")
	}

	containers, err := docker.DockerClient.ContainerList(ctx, container.ListOptions{
		All: true,
		Filters: filters.NewArgs(
			filters.Arg("label", sandboxLabelKey+"=true"),
			filters.Arg("label", sandboxOwnerLabelKey+"="+sandboxOwner()),
		),
	})
	if err != nil {
		return summary, fmt.Errorf("list orphaned judge containers: %w", err)
	}

	var removeErrors []error
	for _, item := range containers {
		if err := docker.DockerClient.ContainerRemove(ctx, item.ID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			removeErrors = append(removeErrors, fmt.Errorf("remove judge container %s: %w", item.ID, err))
			continue
		}
		summary.ContainersRemoved++
	}
	if len(removeErrors) > 0 {
		// Do not delete host work directories while a labeled container may still
		// have one mounted. The next startup can safely retry the whole cleanup.
		return summary, errors.Join(removeErrors...)
	}

	summary.WorkDirsRemoved, err = cleanupOrphanedWorkDirs(os.TempDir())
	if err != nil {
		return summary, fmt.Errorf("remove orphaned judge work directories: %w", err)
	}
	return summary, nil
}

func sandboxLabels(sandboxType string, submissionID uint) map[string]string {
	return map[string]string{
		sandboxLabelKey:           "true",
		sandboxTypeLabelKey:       sandboxType,
		sandboxSubmissionLabelKey: strconv.FormatUint(uint64(submissionID), 10),
		sandboxOwnerLabelKey:      sandboxOwner(),
	}
}

func sandboxOwner() string {
	owner := strings.TrimSpace(config.GlobalConfig.App.Env)
	if owner == "" {
		return "unknown"
	}
	return owner
}

func cleanupOrphanedWorkDirs(root string) (int, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), judgeWorkDirPrefix) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func writeJudgeRunner(workDir string) error {
	judgeDir := filepath.Join(workDir, ".judge")
	if err := os.MkdirAll(judgeDir, 0777); err != nil {
		return err
	}
	if err := os.Chmod(judgeDir, 0777); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(judgeDir, "runner.go"), []byte(judgeRunnerSource), 0644)
}

func prepareWorkDir(workDir string) error {
	// The compiler runs as an unprivileged UID. This directory is unique to one
	// submission and is mounted read-only during execution.
	return os.Chmod(workDir, 0777)
}

func int64Ptr(value int64) *int64 {
	return &value
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

	stdoutBuf := newCappedBuffer(maxCompileOutputBytes)
	stderrBuf := newCappedBuffer(maxCompileOutputBytes)
	_, copyErr := stdcopy.StdCopy(stdoutBuf, stderrBuf, out)
	truncated := errors.Is(copyErr, errOutputTooLarge)
	if copyErr != nil && !truncated {
		return "", copyErr
	}

	stderrText := strings.TrimSpace(stderrBuf.String())
	stdoutText := strings.TrimSpace(stdoutBuf.String())

	var output string
	switch {
	case stderrText != "" && stdoutText != "":
		output = stderrText + "\n" + stdoutText
	case stderrText != "":
		output = stderrText
	default:
		output = stdoutText
	}
	if truncated {
		output += fmt.Sprintf("\n[compile output truncated at %d bytes]", maxCompileOutputBytes)
	}
	return output, nil
}

var errOutputTooLarge = errors.New("output exceeds limit")

type cappedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func newCappedBuffer(limit int) *cappedBuffer {
	return &cappedBuffer{limit: limit}
}

func (b *cappedBuffer) Write(data []byte) (int, error) {
	remaining := b.limit - b.buffer.Len()
	if remaining <= 0 {
		return len(data), errOutputTooLarge
	}
	if len(data) > remaining {
		_, _ = b.buffer.Write(data[:remaining])
		return len(data), errOutputTooLarge
	}
	return b.buffer.Write(data)
}

func (b *cappedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *cappedBuffer) String() string { return b.buffer.String() }
