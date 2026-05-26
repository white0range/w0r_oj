package middlewares

import (
	"gojo/pkg/jwt"
	"strings"

	"github.com/gin-gonic/gin"
)

// OptionalAuth 柔性鉴权中间件
// 功能：尝试解析 Token。如果有且有效，就把 userID 塞进上下文；如果没有或无效，当作游客直接放行。
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// 1. 如果没有传 Token，直接放行（当游客）
		if authHeader == "" {
			c.Next()
			return
		}

		// 2. 如果传了，尝试按规范截取
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			// 3. 尝试解析 Token
			claims, err := jwt.ParseToken(parts[1])
			if err == nil {
				// 💥 只有当 Token 完全合法时，才把身份信息贴在上下文中
				// 注意：这里 claims 里的字段名取决于你具体是怎么写 JWT 的
				if userIDAny, ok := (*claims)["user_id"]; ok {
					c.Set("userID", uint(userIDAny.(float64)))
				}
			}
			// 🚨 极其关键的细节：如果 err != nil (比如 Token 过期或伪造)
			// 我们【绝对不】执行 c.Abort()，而是当作没看见，让他以游客身份继续访问！
		}

		// 4. 交接给下一个处理函数 (Controller)
		c.Next()
	}
}
