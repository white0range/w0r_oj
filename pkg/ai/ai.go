package ai

import (
	"context"
	"fmt"
	"strings"

	"gojo/config"

	"github.com/sashabaranov/go-openai"
)

// AnalysisContext 汇总错题分析任务需要的上下文。
type AnalysisContext struct {
	Code               string
	Language           string
	ActualOutput       string
	ProblemTitle       string
	ProblemDescription string
	ProblemTags        []string
}

type AIProvider struct{}

func NewAIProvider() *AIProvider {
	return &AIProvider{}
}

// buildPrompt 统一组装分析任务的提示词。
func buildPrompt(analysisCtx AnalysisContext) (string, string) {
	systemPrompt := `你是一个极其严格但富有耐心的顶级算法竞赛导师。你的任务是帮助学生 Debug。
【绝对禁止】：你绝对不能直接给出正确代码，也不能代替用户重写代码。
【你的行为】：
1. 先结合题目要求理解这段代码本来应该做什么。
2. 用一句话指出代码的致命逻辑漏洞在哪里。
3. 构造一个极端测试用例（Input 和 Expected Output），让用户在这个用例下自己想通为什么报错。
4. 语气要简洁、专业。`

	tagText := "无"
	if len(analysisCtx.ProblemTags) > 0 {
		tagText = strings.Join(analysisCtx.ProblemTags, ", ")
	}

	userPrompt := fmt.Sprintf(
		"题目标题: %s\n题目描述: %s\n题目标签: %s\n语言: %s\n报错信息/状态: %s\n学生代码:\n```%s\n%s\n```\n请导师指点！",
		analysisCtx.ProblemTitle,
		analysisCtx.ProblemDescription,
		tagText,
		analysisCtx.Language,
		analysisCtx.ActualOutput,
		analysisCtx.Language,
		analysisCtx.Code,
	)

	return systemPrompt, userPrompt
}

func newClient() *openai.Client {
	apiKey := config.GlobalConfig.AI.APIKey

	clientConfig := openai.DefaultConfig(apiKey)
	clientConfig.BaseURL = config.GlobalConfig.AI.BaseURL

	return openai.NewClientWithConfig(clientConfig)
}

// AskAIWithContext 供异步 analysis worker 使用，返回完整分析结果。
func (p *AIProvider) AskAIWithContext(ctx context.Context, analysisCtx AnalysisContext) (string, error) {
	client := newClient()

	systemPrompt, userPrompt := buildPrompt(analysisCtx)

	req := openai.ChatCompletionRequest{
		Model: config.GlobalConfig.AI.Model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userPrompt},
		},
		Stream: false,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty ai response")
	}

	return resp.Choices[0].Message.Content, nil
}
