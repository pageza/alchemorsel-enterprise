// Package messaging provides message bus implementations for event-driven architecture
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PubSubInterface defines the interface for Redis PubSub operations we need
type PubSubInterface interface {
	Receive(ctx context.Context) (interface{}, error)
	Channel() <-chan *redis.Message
	Close() error
}

// PipelineInterface defines the interface for Redis pipeline operations we need
type PipelineInterface interface {
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	Exec(ctx context.Context) ([]redis.Cmder, error)
}

// RedisClientInterface defines the interface for Redis operations we need
type RedisClientInterface interface {
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
	Subscribe(ctx context.Context, channels ...string) PubSubInterface
	Pipeline() PipelineInterface
	Ping(ctx context.Context) *redis.StatusCmd
}

// RedisMessageBus implements MessageBus interface using Redis Pub/Sub
type RedisMessageBus struct {
	client      RedisClientInterface
	logger      *zap.Logger
	subscribers map[string][]subscription
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// subscription represents a topic subscription
type subscription struct {
	handler outbound.MessageHandler
	pubsub  PubSubInterface
}

// NewRedisMessageBus creates a new Redis-based message bus
func NewRedisMessageBus(client RedisClientInterface, logger *zap.Logger) *RedisMessageBus {
	ctx, cancel := context.WithCancel(context.Background())

	bus := &RedisMessageBus{
		client:      client,
		logger:      logger.Named("redis-message-bus"),
		subscribers: make(map[string][]subscription),
		ctx:         ctx,
		cancel:      cancel,
	}

	logger.Info("Redis message bus initialized")
	return bus
}

// PubSubWrapper wraps redis.PubSub to implement PubSubInterface
type PubSubWrapper struct {
	pubsub *redis.PubSub
}

func (w *PubSubWrapper) Receive(ctx context.Context) (interface{}, error) {
	return w.pubsub.Receive(ctx)
}

func (w *PubSubWrapper) Channel() <-chan *redis.Message {
	return w.pubsub.Channel()
}

func (w *PubSubWrapper) Close() error {
	return w.pubsub.Close()
}

// PipelineWrapper wraps redis.Pipeliner to implement PipelineInterface
type PipelineWrapper struct {
	pipeline redis.Pipeliner
}

func (w *PipelineWrapper) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	return w.pipeline.Publish(ctx, channel, message)
}

func (w *PipelineWrapper) Exec(ctx context.Context) ([]redis.Cmder, error) {
	return w.pipeline.Exec(ctx)
}

// ClientWrapper wraps redis.UniversalClient to implement RedisClientInterface
type ClientWrapper struct {
	client redis.UniversalClient
}

func (w *ClientWrapper) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	return w.client.Publish(ctx, channel, message)
}

func (w *ClientWrapper) Subscribe(ctx context.Context, channels ...string) PubSubInterface {
	return &PubSubWrapper{pubsub: w.client.Subscribe(ctx, channels...)}
}

func (w *ClientWrapper) Pipeline() PipelineInterface {
	return &PipelineWrapper{pipeline: w.client.Pipeline()}
}

func (w *ClientWrapper) Ping(ctx context.Context) *redis.StatusCmd {
	return w.client.Ping(ctx)
}

// NewRedisMessageBusWithUniversalClient creates a new Redis-based message bus with redis.UniversalClient
func NewRedisMessageBusWithUniversalClient(client redis.UniversalClient, logger *zap.Logger) *RedisMessageBus {
	return NewRedisMessageBus(&ClientWrapper{client: client}, logger)
}

// Publish publishes a message to a topic
func (r *RedisMessageBus) Publish(ctx context.Context, topic string, message outbound.Message) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	// Set timestamp if not provided
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Set ID if not provided
	if message.ID == "" {
		message.ID = uuid.New().String()
	}

	// Serialize message to JSON
	data, err := json.Marshal(message)
	if err != nil {
		r.logger.Error("Failed to serialize message",
			zap.String("topic", topic),
			zap.String("message_id", message.ID),
			zap.Error(err))
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Publish to Redis
	err = r.client.Publish(ctx, topic, data).Err()
	if err != nil {
		r.logger.Error("Failed to publish message",
			zap.String("topic", topic),
			zap.String("message_id", message.ID),
			zap.Error(err))
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	r.logger.Debug("Message published successfully",
		zap.String("topic", topic),
		zap.String("message_id", message.ID),
		zap.String("message_type", message.Type))

	return nil
}

// PublishBatch publishes multiple messages to a topic atomically
func (r *RedisMessageBus) PublishBatch(ctx context.Context, topic string, messages []outbound.Message) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	if len(messages) == 0 {
		return nil // Nothing to publish
	}

	// Use Redis pipeline for batch operations
	pipe := r.client.Pipeline()

	serializedMessages := make([]interface{}, 0, len(messages))

	for i, message := range messages {
		// Set timestamp if not provided
		if message.Timestamp.IsZero() {
			messages[i].Timestamp = time.Now()
		}

		// Set ID if not provided
		if message.ID == "" {
			messages[i].ID = uuid.New().String()
		}

		// Serialize message to JSON
		data, err := json.Marshal(messages[i])
		if err != nil {
			r.logger.Error("Failed to serialize batch message",
				zap.String("topic", topic),
				zap.String("message_id", message.ID),
				zap.Int("message_index", i),
				zap.Error(err))
			return fmt.Errorf("failed to serialize message at index %d: %w", i, err)
		}

		pipe.Publish(ctx, topic, data)
		serializedMessages = append(serializedMessages, data)
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		r.logger.Error("Failed to publish batch messages",
			zap.String("topic", topic),
			zap.Int("message_count", len(messages)),
			zap.Error(err))
		return fmt.Errorf("failed to publish batch messages to topic %s: %w", topic, err)
	}

	r.logger.Debug("Batch messages published successfully",
		zap.String("topic", topic),
		zap.Int("message_count", len(messages)))

	return nil
}

// Subscribe subscribes to a topic with a message handler
func (r *RedisMessageBus) Subscribe(ctx context.Context, topic string, handler outbound.MessageHandler) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	// Create Redis pub/sub subscription
	pubsub := r.client.Subscribe(ctx, topic)

	// Test the subscription
	_, err := pubsub.Receive(ctx)
	if err != nil {
		r.logger.Error("Failed to establish subscription",
			zap.String("topic", topic),
			zap.Error(err))
		return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	// Store subscription
	r.mu.Lock()
	r.subscribers[topic] = append(r.subscribers[topic], subscription{
		handler: handler,
		pubsub:  pubsub,
	})
	r.mu.Unlock()

	r.logger.Info("Subscribed to topic",
		zap.String("topic", topic))

	// Start message processing goroutine
	go r.processMessages(topic, pubsub, handler)

	return nil
}

// processMessages processes incoming messages for a subscription
func (r *RedisMessageBus) processMessages(topic string, pubsub PubSubInterface, handler outbound.MessageHandler) {
	ch := pubsub.Channel()

	for {
		select {
		case <-r.ctx.Done():
			r.logger.Info("Stopping message processing due to context cancellation",
				zap.String("topic", topic))
			return

		case msg, ok := <-ch:
			if !ok {
				r.logger.Warn("Message channel closed",
					zap.String("topic", topic))
				return
			}

			// Deserialize message
			var message outbound.Message
			err := json.Unmarshal([]byte(msg.Payload), &message)
			if err != nil {
				r.logger.Error("Failed to deserialize received message",
					zap.String("topic", topic),
					zap.String("payload", msg.Payload),
					zap.Error(err))
				continue
			}

			// Handle message
			err = handler(r.ctx, message)
			if err != nil {
				r.logger.Error("Message handler failed",
					zap.String("topic", topic),
					zap.String("message_id", message.ID),
					zap.String("message_type", message.Type),
					zap.Error(err))
			} else {
				r.logger.Debug("Message processed successfully",
					zap.String("topic", topic),
					zap.String("message_id", message.ID),
					zap.String("message_type", message.Type))
			}
		}
	}
}

// Unsubscribe unsubscribes from a topic
func (r *RedisMessageBus) Unsubscribe(ctx context.Context, topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	r.mu.Lock()
	subscriptions, exists := r.subscribers[topic]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("no subscriptions found for topic %s", topic)
	}

	// Close all pub/sub connections for this topic
	for _, sub := range subscriptions {
		err := sub.pubsub.Close()
		if err != nil {
			r.logger.Warn("Failed to close pub/sub connection",
				zap.String("topic", topic),
				zap.Error(err))
		}
	}

	// Remove subscriptions
	delete(r.subscribers, topic)
	r.mu.Unlock()

	r.logger.Info("Unsubscribed from topic",
		zap.String("topic", topic),
		zap.Int("subscription_count", len(subscriptions)))

	return nil
}

// Close closes the message bus and all subscriptions
func (r *RedisMessageBus) Close() error {
	r.logger.Info("Closing Redis message bus")

	// Cancel context to stop all message processing
	r.cancel()

	// Close all subscriptions
	r.mu.Lock()
	defer r.mu.Unlock()

	var errors []error
	for topic, subscriptions := range r.subscribers {
		for _, sub := range subscriptions {
			err := sub.pubsub.Close()
			if err != nil {
				r.logger.Error("Failed to close subscription",
					zap.String("topic", topic),
					zap.Error(err))
				errors = append(errors, err)
			}
		}
	}

	// Clear subscribers map
	r.subscribers = make(map[string][]subscription)

	if len(errors) > 0 {
		return fmt.Errorf("failed to close %d subscriptions", len(errors))
	}

	r.logger.Info("Redis message bus closed successfully")
	return nil
}

// GetStats returns message bus statistics
func (r *RedisMessageBus) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	topicStats := make(map[string]int)
	totalSubscriptions := 0

	for topic, subscriptions := range r.subscribers {
		topicStats[topic] = len(subscriptions)
		totalSubscriptions += len(subscriptions)
	}

	return map[string]interface{}{
		"total_topics":        len(r.subscribers),
		"total_subscriptions": totalSubscriptions,
		"topic_stats":         topicStats,
	}
}

// HealthCheck performs a health check on the message bus
func (r *RedisMessageBus) HealthCheck(ctx context.Context) error {
	// Test Redis connection with a ping
	err := r.client.Ping(ctx).Err()
	if err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	return nil
}
