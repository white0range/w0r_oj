package syncer

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gojo/infrastructure/mysql"
	chatModel "gojo/internal/chat/model"
	chatRepo "gojo/internal/chat/repository"
)

// Opt-in integration tests create their own MySQL schema and require a disposable Redis.
func TestIntegrationClaimFencingAndDurableMemoryIntent(t *testing.T) {
	dsn := os.Getenv("GOJO_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set GOJO_TEST_MYSQL_DSN for isolated MySQL integration test")
	}
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("gojo_memory_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE `" + name + "`"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.Exec("DROP DATABASE `" + name + "`"); err != nil {
			t.Error(err)
		}
	}()
	cfg.DBName, cfg.ParseTime = name, true
	db, err := gorm.Open(gormmysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	previous := mysql.DB
	mysql.DB = db
	defer func() { mysql.DB = previous }()
	if err := db.AutoMigrate(&chatModel.ChatSession{}, &chatModel.ChatTurn{}, &chatModel.ChatMessage{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	repo := chatRepo.NewChatRepository()
	session := &chatModel.ChatSession{UserID: 42, Status: chatModel.ChatSessionStatusActive}
	if err := repo.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	_, turn, err := repo.CreateUserMessageTurn(ctx, session, "original query", "test-model")
	if err != nil {
		t.Fatal(err)
	}
	oldAttempt, currentAttempt := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if _, claimed, err := repo.ClaimTurn(ctx, turn.ID, oldAttempt, time.Now().Add(time.Minute)); err != nil || !claimed {
		t.Fatalf("first claim: %v", err)
	}
	if err := db.Model(&chatModel.ChatTurn{}).Where("id = ?", turn.ID).Update("lease_expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if _, claimed, err := repo.ClaimTurn(ctx, turn.ID, currentAttempt, time.Now().Add(time.Minute)); err != nil || !claimed {
		t.Fatalf("reclaim: %v", err)
	}
	if _, completed, err := repo.CompleteClaimedTurn(ctx, turn.ID, oldAttempt, "old", `{"answer":"old"}`, time.Now()); err != nil || completed {
		t.Fatalf("old attempt completed: completed=%v err=%v", completed, err)
	}
	store := &mysqlChatMemoryStore{}
	if payload, err := store.LoadConfirmed(ctx, turn.ID); err != nil || payload != nil {
		t.Fatalf("unconfirmed result exposed: %v", err)
	}
	if _, completed, err := repo.CompleteClaimedTurn(ctx, turn.ID, currentAttempt, "current", `{"answer":"current"}`, time.Now()); err != nil || !completed {
		t.Fatalf("current attempt completion: %v", err)
	}
	if _, completed, err := repo.CompleteClaimedTurn(ctx, turn.ID, currentAttempt, "duplicate", `{"answer":"duplicate"}`, time.Now()); err != nil || completed {
		t.Fatalf("duplicate completion accepted: %v", err)
	}
	// No Redis enqueue occurred: the committed intent must still be discoverable.
	ids, err := store.ListPending(ctx, 0, 100)
	if err != nil || len(ids) != 1 || ids[0] != turn.ID {
		t.Fatalf("missing committed memory intent: %v %v", ids, err)
	}
	payload, err := store.LoadConfirmed(ctx, turn.ID)
	if err != nil || payload == nil || payload.UserID != 42 || payload.QueryText != "original query" || string(payload.Result) != `{"answer":"current"}` {
		t.Fatalf("wrong canonical memory data: %v", err)
	}
	var count int64
	if err := db.Model(&chatModel.ChatMessage{}).Where("role = ?", chatModel.ChatMessageRoleAssistant).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("assistant messages=%d err=%v", count, err)
	}
	if err := store.MarkSynced(ctx, turn.ID); err != nil {
		t.Fatal(err)
	}
	if ids, err := store.ListPending(ctx, 0, 100); err != nil || len(ids) != 0 {
		t.Fatalf("synced intent still pending: %v", err)
	}
}

func TestIntegrationMemoryQueueDeduplicatesRetries(t *testing.T) {
	addr := os.Getenv("GOJO_TEST_REDIS_ADDR")
	if addr == "" || os.Getenv("GOJO_TEST_REDIS_DISPOSABLE") != "1" {
		t.Skip("requires explicitly disposable test Redis")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	ctx := context.Background()
	q := &queue{client: client}
	defer client.Del(ctx, pendingKey, processingKey, processingLeaseKey, retryAtKey, deadLetterKey, chatMemoryDispatchKey(87), chatMemoryDispatchKey(88))
	if err := q.enqueue(ctx, NewTask(TargetChatMemory, ActionChatMemorySync, 87)); err != nil {
		t.Fatal(err)
	}
	if err := q.enqueue(ctx, NewTask(TargetChatMemory, ActionChatMemorySync, 87)); err != nil {
		t.Fatal(err)
	}
	if n := client.LLen(ctx, pendingKey).Val(); n != 1 {
		t.Fatalf("duplicate dispatches=%d", n)
	}
	raw, err := q.claim(ctx, time.Now().Add(time.Minute))
	if err != nil || raw == "" {
		t.Fatalf("claim: %v", err)
	}
	task, _ := parseTask(raw)
	task.RetryCount++
	if err := q.retry(ctx, raw, task, time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := q.enqueue(ctx, NewTask(TargetChatMemory, ActionChatMemorySync, 87)); err != nil {
		t.Fatal(err)
	}
	if client.LLen(ctx, pendingKey).Val() != 0 || client.ZCard(ctx, retryAtKey).Val() != 1 {
		t.Fatal("dispatcher bypassed retry backoff")
	}
	if err := q.promoteRetries(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	raw, err = q.claim(ctx, time.Now().Add(time.Minute))
	if err != nil || raw == "" {
		t.Fatalf("retry claim: %v", err)
	}
	if err := q.acknowledge(ctx, raw); err != nil {
		t.Fatal(err)
	}
	if client.Exists(ctx, chatMemoryDispatchKey(87)).Val() != 0 {
		t.Fatal("acknowledged dispatch marker not removed")
	}
	if err := q.enqueue(ctx, NewTask(TargetChatMemory, ActionChatMemorySync, 88)); err != nil {
		t.Fatal(err)
	}
	raw, err = q.claim(ctx, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	task, _ = parseTask(raw)
	if err := q.deadLetter(ctx, raw, task); err != nil {
		t.Fatal(err)
	}
	if err := q.enqueue(ctx, NewTask(TargetChatMemory, ActionChatMemorySync, 88)); err != nil {
		t.Fatal(err)
	}
	if client.LLen(ctx, pendingKey).Val() != 0 || client.LLen(ctx, deadLetterKey).Val() != 1 {
		t.Fatal("dispatcher must not bypass dead letter handling")
	}
}
