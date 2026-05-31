package service

import (
	"context"
	"encoding/json"
	"fmt"

	"gojo/internal/app/apperror"
	"gojo/internal/submission/dto"
	"gojo/internal/submission/model"
	"gojo/internal/submission/repository"
)

type SubmissionService struct {
	repo repository.SubmissionRepository
}

func NewSubmissionService(r repository.SubmissionRepository) *SubmissionService {
	return &SubmissionService{repo: r}
}

func (s *SubmissionService) SubmitCode(ctx context.Context, userID uint, req dto.SubmitRequest) (*model.Submission, error) {
	submission := model.Submission{
		UserID:    userID,
		ProblemID: req.ProblemID,
		Language:  req.Language,
		Code:      req.Code,
	}

	if err := s.repo.CreateSubmission(ctx, &submission); err != nil {
		return nil, fmt.Errorf("create submission failed: %w", err)
	}

	task := map[string]interface{}{
		"user_id":       userID,
		"submission_id": submission.ID,
		"problem_id":    req.ProblemID,
		"code":          req.Code,
	}

	taskBytes, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("marshal judge task failed: %w", err)
	}

	if err := s.repo.PushToJudgeQueue(ctx, taskBytes); err != nil {
		submission.Status = "judge_failed"
		if err2 := s.repo.UpdateSubmissionStatus(ctx, submission.ID, submission.Status); err2 != nil {
			return nil, fmt.Errorf("update submission status failed: %w, push judge queue failed: %w", err2, err)
		}
		return nil, fmt.Errorf("push judge queue failed: %w", err)
	}

	return &submission, nil
}

func (s *SubmissionService) GetSubmissionResult(ctx context.Context, submissionID string, currentUserID uint) (*model.Submission, error) {
	submission, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	if submission.UserID != currentUserID {
		return nil, apperror.ErrUnauthorizedAccess
	}

	return submission, nil
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
