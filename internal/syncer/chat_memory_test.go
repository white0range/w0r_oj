package syncer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gojo/config"
)

type memoryStoreStub struct {
	payload *chatMemoryPayload
	loadErr error
	markErr error
	marked  int
}

func (s *memoryStoreStub) LoadConfirmed(context.Context, uint) (*chatMemoryPayload, error) {
	return s.payload, s.loadErr
}
func (s *memoryStoreStub) ListPending(context.Context, uint, int) ([]uint, error) { return nil, nil }
func (s *memoryStoreStub) MarkSynced(context.Context, uint) error {
	if s.markErr != nil {
		return s.markErr
	}
	s.marked++
	s.payload = nil
	return nil
}

type memoryTransport func(*http.Request) (*http.Response, error)

func (f memoryTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMemorySyncConfirmsResultAndRetriesStablePayload(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig.Chat.AgentServiceToken = "test-key"
	config.GlobalConfig.Chat.AgentBaseURL = "http://agent.test"
	store := &memoryStoreStub{payload: &chatMemoryPayload{TurnID: 87, UserID: 42, QueryText: "original query", Result: json.RawMessage(`{"answer":"confirmed"}`), FinishedAt: time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)}}
	var sent []string
	status := 503
	h := &chatMemoryHandler{store: store, client: &http.Client{Transport: memoryTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/memory/sync" || req.Header.Get("X-Agent-Service-Token") != "test-key" {
			t.Error("wrong memory sync request")
		}
		if req.Header.Get("Authorization") != "" || req.Header.Get("X-Agent-Processing-Token") != "" {
			t.Error("memory sync must use only service credentials")
		}
		body, _ := io.ReadAll(req.Body)
		sent = append(sent, string(body))
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}}
	task := NewTask(TargetChatMemory, ActionChatMemorySync, 87)
	if err := h.Handle(context.Background(), task); err == nil {
		t.Fatal("remote failure must be retried")
	}
	if store.marked != 0 {
		t.Fatal("failed request consumed pending memory")
	}
	status = 200
	store.markErr = errors.New("database temporarily unavailable")
	if err := h.Handle(context.Background(), task); err == nil {
		t.Fatal("DB acknowledgement failure must be retried")
	}
	store.markErr = nil
	if err := h.Handle(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if store.marked != 1 || len(sent) != 3 || sent[0] != sent[1] || sent[1] != sent[2] {
		t.Fatal("retries must retain the same confirmed data and timestamp")
	}
	if err := h.Handle(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if len(sent) != 3 {
		t.Fatal("already synchronized task contacted Python again")
	}
}

func TestMemorySyncDoesNotSendUnconfirmedResults(t *testing.T) {
	store := &memoryStoreStub{}
	h := &chatMemoryHandler{store: store, client: &http.Client{Transport: memoryTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("no confirmed result must mean no network call")
		return nil, nil
	})}}
	if err := h.Handle(context.Background(), NewTask(TargetChatMemory, ActionChatMemorySync, 87)); err != nil {
		t.Fatal(err)
	}
	store.loadErr = errors.New("database unavailable")
	if err := h.Handle(context.Background(), NewTask(TargetChatMemory, ActionChatMemorySync, 87)); err == nil {
		t.Fatal("load failure must not be acknowledged")
	}
}
