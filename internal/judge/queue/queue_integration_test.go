package queue

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"gojo/internal/judge/dto"
)

func TestFailedQueuePushLeavesSubmissionEligibleForRedispatch(t *testing.T) {
	addr := os.Getenv("GOJO_TEST_REDIS_ADDR")
	if addr == "" || os.Getenv("GOJO_TEST_REDIS_DISPOSABLE") != "1" {
		t.Skip("set GOJO_TEST_REDIS_ADDR and GOJO_TEST_REDIS_DISPOSABLE=1 for isolated judge queue test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	q := &Queue{client: client}
	task := dto.NewJudgeTask(37, 3, 7, "package main\nfunc main() {}")
	defer client.Del(ctx, pendingKey, dispatchKey(task.SubmissionID))
	if err := client.Set(ctx, pendingKey, "unavailable list", 0).Err(); err != nil {
		t.Fatal(err)
	}
	if queued, err := q.Enqueue(ctx, task); err == nil || queued {
		t.Fatalf("expected enqueue failure, queued=%t err=%v", queued, err)
	}
	if exists, err := client.Exists(ctx, dispatchKey(task.SubmissionID)).Result(); err != nil || exists != 0 {
		t.Fatalf("failed enqueue left dispatch marker, exists=%d err=%v", exists, err)
	}
	if err := client.Del(ctx, pendingKey).Err(); err != nil {
		t.Fatal(err)
	}
	if queued, err := q.Enqueue(ctx, task); err != nil || !queued {
		t.Fatalf("expected original submission to be requeued, queued=%t err=%v", queued, err)
	}
	if queued, err := q.Enqueue(ctx, task); err != nil || queued {
		t.Fatalf("expected existing job to be deduplicated, queued=%t err=%v", queued, err)
	}
}
