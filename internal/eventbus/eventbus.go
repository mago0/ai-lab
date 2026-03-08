package eventbus

import (
	"sync"
	"time"
)

// Event represents an activity event in the system.
type Event struct {
	Source    string    `json:"source"`
	Type     string    `json:"type"`
	Summary  string    `json:"summary"`
	SessionID string   `json:"session_id,omitempty"`
	Metadata  any      `json:"metadata,omitempty"`
	Time     time.Time `json:"time"`
}

// EventBus provides a simple pub/sub mechanism for internal events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[int]chan Event
	nextID      int
}

// New creates a new EventBus.
func New() *EventBus {
	return &EventBus{
		subscribers: make(map[int]chan Event),
	}
}

// Subscribe returns a channel that receives all published events.
// Call the returned function to unsubscribe.
func (eb *EventBus) Subscribe() (<-chan Event, func()) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	id := eb.nextID
	eb.nextID++

	ch := make(chan Event, 50)
	eb.subscribers[id] = ch

	unsub := func() {
		eb.mu.Lock()
		defer eb.mu.Unlock()
		delete(eb.subscribers, id)
		close(ch)
	}

	return ch, unsub
}

// Publish sends an event to all subscribers.
func (eb *EventBus) Publish(evt Event) {
	if evt.Time.IsZero() {
		evt.Time = time.Now()
	}

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, ch := range eb.subscribers {
		select {
		case ch <- evt:
		default:
			// Drop if subscriber is too slow
		}
	}
}
