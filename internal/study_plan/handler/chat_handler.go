package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/study_plan/dto"
	"gojo/internal/study_plan/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (h *StudyPlanHandler) CreateSession(c *gin.Context) {
	var req dto.CreateChatSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	session, err := h.svc.CreateChatSession(c.Request.Context(), userID, req.Title)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "create study plan session failed")
		return
	}

	response.OK(c, session)
}

func (h *StudyPlanHandler) ListSessions(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	limit := parsePositiveIntQuery(c, "limit", 20)
	sessions, err := h.svc.ListChatSessions(c.Request.Context(), userID, limit)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "list study plan sessions failed")
		return
	}

	response.OK(c, gin.H{"items": sessions})
}

func (h *StudyPlanHandler) GetSession(c *gin.Context) {
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	session, err := h.svc.GetChatSession(c.Request.Context(), userID, sessionID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan sessions")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan session not found")
		}
		return
	}

	response.OK(c, session)
}

func (h *StudyPlanHandler) DeleteSession(c *gin.Context) {
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	if err := h.svc.ArchiveChatSession(c.Request.Context(), userID, sessionID); err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan sessions")
		case errors.Is(err, apperror.ErrChatSessionBusy):
			response.FailWithMessage(c, http.StatusConflict, ecode.InvalidParams, "session has a running reply and cannot be deleted right now")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan session not found")
		}
		return
	}

	response.OK(c, gin.H{
		"session_id": sessionID,
		"status":     model.ChatSessionStatusArchived,
	})
}

func (h *StudyPlanHandler) ListMessages(c *gin.Context) {
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	limit := parsePositiveIntQuery(c, "limit", 0)

	messages, err := h.svc.GetChatMessages(c.Request.Context(), userID, sessionID, limit)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan sessions")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan session not found")
		}
		return
	}

	response.OK(c, gin.H{"items": messages})
}

func (h *StudyPlanHandler) SendMessage(c *gin.Context) {
	sessionID, ok := parseSessionID(c)
	if !ok {
		return
	}

	var req dto.SendChatMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	turn, err := h.svc.SendChatMessage(c.Request.Context(), userID, sessionID, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan sessions")
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan session not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "send study plan chat message failed")
		}
		return
	}

	response.OK(c, gin.H{
		"turn_id":    turn.ID,
		"status":     turn.Status,
		"model":      turn.Model,
		"session_id": turn.SessionID,
	})
}

func (h *StudyPlanHandler) GetTurn(c *gin.Context) {
	turnID, ok := parseTurnID(c)
	if !ok {
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	turn, err := h.svc.GetChatTurn(c.Request.Context(), userID, turnID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan turns")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan turn not found")
		}
		return
	}

	response.OK(c, turn)
}

func (h *StudyPlanHandler) StreamTurn(c *gin.Context) {
	turnID, ok := parseTurnID(c)
	if !ok {
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	turn, err := h.svc.GetChatTurn(c.Request.Context(), userID, turnID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' study plan turns")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "study plan turn not found")
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

	lastSnapshot, err := writeTurnEvent(c, turn)
	if err != nil {
		return
	}
	flusher.Flush()

	if isTerminalStudyPlanStatus(turn.Status) {
		return
	}

	turnTicker := time.NewTicker(time.Second)
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer turnTicker.Stop()
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
		case <-turnTicker.C:
			latestTurn, err := h.svc.GetChatTurn(c.Request.Context(), userID, turnID)
			if err != nil {
				return
			}
			currentSnapshot, err := snapshotTurn(latestTurn)
			if err != nil {
				return
			}
			if currentSnapshot != lastSnapshot {
				if _, err := writeTurnEvent(c, latestTurn); err != nil {
					return
				}
				flusher.Flush()
				lastSnapshot = currentSnapshot
			}
			if isTerminalStudyPlanStatus(latestTurn.Status) {
				return
			}
		}
	}
}

func parseSessionID(c *gin.Context) (uint, bool) {
	sessionIDStr := c.Param("session_id")
	sessionIDUint64, err := strconv.ParseUint(sessionIDStr, 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid session id")
		return 0, false
	}
	return uint(sessionIDUint64), true
}

func parseTurnID(c *gin.Context) (uint, bool) {
	turnIDStr := c.Param("turn_id")
	turnIDUint64, err := strconv.ParseUint(turnIDStr, 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid turn id")
		return 0, false
	}
	return uint(turnIDUint64), true
}

func writeTurnEvent(c *gin.Context, turn *model.ChatTurn) (string, error) {
	payload, err := snapshotTurn(turn)
	if err != nil {
		return "", err
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", payload); err != nil {
		return "", err
	}
	return payload, nil
}

func snapshotTurn(turn *model.ChatTurn) (string, error) {
	payload, err := json.Marshal(turn)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}
