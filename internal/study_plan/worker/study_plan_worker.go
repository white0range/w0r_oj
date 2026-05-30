package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/internal/study_plan/dto"
	"gojo/internal/study_plan/model"
	"gojo/internal/study_plan/repository"
	userModel "gojo/internal/user/model"
	jwtPkg "gojo/pkg/jwt"
)

type studyPlanAgentRequest struct {
	UserID uint   `json:"user_id"`
	Goal   string `json:"goal"`
}

type studyPlanAgentResponse struct {
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

type StudyPlanWorker struct {
	repo         repository.StudyPlanRepository
	httpClient   *http.Client
	agentBaseURL string
	agentToken   string
}

func NewStudyPlanWorker(repo repository.StudyPlanRepository) (*StudyPlanWorker, error) {
	serviceUser := &userModel.User{
		Username: "study-plan-agent",
		Role:     1,
	}

	agentToken, err := jwtPkg.GenerateToken(serviceUser)
	if err != nil {
		return nil, fmt.Errorf("generate study plan agent token failed: %w", err)
	}

	timeoutSeconds := config.GlobalConfig.StudyPlan.AgentTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60
	}

	agentBaseURL := strings.TrimRight(config.GlobalConfig.StudyPlan.AgentBaseURL, "/")
	if agentBaseURL == "" {
		agentBaseURL = "http://localhost:8000"
	}

	return &StudyPlanWorker{
		repo: repo,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		agentBaseURL: agentBaseURL,
		agentToken:   agentToken,
	}, nil
}

func (w *StudyPlanWorker) StartWorkerPool(workerCount int) {
	log.Printf("starting study plan worker pool, workers=%d\n", workerCount)
	for i := 1; i <= workerCount; i++ {
		go w.run(i)
	}
}

func (w *StudyPlanWorker) ProcessTask(ctx context.Context, taskID uint) error {
	task, err := w.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return fmt.Errorf("get study plan task failed: %w", err)
	}

	if err := w.repo.UpdateTaskStatus(ctx, task.ID, model.TaskStatusRunning); err != nil {
		return fmt.Errorf("update study plan task status to running failed: %w", err)
	}

	startedAt := time.Now()

	resultJSON, err := w.callStudyPlanAgent(ctx, task.UserID, task.Goal)
	if err != nil {
		_ = w.repo.UpdateTaskFailed(ctx, task.ID, err.Error(), time.Now())
		return fmt.Errorf("call study plan agent failed: %w", err)
	}

	if err := w.repo.UpdateTaskResult(ctx, task.ID, resultJSON, time.Now()); err != nil {
		_ = w.repo.UpdateTaskFailed(ctx, task.ID, err.Error(), time.Now())
		return fmt.Errorf("update study plan task result failed: %w", err)
	}

	log.Printf(
		"study plan task completed: task_id=%d user_id=%d duration_ms=%d status=%s\n",
		task.ID,
		task.UserID,
		time.Since(startedAt).Milliseconds(),
		model.TaskStatusSucceeded,
	)

	return nil
}

func (w *StudyPlanWorker) callStudyPlanAgent(ctx context.Context, userID uint, goal string) (string, error) {
	reqBody := studyPlanAgentRequest{
		UserID: userID,
		Goal:   goal,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal study plan agent request failed: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		w.agentBaseURL+"/study-plan/run",
		bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return "", fmt.Errorf("create study plan agent request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.agentToken)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do study plan agent request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read study plan agent response failed: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("study plan agent returned status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var agentResp studyPlanAgentResponse
	if err := json.Unmarshal(respBody, &agentResp); err != nil {
		return "", fmt.Errorf("unmarshal study plan agent response failed: %w", err)
	}

	if len(agentResp.Result) == 0 {
		return "", fmt.Errorf("study plan agent returned empty result")
	}

	return string(agentResp.Result), nil
}

func (w *StudyPlanWorker) run(id int) {
	ctx := context.Background()

	for {
		result, err := cache.Rdb.BRPop(ctx, 0, "study_plan_queue").Result()
		if err != nil {
			log.Printf("study plan worker %d pop task failed: %v\n", id, err)
			continue
		}

		var task dto.StudyPlanQueueTask
		if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
			log.Printf("study plan worker %d unmarshal task failed: %v\n", id, err)
			continue
		}

		log.Printf("study plan worker %d processing task_id=%d user_id=%d\n", id, task.TaskID, task.UserID)

		if err := w.ProcessTask(ctx, task.TaskID); err != nil {
			log.Printf("study plan worker %d process task failed: %v\n", id, err)
		}
	}
}
