package middlewares

import (
	"strings"

	"github.com/gin-gonic/gin"

	"gojo/internal/user/model"
	"gojo/internal/user/repository"
	"gojo/pkg/jwt"
)

func OptionalAuth() gin.HandlerFunc {
	userRepo := repository.NewUserRepository()

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		claims, err := jwt.ParseToken(parts[1], jwt.TokenTypeAccess)
		if err != nil {
			c.Next()
			return
		}

		user, err := userRepo.GetUserAuthByID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.Next()
			return
		}

		if user.Status != model.UserStatusActive || user.TokenVersion != claims.TokenVersion {
			c.Next()
			return
		}

		c.Set("userID", user.ID)
		c.Set("username", user.Username)
		c.Set("role", uint(user.Role))
		c.Next()
	}
}
