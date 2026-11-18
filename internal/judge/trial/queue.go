package trial

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"gojo/infrastructure/cache"
)

const (
	MaxInputBytes = 4 * 1024
	Timeout       = 30 * time.Second
	pendingKey    = "judge:trial:pending"
)

var (
	ErrBusy = errors.New("trial queue is full")
	ErrUsed = errors.New("turn already requested a trial")
	enqueue = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then return 0 end
if redis.call('LLEN', KEYS[1]) >= 4 then return -1 end
redis.call('SET', KEYS[2], '1', 'EX', 86400)
redis.call('LPUSH', KEYS[1], ARGV[1])
redis.call('EXPIRE', KEYS[1], 60)
return 1`)
)

type Task struct {
	TurnID       uint      `json:"turn_id"`
	AttemptHash  string    `json:"attempt_hash"`
	SubmissionID uint      `json:"submission_id"`
	Input        string    `json:"input"`
	Deadline     time.Time `json:"deadline"`
}

type Result struct {
	Status     string `json:"status"`
	Output     string `json:"output"`
	DurationMS int    `json:"duration_ms"`
	Truncated  bool   `json:"truncated,omitempty"`
}

func Enqueue(ctx context.Context, task Task) error {
	payload, err := json.Marshal(task)
	if err != nil {
		return err
	}
	code, err := enqueue.Run(ctx, cache.Rdb, []string{pendingKey, key("used", task.TurnID)}, payload).Int()
	if err != nil {
		return err
	}
	switch code {
	case 1:
		return nil
	case 0:
		return ErrUsed
	default:
		return ErrBusy
	}
}

func Pop(ctx context.Context) (Task, error) {
	var task Task
	payload, err := cache.Rdb.RPop(ctx, pendingKey).Bytes()
	if err != nil {
		return task, err
	}
	return task, json.Unmarshal(payload, &task)
}

func Publish(ctx context.Context, turnID uint, result Result) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return cache.Rdb.Set(ctx, key("result", turnID), payload, 2*time.Minute).Err()
}

func Wait(ctx context.Context, turnID uint) (Result, error) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-ticker.C:
			payload, err := cache.Rdb.Get(ctx, key("result", turnID)).Bytes()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				return Result{}, err
			}
			if err := ctx.Err(); err != nil {
				return Result{}, err
			}
			var result Result
			err = json.Unmarshal(payload, &result)
			return result, err
		}
	}
}

func key(kind string, turnID uint) string {
	return "judge:trial:" + kind + ":" + strconv.FormatUint(uint64(turnID), 10)
}
