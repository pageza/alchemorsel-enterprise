package performance

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/infrastructure/websocket"
	"github.com/google/uuid"
	gorillaws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WebSocketPerformanceMetrics tracks performance metrics during testing
type WebSocketPerformanceMetrics struct {
	ConnectionsEstablished  int64
	MessagesSent           int64
	MessagesReceived       int64
	ConnectionErrors       int64
	MessageErrors          int64
	TotalConnectionTime    time.Duration
	TotalResponseTime      time.Duration
	MaxConnectionTime      time.Duration
	MaxResponseTime        time.Duration
	MinConnectionTime      time.Duration
	MinResponseTime        time.Duration
	mutex                  sync.RWMutex
}

// NewWebSocketPerformanceMetrics creates new performance metrics tracker
func NewWebSocketPerformanceMetrics() *WebSocketPerformanceMetrics {
	return &WebSocketPerformanceMetrics{
		MinConnectionTime: time.Hour, // Initialize to high value
		MinResponseTime:   time.Hour,
	}
}

// RecordConnection records connection establishment metrics
func (m *WebSocketPerformanceMetrics) RecordConnection(duration time.Duration, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if success {
		atomic.AddInt64(&m.ConnectionsEstablished, 1)
		m.TotalConnectionTime += duration
		
		if duration > m.MaxConnectionTime {
			m.MaxConnectionTime = duration
		}
		if duration < m.MinConnectionTime {
			m.MinConnectionTime = duration
		}
	} else {
		atomic.AddInt64(&m.ConnectionErrors, 1)
	}
}

// RecordMessage records message sending/receiving metrics
func (m *WebSocketPerformanceMetrics) RecordMessage(responseTime time.Duration, sent bool, success bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if sent {
		atomic.AddInt64(&m.MessagesSent, 1)
	}
	
	if success {
		atomic.AddInt64(&m.MessagesReceived, 1)
		m.TotalResponseTime += responseTime
		
		if responseTime > m.MaxResponseTime {
			m.MaxResponseTime = responseTime
		}
		if responseTime < m.MinResponseTime {
			m.MinResponseTime = responseTime
		}
	} else {
		atomic.AddInt64(&m.MessageErrors, 1)
	}
}

// GetAverageConnectionTime returns average connection establishment time
func (m *WebSocketPerformanceMetrics) GetAverageConnectionTime() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.ConnectionsEstablished == 0 {
		return 0
	}
	return m.TotalConnectionTime / time.Duration(m.ConnectionsEstablished)
}

// GetAverageResponseTime returns average message response time
func (m *WebSocketPerformanceMetrics) GetAverageResponseTime() time.Duration {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if m.MessagesReceived == 0 {
		return 0
	}
	return m.TotalResponseTime / time.Duration(m.MessagesReceived)
}

// GetReport generates a performance report
func (m *WebSocketPerformanceMetrics) GetReport() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return map[string]interface{}{
		"connections_established":    m.ConnectionsEstablished,
		"connection_errors":         m.ConnectionErrors,
		"messages_sent":             m.MessagesSent,
		"messages_received":         m.MessagesReceived,
		"message_errors":            m.MessageErrors,
		"avg_connection_time_ms":    m.GetAverageConnectionTime().Milliseconds(),
		"max_connection_time_ms":    m.MaxConnectionTime.Milliseconds(),
		"min_connection_time_ms":    m.MinConnectionTime.Milliseconds(),
		"avg_response_time_ms":      m.GetAverageResponseTime().Milliseconds(),
		"max_response_time_ms":      m.MaxResponseTime.Milliseconds(),
		"min_response_time_ms":      m.MinResponseTime.Milliseconds(),
		"connection_success_rate":   float64(m.ConnectionsEstablished) / float64(m.ConnectionsEstablished + m.ConnectionErrors),
		"message_success_rate":      float64(m.MessagesReceived) / float64(m.MessagesSent),
	}
}

// TestWebSocketConcurrentConnections tests concurrent connection handling
func TestWebSocketConcurrentConnections(t *testing.T) {
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	// Start manager
	go manager.Start(ctx)
	
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "test-user-" + uuid.New().String()
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	testCases := []struct {
		name        string
		connections int
		duration    time.Duration
	}{
		{"Low Load", 10, 30 * time.Second},
		{"Medium Load", 50, 30 * time.Second},
		{"High Load", 100, 30 * time.Second},
		{"Stress Test", 250, 30 * time.Second},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metrics := NewWebSocketPerformanceMetrics()
			
			var wg sync.WaitGroup
			ctx, cancel := context.WithTimeout(context.Background(), tc.duration)
			defer cancel()
			
			// Create concurrent connections
			for i := 0; i < tc.connections; i++ {
				wg.Add(1)
				go func(connID int) {
					defer wg.Done()
					
					userID := fmt.Sprintf("perf-user-%d-%s", connID, uuid.New().String())
					wsURL := "ws" + server.URL[4:] + "?user_id=" + userID
					
					start := time.Now()
					conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
					connectionTime := time.Since(start)
					
					if err != nil {
						metrics.RecordConnection(connectionTime, false)
						return
					}
					defer conn.Close()
					
					metrics.RecordConnection(connectionTime, true)
					
					// Send periodic messages
					ticker := time.NewTicker(1 * time.Second)
					defer ticker.Stop()
					
					messageCount := 0
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							messageCount++
							
							msg := websocket.Message{
								ID:        uuid.New().String(),
								Type:      "ping",
								Content:   fmt.Sprintf("Message %d from connection %d", messageCount, connID),
								Timestamp: time.Now(),
							}
							
							start := time.Now()
							err := conn.WriteJSON(msg)
							if err != nil {
								metrics.RecordMessage(0, true, false)
								continue
							}
							metrics.RecordMessage(0, true, true)
							
							// Read response
							var response websocket.Message
							conn.SetReadDeadline(time.Now().Add(5 * time.Second))
							err = conn.ReadJSON(&response)
							responseTime := time.Since(start)
							
							if err != nil {
								metrics.RecordMessage(responseTime, false, false)
							} else {
								metrics.RecordMessage(responseTime, false, true)
							}
						}
					}
				}(i)
			}
			
			// Wait for test completion
			wg.Wait()
			
			// Generate performance report
			report := metrics.GetReport()
			
			t.Logf("Performance Test Results for %s:", tc.name)
			t.Logf("  Connections Established: %d/%d", report["connections_established"], tc.connections)
			t.Logf("  Connection Success Rate: %.2f%%", report["connection_success_rate"].(float64)*100)
			t.Logf("  Average Connection Time: %dms", report["avg_connection_time_ms"])
			t.Logf("  Max Connection Time: %dms", report["max_connection_time_ms"])
			t.Logf("  Messages Sent: %d", report["messages_sent"])
			t.Logf("  Messages Received: %d", report["messages_received"])
			t.Logf("  Message Success Rate: %.2f%%", report["message_success_rate"].(float64)*100)
			t.Logf("  Average Response Time: %dms", report["avg_response_time_ms"])
			t.Logf("  Max Response Time: %dms", report["max_response_time_ms"])
			
			// Assertions for performance requirements
			assert.GreaterOrEqual(t, report["connection_success_rate"].(float64), 0.95, "Connection success rate should be >= 95%")
			assert.LessOrEqual(t, report["avg_connection_time_ms"], int64(1000), "Average connection time should be <= 1000ms")
			assert.LessOrEqual(t, report["avg_response_time_ms"], int64(100), "Average response time should be <= 100ms")
			assert.GreaterOrEqual(t, report["message_success_rate"].(float64), 0.98, "Message success rate should be >= 98%")
		})
	}
}

// TestWebSocketMemoryUsage tests memory usage under load
func TestWebSocketMemoryUsage(t *testing.T) {
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go manager.Start(ctx)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "memory-test-user-" + uuid.New().String()
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	t.Run("Memory Usage Under Load", func(t *testing.T) {
		const numConnections = 100
		const testDuration = 30 * time.Second
		
		var connections []*gorillaws.Conn
		defer func() {
			for _, conn := range connections {
				conn.Close()
			}
		}()
		
		// Establish connections
		for i := 0; i < numConnections; i++ {
			userID := fmt.Sprintf("memory-user-%d", i)
			wsURL := "ws" + server.URL[4:] + "?user_id=" + userID
			
			conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
			require.NoError(t, err, "Connection %d should succeed", i)
			connections = append(connections, conn)
		}
		
		// Verify all connections are tracked
		assert.Equal(t, numConnections, manager.GetConnectionCount())
		
		// Send messages periodically
		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()
		
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		
		messagesSent := 0
		for {
			select {
			case <-ctx.Done():
				goto TestComplete
			case <-ticker.C:
				for i, conn := range connections {
					msg := websocket.Message{
						ID:        uuid.New().String(),
						Type:      "test_message",
						Content:   fmt.Sprintf("Memory test message %d from conn %d", messagesSent, i),
						Timestamp: time.Now(),
					}
					
					err := conn.WriteJSON(msg)
					if err != nil {
						t.Logf("Failed to send message to connection %d: %v", i, err)
					} else {
						messagesSent++
					}
				}
			}
		}
		
	TestComplete:
		t.Logf("Sent %d messages across %d connections", messagesSent, numConnections)
		t.Logf("Manager reports %d active connections", manager.GetConnectionCount())
		
		// Close half the connections and verify cleanup
		for i := 0; i < numConnections/2; i++ {
			connections[i].Close()
		}
		
		// Wait for cleanup
		time.Sleep(2 * time.Second)
		
		// Connection count should decrease (eventual consistency)
		assert.Eventually(t, func() bool {
			return manager.GetConnectionCount() <= numConnections/2+5 // Allow some tolerance
		}, 10*time.Second, 500*time.Millisecond, "Connection count should decrease after closing connections")
	})
}

// TestWebSocketMessageThroughput tests message throughput capabilities
func TestWebSocketMessageThroughput(t *testing.T) {
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go manager.Start(ctx)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "throughput-user-" + uuid.New().String()
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	testCases := []struct {
		name            string
		connections     int
		messagesPerConn int
		targetThroughput int // messages per second
	}{
		{"Low Throughput", 10, 100, 50},
		{"Medium Throughput", 25, 200, 200},
		{"High Throughput", 50, 300, 500},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var connections []*gorillaws.Conn
			defer func() {
				for _, conn := range connections {
					conn.Close()
				}
			}()
			
			// Establish connections
			for i := 0; i < tc.connections; i++ {
				userID := fmt.Sprintf("throughput-user-%d", i)
				wsURL := "ws" + server.URL[4:] + "?user_id=" + userID
				
				conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
				require.NoError(t, err)
				connections = append(connections, conn)
			}
			
			var messagesSent int64
			var messagesReceived int64
			var errors int64
			
			start := time.Now()
			
			// Send messages concurrently
			var wg sync.WaitGroup
			for i, conn := range connections {
				wg.Add(1)
				go func(connIndex int, c *gorillaws.Conn) {
					defer wg.Done()
					
					for j := 0; j < tc.messagesPerConn; j++ {
						msg := websocket.Message{
							ID:        uuid.New().String(),
							Type:      "throughput_test",
							Content:   fmt.Sprintf("Throughput message %d from connection %d", j, connIndex),
							Timestamp: time.Now(),
						}
						
						err := c.WriteJSON(msg)
						if err != nil {
							atomic.AddInt64(&errors, 1)
						} else {
							atomic.AddInt64(&messagesSent, 1)
						}
						
						// Try to read response (non-blocking)
						c.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
						var response websocket.Message
						err = c.ReadJSON(&response)
						if err == nil {
							atomic.AddInt64(&messagesReceived, 1)
						}
					}
				}(i, conn)
			}
			
			wg.Wait()
			duration := time.Since(start)
			
			actualThroughput := float64(messagesSent) / duration.Seconds()
			
			t.Logf("Throughput Test Results for %s:", tc.name)
			t.Logf("  Duration: %v", duration)
			t.Logf("  Messages Sent: %d", messagesSent)
			t.Logf("  Messages Received: %d", messagesReceived)
			t.Logf("  Errors: %d", errors)
			t.Logf("  Actual Throughput: %.2f messages/second", actualThroughput)
			t.Logf("  Target Throughput: %d messages/second", tc.targetThroughput)
			t.Logf("  Error Rate: %.2f%%", float64(errors)/float64(messagesSent)*100)
			
			// Assertions
			assert.GreaterOrEqual(t, actualThroughput, float64(tc.targetThroughput)*0.8, "Should achieve at least 80% of target throughput")
			assert.LessOrEqual(t, float64(errors)/float64(messagesSent), 0.05, "Error rate should be <= 5%")
		})
	}
}

// TestWebSocketConnectionStability tests connection stability over time
func TestWebSocketConnectionStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stability test in short mode")
	}
	
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go manager.Start(ctx)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "stability-user-" + uuid.New().String()
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	t.Run("Long Running Connections", func(t *testing.T) {
		const numConnections = 20
		const testDuration = 2 * time.Minute
		
		var connections []*gorillaws.Conn
		defer func() {
			for _, conn := range connections {
				conn.Close()
			}
		}()
		
		// Establish connections
		for i := 0; i < numConnections; i++ {
			userID := fmt.Sprintf("stability-user-%d", i)
			wsURL := "ws" + server.URL[4:] + "?user_id=" + userID
			
			conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
			require.NoError(t, err)
			connections = append(connections, conn)
		}
		
		var activeConnections int64 = numConnections
		var messagesSent int64
		var messagesReceived int64
		var connectionDrops int64
		
		ctx, cancel := context.WithTimeout(context.Background(), testDuration)
		defer cancel()
		
		// Start message sending goroutines
		var wg sync.WaitGroup
		for i, conn := range connections {
			wg.Add(1)
			go func(connIndex int, c *gorillaws.Conn) {
				defer wg.Done()
				defer atomic.AddInt64(&activeConnections, -1)
				
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				
				for {
					select {
					case <-ctx.Done():
						return
					case <-ticker.C:
						msg := websocket.Message{
							ID:        uuid.New().String(),
							Type:      "stability_test",
							Content:   fmt.Sprintf("Stability message from connection %d", connIndex),
							Timestamp: time.Now(),
						}
						
						err := c.WriteJSON(msg)
						if err != nil {
							atomic.AddInt64(&connectionDrops, 1)
							return
						}
						atomic.AddInt64(&messagesSent, 1)
						
						// Try to read pong response
						c.SetReadDeadline(time.Now().Add(2 * time.Second))
						var response websocket.Message
						err = c.ReadJSON(&response)
						if err == nil {
							atomic.AddInt64(&messagesReceived, 1)
						}
					}
				}
			}(i, conn)
		}
		
		// Monitor connection health
		healthTicker := time.NewTicker(30 * time.Second)
		defer healthTicker.Stop()
		
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-healthTicker.C:
					activeCount := atomic.LoadInt64(&activeConnections)
					managerCount := manager.GetConnectionCount()
					t.Logf("Health Check - Active: %d, Manager: %d, Sent: %d, Received: %d, Drops: %d", 
						activeCount, managerCount, 
						atomic.LoadInt64(&messagesSent), 
						atomic.LoadInt64(&messagesReceived),
						atomic.LoadInt64(&connectionDrops))
				}
			}
		}()
		
		wg.Wait()
		
		finalSent := atomic.LoadInt64(&messagesSent)
		finalReceived := atomic.LoadInt64(&messagesReceived)
		finalDrops := atomic.LoadInt64(&connectionDrops)
		
		t.Logf("Stability Test Results:")
		t.Logf("  Test Duration: %v", testDuration)
		t.Logf("  Initial Connections: %d", numConnections)
		t.Logf("  Connection Drops: %d", finalDrops)
		t.Logf("  Messages Sent: %d", finalSent)
		t.Logf("  Messages Received: %d", finalReceived)
		t.Logf("  Connection Stability: %.2f%%", float64(numConnections-finalDrops)/float64(numConnections)*100)
		t.Logf("  Message Success Rate: %.2f%%", float64(finalReceived)/float64(finalSent)*100)
		
		// Assertions for stability
		assert.LessOrEqual(t, float64(finalDrops)/float64(numConnections), 0.1, "Connection drop rate should be <= 10%")
		assert.GreaterOrEqual(t, float64(finalReceived)/float64(finalSent), 0.9, "Message success rate should be >= 90%")
	})
}

// TestWebSocketResourceCleanup tests proper resource cleanup
func TestWebSocketResourceCleanup(t *testing.T) {
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go manager.Start(ctx)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "cleanup-user-" + uuid.New().String()
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	t.Run("Resource Cleanup", func(t *testing.T) {
		const cycles = 5
		const connectionsPerCycle = 50
		
		for cycle := 0; cycle < cycles; cycle++ {
			t.Logf("Running cleanup cycle %d/%d", cycle+1, cycles)
			
			var connections []*gorillaws.Conn
			
			// Create connections
			for i := 0; i < connectionsPerCycle; i++ {
				userID := fmt.Sprintf("cleanup-user-%d-%d", cycle, i)
				wsURL := "ws" + server.URL[4:] + "?user_id=" + userID
				
				conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
				require.NoError(t, err)
				connections = append(connections, conn)
			}
			
			// Verify connections are tracked
			assert.Equal(t, connectionsPerCycle, manager.GetConnectionCount())
			
			// Send some messages
			for i, conn := range connections {
				msg := websocket.Message{
					ID:        uuid.New().String(),
					Type:      "cleanup_test",
					Content:   fmt.Sprintf("Cleanup test message %d", i),
					Timestamp: time.Now(),
				}
				conn.WriteJSON(msg)
			}
			
			// Close all connections
			for _, conn := range connections {
				conn.Close()
			}
			
			// Wait for cleanup
			assert.Eventually(t, func() bool {
				return manager.GetConnectionCount() == 0
			}, 10*time.Second, 100*time.Millisecond, "All connections should be cleaned up")
			
			t.Logf("Cycle %d completed - connections cleaned up", cycle+1)
		}
		
		t.Logf("Resource cleanup test completed successfully")
	})
}

// BenchmarkWebSocketConnection benchmarks connection establishment
func BenchmarkWebSocketConnection(b *testing.B) {
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go manager.Start(ctx)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "bench-user-" + uuid.New().String()
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			userID := "bench-user-" + uuid.New().String()
			wsURL := "ws" + server.URL[4:] + "?user_id=" + userID
			
			conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
			if err != nil {
				b.Errorf("Failed to connect: %v", err)
				continue
			}
			conn.Close()
		}
	})
}

// BenchmarkWebSocketMessage benchmarks message sending
func BenchmarkWebSocketMessage(b *testing.B) {
	manager := websocket.NewManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	go manager.Start(ctx)
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			userID = "bench-user-" + uuid.New().String()
		}
		
		err := manager.HandleUpgrade(w, r, userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	
	// Establish connection
	userID := "bench-user-" + uuid.New().String()
	wsURL := "ws" + server.URL[4:] + "?user_id=" + userID
	
	conn, _, err := gorillaws.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		msg := websocket.Message{
			ID:        uuid.New().String(),
			Type:      "benchmark",
			Content:   fmt.Sprintf("Benchmark message %d", i),
			Timestamp: time.Now(),
		}
		
		err := conn.WriteJSON(msg)
		if err != nil {
			b.Errorf("Failed to send message: %v", err)
		}
	}
}