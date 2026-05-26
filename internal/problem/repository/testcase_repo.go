package repository

import (
	"context"

	"gojo/infrastructure/mysql"
	"gojo/internal/app/apperror"
	"gojo/internal/problem/model"
	"gojo/pkg/pagination"
)

type TestCaseRepository interface {
	AddTestCase(ctx context.Context, testCase *model.TestCase) error
	DeleteTestCase(ctx context.Context, caseID string) error
	GetTestCase(ctx context.Context, problemID uint, page, limit int) (int64, []model.TestCase, error)
}

type TestCaseRepoMysql struct{}

func NewTestCaseRepository() TestCaseRepository {
	return &TestCaseRepoMysql{}
}

func (r *TestCaseRepoMysql) AddTestCase(ctx context.Context, testCase *model.TestCase) error {
	var count int64
	mysql.DB.Model(&model.Problem{}).Where("id = ?", testCase.ProblemID).Count(&count)
	if count == 0 {
		return apperror.ErrProblemNotFound
	}

	return mysql.DB.WithContext(ctx).Create(testCase).Error
}

func (r *TestCaseRepoMysql) DeleteTestCase(ctx context.Context, caseID string) error {
	result := mysql.DB.WithContext(ctx).Where("id = ?", caseID).Delete(&model.TestCase{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperror.ErrCaseNotFound
	}
	return nil
}

func (r *TestCaseRepoMysql) GetTestCase(ctx context.Context, problemID uint, page, limit int) (int64, []model.TestCase, error) {
	var total int64
	var items []model.TestCase

	query := mysql.DB.WithContext(ctx).Model(&model.TestCase{}).Where("problem_id = ?", problemID)
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	if err := query.Scopes(pagination.Paginate(page, limit)).Find(&items).Error; err != nil {
		return 0, nil, err
	}

	return total, items, nil
}
