package repository

import (
	"context"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/internal/analysis/dto"
	"gojo/internal/analysis/model"
	"time"

	"gorm.io/gorm/clause"
)

type AnalysisRepository interface {
	CreateTask(ctx context.Context, task *model.AnalysisTask) error
	GetTaskByID(ctx context.Context, taskID uint) (*model.AnalysisTask, error)
	UpdateTaskStatus(ctx context.Context, taskID uint, status string) error
	UpdateTaskResult(ctx context.Context, taskID uint, result string, finishedAt time.Time) error
	UpdateTaskFailed(ctx context.Context, taskID uint, errorMessage string, finishedAt time.Time) error
	PushToAnalysisQueue(ctx context.Context, taskBytes []byte) error
	UpsertFeedback(ctx context.Context, feedback *model.AnalysisFeedback) error
	GetFeedbackByTaskIDAndUserID(ctx context.Context, taskID uint, userID uint) (*model.AnalysisFeedback, error)
	GetAdminStats(ctx context.Context) (*dto.AdminStatsResponse, error)
}

type AnalysisRepoMysql struct{}

func NewAnalysisRepository() AnalysisRepository {
	return &AnalysisRepoMysql{}
}

// CreateTask 负责把一条新的 AI 诊断任务写进数据库
func (r *AnalysisRepoMysql) CreateTask(ctx context.Context, task *model.AnalysisTask) error {
	return mysql.DB.WithContext(ctx).Create(task).Error
}

// GetTaskByID 负责按任务 id 查询任务
func (r *AnalysisRepoMysql) GetTaskByID(ctx context.Context, taskID uint) (*model.AnalysisTask, error) {
	var task model.AnalysisTask

	err := mysql.DB.WithContext(ctx).First(&task, taskID).Error
	if err != nil {
		return nil, err
	}

	return &task, nil
}

// UpdateTaskStatus 只更新任务状态。
// 后面 worker 开始执行任务时会用到。
func (r *AnalysisRepoMysql) UpdateTaskStatus(ctx context.Context, taskID uint, status string) error {
	return mysql.DB.WithContext(ctx).
		Model(&model.AnalysisTask{}).
		Where("id = ?", taskID).
		Update("status", status).Error
}

// UpdateTaskResult 在任务成功完成后，保存结果、完成时间，并把状态改成 succeeded。
func (r *AnalysisRepoMysql) UpdateTaskResult(ctx context.Context, taskID uint, result string, finishedAt time.Time) error {
	return mysql.DB.WithContext(ctx).
		Model(&model.AnalysisTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":      model.TaskStatusSucceeded,
			"result":      result,
			"finished_at": finishedAt,
		}).Error
}

// UpdateTaskFailed 在任务失败后，保存错误信息、完成时间，并把状态改成 failed。
func (r *AnalysisRepoMysql) UpdateTaskFailed(ctx context.Context, taskID uint, errorMessage string, finishedAt time.Time) error {
	return mysql.DB.WithContext(ctx).
		Model(&model.AnalysisTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":        model.TaskStatusFailed,
			"error_message": errorMessage,
			"finished_at":   finishedAt,
		}).Error
}

// PushToAnalysisQueue 把 analysis 任务推进 Redis 队列。
// 后台 worker 会从 analysis_queue 里取任务并处理。
func (r *AnalysisRepoMysql) PushToAnalysisQueue(ctx context.Context, taskBytes []byte) error {
	return cache.Rdb.LPush(ctx, "analysis_queue", taskBytes).Err()
}

// UpsertFeedback 保存用户反馈。
// 如果同一个用户已经给这条任务提交过反馈，就直接更新原记录。
func (r *AnalysisRepoMysql) UpsertFeedback(ctx context.Context, feedback *model.AnalysisFeedback) error {
	return mysql.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "task_id"},
				{Name: "user_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{"helpful", "comment", "updated_at"}),
		}).
		Create(feedback).Error
}

// GetFeedbackByTaskIDAndUserID 查询当前用户对某条任务的反馈。
func (r *AnalysisRepoMysql) GetFeedbackByTaskIDAndUserID(ctx context.Context, taskID uint, userID uint) (*model.AnalysisFeedback, error) {
	var feedback model.AnalysisFeedback

	err := mysql.DB.WithContext(ctx).
		Where("task_id = ? AND user_id = ?", taskID, userID).
		First(&feedback).Error
	if err != nil {
		return nil, err
	}

	return &feedback, nil
}

// GetAdminStats 统计 analysis 任务和反馈的基础概况。
func (r *AnalysisRepoMysql) GetAdminStats(ctx context.Context) (*dto.AdminStatsResponse, error) {
	stats := &dto.AdminStatsResponse{}

	// 任务维度统计
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisTask{}).Count(&stats.TotalTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisTask{}).Where("status = ?", model.TaskStatusPending).Count(&stats.PendingTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisTask{}).Where("status = ?", model.TaskStatusRunning).Count(&stats.RunningTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisTask{}).Where("status = ?", model.TaskStatusSucceeded).Count(&stats.SucceededTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisTask{}).Where("status = ?", model.TaskStatusFailed).Count(&stats.FailedTasks).Error; err != nil {
		return nil, err
	}

	// 反馈维度统计
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisFeedback{}).Count(&stats.TotalFeedbacks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisFeedback{}).Where("helpful = ?", true).Count(&stats.HelpfulCount).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.AnalysisFeedback{}).Where("helpful = ?", false).Count(&stats.UnhelpfulCount).Error; err != nil {
		return nil, err
	}

	return stats, nil
}
