package handler

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"gojo/config"
	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/judge/trial"
)

func (h *AgentToolHandler) RunTrial(c *gin.Context) {
	if !config.GlobalConfig.Judge.TrialEnabled {
		response.FailWithMessage(c, http.StatusServiceUnavailable, ecode.InternalError, "trial slot is disabled")
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	var request struct {
		SubmissionID uint    `json:"submission_id" binding:"required"`
		Input        *string `json:"input" binding:"required"`
	}
	// Allow for JSON escaping while keeping the request body bounded.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024)
	if err := c.ShouldBindJSON(&request); err != nil || request.Input == nil || len(*request.Input) > trial.MaxInputBytes {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}
	deadline := time.Now().Add(trial.Timeout)
	ctx, cancel := context.WithDeadline(c.Request.Context(), deadline)
	defer cancel()
	if _, err := trial.LoadSubmission(ctx, userID, request.SubmissionID); err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.Fail(c, http.StatusForbidden, ecode.Forbidden)
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.Fail(c, http.StatusNotFound, ecode.NotFound)
		case errors.Is(err, trial.ErrIneligible):
			response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, err.Error())
		default:
			response.Fail(c, http.StatusServiceUnavailable, ecode.InternalError)
		}
		return
	}

	turnID := c.GetUint("agentTurnID")
	task := trial.Task{
		TurnID: turnID, AttemptHash: trial.HashAttempt(c.GetHeader("X-Agent-Processing-Token")),
		SubmissionID: request.SubmissionID, Input: *request.Input, Deadline: deadline,
	}
	if err := trial.Enqueue(ctx, task); err != nil {
		switch {
		case errors.Is(err, trial.ErrBusy):
			response.FailWithMessage(c, http.StatusTooManyRequests, ecode.TooManyRequests, "trial queue is full")
		case errors.Is(err, trial.ErrUsed):
			response.FailWithMessage(c, http.StatusConflict, ecode.InvalidParams, "turn already requested a trial")
		default:
			response.Fail(c, http.StatusServiceUnavailable, ecode.InternalError)
		}
		return
	}
	log.Printf("[chat] turn_id=%d stage=trial_queued", turnID)
	result, err := trial.Wait(ctx, turnID)
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		log.Printf("[chat] turn_id=%d stage=trial_timeout", turnID)
		response.FailWithMessage(c, http.StatusGatewayTimeout, ecode.InternalError, "trial timed out")
	case err != nil:
		response.Fail(c, http.StatusServiceUnavailable, ecode.InternalError)
	case result.Status == "stale":
		response.Fail(c, http.StatusForbidden, ecode.AgentExecutionExpired)
	default:
		response.OK(c, result)
	}
}
