package middlewares

import (
	"fmt"
	"gojo/infrastructure/cache"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SubmitRateLimit 是专门针对代码提交接口的限流保安
func SubmitRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 获取玩家的唯一标识（这里用 IP 地址，如果你有 JWT，也可以换成解析出来的 UserID）
		//clientIP := c.ClientIP()
		userID, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "系统异常：无法获取当前用户身份"})
			return
		}

		// 2. 拼接他在 Redis 里的专属门牌号
		// 比如: rate_limit:submit:127.0.0.1
		redisKey := fmt.Sprintf("rate_limit:submit:%d", userID.(uint))

		// 3. 核心魔法：向 Redis 请求自增 1 (原子操作)
		// 如果 key 不存在，Redis 会自动创建并设为 1
		ctx := c.Request.Context()
		count, err := cache.Rdb.Incr(ctx, redisKey).Result()
		if err != nil {
			// 如果 Redis 挂了，安全起见直接拦截（或者你也可以放行）
			c.JSON(http.StatusInternalServerError, gin.H{"error": "系统繁忙，限流器异常"})
			c.Abort() // 🛑 极其关键：拦截请求，不再往下传递！
			return
		}

		// 4. 判决时刻
		if count == 1 {
			// 如果是 1，说明他是这 5 秒内的第一次请求！
			// 立刻给这个 Key 设置 5 秒的寿命。5 秒后它会自动销毁。
			cache.Rdb.Expire(ctx, redisKey, 5*time.Second)
		} else {
			// 如果大于 1，说明这个 Key 还没死（5秒还没过），他又来点提交了！
			// 直接一脚踢飞！
			c.JSON(http.StatusTooManyRequests, gin.H{ // 状态码 429
				"status":  "Error",
				"message": "手速太快啦！请 5 秒后再试！",
			})
			c.Abort() // 🛑 必须 Abort，否则请求还是会跑到 Controller 去！
			return
		}

		// 5. 检查通过，安检门放行！请求交接给你的 Controller
		c.Next()
	}
}

// AIRateLimit 是专门保护 AI 钱包的每日配额限流器
func AIRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDAny, exists := c.Get("userID")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "系统异常：无法获取当前用户身份"})
			c.Abort()
			return
		}

		// 🚨 小细节：最好显式断言为 uint，因为传给 Sprintf 的 %d 更安全
		userID := userIDAny.(uint)

		// 1. 获取当天的日期字符串 (例如 "2026-05-05")
		today := time.Now().Format("2006-01-02")

		// 2. 拼接带有日期的专属门牌号
		// 比如: rate_limit:ai:999:2026-05-05
		redisKey := fmt.Sprintf("rate_limit:ai:%d:%s", userID, today)

		ctx := c.Request.Context()

		// 3. 核心魔法：向 Redis 请求自增
		count, err := cache.Rdb.Incr(ctx, redisKey).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "系统繁忙，限流器异常"})
			c.Abort()
			return
		}

		// 4. 如果是今天第一次点击，给这个 Key 设置 24 小时的寿命（其实设到今晚12点更严谨，但24小时最简单）
		if count == 1 {
			cache.Rdb.Expire(ctx, redisKey, 24*time.Hour)
		}

		// 5. 判决时刻：假设每天最多免费呼叫 3 次
		if count > 3 {
			c.JSON(http.StatusTooManyRequests, gin.H{ // 状态码 429
				"error": "今日 AI 导师免费指导次数 (3/3) 已用尽，请明天再来复盘吧！",
			})
			c.Abort() // 🛑 无情拦截！绝对不给模型厂商送钱！
			return
		}

		// 放行
		c.Next()
	}
}
