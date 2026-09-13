package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

// 【新增】定义一个全局可见的 Docker 客户端变量
var DockerClient *client.Client

// InitDockerClient 初始化并测试与 Docker 引擎的连接
func InitDockerClient() error {
	// 1. 亮出兵符：使用系统默认环境变量，创建一个 Docker 客户端
	// client.WithAPIVersionNegotiation() 是一个极其核心的防御机制：
	// 它会自动协商 API 版本，防止你的 Go SDK 版本和本机的 Docker Desktop 版本不匹配而报错。
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return fmt.Errorf("创建 Docker 客户端失败: %v", err)
	}

	// 2. 派个通信兵去 Ping 一下 Docker 引擎，看它是不是醒着
	// context.Background() 是 Go 并发编程里的标准“上下文”，暂时理解为这次请求的“生存环境”
	ping, err := cli.Ping(context.Background())
	if err != nil {
		return fmt.Errorf("无法连接到 Docker 引擎，请确认小鲸鱼已亮绿灯: %v", err)
	}

	// 3. 握手成功！
	fmt.Printf("✅ 成功连接到本地 Docker 引擎！当前 API 版本: %s\n", ping.APIVersion)

	DockerClient = cli
	return nil
}

// Close releases the Docker API client during shutdown.
func Close() error {
	if DockerClient == nil {
		return nil
	}
	return DockerClient.Close()
}
