package dto

type CreateChatSessionRequest struct {
	Title string `json:"title"`
}

type SendChatMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type ChatTurnQueueTask struct {
	TurnID uint `json:"turn_id"`
}
