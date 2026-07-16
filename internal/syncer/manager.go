package syncer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"gojo/internal/problem/repository"
)

const (
	workerCount          = 2
	maxRetryCount        = 8
	taskLeaseDuration    = 90 * time.Second
	queuePollInterval    = 200 * time.Millisecond
	retryPollInterval    = 5 * time.Second
	recoveryPollInterval = 30 * time.Second
	reconcileInterval    = 30 * time.Minute
)

type Producer interface {
	EnqueueProblemUpsert(ctx context.Context, problemID uint) error
	EnqueueProblemDelete(ctx context.Context, problemID uint) error
	EnqueueUserScoreSync(ctx context.Context, userID uint) error
}

type Manager struct {
	queue       *queue
	problemRepo repository.ProblemRepository
	handlers    map[Target]handler
}

type handler interface {
	Handle(ctx context.Context, task Task) error
}

func NewManager(problemRepo repository.ProblemRepository, searchRepo repository.ProblemSearchRepository) *Manager {
	manager := &Manager{
		queue:       newQueue(),
		problemRepo: problemRepo,
		handlers:    make(map[Target]handler),
	}
	manager.handlers[TargetES] = &esHandler{problems: problemRepo, search: searchRepo}
	manager.handlers[TargetRAG] = &ragHandler{}
	manager.handlers[TargetLeaderboard] = &leaderboardHandler{}
	return manager
}

func (m *Manager) Start(ctx context.Context) {
	if err := m.queue.recoverExpired(ctx, time.Now()); err != nil {
		log.Printf("recover expired sync tasks at startup failed: %v", err)
	}

	for i := 0; i < workerCount; i++ {
		go m.runWorker(ctx, i+1)
	}
	go m.runRetryScheduler(ctx)
	go m.runRecoveryWorker(ctx)
	go m.runReconciler(ctx)

	if err := m.enqueueLeaderboardReconcile(ctx); err != nil {
		log.Printf("enqueue initial leaderboard reconciliation failed: %v", err)
	}
	if err := m.enqueueAllProblems(ctx); err != nil {
		log.Printf("enqueue initial problem reconciliation failed: %v", err)
	}
}

func (m *Manager) EnqueueProblemUpsert(ctx context.Context, problemID uint) error {
	esErr := m.queue.enqueue(ctx, NewTask(TargetES, ActionProblemUpsert, problemID))
	ragErr := m.queue.enqueue(ctx, NewTask(TargetRAG, ActionProblemUpsert, problemID))
	return errors.Join(esErr, ragErr)
}

func (m *Manager) EnqueueProblemDelete(ctx context.Context, problemID uint) error {
	esErr := m.queue.enqueue(ctx, NewTask(TargetES, ActionProblemDelete, problemID))
	ragErr := m.queue.enqueue(ctx, NewTask(TargetRAG, ActionProblemDelete, problemID))
	return errors.Join(esErr, ragErr)
}

func (m *Manager) EnqueueUserScoreSync(ctx context.Context, userID uint) error {
	return m.queue.enqueue(ctx, NewTask(TargetLeaderboard, ActionUserScoreSync, userID))
}

func (m *Manager) enqueueLeaderboardReconcile(ctx context.Context) error {
	return m.queue.enqueue(ctx, NewTask(TargetLeaderboard, ActionLeaderboardReconcile, 0))
}

func (m *Manager) enqueueAllProblems(ctx context.Context) error {
	problems, err := m.problemRepo.GetAllProblemsWithTags(ctx)
	if err != nil {
		return fmt.Errorf("load problems for reconciliation: %w", err)
	}
	for _, problem := range problems {
		if err := m.EnqueueProblemUpsert(ctx, problem.ID); err != nil {
			return fmt.Errorf("enqueue problem %d reconciliation: %w", problem.ID, err)
		}
	}
	return nil
}

func (m *Manager) runWorker(ctx context.Context, workerID int) {
	ticker := time.NewTicker(queuePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			raw, err := m.queue.claim(ctx, time.Now().Add(taskLeaseDuration))
			if err != nil {
				log.Printf("sync worker %d claim failed: %v", workerID, err)
				continue
			}
			if raw == "" {
				continue
			}
			m.process(ctx, raw)
		}
	}
}

func (m *Manager) process(ctx context.Context, raw string) {
	task, err := parseTask(raw)
	if err != nil {
		log.Printf("move malformed sync task to dead letter: %v", err)
		if deadErr := m.queue.deadLetterRaw(ctx, raw); deadErr != nil {
			log.Printf("move malformed sync task to dead letter failed: %v", deadErr)
		}
		return
	}

	h, ok := m.handlers[task.Target]
	if !ok {
		m.handleFailure(ctx, raw, task, fmt.Errorf("no handler for target %q", task.Target))
		return
	}

	taskCtx, cancel := context.WithTimeout(ctx, taskLeaseDuration-5*time.Second)
	err = h.Handle(taskCtx, task)
	cancel()
	if err != nil {
		m.handleFailure(ctx, raw, task, err)
		return
	}

	if err := m.queue.acknowledge(ctx, raw); err != nil {
		log.Printf("acknowledge sync task %s failed: %v", task.ID, err)
	}
}

func (m *Manager) handleFailure(ctx context.Context, raw string, task Task, cause error) {
	task.RetryCount++
	task.LastError = truncateError(cause)
	if task.RetryCount > maxRetryCount {
		log.Printf("sync task %s exhausted retries: %v", task.ID, cause)
		if err := m.queue.deadLetter(ctx, raw, task); err != nil {
			log.Printf("move sync task %s to dead letter failed: %v", task.ID, err)
		}
		return
	}

	retryAt := time.Now().Add(backoff(task.RetryCount))
	log.Printf("sync task %s failed (attempt %d), retry at %s: %v", task.ID, task.RetryCount, retryAt.Format(time.RFC3339), cause)
	if err := m.queue.retry(ctx, raw, task, retryAt); err != nil {
		log.Printf("schedule retry for sync task %s failed: %v", task.ID, err)
	}
}

func (m *Manager) runRetryScheduler(ctx context.Context) {
	ticker := time.NewTicker(retryPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := m.queue.promoteRetries(ctx, now); err != nil {
				log.Printf("promote sync retries failed: %v", err)
			}
		}
	}
}

func (m *Manager) runRecoveryWorker(ctx context.Context) {
	ticker := time.NewTicker(recoveryPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if err := m.queue.recoverExpired(ctx, now); err != nil {
				log.Printf("recover expired sync tasks failed: %v", err)
			}
		}
	}
}

func (m *Manager) runReconciler(ctx context.Context) {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.enqueueLeaderboardReconcile(ctx); err != nil {
				log.Printf("enqueue leaderboard reconciliation failed: %v", err)
			}
			if err := m.enqueueAllProblems(ctx); err != nil {
				log.Printf("enqueue problem reconciliation failed: %v", err)
			}
		}
	}
}

func backoff(retryCount int) time.Duration {
	delays := []time.Duration{time.Minute, 5 * time.Minute, 30 * time.Minute, 2 * time.Hour, 12 * time.Hour}
	if retryCount <= 0 {
		return delays[0]
	}
	if retryCount > len(delays) {
		return delays[len(delays)-1]
	}
	return delays[retryCount-1]
}

func truncateError(err error) string {
	message := err.Error()
	if len(message) <= 1024 {
		return message
	}
	return strings.TrimSpace(message[:1024])
}
