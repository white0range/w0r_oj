package repository

import (
	"context"

	"gojo/infrastructure/mysql"
	"gojo/internal/app/apperror"
	"gojo/internal/problem/model"
	model2 "gojo/internal/submission/model"
	"gojo/pkg/pagination"

	"gorm.io/gorm"
)

type ProblemRepository interface {
	CreateProblem(ctx context.Context, problem *model.Problem) error
	GetProblemByID(ctx context.Context, id string) (*model.Problem, error)
	GetProblemList(ctx context.Context, offset, limit int, tagID string) ([]model.Problem, int64, error)
	GetUserACProblemIDs(ctx context.Context, uid uint, problemIDs []uint) ([]uint, error)
	UpdateProblem(ctx context.Context, problemID string, data map[string]interface{}) error
	DeleteProblem(ctx context.Context, id string) error
	UpdateProblemTags(ctx context.Context, id string, tagIDs []uint) error
	GetAllProblemsWithTags(ctx context.Context) ([]model.Problem, error)
	GetTagsByIDs(ctx context.Context, ids []uint) ([]model.Tag, error)
}

type problemRepoMysql struct{}

func NewProblemRepository() ProblemRepository {
	return &problemRepoMysql{}
}

func (r *problemRepoMysql) CreateProblem(ctx context.Context, problem *model.Problem) error {
	return mysql.DB.WithContext(ctx).Create(problem).Error
}

func (r *problemRepoMysql) GetTagsByIDs(ctx context.Context, ids []uint) ([]model.Tag, error) {
	var tags []model.Tag
	if err := mysql.DB.WithContext(ctx).Find(&tags, ids).Error; err != nil {
		return nil, err
	}
	if len(tags) != len(ids) {
		return nil, apperror.ErrTagNotFound
	}
	return tags, nil
}

func (r *problemRepoMysql) GetProblemByID(ctx context.Context, id string) (*model.Problem, error) {
	var problem model.Problem
	err := mysql.DB.WithContext(ctx).Preload("Tags").First(&problem, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, apperror.ErrProblemNotFound
		}
		return nil, err
	}
	return &problem, nil
}

func (r *problemRepoMysql) GetProblemList(ctx context.Context, page, limit int, tagID string) ([]model.Problem, int64, error) {
	var items []model.Problem
	var total int64

	query := mysql.DB.WithContext(ctx).Model(&model.Problem{})
	if tagID != "" {
		query = query.Joins("JOIN problem_tags ON problem_tags.problem_id = problems.id").
			Where("problem_tags.tag_id = ?", tagID)
	}

	query.Count(&total)

	err := query.Select("problems.id", "problems.title", "problems.submit_count", "problems.accepted_count").
		Preload("Tags").
		Scopes(pagination.Paginate(page, limit)).
		Find(&items).Error

	return items, total, err
}

func (r *problemRepoMysql) GetUserACProblemIDs(ctx context.Context, uid uint, problemIDs []uint) ([]uint, error) {
	var userACList []uint
	err := mysql.DB.WithContext(ctx).Model(&model2.Submission{}).
		Where("user_id = ? AND status = 'AC' AND problem_id IN ?", uid, problemIDs).
		Distinct("problem_id").
		Pluck("problem_id", &userACList).Error
	return userACList, err
}

func (r *problemRepoMysql) UpdateProblem(ctx context.Context, problemID string, data map[string]interface{}) error {
	result := mysql.DB.WithContext(ctx).Model(&model.Problem{}).Where("id = ?", problemID).Updates(data)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperror.ErrProblemNotFound
	}
	return nil
}

func (r *problemRepoMysql) DeleteProblem(ctx context.Context, id string) error {
	return mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var problem model.Problem
		if err := tx.First(&problem, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return apperror.ErrProblemNotFound
			}
			return err
		}

		if err := tx.Where("problem_id = ?", id).Delete(&model.TestCase{}).Error; err != nil {
			return err
		}

		if err := tx.Model(&problem).Association("Tags").Clear(); err != nil {
			return err
		}

		if err := tx.Delete(&problem).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *problemRepoMysql) UpdateProblemTags(ctx context.Context, id string, tagIDs []uint) error {
	return mysql.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var problem model.Problem
		if err := tx.First(&problem, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return apperror.ErrProblemNotFound
			}
			return err
		}

		var tags []model.Tag
		if len(tagIDs) > 0 {
			if err := tx.Find(&tags, tagIDs).Error; err != nil {
				return err
			}
			if len(tags) != len(tagIDs) {
				return apperror.ErrTagNotFound
			}
		}

		if err := tx.Model(&problem).Association("Tags").Replace(tags); err != nil {
			return err
		}

		return nil
	})
}

func (r *problemRepoMysql) GetAllProblemsWithTags(ctx context.Context) ([]model.Problem, error) {
	var problems []model.Problem
	err := mysql.DB.WithContext(ctx).Preload("Tags").Find(&problems).Error
	return problems, err
}
