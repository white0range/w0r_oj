package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gojo/internal/app/ecode"
	"gojo/internal/app/response"
	"gojo/internal/user/model"
	"gojo/internal/user/repository"
	"gojo/pkg/jwt"
)

func AuthMiddleware() gin.HandlerFunc {
	userRepo := repository.NewUserRepository()

	return func(c *gin.Context) {
		tokenString := extractBearerToken(c)
		if tokenString == "" {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "missing access token")
			c.Abort()
			return
		}

		claims, err := jwt.ParseToken(tokenString, jwt.TokenTypeAccess)
		if err != nil {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "invalid or expired access token")
			c.Abort()
			return
		}

		user, err := userRepo.GetUserAuthByID(c.Request.Context(), claims.UserID)
		if err != nil {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "user session not found")
			c.Abort()
			return
		}

		if user.Status == model.UserStatusBanned {
			response.FailWithMessage(c, http.StatusForbidden, ecode.Forbidden, "account has been banned")
			c.Abort()
			return
		}

		if user.TokenVersion != claims.TokenVersion {
			response.FailWithMessage(c, http.StatusUnauthorized, ecode.Unauthorized, "session has expired")
			c.Abort()
			return
		}

		c.Set("userID", user.ID)
		c.Set("username", user.Username)
		c.Set("role", uint(user.Role))
		c.Next()
	}
}

func extractBearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
		return ""
	}

	return c.Query("token")
}
