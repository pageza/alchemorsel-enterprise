// Package messaging provides comprehensive tests for Redis Message Bus implementation
package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/ports/outbound"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// MockRedisClient is a mock implementation of RedisClientInterface
type MockRedisClient struct {
	mock.Mock
	pubsubChannels map[string]chan *redis.Message
	mu             sync.RWMutex
}

func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		pubsubChannels: make(map[string]chan *redis.Message),
	}
}

func (m *MockRedisClient) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	args := m.Called(ctx, channel, message)

	// Simulate publishing by sending to subscribers
	m.mu.RLock()
	if ch, exists := m.pubsubChannels[channel]; exists {
		var payload string
		if dataBytes, ok := message.([]byte); ok {
			payload = string(dataBytes)
		} else if dataString, ok := message.(string); ok {
			payload = dataString
		} else {
			payload = fmt.Sprintf("%v", message)
		}

		select {
		case ch <- &redis.Message{
			Channel: channel,
			Payload: payload,
		}:
		default:
			// Channel full, drop message (non-blocking)
		}
	}
	m.mu.RUnlock()

	// Return the command that was set up in the mock expectation
	return args.Get(0).(*redis.IntCmd)
}

func (m *MockRedisClient) Subscribe(ctx context.Context, channels ...string) PubSubInterface {
	args := m.Called(ctx, channels)

	if args.Get(0) == nil {
		return nil
	}

	// Return the pubsub mock that was set up in the expectation
	pubsub := args.Get(0).(*MockPubSub)

	// Create channels for each subscription
	m.mu.Lock()
	for _, channel := range channels {
		if _, exists := m.pubsubChannels[channel]; !exists {
			m.pubsubChannels[channel] = make(chan *redis.Message, 100)
		}
	}
	m.mu.Unlock()

	return pubsub
}

func (m *MockRedisClient) Pipeline() PipelineInterface {
	args := m.Called()
	return args.Get(0).(PipelineInterface)
}

func (m *MockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	args := m.Called(ctx)
	return args.Get(0).(*redis.StatusCmd)
}

// MockPubSub wraps redis.PubSub for testing
type MockPubSub struct {
	mock.Mock
	channels []string
	client   *MockRedisClient
	ctx      context.Context
	closed   bool
	mu       sync.Mutex
}

func (m *MockPubSub) Receive(ctx context.Context) (interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0), args.Error(1)
}

func (m *MockPubSub) Channel() <-chan *redis.Message {
	// Return a combined channel that receives from all subscribed channels
	combined := make(chan *redis.Message, 100)

	go func() {
		defer close(combined)

		for {
			select {
			case <-m.ctx.Done():
				return
			default:
				for _, channel := range m.channels {
					m.client.mu.RLock()
					ch, exists := m.client.pubsubChannels[channel]
					m.client.mu.RUnlock()

					if exists {
						select {
						case msg, ok := <-ch:
							if !ok {
								return
							}
							combined <- msg
						case <-time.After(10 * time.Millisecond):
							// Continue to next channel
						}
					}
				}
			}
		}
	}()

	return combined
}

func (m *MockPubSub) Close() error {
	args := m.Called()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return args.Error(0)
	}

	m.closed = true

	// Close channels if client is set
	if m.client != nil {
		m.client.mu.Lock()
		for _, channel := range m.channels {
			if ch, exists := m.client.pubsubChannels[channel]; exists {
				close(ch)
				delete(m.client.pubsubChannels, channel)
			}
		}
		m.client.mu.Unlock()
	}

	return args.Error(0)
}

// MockPipeliner is a mock implementation of redis.Pipeliner
type MockPipeliner struct {
	mock.Mock
	commands []interface{}
}

func NewMockPipeliner() *MockPipeliner {
	return &MockPipeliner{
		commands: make([]interface{}, 0),
	}
}

func (m *MockPipeliner) Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd {
	m.commands = append(m.commands, map[string]interface{}{
		"command": "publish",
		"channel": channel,
		"message": message,
	})

	cmd := redis.NewIntCmd(ctx, "publish", channel, message)
	cmd.SetVal(1)
	return cmd
}

func (m *MockPipeliner) Exec(ctx context.Context) ([]redis.Cmder, error) {
	args := m.Called(ctx)

	// Return what was set up in the mock expectation
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	// Return mock commands for each publish if successful
	var commands []redis.Cmder
	for range m.commands {
		cmd := redis.NewIntCmd(ctx, "publish")
		cmd.SetVal(1)
		commands = append(commands, cmd)
	}

	return commands, args.Error(1)
}

// Test setup helper
func setupMessageBus(t *testing.T) (*RedisMessageBus, *MockRedisClient) {
	mockClient := NewMockRedisClient()
	logger := zaptest.NewLogger(t)

	bus := NewRedisMessageBus(mockClient, logger)

	return bus, mockClient
}

// TestNewRedisMessageBus tests the constructor
func TestNewRedisMessageBus(t *testing.T) {
	mockClient := NewMockRedisClient()
	logger := zaptest.NewLogger(t)

	bus := NewRedisMessageBus(mockClient, logger)

	assert.NotNil(t, bus)
	assert.Equal(t, mockClient, bus.client)
	assert.NotNil(t, bus.logger)
	assert.NotNil(t, bus.subscribers)
	assert.NotNil(t, bus.ctx)
	assert.NotNil(t, bus.cancel)
}

// TestRedisMessageBusPublish tests the Publish method
func TestRedisMessageBusPublish(t *testing.T) {
	tests := []struct {
		name          string
		topic         string
		message       outbound.Message
		setupMocks    func(*MockRedisClient)
		expectedError string
		expectError   bool
	}{
		{
			name:  "successful publish",
			topic: "test.topic",
			message: outbound.Message{
				ID:       "test-id",
				Type:     "test.event",
				Payload:  []byte(`{"data":"test"}`),
				Metadata: map[string]string{"source": "test"},
			},
			setupMocks: func(m *MockRedisClient) {
				m.On("Publish", mock.Anything, "test.topic", mock.MatchedBy(func(data interface{}) bool {
					if dataBytes, ok := data.([]byte); ok {
						var msg outbound.Message
						return json.Unmarshal(dataBytes, &msg) == nil
					}
					if dataString, ok := data.(string); ok {
						var msg outbound.Message
						return json.Unmarshal([]byte(dataString), &msg) == nil
					}
					return false
				})).Return(redis.NewIntCmd(context.Background(), "publish")).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:  "message with auto-generated ID and timestamp",
			topic: "test.topic",
			message: outbound.Message{
				Type:    "test.event",
				Payload: []byte(`{"data":"test"}`),
			},
			setupMocks: func(m *MockRedisClient) {
				m.On("Publish", mock.Anything, "test.topic", mock.MatchedBy(func(data interface{}) bool {
					var msg outbound.Message
					var err error
					if dataBytes, ok := data.([]byte); ok {
						err = json.Unmarshal(dataBytes, &msg)
					} else if dataString, ok := data.(string); ok {
						err = json.Unmarshal([]byte(dataString), &msg)
					} else {
						return false
					}
					return err == nil && msg.ID != "" && !msg.Timestamp.IsZero()
				})).Return(redis.NewIntCmd(context.Background(), "publish")).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:          "empty topic",
			topic:         "",
			message:       outbound.Message{Type: "test.event"},
			setupMocks:    func(m *MockRedisClient) {},
			expectedError: "topic cannot be empty",
			expectError:   true,
		},
		{
			name:  "redis publish error",
			topic: "test.topic",
			message: outbound.Message{
				Type: "test.event",
			},
			setupMocks: func(m *MockRedisClient) {
				cmd := redis.NewIntCmd(context.Background(), "publish")
				cmd.SetErr(errors.New("redis error"))
				m.On("Publish", mock.Anything, "test.topic", mock.Anything).Return(cmd).Once()
			},
			expectedError: "failed to publish message to topic test.topic",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus, mockClient := setupMessageBus(t)
			tt.setupMocks(mockClient)

			ctx := context.Background()
			err := bus.Publish(ctx, tt.topic, tt.message)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestRedisMessageBusPublishBatch tests the PublishBatch method
func TestRedisMessageBusPublishBatch(t *testing.T) {
	tests := []struct {
		name          string
		topic         string
		messages      []outbound.Message
		setupMocks    func(*MockRedisClient)
		expectedError string
		expectError   bool
	}{
		{
			name:  "successful batch publish",
			topic: "test.topic",
			messages: []outbound.Message{
				{Type: "test.event1", Payload: []byte(`{"data":"test1"}`)},
				{Type: "test.event2", Payload: []byte(`{"data":"test2"}`)},
			},
			setupMocks: func(m *MockRedisClient) {
				pipeliner := NewMockPipeliner()
				pipeliner.On("Exec", mock.Anything).Return([]redis.Cmder{}, nil).Once()
				m.On("Pipeline").Return(pipeliner).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:          "empty topic",
			topic:         "",
			messages:      []outbound.Message{{Type: "test.event"}},
			setupMocks:    func(m *MockRedisClient) {},
			expectedError: "topic cannot be empty",
			expectError:   true,
		},
		{
			name:        "empty messages",
			topic:       "test.topic",
			messages:    []outbound.Message{},
			setupMocks:  func(m *MockRedisClient) {},
			expectError: false,
		},
		{
			name:  "pipeline execution error",
			topic: "test.topic",
			messages: []outbound.Message{
				{Type: "test.event", Payload: []byte(`{"data":"test"}`)},
			},
			setupMocks: func(m *MockRedisClient) {
				pipeliner := NewMockPipeliner()
				pipeliner.On("Exec", mock.Anything).Return(nil, errors.New("pipeline error")).Once()
				m.On("Pipeline").Return(pipeliner).Once()
			},
			expectedError: "failed to publish batch messages to topic test.topic",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus, mockClient := setupMessageBus(t)
			tt.setupMocks(mockClient)

			ctx := context.Background()
			err := bus.PublishBatch(ctx, tt.topic, tt.messages)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestRedisMessageBusSubscribe tests the Subscribe method
func TestRedisMessageBusSubscribe(t *testing.T) {
	tests := []struct {
		name          string
		topic         string
		handler       outbound.MessageHandler
		setupMocks    func(*MockRedisClient)
		expectedError string
		expectError   bool
	}{
		{
			name:  "successful subscribe",
			topic: "test.topic",
			handler: func(ctx context.Context, message outbound.Message) error {
				return nil
			},
			setupMocks: func(m *MockRedisClient) {
				pubsub := &MockPubSub{
					channels: []string{"test.topic"},
					client:   m,
					ctx:      context.Background(),
				}
				pubsub.On("Receive", mock.Anything).Return(&redis.Subscription{}, nil).Once()
				m.On("Subscribe", mock.Anything, []string{"test.topic"}).Return(pubsub).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:          "empty topic",
			topic:         "",
			handler:       func(ctx context.Context, message outbound.Message) error { return nil },
			setupMocks:    func(m *MockRedisClient) {},
			expectedError: "topic cannot be empty",
			expectError:   true,
		},
		{
			name:          "nil handler",
			topic:         "test.topic",
			handler:       nil,
			setupMocks:    func(m *MockRedisClient) {},
			expectedError: "handler cannot be nil",
			expectError:   true,
		},
		{
			name:  "subscription receive error",
			topic: "test.topic",
			handler: func(ctx context.Context, message outbound.Message) error {
				return nil
			},
			setupMocks: func(m *MockRedisClient) {
				pubsub := &MockPubSub{
					channels: []string{"test.topic"},
					client:   m,
					ctx:      context.Background(),
				}
				pubsub.On("Receive", mock.Anything).Return(nil, errors.New("receive error")).Once()
				m.On("Subscribe", mock.Anything, []string{"test.topic"}).Return(pubsub).Once()
			},
			expectedError: "failed to subscribe to topic test.topic",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus, mockClient := setupMessageBus(t)
			tt.setupMocks(mockClient)

			ctx := context.Background()
			err := bus.Subscribe(ctx, tt.topic, tt.handler)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestRedisMessageBusUnsubscribe tests the Unsubscribe method
func TestRedisMessageBusUnsubscribe(t *testing.T) {
	tests := []struct {
		name          string
		topic         string
		setupBus      func(*RedisMessageBus, *MockRedisClient)
		expectedError string
		expectError   bool
	}{
		{
			name:  "successful unsubscribe",
			topic: "test.topic",
			setupBus: func(bus *RedisMessageBus, mockClient *MockRedisClient) {
				// Add a subscription
				pubsub := &MockPubSub{
					channels: []string{"test.topic"},
					client:   mockClient,
					ctx:      context.Background(),
				}
				pubsub.On("Close").Return(nil).Once()
				bus.subscribers["test.topic"] = []subscription{{
					handler: func(ctx context.Context, message outbound.Message) error { return nil },
					pubsub:  pubsub,
				}}
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:          "empty topic",
			topic:         "",
			setupBus:      func(bus *RedisMessageBus, mockClient *MockRedisClient) {},
			expectedError: "topic cannot be empty",
			expectError:   true,
		},
		{
			name:          "no subscriptions found",
			topic:         "nonexistent.topic",
			setupBus:      func(bus *RedisMessageBus, mockClient *MockRedisClient) {},
			expectedError: "no subscriptions found for topic nonexistent.topic",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus, mockClient := setupMessageBus(t)
			tt.setupBus(bus, mockClient)

			ctx := context.Background()
			err := bus.Unsubscribe(ctx, tt.topic)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestRedisMessageBusClose tests the Close method
func TestRedisMessageBusClose(t *testing.T) {
	bus, mockClient := setupMessageBus(t)

	// Add some subscriptions
	pubsub1 := &MockPubSub{}
	pubsub1.On("Close").Return(nil).Once()
	pubsub2 := &MockPubSub{}
	pubsub2.On("Close").Return(nil).Once()

	bus.subscribers["topic1"] = []subscription{{
		handler: func(ctx context.Context, message outbound.Message) error { return nil },
		pubsub:  pubsub1,
	}}
	bus.subscribers["topic2"] = []subscription{{
		handler: func(ctx context.Context, message outbound.Message) error { return nil },
		pubsub:  pubsub2,
	}}

	err := bus.Close()

	assert.NoError(t, err)
	assert.Empty(t, bus.subscribers)

	// Verify context was cancelled
	select {
	case <-bus.ctx.Done():
		// Expected
	default:
		t.Error("Expected context to be cancelled")
	}

	mockClient.AssertExpectations(t)
}

// TestRedisMessageBusGetStats tests the GetStats method
func TestRedisMessageBusGetStats(t *testing.T) {
	bus, _ := setupMessageBus(t)

	// Add some mock subscriptions
	bus.subscribers["topic1"] = []subscription{{}, {}} // 2 subscriptions
	bus.subscribers["topic2"] = []subscription{{}}     // 1 subscription

	stats := bus.GetStats()

	assert.Equal(t, 2, stats["total_topics"])
	assert.Equal(t, 3, stats["total_subscriptions"])

	topicStats := stats["topic_stats"].(map[string]int)
	assert.Equal(t, 2, topicStats["topic1"])
	assert.Equal(t, 1, topicStats["topic2"])
}

// TestRedisMessageBusHealthCheck tests the HealthCheck method
func TestRedisMessageBusHealthCheck(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*MockRedisClient)
		expectedError string
		expectError   bool
	}{
		{
			name: "successful health check",
			setupMocks: func(m *MockRedisClient) {
				m.On("Ping", mock.Anything).Return(redis.NewStatusCmd(context.Background(), "ping")).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name: "redis connection failed",
			setupMocks: func(m *MockRedisClient) {
				cmd := redis.NewStatusCmd(context.Background(), "ping")
				cmd.SetErr(errors.New("connection failed"))
				m.On("Ping", mock.Anything).Return(cmd).Once()
			},
			expectedError: "redis connection failed",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus, mockClient := setupMessageBus(t)
			tt.setupMocks(mockClient)

			ctx := context.Background()
			err := bus.HealthCheck(ctx)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestRedisMessageBusMessageFlow tests end-to-end message flow
func TestRedisMessageBusMessageFlow(t *testing.T) {
	t.Run("complete message flow", func(t *testing.T) {
		bus, mockClient := setupMessageBus(t)
		ctx := context.Background()

		// Setup message handling
		var receivedMessages []outbound.Message
		var mu sync.Mutex

		handler := func(ctx context.Context, message outbound.Message) error {
			mu.Lock()
			receivedMessages = append(receivedMessages, message)
			mu.Unlock()
			return nil
		}

		// Mock subscription setup
		pubsub := &MockPubSub{
			channels: []string{"test.flow"},
			client:   mockClient,
			ctx:      ctx,
		}
		pubsub.On("Receive", mock.Anything).Return(&redis.Subscription{}, nil).Once()
		pubsub.On("Close").Return(nil).Once()
		mockClient.On("Subscribe", mock.Anything, []string{"test.flow"}).Return(pubsub).Once()

		// Subscribe
		err := bus.Subscribe(ctx, "test.flow", handler)
		require.NoError(t, err)

		// Mock publish
		mockClient.On("Publish", mock.Anything, "test.flow", mock.Anything).Return(redis.NewIntCmd(ctx, "publish")).Once()

		// Publish message
		testMessage := outbound.Message{
			ID:      uuid.New().String(),
			Type:    "test.message",
			Payload: []byte(`{"data":"test"}`),
			Metadata: map[string]string{
				"source": "test",
			},
			Timestamp: time.Now(),
		}

		err = bus.Publish(ctx, "test.flow", testMessage)
		require.NoError(t, err)

		// Verify subscription exists
		stats := bus.GetStats()
		assert.Equal(t, 1, stats["total_topics"])
		assert.Equal(t, 1, stats["total_subscriptions"])

		// Clean up
		err = bus.Close()
		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})
}

// TestRedisMessageBusEdgeCases tests edge cases and boundary conditions
func TestRedisMessageBusEdgeCases(t *testing.T) {
	t.Run("large message payload", func(t *testing.T) {
		bus, mockClient := setupMessageBus(t)
		defer bus.Close() // Ensure cleanup

		largePayload := make([]byte, 1024*1024) // 1MB payload
		for i := range largePayload {
			largePayload[i] = byte(i % 256)
		}

		message := outbound.Message{
			Type:    "large.message",
			Payload: largePayload,
		}

		mockClient.On("Publish", mock.Anything, "large.topic", mock.Anything).Return(redis.NewIntCmd(context.Background(), "publish")).Once()

		err := bus.Publish(context.Background(), "large.topic", message)
		assert.NoError(t, err)

		mockClient.AssertExpectations(t)
	})

	t.Run("multiple subscriptions to same topic", func(t *testing.T) {
		bus, mockClient := setupMessageBus(t)
		defer bus.Close() // Ensure cleanup
		ctx := context.Background()

		handler1 := func(ctx context.Context, message outbound.Message) error { return nil }
		handler2 := func(ctx context.Context, message outbound.Message) error { return nil }

		// Setup mocks for both subscriptions
		for i := 0; i < 2; i++ {
			pubsub := &MockPubSub{
				channels: []string{"multi.topic"},
				client:   mockClient,
				ctx:      ctx,
			}
			pubsub.On("Receive", mock.Anything).Return(&redis.Subscription{}, nil).Once()
			pubsub.On("Close").Return(nil).Once()
			mockClient.On("Subscribe", mock.Anything, []string{"multi.topic"}).Return(pubsub).Once()
		}

		// Subscribe with both handlers
		err := bus.Subscribe(ctx, "multi.topic", handler1)
		require.NoError(t, err)

		err = bus.Subscribe(ctx, "multi.topic", handler2)
		require.NoError(t, err)

		// Check stats
		stats := bus.GetStats()
		assert.Equal(t, 1, stats["total_topics"])
		assert.Equal(t, 2, stats["total_subscriptions"])

		mockClient.AssertExpectations(t)
	})

	t.Run("context cancellation during subscribe", func(t *testing.T) {
		bus, mockClient := setupMessageBus(t)
		defer bus.Close() // Ensure cleanup

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		handler := func(ctx context.Context, message outbound.Message) error { return nil }

		// Set up mock to expect the Subscribe call with cancelled context
		pubsub := &MockPubSub{
			channels: []string{"cancelled.topic"},
			client:   mockClient,
			ctx:      ctx,
		}
		pubsub.On("Receive", mock.Anything).Return(nil, context.Canceled).Once()
		mockClient.On("Subscribe", mock.Anything, []string{"cancelled.topic"}).Return(pubsub).Once()

		err := bus.Subscribe(ctx, "cancelled.topic", handler)
		// Should fail due to cancelled context
		assert.Error(t, err)

		mockClient.AssertExpectations(t)
	})
}

// BenchmarkRedisMessageBus benchmarks message bus operations
func BenchmarkRedisMessageBus(b *testing.B) {
	bus, mockClient := setupMessageBusBench(b)
	ctx := context.Background()

	// Setup common mock expectations
	mockClient.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(redis.NewIntCmd(ctx, "publish"))

	b.Run("Publish", func(b *testing.B) {
		message := outbound.Message{
			Type:    "benchmark.message",
			Payload: []byte(`{"data":"benchmark"}`),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			bus.Publish(ctx, "benchmark.topic", message)
		}
	})

	b.Run("PublishBatch", func(b *testing.B) {
		messages := make([]outbound.Message, 10)
		for i := range messages {
			messages[i] = outbound.Message{
				Type:    "benchmark.batch",
				Payload: []byte(`{"data":"benchmark"}`),
			}
		}

		pipeliner := NewMockPipeliner()
		pipeliner.On("Exec", mock.Anything).Return([]redis.Cmder{}, nil)
		mockClient.On("Pipeline").Return(pipeliner)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			bus.PublishBatch(ctx, "benchmark.topic", messages)
		}
	})
}

// Benchmark setup helper
func setupMessageBusBench(b *testing.B) (*RedisMessageBus, *MockRedisClient) {
	mockClient := NewMockRedisClient()
	logger := zaptest.NewLogger(b)

	bus := NewRedisMessageBus(mockClient, logger)

	return bus, mockClient
}
