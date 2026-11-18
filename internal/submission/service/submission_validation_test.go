package service

import (
	"context"
	"errors"
	"testing"

	"gojo/internal/app/apperror"
	judgeDTO "gojo/internal/judge/dto"
	problemModel "gojo/internal/problem/model"
	"gojo/internal/submission/dto"
	"gojo/internal/submission/model"
	"gojo/internal/submission/repository"
)

type missingProblemReader struct{}

func (missingProblemReader) GetProblemByID(context.Context, uint) (*problemModel.Problem, error) {
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

type existingProblemReader struct{}

func (existingProblemReader) GetProblemByID(context.Context, uint) (*problemModel.Problem, error) {
	return &problemModel.Problem{}, nil
}

type submissionWriterStub struct {
	repository.SubmissionRepository
	saved model.Submission
	err   error
}

func (s *submissionWriterStub) CreateSubmission(_ context.Context, submission *model.Submission) error {
	if s.err != nil {
		return s.err
	}
	submission.ID = 37
	s.saved = *submission
	return nil
}

type judgeEnqueueFunc func(context.Context, judgeDTO.JudgeTask) (bool, error)

func (f judgeEnqueueFunc) Enqueue(ctx context.Context, task judgeDTO.JudgeTask) (bool, error) {
	return f(ctx, task)
}

func TestSavedSubmissionReturnsReceiptWhenDispatchFails(t *testing.T) {
	dispatchError := errors.New("queue unavailable")
	for _, test := range []struct {
		name        string
		dispatchErr error
	}{
		{"queued", nil},
		{"awaiting dispatch", dispatchError},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := &submissionWriterStub{}
			svc := NewSubmissionService(writer, existingProblemReader{})
			svc.queue = judgeEnqueueFunc(func(_ context.Context, task judgeDTO.JudgeTask) (bool, error) {
				if writer.saved.ID != 37 || writer.saved.Status != "Pending" || task.SubmissionID != writer.saved.ID {
					t.Fatalf("dispatch should use the already saved submission: %+v", writer.saved)
				}
				return test.dispatchErr == nil, test.dispatchErr
			})
			receipt, err := svc.SubmitCode(context.Background(), 7, dto.SubmitRequest{
				ProblemID: 3, Language: "go", Code: "package main\nfunc main() {}",
			})
			if err != nil || receipt == nil || receipt.SubmissionID != 37 || receipt.Status != "Pending" ||
				receipt.DispatchPending != (test.dispatchErr != nil) {
				t.Fatalf("unexpected receipt: %+v, err=%v", receipt, err)
			}
		})
	}
}

func TestSubmitCodeDoesNotDispatchWhenPersistenceFails(t *testing.T) {
	saveError := errors.New("database unavailable")
	svc := NewSubmissionService(&submissionWriterStub{err: saveError}, existingProblemReader{})
	svc.queue = judgeEnqueueFunc(func(context.Context, judgeDTO.JudgeTask) (bool, error) {
		t.Fatal("dispatch happened before persistence")
		return false, nil
	})
	receipt, err := svc.SubmitCode(context.Background(), 7, dto.SubmitRequest{
		ProblemID: 3, Language: "go", Code: "package main\nfunc main() {}",
	})
	if receipt != nil || !errors.Is(err, saveError) {
		t.Fatalf("expected persistence error, got receipt=%+v err=%v", receipt, err)
	}
}
