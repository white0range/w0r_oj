package trial

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"gojo/infrastructure/cache"
)

func TestCodeRunAndAgentTrialShareQueue(t *testing.T) {
	addr := os.Getenv("GOJO_TEST_REDIS_ADDR")
	if addr == "" || os.Getenv("GOJO_TEST_REDIS_DISPOSABLE") != "1" {
		t.Skip("set GOJO_TEST_REDIS_ADDR and GOJO_TEST_REDIS_DISPOSABLE=1 for isolated trial queue test")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	previous := cache.Rdb
	cache.Rdb = client
	t.Cleanup(func() {
		cache.Rdb = previous
		_ = client.Close()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	deadline := time.Now().Add(Timeout)
	agent := Task{TurnID: 51, AttemptHash: "attempt", SubmissionID: 7, Input: "agent", Deadline: deadline}
	browser := Task{
		CodeRun: &CodeRun{ID: "run-a", UserID: 51, ProblemID: 2, Code: "package main"},
		Input:   "browser", Deadline: deadline,
	}
	nextBrowser := browser
	nextBrowser.CodeRun = &CodeRun{ID: "run-b", UserID: 51, ProblemID: 2, Code: "package main"}
	defer client.Del(ctx, pendingKey, turnKey("used", agent.TurnID), activeRunKey(browser.CodeRun.UserID),
		agent.resultKey(), browser.resultKey(), nextBrowser.resultKey())
	for _, task := range []Task{agent, browser} {
		if err := Enqueue(ctx, task); err != nil {
			t.Fatal(err)
		}
	}
	for _, task := range []Task{agent, nextBrowser} {
		if err := Enqueue(ctx, task); !errors.Is(err, ErrUsed) {
			t.Fatalf("duplicate enqueue returned %v, want ErrUsed", err)
		}
	}
	for _, task := range []Task{agent, browser} {
		popped, err := Pop(ctx)
		if err != nil || popped.resultKey() != task.resultKey() || popped.Input != task.Input {
			t.Fatalf("unexpected trial queue order: got %s, want %s, err=%v", popped.resultKey(), task.resultKey(), err)
		}
		if err := Publish(ctx, popped, Result{Status: "ok", Output: popped.Input}); err != nil {
			t.Fatal(err)
		}
	}
	for _, task := range []Task{agent, browser} {
		result, err := Wait(ctx, task)
		if err != nil || result.Output != task.Input {
			t.Fatalf("result for %s: %+v, err=%v", task.resultKey(), result, err)
		}
	}
	if err := Enqueue(ctx, nextBrowser); err != nil {
		t.Fatalf("browser slot remained held after completing the previous run: %v", err)
	}
	if err := Enqueue(ctx, agent); !errors.Is(err, ErrUsed) {
		t.Fatalf("agent turn lost its trial limit: %v", err)
	}
}
