package service

import (
	"context"
	"encoding/json"
	"log"
	"gojo/internal/analysis/dto"
	"gojo/internal/analysis/model"
	"gojo/internal/analysis/repository"
	"gojo/internal/app/apperror"
	submissionRepo "gojo/internal/submission/repository"
	"strconv"
)

type AnalysisService struct {
	repo           repository.AnalysisRepository
	submissionRepo submissionRepo.SubmissionRepository
}

func NewAnalysisService(r repository.AnalysisRepository, sr submissionRepo.SubmissionRepository) *AnalysisService {
	return &AnalysisService{repo: r, submissionRepo: sr}
}

// CreateAnalysisTask 创建一条新的 AI 诊断任务。
func (s *AnalysisService) CreateAnalysisTask(ctx context.Context, userID uint, submissionID uint) (*model.AnalysisTask, error) {
	// submission repo 现在的 GetSubmissionByID 收的是 string，
	// 所以这里先把 uint 转成 string
	submissionIDStr := strconv.FormatUint(uint64(submissionID), 10)

	// 1. 先确认 submission 是否存在
	submission, err := s.submissionRepo.GetSubmissionByID(ctx, submissionIDStr)
	if err != nil {
		return nil, apperror.ErrSubmissionNotFound
	}

	// 2. 再确认这条 submission 是不是当前用户自己的
	if submission.UserID != userID {
		return nil, apperror.ErrForbidden
	}

	// 3. 校验通过后，再真正创建分析任务
	task := &model.AnalysisTask{
		UserID:       userID,
		SubmissionID: submissionID,
		Status:       model.TaskStatusPending,

		// 第一版先写死模型名，后面再改成读配置
		Model: "deepseek-chat",
	}

	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	queueTask := dto.AnalysisQueueTask{
		TaskID:       task.ID,
		UserID:       task.UserID,
		SubmissionID: task.SubmissionID,
	}

	taskBytes, err := json.Marshal(queueTask)
	if err != nil {
		return nil, err
	}

	if err := s.repo.PushToAnalysisQueue(ctx, taskBytes); err != nil {
		return nil, err
	}

	log.Printf("analysis task queued: task_id=%d submission_id=%d user_id=%d\n", task.ID, task.SubmissionID, task.UserID)

	return task, nil
}

// GetAnalysisTask 按任务 id 查询任务。

func (s *AnalysisService) GetAnalysisTask(ctx context.Context, userID uint, taskID uint) (*model.AnalysisTask, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// 只允许查询自己的分析任务
	if task.UserID != userID {
		return nil, apperror.ErrForbidden
	}

	return task, nil
}

// SubmitFeedback 保存用户对 AI 分析结果的评价。
// 第一版先要求：只能给自己的任务提交反馈。
func (s *AnalysisService) SubmitFeedback(ctx context.Context, userID uint, taskID uint, helpful bool, comment string) (*model.AnalysisFeedback, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if task.UserID != userID {
		return nil, apperror.ErrForbidden
	}

	feedback := &model.AnalysisFeedback{
		TaskID:   taskID,
		UserID:   userID,
		Helpful:  helpful,
		Comment:  comment,
	}

	if err := s.repo.UpsertFeedback(ctx, feedback); err != nil {
		return nil, err
	}

	return feedback, nil
}

// GetFeedback 查询当前用户对某条分析任务的反馈。
func (s *AnalysisService) GetFeedback(ctx context.Context, userID uint, taskID uint) (*model.AnalysisFeedback, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if task.UserID != userID {
		return nil, apperror.ErrForbidden
	}

	feedback, err := s.repo.GetFeedbackByTaskIDAndUserID(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}

	return feedback, nil
}

// GetAdminStats 返回 analysis 模块的基础统计信息。
func (s *AnalysisService) GetAdminStats(ctx context.Context) (*dto.AdminStatsResponse, error) {
	return s.repo.GetAdminStats(ctx)
}
