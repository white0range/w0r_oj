package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"gojo/infrastructure/cache"

	"github.com/redis/go-redis/v9"
)

type TicketScope string

const (
	ScopeEvents   TicketScope = "events"
	ScopeChatTurn TicketScope = "chat_turn"
	ticketTTL                 = time.Minute
)

var ErrInvalidTicket = errors.New("invalid realtime ticket")

type Ticket struct {
	UserID uint        `json:"user_id"`
	Scope  TicketScope `json:"scope"`
	TurnID uint        `json:"turn_id,omitempty"`
}

func CreateTicket(ctx context.Context, userID uint, scope TicketScope, turnID uint) (string, error) {
	if userID == 0 {
		return "", ErrInvalidTicket
	}

	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	payload, err := json.Marshal(Ticket{UserID: userID, Scope: scope, TurnID: turnID})
	if err != nil {
		return "", err
	}
	if err := cache.Rdb.Set(ctx, ticketKey(token), payload, ticketTTL).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// ConsumeTicket atomically reads and deletes a ticket, so a URL ticket cannot
// be reused after the first accepted SSE connection.
func ConsumeTicket(ctx context.Context, token string, scope TicketScope, turnID uint) (Ticket, error) {
	if len(token) != 64 {
		return Ticket{}, ErrInvalidTicket
	}
	if _, err := hex.DecodeString(token); err != nil {
		return Ticket{}, ErrInvalidTicket
	}

	payload, err := cache.Rdb.GetDel(ctx, ticketKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return Ticket{}, ErrInvalidTicket
	}
	if err != nil {
		return Ticket{}, err
	}

	var ticket Ticket
	if err := json.Unmarshal(payload, &ticket); err != nil {
		return Ticket{}, ErrInvalidTicket
	}
	if ticket.UserID == 0 || ticket.Scope != scope || ticket.TurnID != turnID {
		return Ticket{}, ErrInvalidTicket
	}
	return ticket, nil
}

func TicketTTLSeconds() int { return int(ticketTTL.Seconds()) }

func ticketKey(token string) string { return "realtime:ticket:" + token }

type Event struct {
	Name string
	Data any
}

type subscriber struct {
	events chan Event
}

var subscribers = struct {
	sync.Mutex
	byUser            map[uint]map[*subscriber]struct{}
	activeConnections map[uint]int
}{
	byUser:            make(map[uint]map[*subscriber]struct{}),
	activeConnections: make(map[uint]int),
}

// AcquireConnection limits all SSE streams (events and Chat) for a user.
// The caller must invoke the returned function exactly once.
func AcquireConnection(userID uint, maximum int) (func(), bool) {
	if userID == 0 || maximum <= 0 {
		return nil, false
	}

	subscribers.Lock()
	if subscribers.activeConnections[userID] >= maximum {
		subscribers.Unlock()
		return nil, false
	}
	subscribers.activeConnections[userID]++
	subscribers.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			subscribers.Lock()
			defer subscribers.Unlock()
			if subscribers.activeConnections[userID] <= 1 {
				delete(subscribers.activeConnections, userID)
				return
			}
			subscribers.activeConnections[userID]--
		})
	}, true
}

func Subscribe(userID uint) (<-chan Event, func()) {
	sub := &subscriber{events: make(chan Event, 16)}
	subscribers.Lock()
	if subscribers.byUser[userID] == nil {
		subscribers.byUser[userID] = make(map[*subscriber]struct{})
	}
	subscribers.byUser[userID][sub] = struct{}{}
	subscribers.Unlock()

	return sub.events, func() {
		subscribers.Lock()
		defer subscribers.Unlock()
		userSubscribers := subscribers.byUser[userID]
		if _, ok := userSubscribers[sub]; !ok {
			return
		}
		delete(userSubscribers, sub)
		close(sub.events)
		if len(userSubscribers) == 0 {
			delete(subscribers.byUser, userID)
		}
	}
}

func Publish(userID uint, name string, data any) {
	subscribers.Lock()
	defer subscribers.Unlock()
	for sub := range subscribers.byUser[userID] {
		select {
		case sub.events <- Event{Name: name, Data: data}:
		default:
			// A slow browser must not block judge workers. Polling remains the
			// fallback if an event is dropped.
		}
	}
}

func DisconnectUser(userID uint) {
	subscribers.Lock()
	defer subscribers.Unlock()
	for sub := range subscribers.byUser[userID] {
		close(sub.events)
	}
	delete(subscribers.byUser, userID)
}
