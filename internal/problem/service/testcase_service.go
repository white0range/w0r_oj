package service

import (
	"context"
	"log"

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

func (s *TestCaseService) AddTestCase(ctx context.Context, problemID uint, req dto.TestCaseRequest) (uint, error) {
	testCase := model.TestCase{
		ProblemID:      problemID,
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

func (s *TestCaseService) DeleteTestCase(ctx context.Context, caseID uint) error {
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
func (s *TestCaseService) GetTestCases(ctx context.Context, problemID uint, page, limit int) (*dto.TestCaseListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	total, items, err := s.repo.GetTestCase(ctx, problemID, page, limit)
	res := &dto.TestCaseListResponse{
		Page:  page,
		Limit: limit,
		Total: total,
		Items: items,
	}
	return res, err
}
