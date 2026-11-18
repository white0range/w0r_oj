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
	MaxCodeBytes  = 64 * 1024
	MaxInputBytes = 4 * 1024
	Timeout       = 30 * time.Second
	keyPrefix     = "judge:trial:"
	pendingKey    = keyPrefix + "pending"
)

var (
	ErrBusy = errors.New("trial queue is full")
	ErrUsed = errors.New("trial already requested")
	enqueue = redis.NewScript(`
if redis.call('EXISTS', KEYS[2]) == 1 then return 0 end
if redis.call('LLEN', KEYS[1]) >= 4 then return -1 end
redis.call('SET', KEYS[2], ARGV[2], 'EX', ARGV[3])
redis.call('LPUSH', KEYS[1], ARGV[1])
redis.call('EXPIRE', KEYS[1], 60)
return 1`)
	finishRun = redis.NewScript(`
redis.call('SET', KEYS[1], ARGV[1], 'EX', 120)
if redis.call('GET', KEYS[2]) == ARGV[2] then redis.call('DEL', KEYS[2]) end
return 1`)
)

type Task struct {
	TurnID       uint      `json:"turn_id"`
	AttemptHash  string    `json:"attempt_hash"`
	SubmissionID uint      `json:"submission_id"`
	Input        string    `json:"input"`
	Deadline     time.Time `json:"deadline"`
	CodeRun      *CodeRun  `json:"code_run,omitempty"`
}

// CodeRun carries the current editor buffer for a user's interactive run.
type CodeRun struct {
	ID        string `json:"id"`
	UserID    uint   `json:"user_id"`
	ProblemID uint   `json:"problem_id"`
	Code      string `json:"code"`
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
	guardKey, guardValue, guardTTL := turnKey("used", task.TurnID), "1", 86400
	if run := task.CodeRun; run != nil {
		guardKey, guardValue, guardTTL = activeRunKey(run.UserID), run.ID, int(Timeout.Seconds())
	}
	code, err := enqueue.Run(ctx, cache.Rdb, []string{pendingKey, guardKey}, payload, guardValue, guardTTL).Int()
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

func Publish(ctx context.Context, task Task, result Result) error {
	payload, err := json.Marshal(result)
	if err != nil {
		return err
	}
	if run := task.CodeRun; run != nil {
		return finishRun.Run(ctx, cache.Rdb, []string{task.resultKey(), activeRunKey(run.UserID)}, payload, run.ID).Err()
	}
	return cache.Rdb.Set(ctx, task.resultKey(), payload, 2*time.Minute).Err()
}

func Wait(ctx context.Context, task Task) (Result, error) {
	resultKey := task.resultKey()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-ticker.C:
			payload, err := cache.Rdb.Get(ctx, resultKey).Bytes()
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

func (task Task) resultKey() string {
	if run := task.CodeRun; run != nil {
		return keyPrefix + "run_result:" + run.ID
	}
	return turnKey("result", task.TurnID)
}

func turnKey(kind string, turnID uint) string {
	return keyPrefix + kind + ":" + strconv.FormatUint(uint64(turnID), 10)
}

func activeRunKey(userID uint) string {
	return keyPrefix + "active_run:" + strconv.FormatUint(uint64(userID), 10)
}
