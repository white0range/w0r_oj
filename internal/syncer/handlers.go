package syncer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	problemModel "gojo/internal/problem/model"
	problemRepo "gojo/internal/problem/repository"
	userModel "gojo/internal/user/model"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const leaderboardKey = "leaderboard:infrastructure"

type esHandler struct {
	problems problemRepo.ProblemRepository
	search   problemRepo.ProblemSearchRepository
}

func (h *esHandler) Handle(ctx context.Context, task Task) error {
	switch task.Action {
	case ActionProblemUpsert:
		problem, err := h.problems.GetProblemByID(ctx, strconv.FormatUint(uint64(task.EntityID), 10))
		if err != nil {
			return err
		}
		return h.search.UpsertProblemToES(ctx, buildESProblemDoc(problem))
	case ActionProblemDelete:
		return h.search.DeleteProblemFromES(ctx, task.EntityID)
	default:
		return fmt.Errorf("unsupported es action %q", task.Action)
	}
}

type ragHandler struct{}

func (h *ragHandler) Handle(ctx context.Context, task Task) error {
	var path string
	switch task.Action {
	case ActionProblemUpsert:
		path = "/rag/problems/sync"
	case ActionProblemDelete:
		path = "/rag/problems/delete"
	default:
		return fmt.Errorf("unsupported rag action %q", task.Action)
	}

	body, err := json.Marshal(struct {
		ProblemID uint `json:"problem_id"`
	}{ProblemID: task.EntityID})
	if err != nil {
		return err
	}

	baseURL := strings.TrimRight(config.GlobalConfig.Chat.AgentBaseURL, "/")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Agent-Service-Token", config.GlobalConfig.Chat.AgentServiceToken)

	timeout := config.GlobalConfig.Chat.AgentTimeoutSeconds
	if timeout <= 0 {
		timeout = 60
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if readErr != nil {
			return fmt.Errorf("rag returned status %d and response body could not be read: %w", resp.StatusCode, readErr)
		}
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			return fmt.Errorf("rag returned status %d", resp.StatusCode)
		}
		return fmt.Errorf("rag returned status %d: %s", resp.StatusCode, message)
	}
	return nil
}

type leaderboardHandler struct{}

func (h *leaderboardHandler) Handle(ctx context.Context, task Task) error {
	switch task.Action {
	case ActionUserScoreSync:
		return h.syncUserScore(ctx, task.EntityID)
	case ActionLeaderboardReconcile:
		return h.reconcile(ctx)
	default:
		return fmt.Errorf("unsupported leaderboard action %q", task.Action)
	}
}

func (h *leaderboardHandler) syncUserScore(ctx context.Context, userID uint) error {
	var user userModel.User
	err := mysql.DB.WithContext(ctx).Select("id", "solved_count", "status").First(&user, userID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cache.Rdb.ZRem(ctx, leaderboardKey, strconv.FormatUint(uint64(userID), 10)).Err()
		}
		return err
	}

	member := strconv.FormatUint(uint64(user.ID), 10)
	if user.Status != userModel.UserStatusActive || user.SolvedCount <= 0 {
		return cache.Rdb.ZRem(ctx, leaderboardKey, member).Err()
	}
	return cache.Rdb.ZAdd(ctx, leaderboardKey, redis.Z{
		Score:  float64(user.SolvedCount * 10),
		Member: member,
	}).Err()
}

func (h *leaderboardHandler) reconcile(ctx context.Context) error {
	var users []userModel.User
	if err := mysql.DB.WithContext(ctx).
		Select("id", "solved_count").
		Where("solved_count > 0 AND status = ?", userModel.UserStatusActive).
		Find(&users).Error; err != nil {
		return err
	}

	if len(users) == 0 {
		return cache.Rdb.Del(ctx, leaderboardKey).Err()
	}

	temporaryKey := fmt.Sprintf("%s:rebuild:%d", leaderboardKey, time.Now().UnixNano())
	members := make([]redis.Z, 0, len(users))
	for _, user := range users {
		members = append(members, redis.Z{
			Score:  float64(user.SolvedCount * 10),
			Member: strconv.FormatUint(uint64(user.ID), 10),
		})
	}

	pipe := cache.Rdb.TxPipeline()
	pipe.Del(ctx, temporaryKey)
	pipe.ZAdd(ctx, temporaryKey, members...)
	if _, err := pipe.Exec(ctx); err != nil {
		return err
	}
	return cache.Rdb.Rename(ctx, temporaryKey, leaderboardKey).Err()
}

func buildESProblemDoc(problem *problemModel.Problem) problemModel.EsProblem {
	tags := make([]string, 0, len(problem.Tags))
	for _, tag := range problem.Tags {
		tags = append(tags, tag.Name)
	}
	return problemModel.EsProblem{
		ID:          problem.ID,
		Title:       problem.Title,
		Description: problem.Description,
		Tags:        tags,
	}
}
