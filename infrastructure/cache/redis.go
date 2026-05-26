package cache

import (
	"context"
	"fmt"
	"gojo/config"
	"log"

	"github.com/redis/go-redis/v9"
)

// Rdb 是我们全局使用的 Redis 客户端实例
var Rdb *redis.Client

// Ctx 是全局上下文，Redis v9 版本强制要求所有的操作都必须带上 Context
var Ctx = context.Background()

// InitRedis 负责连接启动 Redis
func InitRedis() {
	Rdb = redis.NewClient(&redis.Options{
		Addr:     config.GlobalConfig.Redis.Addr,
		Password: config.GlobalConfig.Redis.Password,
		DB:       config.GlobalConfig.Redis.DB,
	})

	// 测试一下连通性 (发送 PING 看看有没有 PONG)
	_, err := Rdb.Ping(Ctx).Result()
	if err != nil {
		// 如果连不上，直接熔断退出，因为没有队列系统跑不起来！
		log.Fatalf("❌ 致命错误：无法连接到 Redis！请检查 Docker 容器是否启动。\n报错信息: %v", err)
	}

	fmt.Println("🎉 Redis 引擎连接成功！大坝已建立！")
}
