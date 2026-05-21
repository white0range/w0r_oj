package ai

import (
	"context"
	"fmt"
	"gojo/config"

	"github.com/sashabaranov/go-openai"
)

type AIProvider struct{}

func NewAIProvider() *AIProvider {
	return &AIProvider{}
}

// AskAIStream 呼叫 AI 导师 (流式版)
// 注意返回值：我们不再返回拼接好的 string，而是直接把 openai 的“水龙头 (Stream)”返回出去！
func (p *AIProvider) AskAIStream(ctx context.Context, code string, language string, actualOutput string) (*openai.ChatCompletionStream, error) {
	// 1. 替换为你的真实 API Key
	apiKey := config.AppConfig.AI.APIKey

	// 2. 极其关键：把默认的 OpenAI 网址改成 DeepSeek 的网址
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.deepseek.com" // 注意：各个平台可能后缀不同，通常是这个或加上 /v1

	client := openai.NewClientWithConfig(config)

	// 3. 注入灵魂：System Prompt (系统提示词)
	systemPrompt := `你是一个极其严厉但富有耐心的顶级算法竞赛导师。
你的任务是帮助学生 Debug。
【绝对禁止】：你绝对不能直接给出正确的代码，也不能代替用户重写代码。
【你的行为】：
1. 用一句话指出代码的致命逻辑漏洞在哪里。
2. 构造一个极端测试用例（Input 和 Expected Output），让用户在这个用例下自己想通为什么报错。
3. 语气要简洁、高冷、专业。`

	// 4. 案发现场：User Prompt (用户代码和报错信息)
	userPrompt := fmt.Sprintf("语言: %s\n报错信息/状态: %s\n学生代码:\n```%s\n%s\n```\n请导师指点！",
		language, actualOutput, language, code)

	// 5. 发起网络请求
	req := openai.ChatCompletionRequest{
		//Model: "deepseek-chat", // 使用的模型名字 (注意查看 DeepSeek 文档最新的模型名)
		Model: "deepseek-chat",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Stream: true, // 💥 极其关键：开启流式模式
	}
	// 直接把这个“流式连接”返回给调用方 (Controller)
	return client.CreateChatCompletionStream(ctx, req)
}
