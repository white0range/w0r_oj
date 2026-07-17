package model

import "time"

const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusSucceeded = "succeeded"
	TaskStatusFailed    = "failed"

	ChatSessionStatusActive   = "active"
	ChatSessionStatusArchived = "archived"

	ChatMessageRoleUser      = "user"
	ChatMessageRoleAssistant = "assistant"
	ChatMessageRoleSystem    = "system"
)

type ChatSession struct {
	ID uint `gorm:"primaryKey" json:"id"`

	UserID uint   `gorm:"index;not null" json:"user_id"`
	Title  string `gorm:"type:varchar(255)" json:"title"`
	Status string `gorm:"size:20;not null;default:'active';index" json:"status"`

	SummaryText   string     `gorm:"type:text" json:"summary_text"`
	LastMessageAt *time.Time `json:"last_message_at"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatMessage struct {
	ID uint `gorm:"primaryKey" json:"id"`

	SessionID uint  `gorm:"index;not null" json:"session_id"`
	TurnID    *uint `gorm:"index" json:"turn_id"`

	Role string `gorm:"size:20;not null;index" json:"role"`

	Content           string `gorm:"type:text;not null" json:"content"`
	StructuredPayload string `gorm:"type:text" json:"structured_payload"`

	IsSummary bool `gorm:"not null;default:false" json:"is_summary"`
	Archived  bool `gorm:"not null;default:false" json:"archived"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatTurn struct {
	ID uint `gorm:"primaryKey" json:"id"`

	SessionID uint `gorm:"index;not null" json:"session_id"`
	UserID    uint `gorm:"index;not null" json:"user_id"`

	UserMessageID      uint  `gorm:"not null" json:"user_message_id"`
	AssistantMessageID *uint `json:"assistant_message_id"`

	Status string `gorm:"size:20;not null;index" json:"status"`

	Result       string `gorm:"type:text" json:"result"`
	ErrorMessage string `gorm:"type:text" json:"error_message"`
	Model        string `gorm:"size:100" json:"model"`

	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	ProcessingToken string     `gorm:"size:64;index" json:"-"`
	LeaseExpiresAt  *time.Time `gorm:"index" json:"-"`
}

// ChatPlanFeedback records a user's evaluation of one completed assistant turn.
type ChatPlanFeedback struct {
	ID uint `gorm:"primaryKey" json:"id"`

	ChatTurnID uint `gorm:"not null;uniqueIndex:idx_chat_plan_feedback_turn_user" json:"chat_turn_id"`
	UserID     uint `gorm:"not null;uniqueIndex:idx_chat_plan_feedback_turn_user" json:"user_id"`

	Helpful bool   `gorm:"not null" json:"helpful"`
	Comment string `gorm:"type:text" json:"comment"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
