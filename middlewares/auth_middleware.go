package middlewares

import (
	"gojo/pkg/jwt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware 是验证 JWT 的保安
func AuthMiddleware() gin.HandlerFunc {

	return func(c *gin.Context) {
		var tokenString string
		// 1. 行业规范：前端发请求时，必须把 Token 放在 HTTP 头部的 "Authorization" 字段里
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// 🚪 走大门 (Header)：必须严格遵守 "Bearer xxx" 的行业规范
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "安检失败：Header 手环格式不合法"})
				c.Abort()
				return
			}
			// 拿到被切开的后半段真身
			tokenString = parts[1]
		} else {
			// 🚪 走小门 (WebSocket 的 URL)：没有 Bearer 前缀，拿到的直接就是真身
			tokenString = c.Query("token")
		}

		// 3. 呼叫工具部门，把 token 塞进验钞机
		claims, err := jwt.ParseToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "安检失败：手环无效或已过期，请重新登录"})
			c.Abort() // 驱逐！
			return
		}

		// 4. 【高阶操作】查验通过！把 user_id 贴在顾客的后背上！
		// 注意：JSON 解析数字时默认是浮点数(float64)，我们需要强转成 uint 给数据库用
		val, ok := (*claims)["user_id"].(float64)
		if !ok {
			// 只要 ok 是 false，说明遇到了脏数据，直接把请求踢出去，绝不给它变成 0 并在系统里乱跑的机会！
			c.JSON(401, gin.H{"error": "数据异常"})
			return
		}
		userID := uint(val)
		username, ok := (*claims)["username"].(string)
		if !ok {
			c.JSON(401, gin.H{"error": "用户名数据异常"})
			c.Abort()
			return
		}
		role, ok := (*claims)["role"].(float64)
		if !ok {
			// 只要 ok 是 false，说明遇到了脏数据，直接把请求踢出去，绝不给它变成 0 并在系统里乱跑的机会！
			c.JSON(401, gin.H{"error": "数据异常"})
			return
		}

		// c.Set 就是往当前这次请求的上下文中存数据
		c.Set("userID", userID)
		c.Set("username", username)
		c.Set("role", uint(role)) // 👈 加上这句，把阶级标签贴在他后背上
		// 5. 恭喜你，放行！
		c.Next()
	}
}
