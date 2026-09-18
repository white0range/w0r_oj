package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"gojo/infrastructure/cache"

	"github.com/redis/go-redis/v9"
)

const distributedEventChannel = "realtime:events"

type distributedEvent struct {
	UserID uint            `json:"user_id"`
	Name   string          `json:"name"`
	Data   json.RawMessage `json:"data"`
}

// PublishDistributed publishes a best-effort realtime notification. Durable
// state must already be stored in MySQL before this function is called because
// Redis Pub/Sub intentionally does not retain messages for offline clients.
func PublishDistributed(ctx context.Context, userID uint, name string, data any) error {
	if cache.Rdb == nil {
		return errors.New("redis client is not initialized")
	}
	payload, err := encodeDistributedEvent(userID, name, data)
	if err != nil {
		return err
	}
	return cache.Rdb.Publish(ctx, distributedEventChannel, payload).Err()
}

// StartDistributedBridge subscribes before returning, then forwards Redis
// events into this API process's in-memory SSE subscribers until ctx ends.
func StartDistributedBridge(ctx context.Context) (<-chan error, error) {
	if cache.Rdb == nil {
		return nil, errors.New("redis client is not initialized")
	}

	pubsub := cache.Rdb.Subscribe(ctx, distributedEventChannel)
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, fmt.Errorf("subscribe realtime events: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		defer close(done)
		defer pubsub.Close()
		done <- runDistributedBridge(ctx, pubsub)
	}()
	return done, nil
}

func runDistributedBridge(ctx context.Context, pubsub *redis.PubSub) error {
	messages := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case message, ok := <-messages:
			if !ok {
				if ctx.Err() != nil {
					return nil
				}
				return errors.New("realtime Redis subscription closed unexpectedly")
			}

			event, data, err := decodeDistributedEvent([]byte(message.Payload))
			if err != nil {
				log.Printf("discard malformed realtime event: %v", err)
				continue
			}
			Publish(event.UserID, event.Name, data)
		}
	}
}

func encodeDistributedEvent(userID uint, name string, data any) ([]byte, error) {
	if userID == 0 || strings.TrimSpace(name) == "" {
		return nil, errors.New("invalid realtime event")
	}
	rawData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal realtime event data: %w", err)
	}
	return json.Marshal(distributedEvent{UserID: userID, Name: name, Data: rawData})
}

func decodeDistributedEvent(payload []byte) (distributedEvent, any, error) {
	var event distributedEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return event, nil, fmt.Errorf("decode realtime event: %w", err)
	}
	if event.UserID == 0 || strings.TrimSpace(event.Name) == "" || len(event.Data) == 0 {
		return event, nil, errors.New("invalid realtime event")
	}

	var data any
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return event, nil, fmt.Errorf("decode realtime event data: %w", err)
	}
	return event, data, nil
}
