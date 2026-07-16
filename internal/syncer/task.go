package syncer

import (
	"encoding/json"
	"fmt"
	"time"
)

type Target string

const (
	TargetES          Target = "es"
	TargetRAG         Target = "rag"
	TargetLeaderboard Target = "leaderboard"
)

type Action string

const (
	ActionProblemUpsert         Action = "problem_upsert"
	ActionProblemDelete         Action = "problem_delete"
	ActionUserScoreSync         Action = "user_score_sync"
	ActionLeaderboardReconcile  Action = "leaderboard_reconcile"
)

type Task struct {
	ID         string    `json:"id"`
	Target     Target    `json:"target"`
	Action     Action    `json:"action"`
	EntityID   uint      `json:"entity_id"`
	RetryCount int       `json:"retry_count"`
	CreatedAt  time.Time `json:"created_at"`
	LastError  string    `json:"last_error,omitempty"`
}

func NewTask(target Target, action Action, entityID uint) Task {
	return Task{
		ID:        fmt.Sprintf("%d-%s-%s-%d", time.Now().UnixNano(), target, action, entityID),
		Target:    target,
		Action:    action,
		EntityID:  entityID,
		CreatedAt: time.Now().UTC(),
	}
}

func parseTask(raw string) (Task, error) {
	var task Task
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		return Task{}, err
	}
	if task.ID == "" || task.Target == "" || task.Action == "" {
		return Task{}, fmt.Errorf("invalid sync task")
	}
	return task, nil
}

func marshalTask(task Task) (string, error) {
	data, err := json.Marshal(task)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
