package dto

import "gojo/internal/submission/model"

// MySubmissionsResponse 专属的数据传输对象 (dto)
type MySubmissionsResponse struct {
	Total int64              `json:"total"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
	Items []model.Submission `json:"items"`
}

// 顺便准备好大堂经理的“接客表单”（dto）
// 玩家在网页上点击“提交代码”时，只会发来这三个字段。
// UserID 不需要他传，我们的保安（AuthMiddleware）会从手环里掏出来！
type SubmitRequest struct {
	ProblemID uint   `json:"problem_id" binding:"required"`
	Language  string `json:"language" binding:"required,oneof=go"`
	Code      string `json:"code" binding:"required,max=65536"`
}
