package service

import (
	"context"
	"log"
	"strconv"

	"gojo/internal/app/apperror"
	"gojo/internal/problem/cacheutil"
	"gojo/internal/problem/dto"
	"gojo/internal/problem/model"
	"gojo/internal/problem/repository"
	"gojo/internal/syncer"
)

type TestCaseService struct {
	repo   repository.TestCaseRepository
	syncer syncer.Producer
}

func NewTestCaseService(r repository.TestCaseRepository, producer syncer.Producer) *TestCaseService {
	return &TestCaseService{repo: r, syncer: producer}
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

	cacheutil.InvalidateProblem(ctx, testCase.ProblemID, "add testcase")
	if err := s.syncer.EnqueueProblemUpsert(ctx, testCase.ProblemID); err != nil {
		log.Printf("enqueue problem %d sync after adding testcase failed: %v", testCase.ProblemID, err)
	}

	return testCase.ID, nil
}

func (s *TestCaseService) DeleteTestCase(ctx context.Context, caseID string) error {
	problemID, err := s.repo.DeleteTestCase(ctx, caseID)
	if err != nil {
		return err
	}

	cacheutil.InvalidateProblem(ctx, problemID, "delete testcase")
	if err := s.syncer.EnqueueProblemUpsert(ctx, problemID); err != nil {
		log.Printf("enqueue problem %d sync after deleting testcase failed: %v", problemID, err)
	}
	return nil
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
