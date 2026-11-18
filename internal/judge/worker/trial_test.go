package worker

import (
	"testing"
	"time"

	chatModel "gojo/internal/chat/model"
	"gojo/internal/judge/trial"
)

func TestTrialRequiresCurrentTurnAttempt(t *testing.T) {
	now := time.Now()
	lease := now.Add(time.Minute)
	turn := &chatModel.ChatTurn{
		UserID: 7, Status: chatModel.TaskStatusRunning, ProcessingToken: "new-token", LeaseExpiresAt: &lease,
	}
	hash := trial.HashAttempt(turn.ProcessingToken)
	if !currentTrialAttempt(turn, hash, now) {
		t.Fatal("current attempt was rejected")
	}
	if currentTrialAttempt(turn, trial.HashAttempt("old-token"), now) {
		t.Fatal("previous attempt was accepted")
	}
	if currentTrialAttempt(turn, hash, lease) {
		t.Fatal("expired attempt was accepted")
	}
}
