package repository

import (
	"context"
	"time"

	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/internal/study_plan/dto"
	"gojo/internal/study_plan/model"

	"gorm.io/gorm/clause"
)

type StudyPlanRepository interface {
	CreateTask(ctx context.Context, task *model.StudyPlanTask) error
	GetTaskByID(ctx context.Context, taskID uint) (*model.StudyPlanTask, error)
	UpdateTaskStatus(ctx context.Context, taskID uint, status string) error
	UpdateTaskResult(ctx context.Context, taskID uint, result string, finishedAt time.Time) error
	UpdateTaskFailed(ctx context.Context, taskID uint, errorMessage string, finishedAt time.Time) error
	PushToQueue(ctx context.Context, taskBytes []byte) error
	UpsertFeedback(ctx context.Context, feedback *model.StudyPlanFeedback) error
	GetFeedbackByTaskIDAndUserID(ctx context.Context, taskID uint, userID uint) (*model.StudyPlanFeedback, error)
	GetAdminStats(ctx context.Context) (*dto.StudyPlanAdminStatsResponse, error)
}

type studyPlanRepoMysql struct{}

func NewStudyPlanRepository() StudyPlanRepository {
	return &studyPlanRepoMysql{}
}

// CreateTask 把训练计划任务写入 MySQL。
func (r *studyPlanRepoMysql) CreateTask(ctx context.Context, task *model.StudyPlanTask) error {
	return mysql.DB.WithContext(ctx).Create(task).Error
}

// GetTaskByID 按主键读取一条训练计划任务。
func (r *studyPlanRepoMysql) GetTaskByID(ctx context.Context, taskID uint) (*model.StudyPlanTask, error) {
	var task model.StudyPlanTask

	if err := mysql.DB.WithContext(ctx).First(&task, taskID).Error; err != nil {
		return nil, err
	}

	return &task, nil
}

// UpdateTaskStatus 只更新任务状态字段。
func (r *studyPlanRepoMysql) UpdateTaskStatus(ctx context.Context, taskID uint, status string) error {
	return mysql.DB.WithContext(ctx).
		Model(&model.StudyPlanTask{}).
		Where("id = ?", taskID).
		Update("status", status).Error
}

// UpdateTaskResult 保存最终结果，并把任务标记为 succeeded。
func (r *studyPlanRepoMysql) UpdateTaskResult(ctx context.Context, taskID uint, result string, finishedAt time.Time) error {
	return mysql.DB.WithContext(ctx).
		Model(&model.StudyPlanTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":      model.TaskStatusSucceeded,
			"result":      result,
			"finished_at": finishedAt,
		}).Error
}

// UpdateTaskFailed 保存错误信息，并把任务标记为 failed。
func (r *studyPlanRepoMysql) UpdateTaskFailed(ctx context.Context, taskID uint, errorMessage string, finishedAt time.Time) error {
	return mysql.DB.WithContext(ctx).
		Model(&model.StudyPlanTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":        model.TaskStatusFailed,
			"error_message": errorMessage,
			"finished_at":   finishedAt,
		}).Error
}

// PushToQueue 把任务消息推进 Redis，交给后台 worker 异步处理。
func (r *studyPlanRepoMysql) PushToQueue(ctx context.Context, taskBytes []byte) error {
	return cache.Rdb.LPush(ctx, "study_plan_queue", taskBytes).Err()
}

func (r *studyPlanRepoMysql) UpsertFeedback(ctx context.Context, feedback *model.StudyPlanFeedback) error {
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

func (r *studyPlanRepoMysql) GetFeedbackByTaskIDAndUserID(ctx context.Context, taskID uint, userID uint) (*model.StudyPlanFeedback, error) {
	var feedback model.StudyPlanFeedback

	if err := mysql.DB.WithContext(ctx).
		Where("task_id = ? AND user_id = ?", taskID, userID).
		First(&feedback).Error; err != nil {
		return nil, err
	}

	return &feedback, nil
}

func (r *studyPlanRepoMysql) GetAdminStats(ctx context.Context) (*dto.StudyPlanAdminStatsResponse, error) {
	stats := &dto.StudyPlanAdminStatsResponse{}

	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanTask{}).Count(&stats.TotalTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanTask{}).Where("status = ?", model.TaskStatusPending).Count(&stats.PendingTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanTask{}).Where("status = ?", model.TaskStatusRunning).Count(&stats.RunningTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanTask{}).Where("status = ?", model.TaskStatusSucceeded).Count(&stats.SucceededTasks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanTask{}).Where("status = ?", model.TaskStatusFailed).Count(&stats.FailedTasks).Error; err != nil {
		return nil, err
	}

	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanFeedback{}).Count(&stats.TotalFeedbacks).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanFeedback{}).Where("helpful = ?", true).Count(&stats.HelpfulCount).Error; err != nil {
		return nil, err
	}
	if err := mysql.DB.WithContext(ctx).Model(&model.StudyPlanFeedback{}).Where("helpful = ?", false).Count(&stats.UnhelpfulCount).Error; err != nil {
		return nil, err
	}

	return stats, nil
}
