package handler

import (
	"errors"
	"gojo/internal/analysis/dto"
	"gojo/internal/analysis/service"
	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AnalysisHandler struct {
	svc *service.AnalysisService
}

func NewAnalysisHandler(svc *service.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{
		svc: svc,
	}
}

// CreateAnalysisTask 创建一条新的 AI 诊断任务。
// 第一版先做最小闭环：拿到 submission_id -> 创建任务 -> 返回 task_id。
func (h *AnalysisHandler) CreateAnalysisTask(c *gin.Context) {
	var req dto.CreateAnalysisTaskRequest

	// 1. 解析前端传来的 JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	// 2. 从中间件里取当前登录用户
	userIDRaw, exists := c.Get("userID")
	if !exists {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "please login first")
		return
	}

	userID, ok := userIDRaw.(uint)
	if !ok {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid user identity")
		return
	}

	// 3. 调用 service 创建任务
	task, err := h.svc.CreateAnalysisTask(c.Request.Context(), userID, req.SubmissionID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrSubmissionNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "submission not found")
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot create analysis task for other users' submissions")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "create analysis task failed")
		}
		return
	}

	// 4. 返回创建结果
	response.OK(c, gin.H{
		"task_id":       task.ID,
		"submission_id": task.SubmissionID,
		"status":        task.Status,
	})
}

// GetAnalysisTask 按任务 id 查询任务详情。
func (h *AnalysisHandler) GetAnalysisTask(c *gin.Context) {
	// 1. 先拿路径参数
	taskIDStr := c.Param("id")

	// 2. 字符串转成 uint 需要先转 int
	taskIDUint64, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid task id")
		return
	}
	taskID := uint(taskIDUint64)

	// 3. 从中间件里取当前登录用户
	userIDRaw, exists := c.Get("userID")
	if !exists {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "please login first")
		return
	}

	userID, ok := userIDRaw.(uint)
	if !ok {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid user identity")
		return
	}

	// 4. 调用 service 查询任务
	task, err := h.svc.GetAnalysisTask(c.Request.Context(), userID, taskID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' analysis tasks")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "analysis task not found")
		}
		return
	}
	// 5. 把任务详情返回给前端
	response.OK(c, gin.H{
		"id":            task.ID,
		"user_id":       task.UserID,
		"submission_id": task.SubmissionID,
		"status":        task.Status,
		"result":        task.Result,
		"error_message": task.ErrorMessage,
		"model":         task.Model,
		"created_at":    task.CreatedAt,
		"updated_at":    task.UpdatedAt,
		"finished_at":   task.FinishedAt,
	})
}

// SubmitFeedback 保存当前用户对分析结果的反馈。
func (h *AnalysisHandler) SubmitFeedback(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	var req dto.SubmitFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	feedback, err := h.svc.SubmitFeedback(c.Request.Context(), userID, taskID, req.Helpful, req.Comment)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot submit feedback for other users' analysis tasks")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "submit feedback failed")
		}
		return
	}

	response.OK(c, gin.H{
		"task_id": feedback.TaskID,
		"helpful": feedback.Helpful,
		"comment": feedback.Comment,
	})
}

// GetFeedback 查询当前用户对某条任务的反馈。
func (h *AnalysisHandler) GetFeedback(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	feedback, err := h.svc.GetFeedback(c.Request.Context(), userID, taskID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' feedback")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "feedback not found")
		}
		return
	}

	response.OK(c, feedback)
}

// GetAdminStats 供管理员查看 analysis 模块的整体统计概况。
func (h *AnalysisHandler) GetAdminStats(c *gin.Context) {
	stats, err := h.svc.GetAdminStats(c.Request.Context())
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get analysis stats failed")
		return
	}

	response.OK(c, stats)
}

// parseTaskID 统一解析路径里的任务 id。
func parseTaskID(c *gin.Context) (uint, bool) {
	taskIDStr := c.Param("id")
	taskIDUint64, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid task id")
		return 0, false
	}

	return uint(taskIDUint64), true
}

// getCurrentUserID 统一读取登录用户。
func getCurrentUserID(c *gin.Context) (uint, bool) {
	userIDRaw, exists := c.Get("userID")
	if !exists {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "please login first")
		return 0, false
	}

	userID, ok := userIDRaw.(uint)
	if !ok {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid user identity")
		return 0, false
	}

	return userID, true
}
