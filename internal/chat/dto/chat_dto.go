package dto

import "time"

type CreateChatSessionRequest struct {
	Title string `json:"title"`
}

type SendChatMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type ChatTurnQueueTask struct {
	TurnID uint `json:"turn_id"`
}

type SubmitChatPlanFeedbackRequest struct {
	Helpful bool   `json:"helpful"`
	Comment string `json:"comment"`
}

// The following response types are consumed by the internal Chat agent tools.
type UserACHistoryResponse struct {
	UserID           uint   `json:"user_id"`
	Username         string `json:"username"`
	SolvedCount      int    `json:"solved_count"`
	SolvedProblemIDs []uint `json:"solved_problem_ids"`
}

type FailedSubmissionItem struct {
	SubmissionID uint      `json:"submission_id"`
	ProblemID    uint      `json:"problem_id"`
	ProblemTitle string    `json:"problem_title"`
	Status       string    `json:"status"`
	Language     string    `json:"language"`
	ActualOutput string    `json:"actual_output"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserFailedSubmissionsResponse struct {
	UserID uint                   `json:"user_id"`
	Items  []FailedSubmissionItem `json:"items"`
}

type TagStatItem struct {
	TagName           string `json:"tag_name"`
	TotalSubmissions  int    `json:"total_submissions"`
	FailedSubmissions int    `json:"failed_submissions"`
	SolvedProblems    int    `json:"solved_problems"`
}

type UserTagStatsResponse struct {
	UserID uint          `json:"user_id"`
	Tags   []TagStatItem `json:"tags"`
}

type CandidateProblemItem struct {
	ProblemID     uint     `json:"problem_id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	TagNames      []string `json:"tag_names"`
	SubmitCount   int      `json:"submit_count"`
	AcceptedCount int      `json:"accepted_count"`
}

type CandidateProblemsResponse struct {
	RequestedTags []string               `json:"requested_tags"`
	Items         []CandidateProblemItem `json:"items"`
}

type ProblemDetailResponse struct {
	ProblemID     uint     `json:"problem_id"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	TimeLimit     int      `json:"time_limit"`
	MemoryLimit   int      `json:"memory_limit"`
	SubmitCount   int      `json:"submit_count"`
	AcceptedCount int      `json:"accepted_count"`
	TagNames      []string `json:"tag_names"`
}
