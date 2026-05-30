package model

import "time"

const (
	// TaskStatusPending 表示任务刚创建，等待后续处理。
	TaskStatusPending = "pending"

	// TaskStatusRunning 表示任务正在执行。
	TaskStatusRunning = "running"

	// TaskStatusSucceeded 表示任务已经成功完成。
	TaskStatusSucceeded = "succeeded"

	// TaskStatusFailed 表示任务执行失败。
	TaskStatusFailed = "failed"
)

// StudyPlanTask 表示一次“训练计划生成”任务。
// 第一版先把任务骨架建出来，后面再接 Python agent、向量检索和反馈闭环。
type StudyPlanTask struct {
	ID uint `gorm:"primaryKey" json:"id"`

	UserID uint `json:"user_id"`

	// 用户希望本次计划达成什么目标，比如“准备面试”。
	Goal string `gorm:"type:varchar(255)" json:"goal"`

	Status string `gorm:"size:20;not null" json:"status"`

	// 结果先直接存文本/JSON 字符串，后面如果结构更稳定，再考虑拆字段。
	Result string `gorm:"type:text" json:"result"`

	ErrorMessage string `gorm:"type:text" json:"error_message"`
	Model        string `gorm:"size:100" json:"model"`

	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// StudyPlanFeedback 琛ㄧず鐢ㄦ埛瀵硅缁冭鍒掔粨鏋滅殑鍙嶉銆?
type StudyPlanFeedback struct {
	ID uint `gorm:"primaryKey" json:"id"`

	TaskID uint `gorm:"not null;uniqueIndex:idx_study_plan_feedback_task_user" json:"task_id"`
	UserID uint `gorm:"not null;uniqueIndex:idx_study_plan_feedback_task_user" json:"user_id"`

	Helpful bool   `gorm:"not null" json:"helpful"`
	Comment string `gorm:"type:text" json:"comment"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
