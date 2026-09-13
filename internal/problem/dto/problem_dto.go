package dto

import (
	"gojo/internal/problem/model"
)

// 顺便在这里准备一个接客用的 dto 表单
// 因为我们不希望管理员在发布题目时，还要手填 SubmitCount 这种应该由系统管理的字段
type ProblemRequest struct {
	Title       string `json:"title" binding:"required,max=200"`
	Description string `json:"description" binding:"required,max=32768"`
	TimeLimit   int    `json:"time_limit" binding:"omitempty,min=100,max=10000"` // milliseconds
	MemoryLimit int    `json:"memory_limit" binding:"omitempty,min=16,max=512"`  // megabytes

	// 直接复用 TestCaseRequest！
	TestCases []TestCaseRequest `json:"test_cases" binding:"max=10"`
	TagIDs    []uint            `json:"tag_ids" binding:"max=20"`
}

type UpdateProblemTagsRequest struct {
	TagIDs []uint `json:"tag_ids" binding:"max=20"`
}

type ProblemListResponse struct {
	Total   int64           `json:"total"`
	Page    int             `json:"page"`
	Limit   int             `json:"limit"`
	TagID   string          `json:"tag_id"`
	Message string          `json:"message"`
	Items   []model.Problem `json:"items"`
}
