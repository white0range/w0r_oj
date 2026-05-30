package model

import "time"

const (
	// TaskStatusPending 表示任务刚创建，还没开始处理
	TaskStatusPending = "pending"

	// TaskStatusRunning 表示 worker 正在处理任务
	TaskStatusRunning = "running"

	// TaskStatusSucceeded 表示任务已经成功完成
	TaskStatusSucceeded = "succeeded"

	// TaskStatusFailed 表示任务执行失败
	TaskStatusFailed = "failed"
)

// AnalysisTask 表示一次 AI 诊断任务。
// 用户选择某条 submission 后，系统会创建一条这样的记录。
type AnalysisTask struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 谁发起了这个任务
	UserID uint `json:"user_id"`

	// 这次诊断针对的是哪条提交记录
	SubmissionID uint `json:"submission_id"`

	// 任务状态：
	// pending   -> 刚创建，等待处理
	// running   -> worker 正在处理
	// succeeded -> 成功完成
	// failed    -> 执行失败
	Status string `gorm:"size:20;not null" json:"status"`

	// AI 最终生成的诊断结果
	Result string `gorm:"type:text" json:"result"`

	// 如果失败了，把错误原因记下来，方便排查
	ErrorMessage string `gorm:"type:text" json:"error_message"`

	// 记录本次任务使用的模型，后面做统计和反馈回流会很有用
	Model string `gorm:"size:100" json:"model"`

	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// AnalysisFeedback 表示用户对一次 AI 分析结果的反馈。
// 第一版先收最关键的两个信息：
// 1. 这次分析有没有帮助
// 2. 如果愿意，用户可以补一句简短备注
type AnalysisFeedback struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// 一个用户对同一个任务只保留一条反馈记录。
	TaskID uint `gorm:"not null;uniqueIndex:idx_analysis_feedback_task_user" json:"task_id"`
	UserID uint `gorm:"not null;uniqueIndex:idx_analysis_feedback_task_user" json:"user_id"`

	Helpful bool   `gorm:"not null" json:"helpful"`
	Comment string `gorm:"type:text" json:"comment"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
