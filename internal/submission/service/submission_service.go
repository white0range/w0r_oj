package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"gojo/internal/submission/dto" // 建议把 MySubmissionsResponse 移到这
	"gojo/internal/submission/model"
	"gojo/internal/submission/repository"

	"github.com/sashabaranov/go-openai"
)

type AIProvider interface {
	// 就像你注释里写的，一定要把 ctx 传给 AI！
	AskAIStream(ctx context.Context, code, lang, output string) (*openai.ChatCompletionStream, error)
}

// 1. 结构体注入
type SubmissionService struct {
	repo       repository.SubmissionRepository
	aiProvider AIProvider // 👈 注入 AI 外交官
}

// 2. 实例化方法
func NewSubmissionService(r repository.SubmissionRepository, ai AIProvider) *SubmissionService {
	return &SubmissionService{repo: r, aiProvider: ai}
}

// SubmitCode 将提交记录落库，并推送到 Redis 判题队列
func (s *SubmissionService) SubmitCode(ctx context.Context, userID uint, req dto.SubmitRequest) (*model.Submission, error) {
	submission := model.Submission{
		UserID:    userID,
		ProblemID: req.ProblemID,
		Language:  req.Language,
		Code:      req.Code,
	}

	// 呼叫仓管存数据库
	if err := s.repo.CreateSubmission(ctx, &submission); err != nil {
		return nil, fmt.Errorf("入库失败: %w", err)
	}

	task := map[string]interface{}{
		"user_id":       userID,
		"submission_id": submission.ID,
		"problem_id":    req.ProblemID,
		"code":          req.Code,
	}
	taskBytes, err := json.Marshal(task)
	if err != nil {
		return nil, fmt.Errorf("任务序列化失败: %w", err)
	}

	// 呼叫仓管推队列
	if err := s.repo.PushToJudgeQueue(ctx, taskBytes); err != nil {
		submission.Status = "judge_failed"
		if err2 := s.repo.UpdateSubmissionStatus(ctx, submission.ID, submission.Status); err2 != nil {
			return nil, fmt.Errorf("更新状态失败: %w, 推送判题队列失败: %w", err2, err)
		}
		return nil, fmt.Errorf("推送判题队列失败: %w", err)
	}

	return &submission, nil
}

// GetSubmissionResult 获取提交记录（带水平越权校验）
func (s *SubmissionService) GetSubmissionResult(ctx context.Context, submissionID string, currentUserID uint) (*model.Submission, error) {
	submission, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	// 🛡️ 架构师防御：水平越权校验 (业务逻辑，完美留在 Service)
	if submission.UserID != currentUserID {
		return nil, errors.New("unauthorized access")
	}

	return submission, nil
}

// GetMySubmissions 获取指定用户的提交历史
func (s *SubmissionService) GetMySubmissions(ctx context.Context, userID uint, page, limit int) (*dto.MySubmissionsResponse, error) {
	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// 直接找仓管拿现成的总数和列表！不用自己算 count 和 scopes 了！
	total, items, err := s.repo.GetSubmissionsByUserID(ctx, userID, page, limit)
	if err != nil {
		return nil, err
	}

	return &dto.MySubmissionsResponse{
		Page:  page,
		Limit: limit,
		Total: total,
		Items: items,
	}, nil
}

// GetAIAssistanceStream 负责核心业务校验，并向大模型发起连接
func (s *SubmissionService) GetAIAssistanceStream(ctx context.Context, submissionID string, userID uint) (*openai.ChatCompletionStream, error) {
	// 1. 呼叫仓管
	submission, err := s.repo.GetSubmissionByID(ctx, submissionID)
	if err != nil {
		return nil, errors.New("not_found")
	}

	// 2. 🛡️ 安全边界校验
	if submission.UserID != userID {
		return nil, errors.New("forbidden")
	}

	// 3. 🚦 业务规则校验
	if submission.Status == "AC" {
		return nil, errors.New("already_ac")
	}

	// 4. 召唤 AI 外交官！彻底切断了与 `ai` 包的强耦合！
	// 💡 注意：这里把 ctx 传下去了，极大地增强了取消控制
	stream, err := s.aiProvider.AskAIStream(ctx, submission.Code, submission.Language, submission.ActualOutput)
	if err != nil {
		return nil, errors.New("ai_connect_failed")
	}

	return stream, nil
}

// GetACProblemIDsByUserID 充当跨域外交官！
// 这个方法名、参数和返回值，完美契合了 User 模块的 SubmissionProvider 接口
func (s *SubmissionService) GetACProblemIDsByUserID(ctx context.Context, userID uint) ([]uint, error) {
	// 没有任何废话，直接呼叫自己的仓管去查！
	// 因为仓管返回的正好就是 ([]uint, error)，直接 return 即可。
	return s.repo.GetACProblemIDsByUserID(ctx, userID)
}
