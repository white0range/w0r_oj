package service

import (
	"context"
	"errors"
	"testing"

	"gojo/internal/app/apperror"
	"gojo/internal/problem/dto"
)

func TestCreateProblemRejectsUnsafeResourceLimits(t *testing.T) {
	svc := &ProblemService{}
	tests := []dto.ProblemRequest{
		{TimeLimit: 99, MemoryLimit: 256},
		{TimeLimit: 10001, MemoryLimit: 256},
		{TimeLimit: 1000, MemoryLimit: 15},
		{TimeLimit: 1000, MemoryLimit: 513},
	}

	for _, req := range tests {
		_, err := svc.CreateProblem(context.Background(), req)
		if !errors.Is(err, apperror.ErrInvalidProblemLimits) {
			t.Fatalf("CreateProblem(%+v) error = %v, want ErrInvalidProblemLimits", req, err)
		}
	}
}

func TestUpdateProblemRejectsUnsafeResourceLimits(t *testing.T) {
	svc := &ProblemService{}

	err := svc.UpdateProblem(context.Background(), "1", dto.ProblemRequest{TimeLimit: 10001})
	if !errors.Is(err, apperror.ErrInvalidProblemLimits) {
		t.Fatalf("UpdateProblem() error = %v, want ErrInvalidProblemLimits", err)
	}
}
