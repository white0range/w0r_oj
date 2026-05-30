package dto

import "time"

// CreateStudyPlanTaskRequest 表示创建训练计划任务时的请求体。
// goal 先做成可选字段，后面可以扩展成“面试冲刺”“补某个知识点”等目标。
type CreateStudyPlanTaskRequest struct {
	Goal string `json:"goal"`
}

// StudyPlanQueueTask 是推到 Redis 队列里的任务内容。
// worker 收到消息后，主要通过 task_id 去数据库读取最新任务状态。
type StudyPlanQueueTask struct {
	TaskID uint   `json:"task_id"`
	UserID uint   `json:"user_id"`
	Goal   string `json:"goal"`
}

// UserACHistoryResponse 是第一个 internal agent tool 的响应体。
// 这份数据会直接给未来的 Python agent 使用，所以字段尽量保持简单稳定。
type UserACHistoryResponse struct {
	UserID           uint   `json:"user_id"`
	Username         string `json:"username"`
	SolvedCount      int    `json:"solved_count"`
	SolvedProblemIDs []uint `json:"solved_problem_ids"`
}

// FailedSubmissionItem 表示一条最近失败的提交记录。
type FailedSubmissionItem struct {
	SubmissionID uint      `json:"submission_id"`
	ProblemID    uint      `json:"problem_id"`
	ProblemTitle string    `json:"problem_title"`
	Status       string    `json:"status"`
	Language     string    `json:"language"`
	ActualOutput string    `json:"actual_output"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserFailedSubmissionsResponse 是“最近失败提交”工具接口的返回结果。
type UserFailedSubmissionsResponse struct {
	UserID uint                   `json:"user_id"`
	Items  []FailedSubmissionItem `json:"items"`
}

// TagStatItem 表示用户在某个标签下的练习统计。
type TagStatItem struct {
	TagName          string `json:"tag_name"`
	TotalSubmissions int    `json:"total_submissions"`
	FailedSubmissions int   `json:"failed_submissions"`
	SolvedProblems   int    `json:"solved_problems"`
}

// UserTagStatsResponse 是标签统计工具接口的返回结果。
type UserTagStatsResponse struct {
	UserID uint          `json:"user_id"`
	Tags   []TagStatItem `json:"tags"`
}

// CandidateProblemItem 表示一条推荐候选题目的基础信息。
type CandidateProblemItem struct {
	ProblemID      uint     `json:"problem_id"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	TagNames       []string `json:"tag_names"`
	SubmitCount    int      `json:"submit_count"`
	AcceptedCount  int      `json:"accepted_count"`
}

// CandidateProblemsResponse 是候选题检索工具接口的返回结果。
type CandidateProblemsResponse struct {
	RequestedTags []string               `json:"requested_tags"`
	Items         []CandidateProblemItem `json:"items"`
}

// ProblemDetailResponse 是单题详情工具接口的返回结果。
type ProblemDetailResponse struct {
	ProblemID    uint     `json:"problem_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	TimeLimit    int      `json:"time_limit"`
	MemoryLimit  int      `json:"memory_limit"`
	SubmitCount  int      `json:"submit_count"`
	AcceptedCount int     `json:"accepted_count"`
	TagNames     []string `json:"tag_names"`
}

type SubmitStudyPlanFeedbackRequest struct {
	Helpful bool   `json:"helpful"`
	Comment string `json:"comment"`
}

type StudyPlanAdminStatsResponse struct {
	TotalTasks       int64 `json:"total_tasks"`
	PendingTasks     int64 `json:"pending_tasks"`
	RunningTasks     int64 `json:"running_tasks"`
	SucceededTasks   int64 `json:"succeeded_tasks"`
	FailedTasks      int64 `json:"failed_tasks"`
	TotalFeedbacks   int64 `json:"total_feedbacks"`
	HelpfulCount     int64 `json:"helpful_count"`
	UnhelpfulCount   int64 `json:"unhelpful_count"`
}
