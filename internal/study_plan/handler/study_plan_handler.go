package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/study_plan/dto"
	"gojo/internal/study_plan/model"
	"gojo/internal/study_plan/service"

	"github.com/gin-gonic/gin"
)

type StudyPlanHandler struct {
	svc *service.StudyPlanService
}

func NewStudyPlanHandler(svc *service.StudyPlanService) *StudyPlanHandler {
	return &StudyPlanHandler{svc: svc}
}

// CreateTask 创建训练计划任务。
func (h *StudyPlanHandler) CreateTask(c *gin.Context) {
	var req dto.CreateStudyPlanTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	task, err := h.svc.CreateTask(c.Request.Context(), userID, req.Goal)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "create study plan task failed")
		return
	}

	response.OK(c, gin.H{
		"task_id": task.ID,
		"status":  task.Status,
		"goal":    task.Goal,
	})
}

// GetTask 查询训练计划任务详情。
func (h *StudyPlanHandler) GetTask(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), userID, taskID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan tasks")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan task not found")
		}
		return
	}

	response.OK(c, task)
}

func (h *StudyPlanHandler) StreamTask(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	task, err := h.svc.GetTask(c.Request.Context(), userID, taskID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan tasks")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan task not found")
		}
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "streaming is not supported")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)

	lastSnapshot, err := writeTaskEvent(c, task)
	if err != nil {
		return
	}
	flusher.Flush()

	if isTerminalStudyPlanStatus(task.Status) {
		return
	}

	taskTicker := time.NewTicker(time.Second)
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer taskTicker.Stop()
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeatTicker.C:
			if _, err := c.Writer.Write([]byte(": keep-alive\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case <-taskTicker.C:
			latestTask, err := h.svc.GetTask(c.Request.Context(), userID, taskID)
			if err != nil {
				return
			}

			currentSnapshot, err := snapshotTask(latestTask)
			if err != nil {
				return
			}

			if currentSnapshot != lastSnapshot {
				if _, err := writeTaskEvent(c, latestTask); err != nil {
					return
				}
				flusher.Flush()
				lastSnapshot = currentSnapshot
			}

			if isTerminalStudyPlanStatus(latestTask.Status) {
				return
			}
		}
	}
}

func (h *StudyPlanHandler) SubmitFeedback(c *gin.Context) {
	taskID, ok := parseTaskID(c)
	if !ok {
		return
	}

	var req dto.SubmitStudyPlanFeedbackRequest
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot submit feedback for other users' study plan tasks")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "submit study plan feedback failed")
		}
		return
	}

	response.OK(c, gin.H{
		"task_id": feedback.TaskID,
		"helpful": feedback.Helpful,
		"comment": feedback.Comment,
	})
}

func (h *StudyPlanHandler) GetFeedback(c *gin.Context) {
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan feedback")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan feedback not found")
		}
		return
	}

	response.OK(c, feedback)
}

func (h *StudyPlanHandler) GetAdminStats(c *gin.Context) {
	stats, err := h.svc.GetAdminStats(c.Request.Context())
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get study plan stats failed")
		return
	}

	response.OK(c, stats)
}

// GetUserACHistory 暴露给 future agent 的第一个内部数据接口。
// 这一版先挂在管理员路由下，方便你联调和观察返回结构。
func (h *StudyPlanHandler) GetUserACHistory(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	data, err := h.svc.GetUserACHistory(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrUserNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "user not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get user ac history failed")
		}
		return
	}

	response.OK(c, data)
}

// GetUserFailedSubmissions 返回用户最近失败的提交，作为训练计划 agent 的输入之一。
func (h *StudyPlanHandler) GetUserFailedSubmissions(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	limit := parsePositiveIntQuery(c, "limit", 10)

	data, err := h.svc.GetUserFailedSubmissions(c.Request.Context(), userID, limit)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get user failed submissions failed")
		return
	}

	response.OK(c, data)
}

// GetUserTagStats 返回用户在各标签下的练习统计，用于识别薄弱点。
func (h *StudyPlanHandler) GetUserTagStats(c *gin.Context) {
	userID, ok := parseUserID(c)
	if !ok {
		return
	}

	data, err := h.svc.GetUserTagStats(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrUserNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "user not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get user tag stats failed")
		}
		return
	}

	response.OK(c, data)
}

// GetCandidateProblems 返回候选题列表，后面可以给 Python agent 做候选集检索。
func (h *StudyPlanHandler) GetCandidateProblems(c *gin.Context) {
	requestedTags := parseCSVQuery(c, "tags")
	excludeIDs := parseUintCSVQuery(c, "exclude_ids")
	limit := parsePositiveIntQuery(c, "limit", 10)

	data, err := h.svc.GetCandidateProblems(c.Request.Context(), requestedTags, excludeIDs, limit)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get candidate problems failed")
		return
	}

	response.OK(c, data)
}

// GetProblemDetail 返回单题详情，方便 agent 获取更完整的题目上下文。
func (h *StudyPlanHandler) GetProblemDetail(c *gin.Context) {
	problemID, ok := parseProblemID(c)
	if !ok {
		return
	}

	data, err := h.svc.GetProblemDetail(c.Request.Context(), problemID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrProblemNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "problem not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get problem detail failed")
		}
		return
	}

	response.OK(c, data)
}

func parseTaskID(c *gin.Context) (uint, bool) {
	taskIDStr := c.Param("id")
	taskIDUint64, err := strconv.ParseUint(taskIDStr, 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid task id")
		return 0, false
	}

	return uint(taskIDUint64), true
}

func parseUserID(c *gin.Context) (uint, bool) {
	userIDStr := c.Param("id")
	userIDUint64, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid user id")
		return 0, false
	}

	return uint(userIDUint64), true
}

func parseProblemID(c *gin.Context) (uint, bool) {
	problemIDStr := c.Param("id")
	problemIDUint64, err := strconv.ParseUint(problemIDStr, 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid problem id")
		return 0, false
	}

	return uint(problemIDUint64), true
}

func parsePositiveIntQuery(c *gin.Context, key string, defaultValue int) int {
	valueStr := c.Query(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil || value <= 0 {
		return defaultValue
	}

	return value
}

func parseCSVQuery(c *gin.Context, key string) []string {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		items = append(items, part)
	}

	return items
}

func parseUintCSVQuery(c *gin.Context, key string) []uint {
	raw := c.Query(key)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	items := make([]uint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			continue
		}

		items = append(items, uint(value))
	}

	return items
}

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

func writeTaskEvent(c *gin.Context, task *model.StudyPlanTask) (string, error) {
	payload, err := snapshotTask(task)
	if err != nil {
		return "", err
	}

	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", payload); err != nil {
		return "", err
	}

	return payload, nil
}

func snapshotTask(task *model.StudyPlanTask) (string, error) {
	payload, err := json.Marshal(task)
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func isTerminalStudyPlanStatus(status string) bool {
	return status == model.TaskStatusSucceeded || status == model.TaskStatusFailed
}
