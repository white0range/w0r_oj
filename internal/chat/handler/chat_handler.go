package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/chat/dto"
	"gojo/internal/chat/model"
	"gojo/internal/chat/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ChatHandler struct {
	svc *service.ChatService
}

func NewChatHandler(svc *service.ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}
func (h *ChatHandler) SubmitPlanFeedback(c *gin.Context) {
	turnID, ok := parseTurnID(c)
	if !ok {
		return
	}
	var req dto.SubmitChatPlanFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	feedback, err := h.svc.SubmitChatPlanFeedback(c.Request.Context(), userID, turnID, req.Helpful, req.Comment)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot submit feedback for another user's chat turn")
		case errors.Is(err, apperror.ErrChatSessionBusy):
			response.FailWithMessage(c, http.StatusConflict, ecode.InvalidParams, "feedback is available after the assistant reply is completed")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "submit chat plan feedback failed")
		}
		return
	}
	response.OK(c, feedback)
}

func (h *ChatHandler) GetPlanFeedback(c *gin.Context) {
	turnID, ok := parseTurnID(c)
	if !ok {
		return
	}
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}
	feedback, err := h.svc.GetChatPlanFeedback(c.Request.Context(), userID, turnID)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access feedback for another user's chat turn")
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "chat plan feedback not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get chat plan feedback failed")
		}
		return
	}
	response.OK(c, feedback)
}
func (h *ChatHandler) CreateSession(c *gin.Context) {
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
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "create chat session failed")
		return
	}

	response.OK(c, session)
}

func (h *ChatHandler) ListSessions(c *gin.Context) {
	userID, ok := getCurrentUserID(c)
	if !ok {
		return
	}

	limit := parsePositiveIntQuery(c, "limit", 20)
	sessions, err := h.svc.ListChatSessions(c.Request.Context(), userID, limit)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "list chat sessions failed")
		return
	}

	response.OK(c, gin.H{"items": sessions})
}

func (h *ChatHandler) GetSession(c *gin.Context) {
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' chat sessions")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "chat session not found")
		}
		return
	}

	response.OK(c, session)
}

func (h *ChatHandler) DeleteSession(c *gin.Context) {
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' chat sessions")
		case errors.Is(err, apperror.ErrChatSessionBusy):
			response.FailWithMessage(c, http.StatusConflict, ecode.InvalidParams, "session has a running reply and cannot be deleted right now")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "chat session not found")
		}
		return
	}

	response.OK(c, gin.H{
		"session_id": sessionID,
		"status":     model.ChatSessionStatusArchived,
	})
}

func (h *ChatHandler) ListMessages(c *gin.Context) {
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' chat sessions")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "chat session not found")
		}
		return
	}

	response.OK(c, gin.H{"items": messages})
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' chat sessions")
		case errors.Is(err, gorm.ErrRecordNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "chat session not found")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "send chat chat message failed")
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

func (h *ChatHandler) GetTurn(c *gin.Context) {
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' chat turns")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "chat turn not found")
		}
		return
	}

	response.OK(c, turn)
}

func (h *ChatHandler) StreamTurn(c *gin.Context) {
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
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot access other users' chat turns")
		default:
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "chat turn not found")
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

	if isTerminalChatStatus(turn.Status) {
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
			if isTerminalChatStatus(latestTurn.Status) {
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

// GetUserACHistory 閺嗘挳婀剁紒?future agent 閻ㄥ嫮顑囨稉鈧稉顏勫敶闁劍鏆熼幑顔藉复閸欙絻鈧?
// 鏉╂瑤绔撮悧鍫濆帥閹稿倸婀粻锛勬倞閸涙鐭鹃悽鍙樼瑓閿涘本鏌熸笟澶哥稑閼辨棁鐨熼崪宀冾潎鐎电喕绻戦崶鐐电波閺嬪嫨鈧?
func (h *ChatHandler) GetUserACHistory(c *gin.Context) {
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

// GetUserFailedSubmissions 鏉╂柨娲栭悽銊﹀煕閺堚偓鏉╂垵銇戠拹銉ф畱閹绘劒姘﹂敍灞肩稊娑撻缚顔勭紒鍐吀閸?agent 閻ㄥ嫯绶崗銉ょ娑撯偓閵?
func (h *ChatHandler) GetUserFailedSubmissions(c *gin.Context) {
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

// GetUserTagStats 鏉╂柨娲栭悽銊﹀煕閸︺劌鎮囬弽鍥╊劮娑撳娈戠紒鍐х瘎缂佺喕顓搁敍宀€鏁ゆ禍搴ょ槕閸掝偉鏉藉杈╁仯閵?
func (h *ChatHandler) GetUserTagStats(c *gin.Context) {
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

// GetCandidateProblems 鏉╂柨娲栭崐娆撯偓澶愵暯閸掓銆冮敍灞芥倵闂堛垹褰叉禒銉х舶 Python agent 閸嬫艾鈧瑩鈧娉﹀Λ鈧槐顫偓?
func (h *ChatHandler) GetCandidateProblems(c *gin.Context) {
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

// GetProblemDetail 鏉╂柨娲栭崡鏇㈩暯鐠囷附鍎忛敍灞炬煙娓?agent 閼惧嘲褰囬弴鏉戠暚閺佸娈戞０妯兼窗娑撳﹣绗呴弬鍥モ偓?
func (h *ChatHandler) GetProblemDetail(c *gin.Context) {
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

func isTerminalChatStatus(status string) bool {
	return status == model.TaskStatusSucceeded || status == model.TaskStatusFailed
}
