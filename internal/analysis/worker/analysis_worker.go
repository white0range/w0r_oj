package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"gojo/infrastructure/cache"
	analysisDto "gojo/internal/analysis/dto"
	"gojo/internal/analysis/model"
	"gojo/internal/analysis/repository"
	problemModel "gojo/internal/problem/model"
	submissionRepo "gojo/internal/submission/repository"
	aiPkg "gojo/pkg/ai"
	"log"
	"strconv"
	"time"
)

type AIProvider interface {
	AskAIWithContext(ctx context.Context, analysisCtx aiPkg.AnalysisContext) (string, error)
}

type ProblemProvider interface {
	GetProblemByID(ctx context.Context, id string) (*problemModel.Problem, error)
}

// AnalysisWorker 负责在后台消费 Redis 队列里的分析任务。
type AnalysisWorker struct {
	repo           repository.AnalysisRepository
	submissionRepo submissionRepo.SubmissionRepository
	problemRepo    ProblemProvider
	aiProvider     AIProvider
}

// NewAnalysisWorker 创建一个新的分析任务 worker。
func NewAnalysisWorker(
	repo repository.AnalysisRepository,
	subRepo submissionRepo.SubmissionRepository,
	problemRepo ProblemProvider,
	aiProvider AIProvider,
) *AnalysisWorker {
	return &AnalysisWorker{
		repo:           repo,
		submissionRepo: subRepo,
		problemRepo:    problemRepo,
		aiProvider:     aiProvider,
	}
}

// ProcessTask 负责处理一条 AI 诊断任务。
// 流程是：查任务 -> 改 running -> 查 submission -> 查 problem 上下文 -> 调 AI -> 写回结果。
func (w *AnalysisWorker) ProcessTask(ctx context.Context, taskID uint) error {
	startedAt := time.Now()

	task, err := w.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get analysis task failed: %w", err)
	}

	if err := w.repo.UpdateTaskStatus(ctx, task.ID, model.TaskStatusRunning); err != nil {
		return fmt.Errorf("update task status to running failed: %w", err)
	}

	// analysis_task 里只存 submission_id，真正给 AI 的代码内容还得回 submission 表里查。
	submissionIDStr := strconv.FormatUint(uint64(task.SubmissionID), 10)
	submission, err := w.submissionRepo.GetSubmissionByID(ctx, submissionIDStr)
	if err != nil {
		_ = w.repo.UpdateTaskFailed(ctx, task.ID, "submission not found", time.Now())
		return fmt.Errorf("get submission failed: %w", err)
	}

	// 轻量 RAG：根据 submission 反查题目信息，把标题、描述、标签一起喂给模型。
	problemIDStr := strconv.FormatUint(uint64(submission.ProblemID), 10)
	problem, err := w.problemRepo.GetProblemByID(ctx, problemIDStr)
	if err != nil {
		_ = w.repo.UpdateTaskFailed(ctx, task.ID, "problem not found", time.Now())
		return fmt.Errorf("get problem failed: %w", err)
	}

	var tagNames []string
	for _, tag := range problem.Tags {
		tagNames = append(tagNames, tag.Name)
	}

	analysisCtx := aiPkg.AnalysisContext{
		Code:               submission.Code,
		Language:           submission.Language,
		ActualOutput:       submission.ActualOutput,
		ProblemTitle:       problem.Title,
		ProblemDescription: problem.Description,
		ProblemTags:        tagNames,
	}

	result, err := w.aiProvider.AskAIWithContext(ctx, analysisCtx)
	if err != nil {
		_ = w.repo.UpdateTaskFailed(ctx, task.ID, err.Error(), time.Now())
		return fmt.Errorf("ask ai failed: %w", err)
	}

	if err := w.repo.UpdateTaskResult(ctx, task.ID, result, time.Now()); err != nil {
		_ = w.repo.UpdateTaskFailed(ctx, task.ID, err.Error(), time.Now())
		return fmt.Errorf("update task result failed: %w", err)
	}

	log.Printf(
		"analysis task finished: task_id=%d submission_id=%d user_id=%d duration_ms=%d status=%s\n",
		task.ID,
		task.SubmissionID,
		task.UserID,
		time.Since(startedAt).Milliseconds(),
		model.TaskStatusSucceeded,
	)

	return nil
}

// StartWorkerPool 启动多个 analysis worker。
func (w *AnalysisWorker) StartWorkerPool(workerCount int) {
	fmt.Printf("starting analysis worker pool, workers=%d\n", workerCount)
	for i := 1; i <= workerCount; i++ {
		go w.run(i)
	}
}

// run 是单个 worker 的循环。
// 它会一直阻塞等待 Redis 队列里的 analysis 任务。
func (w *AnalysisWorker) run(id int) {
	ctx := context.Background()

	for {
		result, err := cache.Rdb.BRPop(ctx, 0, "analysis_queue").Result()
		if err != nil {
			log.Printf("analysis worker %d pop task failed: %v\n", id, err)
			continue
		}

		var task analysisDto.AnalysisQueueTask
		if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
			log.Printf("analysis worker %d unmarshal task failed: %v\n", id, err)
			continue
		}

		log.Printf("analysis worker %d processing task_id=%d submission_id=%d\n", id, task.TaskID, task.SubmissionID)

		if err := w.ProcessTask(ctx, task.TaskID); err != nil {
			log.Printf("analysis worker %d process task failed: %v\n", id, err)
		}
	}
}
