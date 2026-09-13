package service

import (
	"context"
	"errors"
	"testing"

	"gojo/internal/app/apperror"
	problemModel "gojo/internal/problem/model"
	"gojo/internal/submission/dto"
)

type missingProblemReader struct{}

func (missingProblemReader) GetProblemByID(context.Context, string) (*problemModel.Problem, error) {
	return nil, apperror.ErrProblemNotFound
}

func TestSubmitCodeRejectsUnsupportedLanguageBeforePersistence(t *testing.T) {
	svc := NewSubmissionService(nil, missingProblemReader{})

	_, err := svc.SubmitCode(context.Background(), 1, dto.SubmitRequest{
		ProblemID: 1,
		Language:  "python",
		Code:      "print('hello')",
	})
	if !errors.Is(err, apperror.ErrUnsupportedLanguage) {
		t.Fatalf("SubmitCode() error = %v, want ErrUnsupportedLanguage", err)
	}
}

func TestSubmitCodeRejectsMissingProblemBeforePersistence(t *testing.T) {
	svc := NewSubmissionService(nil, missingProblemReader{})

	_, err := svc.SubmitCode(context.Background(), 1, dto.SubmitRequest{
		ProblemID: 999,
		Language:  "go",
		Code:      "package main\nfunc main() {}",
	})
	if !errors.Is(err, apperror.ErrProblemNotFound) {
		t.Fatalf("SubmitCode() error = %v, want ErrProblemNotFound", err)
	}
}
