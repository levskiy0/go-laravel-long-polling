package redis

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/redis/go-redis/v9"
)

// EventNotification represents a notification from Redis about a new event
type EventNotification struct {
	ChannelID string `json:"channel_id"`
	EventID   int64  `json:"event_id"`
	Timestamp int64  `json:"timestamp"`
}

// Subscriber manages Redis pub/sub subscriptions
type Subscriber struct {
	client   *redis.Client
	channel  string
	logger   *slog.Logger
	handlers map[string][]chan EventNotification
	mu       sync.RWMutex
	cancel   context.CancelFunc
}

// NewSubscriber creates a new Redis subscriber
func NewSubscriber(client *redis.Client, channel string, logger *slog.Logger) *Subscriber {
	return &Subscriber{
		client:   client,
		channel:  channel,
		logger:   logger,
		handlers: make(map[string][]chan EventNotification),
	}
}

// Start begins listening for Redis pub/sub messages
func (s *Subscriber) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	pubsub := s.client.Subscribe(ctx, s.channel)
	defer pubsub.Close()

	s.logger.Info("Redis subscriber started", "channel", s.channel)

	// Wait for subscription confirmation
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}

	ch := pubsub.Channel()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Redis subscriber stopped")
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				// Channel closed - Redis connection lost
				s.logger.Warn("Redis subscription channel closed, reconnection needed")
				return nil
			}
			if msg == nil {
				continue
			}
			s.handleMessage(msg.Payload)
		}
	}
}

// Stop gracefully stops the subscriber
func (s *Subscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Subscribe registers a channel to receive notifications for a specific channel ID
func (s *Subscriber) Subscribe(channelID string) chan EventNotification {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan EventNotification, 10)
	s.handlers[channelID] = append(s.handlers[channelID], ch)

	s.logger.Debug("subscribed to channel", "channel_id", channelID)

	return ch
}

// Unsubscribe removes a notification channel
func (s *Subscriber) Unsubscribe(channelID string, ch chan EventNotification) {
	s.mu.Lock()
	defer s.mu.Unlock()

	handlers := s.handlers[channelID]
	for i, handler := range handlers {
		if handler == ch {
			s.handlers[channelID] = append(handlers[:i], handlers[i+1:]...)
			close(ch)
			break
		}
	}

	if len(s.handlers[channelID]) == 0 {
		delete(s.handlers, channelID)
	}

	s.logger.Debug("unsubscribed from channel", "channel_id", channelID)
}

// handleMessage processes an incoming Redis message
func (s *Subscriber) handleMessage(payload string) {
	var notification EventNotification
	if err := json.Unmarshal([]byte(payload), &notification); err != nil {
		s.logger.Error("failed to parse notification", "error", err, "payload", payload)
		return
	}

	s.logger.Debug("received notification",
		"channel_id", notification.ChannelID,
		"event_id", notification.EventID,
	)

	// Hold RLock for the entire duration to prevent channels from being closed
	// while we're sending to them. This is safe because send with default doesn't block.
	s.mu.RLock()
	defer s.mu.RUnlock()

	handlers := s.handlers[notification.ChannelID]
	for _, handler := range handlers {
		select {
		case handler <- notification:
		default:
			s.logger.Warn("notification channel is full", "channel_id", notification.ChannelID)
		}
	}
}
