package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	gorillaWS "github.com/gorilla/websocket"

	"gojo/config"
	"gojo/infrastructure/websocket"
	"gojo/internal/app/apperror"
	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/user/dto"
	"gojo/internal/user/service"
)

type UserHandler struct {
	svc *service.UserService
}

const refreshTokenCookieName = "gojo_refresh_token"

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) Register(c *gin.Context) {
	var req dto.UserAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	if err := h.svc.RegisterUser(c.Request.Context(), req); err != nil {
		if errors.Is(err, apperror.ErrUsernameExists) {
			response.FailWithMessage(c, http.StatusConflict, ecode.Conflict, "username already exists")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "register failed")
		return
	}

	response.OK(c, nil)
}

func (h *UserHandler) Login(c *gin.Context) {
	var req dto.UserAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	tokens, err := h.svc.LoginUser(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrUserNotFound), errors.Is(err, apperror.ErrWrongPassword):
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid username or password")
		case errors.Is(err, apperror.ErrUserBanned):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "account has been banned")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "login failed")
		}
		return
	}

	writeRefreshTokenCookie(c, tokens.RefreshToken)
	response.OK(c, gin.H{
		"access_token": tokens.AccessToken,
	})
}

func (h *UserHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshTokenCookieName)
	if err != nil || refreshToken == "" {
		clearRefreshTokenCookie(c)
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "missing refresh token")
		return
	}

	tokens, newRefreshToken, err := h.svc.RefreshSession(c.Request.Context(), refreshToken)
	if err != nil {
		clearRefreshTokenCookie(c)
		switch {
		case errors.Is(err, apperror.ErrInvalidToken):
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid refresh token")
		case errors.Is(err, apperror.ErrUserBanned):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "account has been banned")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "refresh session failed")
		}
		return
	}

	writeRefreshTokenCookie(c, newRefreshToken)
	response.OK(c, gin.H{
		"access_token": tokens.AccessToken,
	})
}

func (h *UserHandler) Logout(c *gin.Context) {
	refreshToken, _ := c.Cookie(refreshTokenCookieName)
	if refreshToken != "" {
		_ = h.svc.LogoutSession(c.Request.Context(), refreshToken)
	}

	clearRefreshTokenCookie(c)
	response.OK(c, nil)
}

func (h *UserHandler) GetProfile(c *gin.Context) {
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

	profile, err := h.svc.GetUserProfile(c.Request.Context(), userID)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get profile failed")
		return
	}

	response.OK(c, profile)
}

func (h *UserHandler) AdminListUsers(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	items, err := h.svc.ListUsers(c.Request.Context(), c.Query("keyword"), limit)
	if err != nil {
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "get users failed")
		return
	}

	response.OK(c, gin.H{
		"items": items,
	})
}

func (h *UserHandler) AdminBanUser(c *gin.Context) {
	targetID, ok := parseUserID(c)
	if !ok {
		return
	}

	actorID, ok := currentUserID(c)
	if !ok {
		return
	}

	var req dto.BanUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, ecode.InvalidParams)
		return
	}

	if err := h.svc.BanUser(c.Request.Context(), actorID, targetID, req.Reason); err != nil {
		switch {
		case errors.Is(err, apperror.ErrUserNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "user not found")
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot ban this account")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "ban user failed")
		}
		return
	}

	disconnectUserRealtime(targetID)
	response.OK(c, gin.H{"user_id": targetID, "status": "banned"})
}

func (h *UserHandler) AdminUnbanUser(c *gin.Context) {
	targetID, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := h.svc.UnbanUser(c.Request.Context(), targetID); err != nil {
		switch {
		case errors.Is(err, apperror.ErrUserNotFound):
			response.FailWithMessage(c, http.StatusNotFound, ecode.NotFound, "user not found")
		case errors.Is(err, apperror.ErrForbidden):
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "cannot unban this account")
		default:
			response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "unban user failed")
		}
		return
	}

	response.OK(c, gin.H{"user_id": targetID, "status": "active"})
}

func (h *UserHandler) ConnectWS(c *gin.Context) {
	userIDAny, exists := c.Get("userID")
	if !exists {
		response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "unauthorized websocket request")
		return
	}

	userID := fmt.Sprintf("%v", userIDAny)
	conn, err := websocket.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("websocket upgrade failed:", err)
		return
	}

	websocket.WsClients.Store(userID, conn)
	defer func() {
		conn.Close()
		websocket.WsClients.Delete(userID)
	}()

	for {
		if _, _, readErr := conn.ReadMessage(); readErr != nil {
			break
		}
	}
}

func parseUserID(c *gin.Context) (uint, bool) {
	userIDUint64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.FailWithMessage(c, http.StatusBadRequest, ecode.InvalidParams, "invalid user id")
		return 0, false
	}

	return uint(userIDUint64), true
}

func currentUserID(c *gin.Context) (uint, bool) {
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

func disconnectUserRealtime(userID uint) {
	if client, ok := websocket.WsClients.Load(fmt.Sprintf("%d", userID)); ok {
		conn, ok := client.(*gorillaWS.Conn)
		if ok {
			conn.Close()
		}
		websocket.WsClients.Delete(fmt.Sprintf("%d", userID))
	}
}

func writeRefreshTokenCookie(c *gin.Context, token string) {
	cookie := &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    token,
		Path:     "/api",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   config.GlobalConfig.App.Env != "dev",
		MaxAge:   refreshCookieMaxAge(),
		Expires:  time.Now().Add(time.Duration(refreshCookieMaxAge()) * time.Second),
	}
	http.SetCookie(c.Writer, cookie)
}

func clearRefreshTokenCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     "/api",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   config.GlobalConfig.App.Env != "dev",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func refreshCookieMaxAge() int {
	hours := config.GlobalConfig.JWT.RefreshTTLHours
	if hours <= 0 {
		hours = 168
	}
	return hours * 3600
}
