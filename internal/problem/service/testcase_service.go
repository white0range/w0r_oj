package service

import (
	"context"
	"strconv"

	"gojo/internal/app/apperror"
	"gojo/internal/problem/dto"
	"gojo/internal/problem/model"
	"gojo/internal/problem/repository"
)

type TestCaseService struct {
	repo repository.TestCaseRepository
}

func NewTestCaseService(r repository.TestCaseRepository) *TestCaseService {
	return &TestCaseService{repo: r}
}

func (s *TestCaseService) AddTestCase(ctx context.Context, problemIDStr string, req dto.TestCaseRequest) (uint, error) {
	problemID, err := strconv.Atoi(problemIDStr)
	if err != nil {
		return 0, apperror.ErrInvalidID
	}

	testCase := model.TestCase{
		ProblemID:      uint(problemID),
		Input:          req.Input,
		ExpectedOutput: req.ExpectedOutput,
	}

	if err := s.repo.AddTestCase(ctx, &testCase); err != nil {
		return 0, err
	}

	return testCase.ID, nil
}

func (s *TestCaseService) DeleteTestCase(ctx context.Context, caseID string) error {
	return s.repo.DeleteTestCase(ctx, caseID)
}

func (s *TestCaseService) GetTestCases(ctx context.Context, problemIDStr string, page, limit int) (*dto.TestCaseListResponse, error) {
	problemID, err := strconv.Atoi(problemIDStr)
	if err != nil {
		return nil, apperror.ErrInvalidID
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	res := &dto.TestCaseListResponse{
		Page:  page,
		Limit: limit,
	}
	res.Total, res.Items, err = s.repo.GetTestCase(ctx, uint(problemID), page, limit)
	return res, err
}
