package service

import (
	"context"
	"errors"
	"fmt"
	"log"

	"gojo/internal/app/apperror"
	judgeDTO "gojo/internal/judge/dto"
	judgeQueue "gojo/internal/judge/queue"
	problemModel "gojo/internal/problem/model"
	"gojo/internal/submission/dto"
	"gojo/internal/submission/model"
	"gojo/internal/submission/repository"
)

type problemReader interface {
	GetProblemByID(ctx context.Context, id uint) (*problemModel.Problem, error)
}

type judgeEnqueuer interface {
	Enqueue(context.Context, judgeDTO.JudgeTask) (bool, error)
}

type SubmissionService struct {
	repo     repository.SubmissionRepository
	problems problemReader
	queue    judgeEnqueuer
}

func NewSubmissionService(r repository.SubmissionRepository, problems problemReader) *SubmissionService {
	return &SubmissionService{repo: r, problems: problems, queue: judgeQueue.New()}
}

func (s *SubmissionService) SubmitCode(ctx context.Context, userID uint, req dto.SubmitRequest) (*dto.SubmitReceipt, error) {
	if req.Language != "go" {
		return nil, apperror.ErrUnsupportedLanguage
	}
	if s.problems == nil {
		return nil, errors.New("problem repository is not configured")
	}
	if _, err := s.problems.GetProblemByID(ctx, req.ProblemID); err != nil {
		if errors.Is(err, apperror.ErrProblemNotFound) {
			return nil, apperror.ErrProblemNotFound
		}
		return nil, fmt.Errorf("validate problem: %w", err)
	}

	submission := model.Submission{
		UserID:    userID,
		ProblemID: req.ProblemID,
		Language:  req.Language,
		Code:      req.Code,
		Status:    "Pending",
	}

	if err := s.repo.CreateSubmission(ctx, &submission); err != nil {
		return nil, fmt.Errorf("create submission failed: %w", err)
	}

	task := judgeDTO.NewJudgeTask(submission.ID, req.ProblemID, userID, req.Code)
	receipt := &dto.SubmitReceipt{SubmissionID: submission.ID, Status: submission.Status}
	if _, err := s.queue.Enqueue(ctx, task); err != nil {
		receipt.DispatchPending = true
		log.Printf("submission_id=%d saved, judge dispatch pending: %v", submission.ID, err)
	}

	return receipt, nil
}

func (s *SubmissionService) GetSubmissionResult(ctx context.Context, submissionID, currentUserID uint) (*model.Submission, error) {
	return s.repo.GetSubmissionByUserID(ctx, submissionID, currentUserID)
}

func (s *SubmissionService) GetMySubmissions(ctx context.Context, userID uint, page, limit int) (*dto.MySubmissionsResponse, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	total, items, err := s.repo.GetSubmissionsByUserID(ctx, userID, page, limit)
	if err != nil {
		return nil, err
	}

	return &dto.MySubmissionsResponse{
		Page:  page,
		Limit: limit,
		Total: total,
		Items: items,
	}, nil
}

func (s *SubmissionService) GetACProblemIDsByUserID(ctx context.Context, userID uint) ([]uint, error) {
	return s.repo.GetACProblemIDsByUserID(ctx, userID)
}
