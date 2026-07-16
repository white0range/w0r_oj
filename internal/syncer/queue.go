package syncer

import (
	"context"
	"fmt"
	"time"

	"gojo/infrastructure/cache"

	"github.com/redis/go-redis/v9"
)

const (
	pendingKey         = "sync:pending"
	processingKey      = "sync:processing"
	processingLeaseKey = "sync:processing:leases"
	retryAtKey         = "sync:retry_at"
	deadLetterKey      = "sync:dead_letter"
)

type queue struct {
	client *redis.Client
}

func newQueue() *queue {
	return &queue{client: cache.Rdb}
}

func (q *queue) enqueue(ctx context.Context, task Task) error {
	raw, err := marshalTask(task)
	if err != nil {
		return err
	}
	return q.client.LPush(ctx, pendingKey, raw).Err()
}

// claim atomically moves a task into processing and attaches a lease to it.
func (q *queue) claim(ctx context.Context, leaseUntil time.Time) (string, error) {
	result, err := q.client.Eval(ctx, `
local task = redis.call('RPOP', KEYS[1])
if not task then return false end
redis.call('LPUSH', KEYS[2], task)
redis.call('ZADD', KEYS[3], ARGV[1], task)
return task
`, []string{pendingKey, processingKey, processingLeaseKey}, leaseUntil.Unix()).Result()
	if err != nil {
		return "", err
	}
	if result == nil || result == false {
		return "", nil
	}
	raw, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected claimed task type %T", result)
	}
	return raw, nil
}

func (q *queue) acknowledge(ctx context.Context, raw string) error {
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
return 1
`, []string{processingKey, processingLeaseKey}, raw).Err()
}

func (q *queue) retry(ctx context.Context, processingRaw string, task Task, retryAt time.Time) error {
	retryRaw, err := marshalTask(task)
	if err != nil {
		return err
	}
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('ZADD', KEYS[3], ARGV[2], ARGV[3])
return 1
`, []string{processingKey, processingLeaseKey, retryAtKey}, processingRaw, retryAt.Unix(), retryRaw).Err()
}

func (q *queue) deadLetter(ctx context.Context, processingRaw string, task Task) error {
	raw, err := marshalTask(task)
	if err != nil {
		return err
	}
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('LPUSH', KEYS[3], ARGV[2])
return 1
`, []string{processingKey, processingLeaseKey, deadLetterKey}, processingRaw, raw).Err()
}

func (q *queue) deadLetterRaw(ctx context.Context, raw string) error {
	return q.client.Eval(ctx, `
redis.call('LREM', KEYS[1], 1, ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('LPUSH', KEYS[3], ARGV[1])
return 1
`, []string{processingKey, processingLeaseKey, deadLetterKey}, raw).Err()
}

func (q *queue) promoteRetries(ctx context.Context, now time.Time) error {
	tasks, err := q.client.ZRangeByScore(ctx, retryAtKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now.Unix()),
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

func (q *queue) recoverExpired(ctx context.Context, now time.Time) error {
	tasks, err := q.client.ZRangeByScore(ctx, processingLeaseKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now.Unix()),
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

func (q *queue) recoverAllProcessing(ctx context.Context) error {
	tasks, err := q.client.LRange(ctx, processingKey, 0, -1).Result()
	if err != nil {
		return err
	}
	for _, raw := range tasks {
		if err := q.client.Eval(ctx, `
if redis.call('LREM', KEYS[1], 1, ARGV[1]) == 1 then
  redis.call('LPUSH', KEYS[2], ARGV[1])
end
redis.call('ZREM', KEYS[3], ARGV[1])
return 1
`, []string{processingKey, pendingKey, processingLeaseKey}, raw).Err(); err != nil {
			return err
		}
	}
	return nil
}
