package worker

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/internal/chat/dto"
	"gojo/internal/chat/model"
	"gojo/internal/chat/repository"
	chatService "gojo/internal/chat/service"
	userModel "gojo/internal/user/model"
	jwtPkg "gojo/pkg/jwt"

	"gorm.io/gorm"
)

type chatAgentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatAgentRequest struct {
	UserID         uint               `json:"user_id"`
	Goal           string             `json:"goal"`
	SessionSummary string             `json:"session_summary,omitempty"`
	Messages       []chatAgentMessage `json:"messages,omitempty"`
}

type chatAgentResponse struct {
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

type sessionSummaryRequest struct {
	ExistingSummary string             `json:"existing_summary,omitempty"`
	Messages        []chatAgentMessage `json:"messages"`
}

type sessionSummaryResponse struct {
	Message string `json:"message"`
	Summary string `json:"summary"`
}

type ChatWorker struct {
	repo         repository.ChatRepository
	httpClient   *http.Client
	agentBaseURL string
	serviceUser  *userModel.User
	serviceToken string
}

func NewChatWorker(repo repository.ChatRepository) (*ChatWorker, error) {
	serviceUser, err := loadChatAgentUser(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load chat agent user failed: %w", err)
	}

	timeoutSeconds := config.GlobalConfig.Chat.AgentTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}

	serviceToken := strings.TrimSpace(config.GlobalConfig.Chat.AgentServiceToken)
	if serviceToken == "" {
		return nil, errors.New("chat.agent_service_token must be configured")
	}

	agentBaseURL := strings.TrimRight(config.GlobalConfig.Chat.AgentBaseURL, "/")
	if agentBaseURL == "" {
		agentBaseURL = "http://localhost:8000"
	}

	return &ChatWorker{
		repo: repo,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		agentBaseURL: agentBaseURL,
		serviceUser:  serviceUser,
		serviceToken: serviceToken,
	}, nil
}

func loadChatAgentUser(ctx context.Context) (*userModel.User, error) {
	var user userModel.User
	err := mysql.DB.WithContext(ctx).
		Where("role = ? AND status = ?", 1, userModel.UserStatusActive).
		Order("id asc").
		First(&user).Error
	if err == nil {
		if user.TokenVersion <= 0 {
			return nil, fmt.Errorf("admin user %d has invalid token_version=%d", user.ID, user.TokenVersion)
		}
		return &user, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("no active admin user found; please create or enable an admin account first")
	}
	return nil, err
}

func (w *ChatWorker) callchatAgentWithPayload(ctx context.Context, reqBody chatAgentRequest) (string, error) {
	respBody, err := w.postAgentJSON(ctx, "/chat/run", reqBody)
	if err != nil {
		return "", err
	}

	var agentResp chatAgentResponse
	if err := json.Unmarshal(respBody, &agentResp); err != nil {
		return "", fmt.Errorf("unmarshal chat agent response failed: %w", err)
	}

	if len(agentResp.Result) == 0 {
		return "", fmt.Errorf("chat agent returned empty result")
	}

	return string(agentResp.Result), nil
}

func (w *ChatWorker) summarizeSessionMessages(ctx context.Context, existingSummary string, messages []chatAgentMessage) (string, error) {
	respBody, err := w.postAgentJSON(ctx, "/chat/summarize-session", sessionSummaryRequest{
		ExistingSummary: existingSummary,
		Messages:        messages,
	})
	if err != nil {
		return "", err
	}

	var summaryResp sessionSummaryResponse
	if err := json.Unmarshal(respBody, &summaryResp); err != nil {
		return "", fmt.Errorf("unmarshal session summary response failed: %w", err)
	}

	summary := strings.TrimSpace(summaryResp.Summary)
	if summary == "" {
		return "", fmt.Errorf("chat agent returned empty session summary")
	}
	return summary, nil
}

func (w *ChatWorker) postAgentJSON(ctx context.Context, path string, payload any) ([]byte, error) {
	agentToken, err := jwtPkg.GenerateToken(w.serviceUser)
	if err != nil {
		return nil, fmt.Errorf("generate chat agent token failed: %w", err)
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal chat agent request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		w.agentBaseURL+path,
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("create chat agent request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+agentToken)
	req.Header.Set("X-Agent-Service-Token", w.serviceToken)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do chat agent request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read chat agent response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("chat agent returned status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return respBody, nil
}

const (
	chatContextWindowSize      = 8
	chatTurnLeaseDuration      = 5 * time.Minute
	chatTurnLeaseRenewInterval = 30 * time.Second
	chatTurnRecoveryInterval   = 15 * time.Second
	chatTurnDispatchTTL        = 5 * time.Minute
)

func (w *ChatWorker) StartTurnWorkerPool(workerCount int) {
	log.Printf("starting chat turn worker pool, workers=%d\n", workerCount)
	go w.recoverDispatchableTurns(context.Background())
	for i := 1; i <= workerCount; i++ {
		go w.runTurnWorker(i)
	}
}

func (w *ChatWorker) chatRepo() (repository.ChatRepository, error) {
	repo, ok := w.repo.(repository.ChatRepository)
	if !ok {
		return nil, fmt.Errorf("chat repository not configured")
	}
	return repo, nil
}

func (w *ChatWorker) ProcessTurn(ctx context.Context, turnID uint) error {
	repo, err := w.chatRepo()
	if err != nil {
		return err
	}

	processingToken, err := newProcessingToken()
	if err != nil {
		return fmt.Errorf("generate chat turn processing token: %w", err)
	}
	turn, claimed, err := repo.ClaimTurn(ctx, turnID, processingToken, time.Now().Add(chatTurnLeaseDuration))
	if err != nil {
		return fmt.Errorf("claim chat turn failed: %w", err)
	}
	if !claimed {
		return nil
	}

	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go w.renewTurnLease(processCtx, cancel, repo, turnID, processingToken)

	fail := func(cause error) error {
		failed, failErr := repo.FailClaimedTurn(context.Background(), turnID, processingToken, cause.Error(), time.Now())
		if failErr != nil {
			return fmt.Errorf("%w; mark chat turn failed: %v", cause, failErr)
		}
		if !failed {
			return fmt.Errorf("%w; chat turn claim was lost", cause)
		}
		return cause
	}

	session, archivedChunk, err := repo.PrepareSessionCompaction(processCtx, turn.SessionID, chatContextWindowSize)
	if err != nil {
		return fail(fmt.Errorf("prepare session compaction failed: %w", err))
	}

	if len(archivedChunk) > 0 {
		mergedSummary, summaryErr := w.summarizeSessionMessages(processCtx, session.SummaryText, toAgentMessages(archivedChunk))
		if summaryErr != nil {
			log.Printf("chat turn %d llm session summary failed, fallback to rule summary: %v", turn.ID, summaryErr)
			mergedSummary = repository.BuildCompactSessionSummary(session.SummaryText, archivedChunk)
		}

		session, err = repo.ApplySessionCompaction(processCtx, turn.SessionID, chatMessageIDs(archivedChunk), mergedSummary)
		if err != nil {
			return fail(fmt.Errorf("apply session compaction failed: %w", err))
		}
	}

	messages, err := repo.ListRecentMessagesBySessionID(processCtx, turn.SessionID, chatContextWindowSize)
	if err != nil {
		return fail(fmt.Errorf("load session messages failed: %w", err))
	}

	payload := chatAgentRequest{
		UserID:         turn.UserID,
		SessionSummary: strings.TrimSpace(session.SummaryText),
		Messages:       toAgentMessages(messages),
	}
	resultJSON, err := w.callchatAgentWithPayload(processCtx, payload)
	if err != nil {
		return fail(fmt.Errorf("call chat agent failed: %w", err))
	}

	assistantContent := chatService.FormatChatAssistantContentForWorker(resultJSON)
	if _, completed, err := repo.CompleteClaimedTurn(processCtx, turn.ID, processingToken, assistantContent, resultJSON, time.Now()); err != nil {
		return fail(fmt.Errorf("complete chat turn failed: %w", err))
	} else if !completed {
		return nil
	}

	return nil
}

func newProcessingToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (w *ChatWorker) renewTurnLease(ctx context.Context, cancel context.CancelFunc, repo repository.ChatRepository, turnID uint, processingToken string) {
	ticker := time.NewTicker(chatTurnLeaseRenewInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			renewed, err := repo.RenewTurnLease(context.Background(), turnID, processingToken, time.Now().Add(chatTurnLeaseDuration))
			if err != nil || !renewed {
				log.Printf("chat turn %d lease renewal failed: %v", turnID, err)
				cancel()
				return
			}
			_ = cache.Rdb.Set(context.Background(), repository.ChatTurnDispatchPrefix+fmt.Sprintf("%d", turnID), "1", chatTurnDispatchTTL).Err()
		}
	}
}
func runRoleForAgent(role string) string {
	switch role {
	case model.ChatMessageRoleAssistant:
		return model.ChatMessageRoleAssistant
	case model.ChatMessageRoleSystem:
		return model.ChatMessageRoleSystem
	default:
		return model.ChatMessageRoleUser
	}
}

func toAgentMessages(messages []model.ChatMessage) []chatAgentMessage {
	items := make([]chatAgentMessage, 0, len(messages))
	for _, message := range messages {
		items = append(items, chatAgentMessage{
			Role:    runRoleForAgent(message.Role),
			Content: strings.TrimSpace(message.Content),
		})
	}
	return items
}

func chatMessageIDs(messages []model.ChatMessage) []uint {
	ids := make([]uint, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return ids
}

func (w *ChatWorker) runTurnWorker(id int) {
	ctx := context.Background()
	for {
		payload, err := cache.Rdb.BRPopLPush(ctx, repository.ChatTurnQueueKey, repository.ChatTurnProcessingQueueKey, 0).Result()
		if err != nil {
			log.Printf("chat turn worker %d pop task failed: %v\n", id, err)
			continue
		}

		var task dto.ChatTurnQueueTask
		if err := json.Unmarshal([]byte(payload), &task); err != nil {
			_ = cache.Rdb.LRem(ctx, repository.ChatTurnProcessingQueueKey, 0, payload).Err()
			log.Printf("chat turn worker %d unmarshal task failed: %v\n", id, err)
			continue
		}

		log.Printf("chat turn worker %d processing turn_id=%d\n", id, task.TurnID)
		if err := w.ProcessTurn(ctx, task.TurnID); err != nil {
			log.Printf("chat turn worker %d process turn failed: %v\n", id, err)
		}
		_ = cache.Rdb.LRem(ctx, repository.ChatTurnProcessingQueueKey, 0, payload).Err()
		_ = cache.Rdb.Del(ctx, repository.ChatTurnDispatchPrefix+fmt.Sprintf("%d", task.TurnID)).Err()
	}
}

func (w *ChatWorker) recoverDispatchableTurns(ctx context.Context) {
	w.recoverDispatchableTurnsOnce(ctx)
	ticker := time.NewTicker(chatTurnRecoveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.recoverDispatchableTurnsOnce(ctx)
		}
	}
}

func (w *ChatWorker) recoverDispatchableTurnsOnce(ctx context.Context) {
	repo, err := w.chatRepo()
	if err != nil {
		log.Printf("chat turn recovery repository failed: %v", err)
		return
	}
	turnIDs, err := repo.ListDispatchableTurnIDs(ctx, time.Now(), 100)
	if err != nil {
		log.Printf("chat turn recovery database scan failed: %v", err)
		return
	}
	for _, turnID := range turnIDs {
		if err := w.enqueueTurn(ctx, turnID); err != nil {
			log.Printf("chat turn recovery enqueue turn_id=%d failed: %v", turnID, err)
		}
	}
}

func (w *ChatWorker) enqueueTurn(ctx context.Context, turnID uint) error {
	key := repository.ChatTurnDispatchPrefix + fmt.Sprintf("%d", turnID)
	queued, err := cache.Rdb.SetNX(ctx, key, "1", chatTurnDispatchTTL).Result()
	if err != nil || !queued {
		return err
	}

	payload, err := json.Marshal(dto.ChatTurnQueueTask{TurnID: turnID})
	if err != nil {
		_ = cache.Rdb.Del(ctx, key).Err()
		return err
	}
	if err := cache.Rdb.LPush(ctx, repository.ChatTurnQueueKey, payload).Err(); err != nil {
		_ = cache.Rdb.Del(ctx, key).Err()
		return err
	}
	return nil
}
