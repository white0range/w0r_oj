package handler

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

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

	token, err := h.svc.LoginUser(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, apperror.ErrUserNotFound) || errors.Is(err, apperror.ErrWrongPassword) {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid username or password")
			return
		}
		response.FailWithMessage(c, http.StatusInternalServerError, ecode.InternalError, "login failed")
		return
	}

	response.OK(c, gin.H{
		"token": token,
	})
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
