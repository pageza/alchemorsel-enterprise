// Package healthcheck circuit breaker implementation
// Provides circuit breaker pattern for health checks to prevent cascading failures
package healthcheck

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	StateClosed CircuitBreakerState = iota
	StateHalfOpen
	StateOpen
)

// String returns the string representation of the state
func (s CircuitBreakerState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateHalfOpen:
		return "half-open"
	case StateOpen:
		return "open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig holds configuration for circuit breaker
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of failures required to open the circuit
	FailureThreshold int `json:"failure_threshold"`

	// SuccessThreshold is the number of successes required to close the circuit when half-open
	SuccessThreshold int `json:"success_threshold"`

	// Timeout is the period after which the circuit breaker will attempt to close
	Timeout time.Duration `json:"timeout"`

	// MaxRequests is the maximum number of requests allowed when half-open
	MaxRequests int `json:"max_requests"`

	// OnStateChange is called when the state changes
	OnStateChange func(name string, from, to CircuitBreakerState)
}

// CircuitBreakerStatus represents the current status of a circuit breaker
type CircuitBreakerStatus struct {
	Name            string              `json:"name"`
	State           CircuitBreakerState `json:"state"`
	FailureCount    int                 `json:"failure_count"`
	SuccessCount    int                 `json:"success_count"`
	RequestCount    int                 `json:"request_count"`
	LastFailureTime time.Time           `json:"last_failure_time,omitempty"`
	LastSuccessTime time.Time           `json:"last_success_time,omitempty"`
	NextAttempt     time.Time           `json:"next_attempt,omitempty"`
}

// CircuitBreakerStats holds statistics about circuit breaker operations
type CircuitBreakerStats struct {
	TotalRequests        int64 `json:"total_requests"`
	TotalSuccesses       int64 `json:"total_successes"`
	TotalFailures        int64 `json:"total_failures"`
	TotalRejections      int64 `json:"total_rejections"`
	ConsecutiveFailures  int   `json:"consecutive_failures"`
	ConsecutiveSuccesses int   `json:"consecutive_successes"`
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	name            string
	config          CircuitBreakerConfig
	state           CircuitBreakerState
	stats           CircuitBreakerStats
	lastFailureTime time.Time
	lastSuccessTime time.Time
	nextAttempt     time.Time
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(name string, config CircuitBreakerConfig) *CircuitBreaker {
	// Set default values if not provided
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 2
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRequests <= 0 {
		config.MaxRequests = 3
	}

	return &CircuitBreaker{
		name:   name,
		config: config,
		state:  StateClosed,
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() (interface{}, error)) (interface{}, error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Increment total requests
	cb.stats.TotalRequests++

	// Check if request should be allowed
	if !cb.allowRequest() {
		cb.stats.TotalRejections++
		return nil, fmt.Errorf("circuit breaker '%s' is open", cb.name)
	}

	// Execute the function
	result, err := fn()

	if err != nil {
		cb.onFailure()
		return nil, err
	}

	cb.onSuccess()
	return result, nil
}

// allowRequest determines if a request should be allowed based on current state
func (cb *CircuitBreaker) allowRequest() bool {
	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if timeout has passed to transition to half-open
		if time.Now().After(cb.nextAttempt) {
			cb.setState(StateHalfOpen)
			return true
		}
		return false
	case StateHalfOpen:
		// Allow limited requests in half-open state
		return cb.stats.ConsecutiveSuccesses < cb.config.MaxRequests
	default:
		return false
	}
}

// onSuccess handles successful execution
func (cb *CircuitBreaker) onSuccess() {
	cb.stats.TotalSuccesses++
	cb.stats.ConsecutiveFailures = 0
	cb.stats.ConsecutiveSuccesses++
	cb.lastSuccessTime = time.Now()

	switch cb.state {
	case StateHalfOpen:
		// Check if we have enough successes to close the circuit
		if cb.stats.ConsecutiveSuccesses >= cb.config.SuccessThreshold {
			cb.setState(StateClosed)
		}
	}
}

// onFailure handles failed execution
func (cb *CircuitBreaker) onFailure() {
	cb.stats.TotalFailures++
	cb.stats.ConsecutiveSuccesses = 0
	cb.stats.ConsecutiveFailures++
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case StateClosed:
		// Check if we should open the circuit
		if cb.stats.ConsecutiveFailures >= cb.config.FailureThreshold {
			cb.setState(StateOpen)
		}
	case StateHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.setState(StateOpen)
	}
}

// setState changes the circuit breaker state and handles side effects
func (cb *CircuitBreaker) setState(newState CircuitBreakerState) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	// Handle state-specific logic
	switch newState {
	case StateOpen:
		cb.nextAttempt = time.Now().Add(cb.config.Timeout)
	case StateHalfOpen:
		cb.stats.ConsecutiveSuccesses = 0
	case StateClosed:
		cb.stats.ConsecutiveFailures = 0
		cb.stats.ConsecutiveSuccesses = 0
	}

	// Call state change callback if provided
	if cb.config.OnStateChange != nil {
		cb.config.OnStateChange(cb.name, oldState, newState)
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStatus returns the current status of the circuit breaker
func (cb *CircuitBreaker) GetStatus() CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	status := CircuitBreakerStatus{
		Name:            cb.name,
		State:           cb.state,
		FailureCount:    cb.stats.ConsecutiveFailures,
		SuccessCount:    cb.stats.ConsecutiveSuccesses,
		RequestCount:    int(cb.stats.TotalRequests),
		LastFailureTime: cb.lastFailureTime,
		LastSuccessTime: cb.lastSuccessTime,
	}

	if cb.state == StateOpen {
		status.NextAttempt = cb.nextAttempt
	}

	return status
}

// GetStats returns comprehensive statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.stats
}

// Reset resets the circuit breaker to its initial state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := cb.state
	cb.state = StateClosed
	cb.stats = CircuitBreakerStats{}
	cb.lastFailureTime = time.Time{}
	cb.lastSuccessTime = time.Time{}
	cb.nextAttempt = time.Time{}

	// Call state change callback if provided
	if cb.config.OnStateChange != nil && oldState != StateClosed {
		cb.config.OnStateChange(cb.name, oldState, StateClosed)
	}
}

// ForceOpen forces the circuit breaker to open state
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.setState(StateOpen)
}

// ForceClose forces the circuit breaker to close state
func (cb *CircuitBreaker) ForceClose() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.setState(StateClosed)
}

// IsAllowingRequests returns true if the circuit breaker is allowing requests
func (cb *CircuitBreaker) IsAllowingRequests() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.allowRequest()
}

// DefaultCircuitBreakerConfig returns a default configuration for circuit breakers
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		SuccessThreshold: 2,
		Timeout:          30 * time.Second,
		MaxRequests:      3,
	}
}

// AggressiveCircuitBreakerConfig returns a more aggressive configuration
func AggressiveCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 3,
		SuccessThreshold: 1,
		Timeout:          10 * time.Second,
		MaxRequests:      1,
	}
}

// ConservativeCircuitBreakerConfig returns a more conservative configuration
func ConservativeCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 5,
		Timeout:          60 * time.Second,
		MaxRequests:      5,
	}
}
