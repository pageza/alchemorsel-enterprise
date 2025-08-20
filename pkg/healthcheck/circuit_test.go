// Package healthcheck circuit breaker tests
// Tests for circuit breaker functionality including failure scenarios and recovery
package healthcheck

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCircuitBreaker(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	assert.NotNil(t, cb)
	assert.Equal(t, "test", cb.name)
	assert.Equal(t, config, cb.config)
	assert.Equal(t, StateClosed, cb.state)
}

func TestNewCircuitBreaker_DefaultValues(t *testing.T) {
	config := CircuitBreakerConfig{} // Empty config
	cb := NewCircuitBreaker("test", config)

	assert.Equal(t, 5, cb.config.FailureThreshold)
	assert.Equal(t, 2, cb.config.SuccessThreshold)
	assert.Equal(t, 30*time.Second, cb.config.Timeout)
	assert.Equal(t, 3, cb.config.MaxRequests)
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(successFunc)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, StateClosed, cb.GetState())
	
	stats := cb.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.TotalSuccesses)
	assert.Equal(t, int64(0), stats.TotalFailures)
	assert.Equal(t, 1, stats.ConsecutiveSuccesses)
	assert.Equal(t, 0, stats.ConsecutiveFailures)
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	result, err := cb.Execute(failureFunc)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, StateClosed, cb.GetState()) // Still closed after single failure
	
	stats := cb.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.TotalSuccesses)
	assert.Equal(t, int64(1), stats.TotalFailures)
	assert.Equal(t, 0, stats.ConsecutiveSuccesses)
	assert.Equal(t, 1, stats.ConsecutiveFailures)
}

func TestCircuitBreaker_Execute_TripsToOpen(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	// Execute failures up to threshold
	for i := 0; i < config.FailureThreshold; i++ {
		result, err := cb.Execute(failureFunc)
		assert.Error(t, err)
		assert.Nil(t, result)
		
		if i < config.FailureThreshold-1 {
			assert.Equal(t, StateClosed, cb.GetState(), "Should remain closed until threshold")
		}
	}

	// Circuit should now be open
	assert.Equal(t, StateOpen, cb.GetState())
	
	stats := cb.GetStats()
	assert.Equal(t, int64(config.FailureThreshold), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.TotalSuccesses)
	assert.Equal(t, int64(config.FailureThreshold), stats.TotalFailures)
	assert.Equal(t, config.FailureThreshold, stats.ConsecutiveFailures)
}

func TestCircuitBreaker_Execute_RejectsWhenOpen(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	assert.Equal(t, StateOpen, cb.GetState())

	// Now try to execute - should be rejected
	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(successFunc)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker 'test' is open")
	assert.Nil(t, result)
	
	stats := cb.GetStats()
	assert.Equal(t, int64(1), stats.TotalRejections)
}

func TestCircuitBreaker_Execute_HalfOpenTransition(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	assert.Equal(t, StateOpen, cb.GetState())

	// Wait for timeout to allow half-open transition
	time.Sleep(config.Timeout + 10*time.Millisecond)

	// Next request should transition to half-open
	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(successFunc)

	assert.NoError(t, err)
	assert.Equal(t, "success", result)
	assert.Equal(t, StateHalfOpen, cb.GetState())
}

func TestCircuitBreaker_Execute_HalfOpenToClosedRecovery(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)

	// Execute successful requests to close the circuit
	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	for i := 0; i < config.SuccessThreshold; i++ {
		result, err := cb.Execute(successFunc)
		assert.NoError(t, err)
		assert.Equal(t, "success", result)
		
		if i < config.SuccessThreshold-1 {
			assert.Equal(t, StateHalfOpen, cb.GetState(), "Should remain half-open until success threshold")
		}
	}

	// Circuit should now be closed
	assert.Equal(t, StateClosed, cb.GetState())
}

func TestCircuitBreaker_Execute_HalfOpenToOpenOnFailure(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)

	// First request transitions to half-open
	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(successFunc)
	assert.NoError(t, err)
	assert.Equal(t, StateHalfOpen, cb.GetState())

	// Next failure should open the circuit again
	result, err = cb.Execute(failureFunc)
	assert.Error(t, err)
	assert.Equal(t, StateOpen, cb.GetState())
}

func TestCircuitBreaker_Execute_MaxRequestsInHalfOpen(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)

	// Execute max requests in half-open state
	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	for i := 0; i < config.MaxRequests; i++ {
		result, err := cb.Execute(successFunc)
		assert.NoError(t, err)
		assert.Equal(t, "success", result)
	}

	// Additional requests should be rejected
	result, err := cb.Execute(successFunc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker 'test' is open")
}

func TestCircuitBreaker_GetStatus(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	status := cb.GetStatus()

	assert.Equal(t, "test", status.Name)
	assert.Equal(t, StateClosed, status.State)
	assert.Equal(t, 0, status.FailureCount)
	assert.Equal(t, 0, status.SuccessCount)
	assert.Equal(t, 0, status.RequestCount)
	assert.True(t, status.LastFailureTime.IsZero())
	assert.True(t, status.LastSuccessTime.IsZero())
	assert.True(t, status.NextAttempt.IsZero())
}

func TestCircuitBreaker_GetStatus_WithFailures(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	// Execute some failures
	for i := 0; i < 2; i++ {
		cb.Execute(failureFunc)
	}

	status := cb.GetStatus()

	assert.Equal(t, StateClosed, status.State)
	assert.Equal(t, 2, status.FailureCount)
	assert.Equal(t, 0, status.SuccessCount)
	assert.Equal(t, 2, status.RequestCount)
	assert.False(t, status.LastFailureTime.IsZero())
	assert.True(t, status.LastSuccessTime.IsZero())
}

func TestCircuitBreaker_GetStatus_WhenOpen(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	status := cb.GetStatus()

	assert.Equal(t, StateOpen, status.State)
	assert.Equal(t, config.FailureThreshold, status.FailureCount)
	assert.False(t, status.NextAttempt.IsZero())
	assert.True(t, status.NextAttempt.After(time.Now()))
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Execute some operations
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	cb.Execute(failureFunc)
	cb.Execute(successFunc)

	// Reset the circuit breaker
	cb.Reset()

	status := cb.GetStatus()
	stats := cb.GetStats()

	assert.Equal(t, StateClosed, status.State)
	assert.Equal(t, 0, status.FailureCount)
	assert.Equal(t, 0, status.SuccessCount)
	assert.Equal(t, 0, status.RequestCount)
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, int64(0), stats.TotalSuccesses)
	assert.Equal(t, int64(0), stats.TotalFailures)
	assert.True(t, status.LastFailureTime.IsZero())
	assert.True(t, status.LastSuccessTime.IsZero())
}

func TestCircuitBreaker_ForceOpen(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	assert.Equal(t, StateClosed, cb.GetState())

	cb.ForceOpen()

	assert.Equal(t, StateOpen, cb.GetState())

	// Should reject requests
	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(successFunc)
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCircuitBreaker_ForceClose(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	assert.Equal(t, StateOpen, cb.GetState())

	cb.ForceClose()

	assert.Equal(t, StateClosed, cb.GetState())

	// Should allow requests again
	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	result, err := cb.Execute(successFunc)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)
}

func TestCircuitBreaker_IsAllowingRequests(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Initially should allow requests
	assert.True(t, cb.IsAllowingRequests())

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	// Should not allow requests when open
	assert.False(t, cb.IsAllowingRequests())

	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)

	// Should allow requests in half-open state
	assert.True(t, cb.IsAllowingRequests())
}

func TestCircuitBreaker_StateChangeCallback(t *testing.T) {
	var stateChanges []struct {
		name string
		from CircuitBreakerState
		to   CircuitBreakerState
	}

	config := TestCircuitBreakerConfig()
	config.OnStateChange = func(name string, from, to CircuitBreakerState) {
		stateChanges = append(stateChanges, struct {
			name string
			from CircuitBreakerState
			to   CircuitBreakerState
		}{name, from, to})
	}

	cb := NewCircuitBreaker("test", config)

	// Trip the circuit breaker
	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(failureFunc)
	}

	// Should have one state change: Closed -> Open
	require.Len(t, stateChanges, 1)
	assert.Equal(t, "test", stateChanges[0].name)
	assert.Equal(t, StateClosed, stateChanges[0].from)
	assert.Equal(t, StateOpen, stateChanges[0].to)

	// Wait for timeout and execute success
	time.Sleep(config.Timeout + 10*time.Millisecond)

	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	cb.Execute(successFunc)

	// Should have another state change: Open -> HalfOpen
	require.Len(t, stateChanges, 2)
	assert.Equal(t, StateOpen, stateChanges[1].from)
	assert.Equal(t, StateHalfOpen, stateChanges[1].to)
}

func TestCircuitBreakerState_String(t *testing.T) {
	assert.Equal(t, "closed", StateClosed.String())
	assert.Equal(t, "half-open", StateHalfOpen.String())
	assert.Equal(t, "open", StateOpen.String())
	assert.Equal(t, "unknown", CircuitBreakerState(999).String())
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig()

	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 2, config.SuccessThreshold)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.MaxRequests)
}

func TestAggressiveCircuitBreakerConfig(t *testing.T) {
	config := AggressiveCircuitBreakerConfig()

	assert.Equal(t, 3, config.FailureThreshold)
	assert.Equal(t, 1, config.SuccessThreshold)
	assert.Equal(t, 10*time.Second, config.Timeout)
	assert.Equal(t, 1, config.MaxRequests)
}

func TestConservativeCircuitBreakerConfig(t *testing.T) {
	config := ConservativeCircuitBreakerConfig()

	assert.Equal(t, 10, config.FailureThreshold)
	assert.Equal(t, 5, config.SuccessThreshold)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.Equal(t, 5, config.MaxRequests)
}

// Benchmark tests
func BenchmarkCircuitBreaker_Execute_Success(b *testing.B) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(successFunc)
	}
}

func BenchmarkCircuitBreaker_Execute_Failure(b *testing.B) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(failureFunc)
		
		// Reset if circuit opens
		if cb.GetState() == StateOpen {
			cb.Reset()
		}
	}
}

func BenchmarkCircuitBreaker_Execute_Open(b *testing.B) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	// Force circuit open
	cb.ForceOpen()

	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Execute(successFunc)
	}
}

// Test concurrent access to circuit breaker
func TestCircuitBreaker_ConcurrentAccess(t *testing.T) {
	config := TestCircuitBreakerConfig()
	cb := NewCircuitBreaker("test", config)

	successFunc := func() (interface{}, error) {
		return "success", nil
	}

	failureFunc := func() (interface{}, error) {
		return nil, fmt.Errorf("operation failed")
	}

	// Run concurrent operations
	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Alternate between success and failure
			if id%2 == 0 {
				cb.Execute(successFunc)
			} else {
				cb.Execute(failureFunc)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify final state is consistent
	stats := cb.GetStats()
	assert.Equal(t, int64(numGoroutines), stats.TotalRequests)
	assert.Equal(t, stats.TotalSuccesses+stats.TotalFailures+stats.TotalRejections, stats.TotalRequests)
}