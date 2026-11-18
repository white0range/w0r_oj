package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/request"
	"gojo/internal/app/response"
	"gojo/internal/chat/service"
)

// AgentToolHandler serves data tools authenticated by the Agent service middleware.
type AgentToolHandler struct {
	svc *service.ChatService
}

func NewAgentToolHandler(svc *service.ChatService) *AgentToolHandler {
	return &AgentToolHandler{svc: svc}
}

func (h *AgentToolHandler) GetExecutionContext(c *gin.Context) {
	response.OK(c, gin.H{"turn_id": c.GetUint("agentTurnID"), "user_id": c.GetUint("userID")})
}

func (h *AgentToolHandler) GetUserACHistory(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
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

func (h *AgentToolHandler) GetUserFailedSubmissions(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
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

func (h *AgentToolHandler) GetFailedSubmissionDetail(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	submissionID, ok := request.PathID(c, "submission_id", "invalid submission id")
	if !ok {
		return
	}

	data, err := h.svc.GetFailedSubmissionDetail(c.Request.Context(), userID, submissionID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' submissions")
		case errors.Is(err, apperror.ErrAlreadyAccepted):
			response.FailWithMessage(c, http.StatusConflict, ecode.InvalidParams, "accepted submissions are not failure-analysis targets")
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "submission or problem not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get failed submission detail failed")
		}
		return
	}

	response.OK(c, data)
}

func (h *AgentToolHandler) GetUserTagStats(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
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

func (h *AgentToolHandler) GetCandidateProblems(c *gin.Context) {
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

func (h *AgentToolHandler) GetProblemDetail(c *gin.Context) {
	problemID, ok := request.PathID(c, "id", "invalid problem id")
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
