package repository

import (
	"context"
	"time"

	"gojo/infrastructure/mysql"
	"gojo/internal/submission/model"
	"gojo/pkg/pagination"
)

// SubmissionRepository defines persistence operations for submissions.
type SubmissionRepository interface {
	CreateSubmission(ctx context.Context, sub *model.Submission) error
	GetSubmissionByID(ctx context.Context, id uint) (*model.Submission, error)
	GetSubmissionByUserID(ctx context.Context, id, userID uint) (*model.Submission, error)
	GetSubmissionsByUserID(ctx context.Context, userID uint, page, limit int) (int64, []model.Submission, error)
	GetAllSubmissionsByUserID(ctx context.Context, userID uint) ([]model.Submission, error)
	GetRecentFailedSubmissionsByUserID(ctx context.Context, userID uint, limit int) ([]model.Submission, error)
	ListPendingSubmissionsBefore(ctx context.Context, before time.Time, limit int) ([]model.Submission, error)
	UpdateSubmissionStatus(ctx context.Context, id uint, status string) error
	GetACProblemIDsByUserID(ctx context.Context, userID uint) ([]uint, error)
}

type submissionRepoMysql struct{}

func NewSubmissionRepository() SubmissionRepository {
	return &submissionRepoMysql{}
}

func (r *submissionRepoMysql) CreateSubmission(ctx context.Context, sub *model.Submission) error {
	return mysql.DB.WithContext(ctx).Create(sub).Error
}

func (r *submissionRepoMysql) UpdateSubmissionStatus(ctx context.Context, id uint, status string) error {
	return mysql.DB.WithContext(ctx).Model(&model.Submission{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": status,
	}).Error
}

func (r *submissionRepoMysql) ListPendingSubmissionsBefore(ctx context.Context, before time.Time, limit int) ([]model.Submission, error) {
	if limit <= 0 {
		limit = 100
	}

	var submissions []model.Submission
	err := mysql.DB.WithContext(ctx).
		Where("status = ? AND created_at <= ?", "Pending", before).
		Order("created_at asc").
		Limit(limit).
		Find(&submissions).Error
	return submissions, err
}

func (r *submissionRepoMysql) GetSubmissionByID(ctx context.Context, id uint) (*model.Submission, error) {
	var sub model.Submission
	err := mysql.DB.WithContext(ctx).Omit("actual_output").Where("id = ?", id).First(&sub).Error
	return &sub, err
}

func (r *submissionRepoMysql) GetSubmissionByUserID(ctx context.Context, id, userID uint) (*model.Submission, error) {
	var sub model.Submission
	err := mysql.DB.WithContext(ctx).Omit("actual_output").Where("id = ? AND user_id = ?", id, userID).First(&sub).Error
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

func (r *submissionRepoMysql) GetAllSubmissionsByUserID(ctx context.Context, userID uint) ([]model.Submission, error) {
	var items []model.Submission

	err := mysql.DB.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&items).Error

	return items, err
}

func (r *submissionRepoMysql) GetRecentFailedSubmissionsByUserID(ctx context.Context, userID uint, limit int) ([]model.Submission, error) {
	if limit <= 0 {
		limit = 10
	}

	var items []model.Submission
	err := mysql.DB.WithContext(ctx).
		Omit("actual_output").
		Where("user_id = ? AND status <> ?", userID, "AC").
		Order("created_at desc").
		Limit(limit).
		Find(&items).Error

	return items, err
}

func (r *submissionRepoMysql) GetACProblemIDsByUserID(ctx context.Context, userID uint) ([]uint, error) {
	var problemIDs []uint

	err := mysql.DB.WithContext(ctx).
		Model(&model.Submission{}).
		Where("user_id = ? AND status = ?", userID, "AC").
		Distinct("problem_id").
		Pluck("problem_id", &problemIDs).Error

	return problemIDs, err
}
