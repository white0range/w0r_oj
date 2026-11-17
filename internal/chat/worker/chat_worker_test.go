package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"gojo/config"
	"gojo/internal/chat/model"
	"gojo/internal/chat/repository"
)

type agentRoundTripper func(*http.Request) (*http.Response, error)

func (f agentRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentRequestUsesServiceKeyWithoutAdminAccount(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig.Chat.AgentServiceToken = "test-service-secret"
	config.GlobalConfig.Chat.AgentBaseURL = "http://agent.test"
	// This test has no database or admin account; constructing the worker must work.
	w, err := NewChatWorker(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.httpClient.Transport = agentRoundTripper(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "" {
			t.Error("user JWT must not be sent to Python")
		}
		if r.Header.Get("X-Agent-Service-Token") != "test-service-secret" {
			t.Error("service key not sent")
		}
		var payload chatAgentRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		if payload.TurnID != 87 || payload.UserID != 42 || payload.ProcessingToken != strings.Repeat("a", 64) {
			t.Errorf("missing turn context: %+v", payload)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"result":{"answer":"ok"}}`))}, nil
	})
	if _, err := w.callchatAgentWithPayload(context.Background(), chatAgentRequest{TurnID: 87, UserID: 42, ProcessingToken: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
}

type completionRepoStub struct {
	repository.ChatRepository
	completed bool
	events    *[]string
}

func (s *completionRepoStub) ClaimTurn(_ context.Context, id uint, _ string, _ time.Time) (*model.ChatTurn, bool, error) {
	return &model.ChatTurn{ID: id, UserID: 42, SessionID: 12}, true, nil
}
func (s *completionRepoStub) PrepareSessionCompaction(context.Context, uint, int) (*model.ChatSession, []model.ChatMessage, error) {
	return &model.ChatSession{ID: 12, UserID: 42}, nil, nil
}
func (s *completionRepoStub) ListRecentMessagesBySessionID(context.Context, uint, int) ([]model.ChatMessage, error) {
	return nil, nil
}
func (s *completionRepoStub) CompleteClaimedTurn(context.Context, uint, string, string, string, time.Time) (*model.ChatMessage, bool, error) {
	*s.events = append(*s.events, "complete")
	return nil, s.completed, nil
}

type memoryProducerStub struct {
	events *[]string
	err    error
}

func (s *memoryProducerStub) EnqueueChatMemorySync(context.Context, uint) error {
	*s.events = append(*s.events, "memory")
	return s.err
}

func TestMemoryIsEnqueuedOnlyAfterConfirmedCompletion(t *testing.T) {
	previous := config.GlobalConfig
	t.Cleanup(func() { config.GlobalConfig = previous })
	config.GlobalConfig.Chat.AgentServiceToken = "test-key"
	for _, tc := range []struct {
		name       string
		completed  bool
		enqueueErr error
		want       string
	}{
		{name: "current attempt", completed: true, want: "complete,memory"},
		{name: "lost attempt", completed: false, want: "complete"},
		{name: "Redis outage preserves successful answer", completed: true, enqueueErr: errors.New("Redis unavailable"), want: "complete,memory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			previousOutput := log.Writer()
			log.SetOutput(&output)
			defer log.SetOutput(previousOutput)
			var events []string
			repo := &completionRepoStub{completed: tc.completed, events: &events}
			producer := &memoryProducerStub{events: &events, err: tc.enqueueErr}
			w, err := NewChatWorker(repo, producer)
			if err != nil {
				t.Fatal(err)
			}
			var sentToken string
			w.httpClient.Transport = agentRoundTripper(func(req *http.Request) (*http.Response, error) {
				var payload chatAgentRequest
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				sentToken = payload.ProcessingToken
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"result":{"answer":"ok"}}`))}, nil
			})
			if err := w.ProcessTurn(context.Background(), 87); err != nil {
				t.Fatal(err)
			}
			if strings.Join(events, ",") != tc.want {
				t.Fatalf("events=%v, want=%s", events, tc.want)
			}
			for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
				if !strings.Contains(line, "turn_id=87") {
					t.Fatalf("uncorrelated chat log: %s", line)
				}
			}
			digest := sha256.Sum256([]byte(sentToken))
			if !strings.Contains(output.String(), "attempt_id="+hex.EncodeToString(digest[:12])) {
				t.Error("execution identifier is missing")
			}
			if strings.Contains(output.String(), sentToken) {
				t.Error("raw processing token leaked into logs")
			}
		})
	}
}
