package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/submission/dto"
	"gojo/internal/submission/service"
)

type SubmissionHandler struct {
	svc *service.SubmissionService
}

func NewSubmissionHandler(svc *service.SubmissionService) *SubmissionHandler {
	return &SubmissionHandler{svc: svc}
}

func (h *SubmissionHandler) SubmitCode(c *gin.Context) {
	var req dto.SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "missing problem_id, language, or code")
		return
	}

	userIDRaw, exists := c.Get("userID")
	if !exists {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "missing current user")
		return
	}

	userID, ok := userIDRaw.(uint)
	if !ok {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid user identity")
		return
	}

	submission, err := h.svc.SubmitCode(c.Request.Context(), userID, req)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "submit code failed")
		return
	}

	response.OK(c, gin.H{
		"submission_id": submission.ID,
		"status":        "Pending",
	})
}

func (h *SubmissionHandler) GetSubmissionResult(c *gin.Context) {
	id := c.Param("id")

	userIDRaw, exists := c.Get("userID")
	if !exists {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "please login first")
		return
	}

	currentUserID, ok := userIDRaw.(uint)
	if !ok {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "invalid user identity")
		return
	}

	submission, err := h.svc.GetSubmissionResult(c.Request.Context(), id, currentUserID)
	if err != nil {
		if errors.Is(err, apperror.ErrUnauthorizedAccess) {
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' submissions")
			return
		}
		response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "submission not found")
		return
	}

	response.OK(c, gin.H{
		"submission_id": submission.ID,
		"problem_id":    submission.ProblemID,
		"status":        submission.Status,
		"actual_output": submission.ActualOutput,
		"code":          submission.Code,
		"language":      submission.Language,
	})
}

func (h *SubmissionHandler) GetMySubmissions(c *gin.Context) {
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

	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "20")
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)

	res, err := h.svc.GetMySubmissions(c.Request.Context(), userID, page, limit)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get submissions failed")
		return
	}

	response.OK(c, res)
}
