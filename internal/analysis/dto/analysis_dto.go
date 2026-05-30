package dto

// CreateAnalysisTaskRequest 表示“创建 AI 诊断任务”的请求体。
type CreateAnalysisTaskRequest struct {
	SubmissionID uint `json:"submission_id"`
}

// 第一版先放最必要的字段，worker 拿到后主要靠 task_id 去数据库查完整任务。
type AnalysisQueueTask struct {
	TaskID       uint `json:"task_id"`
	UserID       uint `json:"user_id"`
	SubmissionID uint `json:"submission_id"`
}

// SubmitFeedbackRequest 表示用户提交分析反馈时的请求体。
type SubmitFeedbackRequest struct {
	Helpful bool   `json:"helpful"`
	Comment string `json:"comment"`
}

// AdminStatsResponse 表示管理员查看 analysis 模块概况时返回的统计结果。
type AdminStatsResponse struct {
	TotalTasks       int64 `json:"total_tasks"`
	PendingTasks     int64 `json:"pending_tasks"`
	RunningTasks     int64 `json:"running_tasks"`
	SucceededTasks   int64 `json:"succeeded_tasks"`
	FailedTasks      int64 `json:"failed_tasks"`
	TotalFeedbacks   int64 `json:"total_feedbacks"`
	HelpfulCount     int64 `json:"helpful_count"`
	UnhelpfulCount   int64 `json:"unhelpful_count"`
}
