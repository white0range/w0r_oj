package service

import (
	"context"
	"errors"
	"testing"

	"gojo/internal/app/apperror"
	problemModel "gojo/internal/problem/model"
	submissionModel "gojo/internal/submission/model"
)

type failedSubmissionProviderStub struct {
	submission *submissionModel.Submission
	err        error
}

func (s *failedSubmissionProviderStub) GetSubmissionByID(context.Context, uint) (*submissionModel.Submission, error) {
	return s.submission, s.err
}

func (s *failedSubmissionProviderStub) GetAllSubmissionsByUserID(context.Context, uint) ([]submissionModel.Submission, error) {
	return nil, nil
}

func (s *failedSubmissionProviderStub) GetRecentFailedSubmissionsByUserID(context.Context, uint, int) ([]submissionModel.Submission, error) {
	return []submissionModel.Submission{*s.submission}, s.err
}

type problemCatalogProviderStub struct {
	problem *problemModel.Problem
	err     error
}

func (s *problemCatalogProviderStub) GetAllProblemsWithTags(context.Context) ([]problemModel.Problem, error) {
	return nil, nil
}

func (s *problemCatalogProviderStub) GetProblemByID(context.Context, uint) (*problemModel.Problem, error) {
	return s.problem, s.err
}

func TestAgentFailureToolsReturnVerdictInsteadOfStoredOutput(t *testing.T) {
	submission := &submissionModel.Submission{
		UserID:       7,
		ProblemID:    40,
		Language:     "go",
		Code:         "package main",
		Status:       "WA",
		ActualOutput: "legacy hidden output marker",
	}
	submission.ID = 99
	problem := &problemModel.Problem{
		Title:       "Shortest Path",
		Description: "Find the shortest path.",
		Tags:        []problemModel.Tag{{Name: "graph"}},
	}
	problem.ID = 40

	svc := NewChatService(nil, nil, &failedSubmissionProviderStub{submission: submission}, &problemCatalogProviderStub{problem: problem})
	detail, err := svc.GetFailedSubmissionDetail(context.Background(), 7, 99)
	if err != nil {
		t.Fatalf("GetFailedSubmissionDetail returned error: %v", err)
	}
	if detail.Code != submission.Code || detail.ProblemDescription != problem.Description {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if len(detail.ProblemTags) != 1 || detail.ProblemTags[0] != "graph" {
		t.Fatalf("unexpected problem tags: %v", detail.ProblemTags)
	}
	if want := "输出与预期不符。"; detail.ActualOutput != want {
		t.Fatalf("detail output=%q, want %q", detail.ActualOutput, want)
	}
	recent, err := svc.GetUserFailedSubmissions(context.Background(), 7, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent.Items) != 1 || recent.Items[0].ActualOutput != detail.ActualOutput {
		t.Fatalf("recent failures did not return the fixed verdict message: %+v", recent.Items)
	}
}

func TestGetFailedSubmissionDetailRejectsAnotherUser(t *testing.T) {
	submission := &submissionModel.Submission{UserID: 8, ProblemID: 40, Status: "WA"}
	submission.ID = 99
	svc := NewChatService(nil, nil, &failedSubmissionProviderStub{submission: submission}, &problemCatalogProviderStub{})

	_, err := svc.GetFailedSubmissionDetail(context.Background(), 7, 99)
	if !errors.Is(err, apperror.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}
