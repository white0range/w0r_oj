package syncer

import (
	"context"
	"errors"
)

// redisProducer only enqueues synchronization work. It lets processes such as
// the judge worker publish follow-up work without also starting sync consumers.
type redisProducer struct {
	queue *queue
}

func NewProducer() Producer {
	return &redisProducer{queue: newQueue()}
}

func (p *redisProducer) EnqueueProblemUpsert(ctx context.Context, problemID uint) error {
	esErr := p.queue.enqueue(ctx, NewTask(TargetES, ActionProblemUpsert, problemID))
	ragErr := p.queue.enqueue(ctx, NewTask(TargetRAG, ActionProblemUpsert, problemID))
	return errors.Join(esErr, ragErr)
}

func (p *redisProducer) EnqueueProblemDelete(ctx context.Context, problemID uint) error {
	esErr := p.queue.enqueue(ctx, NewTask(TargetES, ActionProblemDelete, problemID))
	ragErr := p.queue.enqueue(ctx, NewTask(TargetRAG, ActionProblemDelete, problemID))
	return errors.Join(esErr, ragErr)
}

func (p *redisProducer) EnqueueUserScoreSync(ctx context.Context, userID uint) error {
	return p.queue.enqueue(ctx, NewTask(TargetLeaderboard, ActionUserScoreSync, userID))
}
