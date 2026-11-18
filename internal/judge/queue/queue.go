package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gojo/infrastructure/cache"
	"gojo/internal/judge/dto"

	"github.com/redis/go-redis/v9"
)

const (
	pendingKey         = "judge:pending"
	processingKey      = "judge:processing"
	processingLeaseKey = "judge:processing:leases"
	retryAtKey         = "judge:retry_at"
	deadLetterKey      = "judge:dead_letter"
	dispatchKeyPrefix  = "judge:dispatch:"

	dispatchTTL = 7 * 24 * time.Hour
)

// Queue implements at-least-once delivery for judge tasks. A task remains in
// Redis until it is explicitly acknowledged after judging succeeds.
type Queue struct {
	client *redis.Client
}

func New() *Queue {
	return &Queue{client: cache.Rdb}
}

func (q *Queue) Enqueue(ctx context.Context, task dto.JudgeTask) (bool, error) {
	if !task.Valid() {
		return false, fmt.Errorf("invalid judge task")
	}
	raw, err := json.Marshal(task)
	if err != nil {
		return false, err
	}

	result, err := q.client.Eval(ctx, `
if redis.call('SET', KEYS[1], '1', 'NX', 'EX', ARGV[1]) then
  local pushed = redis.pcall('LPUSH', KEYS[2], ARGV[2])
  if type(pushed) == 'table' and pushed.err then
    redis.call('DEL', KEYS[1])
    return pushed
  end
  return 1
end
return 0
`, []string{dispatchKey(task.SubmissionID), pendingKey}, int(dispatchTTL.Seconds()), string(raw)).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// Claim atomically transfers a task to processing before returning it to a
// worker. A worker crash therefore cannot make the task disappear.
func (q *Queue) Claim(ctx context.Context, leaseUntil time.Time) (string, error) {
	result, err := q.client.Eval(ctx, `
local task = redis.call('RPOP', KEYS[1])
if not task then return false end
redis.call('LPUSH', KEYS[2], task)
redis.call('ZADD', KEYS[3], ARGV[1], task)
return task
`, []string{pendingKey, processingKey, processingLeaseKey}, leaseUntil.Unix()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	if result == nil || result == false {
		return "", nil
	}
	raw, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected claimed judge task type %T", result)
	}
	return raw, nil
}

func (q *Queue) RenewLease(ctx context.Context, raw string, leaseUntil time.Time) error {
	return q.client.ZAddXX(ctx, processingLeaseKey, redis.Z{
		Score:  float64(leaseUntil.Unix()),
		Member: raw,
	}).Err()
}
func (q *Queue) Acknowledge(ctx context.Context, raw string, submissionID uint) error {
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('DEL', KEYS[3])
return 1
`, []string{processingKey, processingLeaseKey, dispatchKey(submissionID)}, raw).Err()
}

func (q *Queue) Retry(ctx context.Context, processingRaw string, task dto.JudgeTask, retryAt time.Time) error {
	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('ZADD', KEYS[3], ARGV[2], ARGV[3])
return 1
`, []string{processingKey, processingLeaseKey, retryAtKey}, processingRaw, retryAt.Unix(), string(raw)).Err()
}

func (q *Queue) DeadLetter(ctx context.Context, processingRaw string, task dto.JudgeTask) error {
	raw, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('LPUSH', KEYS[3], ARGV[2])
redis.call('DEL', KEYS[4])
return 1
`, []string{processingKey, processingLeaseKey, deadLetterKey, dispatchKey(task.SubmissionID)}, processingRaw, string(raw)).Err()
}

func (q *Queue) DeadLetterRaw(ctx context.Context, raw string) error {
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('LPUSH', KEYS[3], ARGV[1])
return 1
`, []string{processingKey, processingLeaseKey, deadLetterKey}, raw).Err()
}

func (q *Queue) PromoteRetries(ctx context.Context, now time.Time) error {
	tasks, err := q.client.ZRangeByScore(ctx, retryAtKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now.Unix()),
		Count: 100,
	}).Result()
	if err != nil {
		return err
	}
	for _, raw := range tasks {
		if err := q.client.Eval(ctx, `
if redis.call('ZREM', KEYS[1], ARGV[1]) == 1 then
  redis.call('LPUSH', KEYS[2], ARGV[1])
end
return 1
`, []string{retryAtKey, pendingKey}, raw).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (q *Queue) RecoverExpired(ctx context.Context, now time.Time) error {
	tasks, err := q.client.ZRangeByScore(ctx, processingLeaseKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%d", now.Unix()),
		Count: 100,
	}).Result()
	if err != nil {
		return err
	}
	for _, raw := range tasks {
		if err := q.client.Eval(ctx, `
if redis.call('ZREM', KEYS[1], ARGV[1]) == 1 then
  redis.call('LREM', KEYS[2], 1, ARGV[1])
  redis.call('LPUSH', KEYS[3], ARGV[1])
end
return 1
`, []string{processingLeaseKey, processingKey, pendingKey}, raw).Err(); err != nil {
			return err
		}
	}
	return nil
}

func TruncateError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 1024 {
		message = message[:1024]
	}
	return strings.TrimSpace(message)
}

func dispatchKey(submissionID uint) string {
	return fmt.Sprintf("%s%d", dispatchKeyPrefix, submissionID)
}
