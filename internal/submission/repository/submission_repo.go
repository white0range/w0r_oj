package repository

import (
	"context"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/internal/submission/model"
	"gojo/pkg/pagination"
)

// 1. 定义接口
type SubmissionRepository interface {
	CreateSubmission(ctx context.Context, sub *model.Submission) error
	PushToJudgeQueue(ctx context.Context, taskBytes []byte) error
	GetSubmissionByID(ctx context.Context, id string) (*model.Submission, error)
	GetSubmissionsByUserID(ctx context.Context, userID uint, page, limit int) (int64, []model.Submission, error)
	UpdateSubmissionStatus(ctx context.Context, id uint, status string) error

	// 🚨 新增：专门获取用户 AC 过的所有题目 ID
	GetACProblemIDsByUserID(ctx context.Context, userID uint) ([]uint, error)
}

type submissionRepoMysql struct{}

func NewSubmissionRepository() SubmissionRepository {
	return &submissionRepoMysql{}
}

// 2. 落地实现
func (r *submissionRepoMysql) CreateSubmission(ctx context.Context, sub *model.Submission) error {
	return mysql.DB.WithContext(ctx).Create(sub).Error
}

func (r *submissionRepoMysql) UpdateSubmissionStatus(ctx context.Context, id uint, status string) error {
	return mysql.DB.WithContext(ctx).Model(&model.Submission{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": status,
	}).Error
}

func (r *submissionRepoMysql) PushToJudgeQueue(ctx context.Context, taskBytes []byte) error {
	// 把 Redis 队列推送也封进仓管里！
	return cache.Rdb.LPush(ctx, "judge_queue", taskBytes).Err()
}

func (r *submissionRepoMysql) GetSubmissionByID(ctx context.Context, id string) (*model.Submission, error) {
	var sub model.Submission
	err := mysql.DB.WithContext(ctx).First(&sub, id).Error
	return &sub, err
}

func (r *submissionRepoMysql) GetSubmissionsByUserID(ctx context.Context, userID uint, page, limit int) (int64, []model.Submission, error) {
	var total int64
	var items []model.Submission

	query := mysql.DB.WithContext(ctx).Model(&model.Submission{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	err := query.Scopes(pagination.Paginate(page, limit)).
		Order("created_at desc").
		Omit("code", "actual_output").
		Find(&items).Error

	return total, items, err
}

// 2. 落地实现
func (r *submissionRepoMysql) GetACProblemIDsByUserID(ctx context.Context, userID uint) ([]uint, error) {
	var problemIDs []uint

	err := mysql.DB.WithContext(ctx).
		Model(&model.Submission{}).
		Where("user_id = ? AND status = ?", userID, "AC"). // 只找当前用户且状态是 AC 的
		Distinct("problem_id").                            // 去重（同一道题 AC 多次只算一次）
		Pluck("problem_id", &problemIDs).Error             // 精准拔出 problem_id 这一列塞进数组

	return problemIDs, err
}
