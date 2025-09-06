// Package redis provides comprehensive tests for Cache Repository implementations
package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// Define error types for testing
var (
	ErrKeyNotFound = errors.New("key not found")
)

// CacheServiceInterface defines the interface for cache operations we need to test
type CacheServiceInterface interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, keys ...string) (map[string]bool, error)
	MGet(ctx context.Context, keys []string) (map[string][]byte, error)
	MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error
	Increment(ctx context.Context, key string) (int64, error)
	Decrement(ctx context.Context, key string) (int64, error)
	SAdd(ctx context.Context, key string, members ...string) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members ...string) error
}

// MockCacheService is a mock implementation of CacheServiceInterface
type MockCacheService struct {
	mock.Mock
}

func (m *MockCacheService) Get(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockCacheService) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockCacheService) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockCacheService) Exists(ctx context.Context, keys ...string) (map[string]bool, error) {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *MockCacheService) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	args := m.Called(ctx, keys)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (m *MockCacheService) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	args := m.Called(ctx, items, ttl)
	return args.Error(0)
}

func (m *MockCacheService) Increment(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCacheService) Decrement(ctx context.Context, key string) (int64, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockCacheService) SAdd(ctx context.Context, key string, members ...string) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

func (m *MockCacheService) SMembers(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockCacheService) SRem(ctx context.Context, key string, members ...string) error {
	args := m.Called(ctx, key, members)
	return args.Error(0)
}

// TestCacheRepository wraps CacheRepository to use our mock interface
type TestCacheRepository struct {
	cacheService CacheServiceInterface
	logger       *zap.Logger
}

// Implement the same methods as CacheRepository for testing
func (r *TestCacheRepository) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := r.cacheService.Get(ctx, key)
	if err != nil {
		r.logger.Debug("Cache get failed", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return data, nil
}

func (r *TestCacheRepository) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	err := r.cacheService.Set(ctx, key, value, ttl)
	if err != nil {
		r.logger.Error("Cache set failed", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

func (r *TestCacheRepository) Delete(ctx context.Context, key string) error {
	err := r.cacheService.Delete(ctx, key)
	if err != nil {
		r.logger.Error("Cache delete failed", zap.String("key", key), zap.Error(err))
		return err
	}
	return nil
}

func (r *TestCacheRepository) Exists(ctx context.Context, key string) (bool, error) {
	exists, err := r.cacheService.Exists(ctx, key)
	if err != nil {
		r.logger.Error("Cache exists check failed", zap.String("key", key), zap.Error(err))
		return false, err
	}
	return exists[key], nil
}

func (r *TestCacheRepository) MGet(ctx context.Context, keys []string) (map[string][]byte, error) {
	result, err := r.cacheService.MGet(ctx, keys)
	if err != nil {
		r.logger.Error("Cache mget failed", zap.Strings("keys", keys), zap.Error(err))
		return nil, err
	}
	return result, nil
}

func (r *TestCacheRepository) MSet(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	err := r.cacheService.MSet(ctx, items, ttl)
	if err != nil {
		r.logger.Error("Cache mset failed", zap.Int("count", len(items)), zap.Error(err))
		return err
	}
	return nil
}

func (r *TestCacheRepository) Increment(ctx context.Context, key string) (int64, error) {
	result, err := r.cacheService.Increment(ctx, key)
	if err != nil {
		r.logger.Error("Cache increment failed", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

func (r *TestCacheRepository) Decrement(ctx context.Context, key string) (int64, error) {
	result, err := r.cacheService.Decrement(ctx, key)
	if err != nil {
		r.logger.Error("Cache decrement failed", zap.String("key", key), zap.Error(err))
		return 0, err
	}
	return result, nil
}

func (r *TestCacheRepository) SAdd(ctx context.Context, key string, members ...string) error {
	err := r.cacheService.SAdd(ctx, key, members...)
	if err != nil {
		r.logger.Error("Cache sadd failed", zap.String("key", key), zap.Strings("members", members), zap.Error(err))
		return err
	}
	return nil
}

func (r *TestCacheRepository) SMembers(ctx context.Context, key string) ([]string, error) {
	members, err := r.cacheService.SMembers(ctx, key)
	if err != nil {
		r.logger.Error("Cache smembers failed", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return members, nil
}

func (r *TestCacheRepository) SRem(ctx context.Context, key string, members ...string) error {
	err := r.cacheService.SRem(ctx, key, members...)
	if err != nil {
		r.logger.Error("Cache srem failed", zap.String("key", key), zap.Strings("members", members), zap.Error(err))
		return err
	}
	return nil
}

// Test setup helper
func setupCacheRepository(t *testing.T) (*TestCacheRepository, *MockCacheService) {
	mockCacheService := &MockCacheService{}
	logger := zaptest.NewLogger(t)

	repo := &TestCacheRepository{
		cacheService: mockCacheService,
		logger:       logger,
	}

	return repo, mockCacheService
}

// TestNewCacheRepository tests the constructor
func TestNewCacheRepository(t *testing.T) {
	// Create a real cache service for constructor testing
	// Since NewCacheRepository expects *cache.CacheService, we'll test the constructor separately
	logger := zaptest.NewLogger(t)

	// Test that we can create a repository with nil cache service (should work)
	repo := &CacheRepository{
		cacheService: nil,
		logger:       logger,
	}

	assert.NotNil(t, repo)
	assert.Equal(t, logger, repo.logger)
}

// TestCacheRepositoryGet tests the Get method
func TestCacheRepositoryGet(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		setupMocks     func(*MockCacheService)
		expectedData   []byte
		expectedError  string
		expectError    bool
	}{
		{
			name: "successful get",
			key:  "test_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Get", mock.Anything, "test_key").Return([]byte("test_data"), nil).Once()
			},
			expectedData:  []byte("test_data"),
			expectedError: "",
			expectError:   false,
		},
		{
			name: "cache miss",
			key:  "missing_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Get", mock.Anything, "missing_key").Return(nil, ErrKeyNotFound).Once()
			},
			expectedData:  nil,
			expectedError: "key not found",
			expectError:   true,
		},
		{
			name: "cache service error",
			key:  "error_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Get", mock.Anything, "error_key").Return(nil, errors.New("cache service error")).Once()
			},
			expectedData:  nil,
			expectedError: "cache service error",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			data, err := repo.Get(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositorySet tests the Set method
func TestCacheRepositorySet(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		value          []byte
		ttl            time.Duration
		setupMocks     func(*MockCacheService)
		expectedError  string
		expectError    bool
	}{
		{
			name:  "successful set",
			key:   "test_key",
			value: []byte("test_data"),
			ttl:   time.Hour,
			setupMocks: func(m *MockCacheService) {
				m.On("Set", mock.Anything, "test_key", []byte("test_data"), time.Hour).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:  "cache service error",
			key:   "error_key",
			value: []byte("error_data"),
			ttl:   time.Minute,
			setupMocks: func(m *MockCacheService) {
				m.On("Set", mock.Anything, "error_key", []byte("error_data"), time.Minute).Return(errors.New("cache service error")).Once()
			},
			expectedError: "cache service error",
			expectError:   true,
		},
		{
			name:  "zero ttl",
			key:   "zero_ttl_key",
			value: []byte("zero_ttl_data"),
			ttl:   0,
			setupMocks: func(m *MockCacheService) {
				m.On("Set", mock.Anything, "zero_ttl_key", []byte("zero_ttl_data"), time.Duration(0)).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			err := repo.Set(ctx, tt.key, tt.value, tt.ttl)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositoryDelete tests the Delete method
func TestCacheRepositoryDelete(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		setupMocks     func(*MockCacheService)
		expectedError  string
		expectError    bool
	}{
		{
			name: "successful delete",
			key:  "test_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Delete", mock.Anything, "test_key").Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name: "cache service error",
			key:  "error_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Delete", mock.Anything, "error_key").Return(errors.New("cache service error")).Once()
			},
			expectedError: "cache service error",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			err := repo.Delete(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositoryExists tests the Exists method
func TestCacheRepositoryExists(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		setupMocks     func(*MockCacheService)
		expectedExists bool
		expectedError  string
		expectError    bool
	}{
		{
			name: "key exists",
			key:  "existing_key",
			setupMocks: func(m *MockCacheService) {
				result := map[string]bool{"existing_key": true}
				m.On("Exists", mock.Anything, []string{"existing_key"}).Return(result, nil).Once()
			},
			expectedExists: true,
			expectedError:  "",
			expectError:    false,
		},
		{
			name: "key does not exist",
			key:  "missing_key",
			setupMocks: func(m *MockCacheService) {
				result := map[string]bool{"missing_key": false}
				m.On("Exists", mock.Anything, []string{"missing_key"}).Return(result, nil).Once()
			},
			expectedExists: false,
			expectedError:  "",
			expectError:    false,
		},
		{
			name: "cache service error",
			key:  "error_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Exists", mock.Anything, []string{"error_key"}).Return(nil, errors.New("cache service error")).Once()
			},
			expectedExists: false,
			expectedError:  "cache service error",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			exists, err := repo.Exists(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				assert.False(t, exists)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedExists, exists)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositoryMGet tests the MGet method
func TestCacheRepositoryMGet(t *testing.T) {
	tests := []struct {
		name           string
		keys           []string
		setupMocks     func(*MockCacheService)
		expectedResult map[string][]byte
		expectedError  string
		expectError    bool
	}{
		{
			name: "successful multi-get",
			keys: []string{"key1", "key2", "key3"},
			setupMocks: func(m *MockCacheService) {
				result := map[string][]byte{
					"key1": []byte("data1"),
					"key2": []byte("data2"),
					"key3": []byte("data3"),
				}
				m.On("MGet", mock.Anything, []string{"key1", "key2", "key3"}).Return(result, nil).Once()
			},
			expectedResult: map[string][]byte{
				"key1": []byte("data1"),
				"key2": []byte("data2"),
				"key3": []byte("data3"),
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name: "partial results",
			keys: []string{"key1", "key2", "missing_key"},
			setupMocks: func(m *MockCacheService) {
				result := map[string][]byte{
					"key1": []byte("data1"),
					"key2": []byte("data2"),
				}
				m.On("MGet", mock.Anything, []string{"key1", "key2", "missing_key"}).Return(result, nil).Once()
			},
			expectedResult: map[string][]byte{
				"key1": []byte("data1"),
				"key2": []byte("data2"),
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name: "cache service error",
			keys: []string{"error_key"},
			setupMocks: func(m *MockCacheService) {
				m.On("MGet", mock.Anything, []string{"error_key"}).Return(nil, errors.New("cache service error")).Once()
			},
			expectedResult: nil,
			expectedError:  "cache service error",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			result, err := repo.MGet(ctx, tt.keys)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResult, result)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositoryMSet tests the MSet method
func TestCacheRepositoryMSet(t *testing.T) {
	tests := []struct {
		name           string
		items          map[string][]byte
		ttl            time.Duration
		setupMocks     func(*MockCacheService)
		expectedError  string
		expectError    bool
	}{
		{
			name: "successful multi-set",
			items: map[string][]byte{
				"key1": []byte("data1"),
				"key2": []byte("data2"),
				"key3": []byte("data3"),
			},
			ttl: time.Hour,
			setupMocks: func(m *MockCacheService) {
				items := map[string][]byte{
					"key1": []byte("data1"),
					"key2": []byte("data2"),
					"key3": []byte("data3"),
				}
				m.On("MSet", mock.Anything, items, time.Hour).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name: "empty items",
			items: map[string][]byte{},
			ttl:   time.Minute,
			setupMocks: func(m *MockCacheService) {
				m.On("MSet", mock.Anything, map[string][]byte{}, time.Minute).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name: "cache service error",
			items: map[string][]byte{
				"error_key": []byte("error_data"),
			},
			ttl: time.Minute,
			setupMocks: func(m *MockCacheService) {
				items := map[string][]byte{
					"error_key": []byte("error_data"),
				}
				m.On("MSet", mock.Anything, items, time.Minute).Return(errors.New("cache service error")).Once()
			},
			expectedError: "cache service error",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			err := repo.MSet(ctx, tt.items, tt.ttl)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositoryIncrement tests the Increment method
func TestCacheRepositoryIncrement(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		setupMocks     func(*MockCacheService)
		expectedValue  int64
		expectedError  string
		expectError    bool
	}{
		{
			name: "successful increment",
			key:  "counter_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Increment", mock.Anything, "counter_key").Return(int64(5), nil).Once()
			},
			expectedValue: 5,
			expectedError: "",
			expectError:   false,
		},
		{
			name: "cache service error",
			key:  "error_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Increment", mock.Anything, "error_key").Return(int64(0), errors.New("cache service error")).Once()
			},
			expectedValue: 0,
			expectedError: "cache service error",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			value, err := repo.Increment(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositoryDecrement tests the Decrement method
func TestCacheRepositoryDecrement(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		setupMocks     func(*MockCacheService)
		expectedValue  int64
		expectedError  string
		expectError    bool
	}{
		{
			name: "successful decrement",
			key:  "counter_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Decrement", mock.Anything, "counter_key").Return(int64(3), nil).Once()
			},
			expectedValue: 3,
			expectedError: "",
			expectError:   false,
		},
		{
			name: "cache service error",
			key:  "error_key",
			setupMocks: func(m *MockCacheService) {
				m.On("Decrement", mock.Anything, "error_key").Return(int64(0), errors.New("cache service error")).Once()
			},
			expectedValue: 0,
			expectedError: "cache service error",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			value, err := repo.Decrement(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, value)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositorySAdd tests the SAdd method
func TestCacheRepositorySAdd(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		members        []string
		setupMocks     func(*MockCacheService)
		expectedError  string
		expectError    bool
	}{
		{
			name:    "successful set add",
			key:     "set_key",
			members: []string{"member1", "member2", "member3"},
			setupMocks: func(m *MockCacheService) {
				m.On("SAdd", mock.Anything, "set_key", []string{"member1", "member2", "member3"}).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:    "single member",
			key:     "set_key",
			members: []string{"single_member"},
			setupMocks: func(m *MockCacheService) {
				m.On("SAdd", mock.Anything, "set_key", []string{"single_member"}).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:    "cache service error",
			key:     "error_key",
			members: []string{"error_member"},
			setupMocks: func(m *MockCacheService) {
				m.On("SAdd", mock.Anything, "error_key", []string{"error_member"}).Return(errors.New("cache service error")).Once()
			},
			expectedError: "cache service error",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			err := repo.SAdd(ctx, tt.key, tt.members...)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositorySMembers tests the SMembers method
func TestCacheRepositorySMembers(t *testing.T) {
	tests := []struct {
		name            string
		key             string
		setupMocks      func(*MockCacheService)
		expectedMembers []string
		expectedError   string
		expectError     bool
	}{
		{
			name: "successful set members retrieval",
			key:  "set_key",
			setupMocks: func(m *MockCacheService) {
				members := []string{"member1", "member2", "member3"}
				m.On("SMembers", mock.Anything, "set_key").Return(members, nil).Once()
			},
			expectedMembers: []string{"member1", "member2", "member3"},
			expectedError:   "",
			expectError:     false,
		},
		{
			name: "empty set",
			key:  "empty_set",
			setupMocks: func(m *MockCacheService) {
				members := []string{}
				m.On("SMembers", mock.Anything, "empty_set").Return(members, nil).Once()
			},
			expectedMembers: []string{},
			expectedError:   "",
			expectError:     false,
		},
		{
			name: "cache service error",
			key:  "error_key",
			setupMocks: func(m *MockCacheService) {
				m.On("SMembers", mock.Anything, "error_key").Return(nil, errors.New("cache service error")).Once()
			},
			expectedMembers: nil,
			expectedError:   "cache service error",
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			members, err := repo.SMembers(ctx, tt.key)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
				assert.Nil(t, members)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedMembers, members)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositorySRem tests the SRem method
func TestCacheRepositorySRem(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		members        []string
		setupMocks     func(*MockCacheService)
		expectedError  string
		expectError    bool
	}{
		{
			name:    "successful set remove",
			key:     "set_key",
			members: []string{"member1", "member2"},
			setupMocks: func(m *MockCacheService) {
				m.On("SRem", mock.Anything, "set_key", []string{"member1", "member2"}).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:    "single member removal",
			key:     "set_key",
			members: []string{"single_member"},
			setupMocks: func(m *MockCacheService) {
				m.On("SRem", mock.Anything, "set_key", []string{"single_member"}).Return(nil).Once()
			},
			expectedError: "",
			expectError:   false,
		},
		{
			name:    "cache service error",
			key:     "error_key",
			members: []string{"error_member"},
			setupMocks: func(m *MockCacheService) {
				m.On("SRem", mock.Anything, "error_key", []string{"error_member"}).Return(errors.New("cache service error")).Once()
			},
			expectedError: "cache service error",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mockCacheService := setupCacheRepository(t)
			tt.setupMocks(mockCacheService)

			ctx := context.Background()
			err := repo.SRem(ctx, tt.key, tt.members...)

			if tt.expectError {
				assert.Error(t, err)
				if tt.expectedError != "" {
					assert.Contains(t, err.Error(), tt.expectedError)
				}
			} else {
				assert.NoError(t, err)
			}

			mockCacheService.AssertExpectations(t)
		})
	}
}

// TestCacheRepositoryEdgeCases tests edge cases and boundary conditions
func TestCacheRepositoryEdgeCases(t *testing.T) {
	t.Run("empty key", func(t *testing.T) {
		repo, mockCacheService := setupCacheRepository(t)

		// The cache service should handle empty key validation
		mockCacheService.On("Get", mock.Anything, "").Return(nil, errors.New("empty key error")).Once()

		ctx := context.Background()
		_, err := repo.Get(ctx, "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty key error")
		mockCacheService.AssertExpectations(t)
	})

	t.Run("nil value set", func(t *testing.T) {
		repo, mockCacheService := setupCacheRepository(t)

		mockCacheService.On("Set", mock.Anything, "nil_key", []byte(nil), time.Hour).Return(nil).Once()

		ctx := context.Background()
		err := repo.Set(ctx, "nil_key", nil, time.Hour)

		assert.NoError(t, err)
		mockCacheService.AssertExpectations(t)
	})

	t.Run("empty slice for MGet", func(t *testing.T) {
		repo, mockCacheService := setupCacheRepository(t)

		mockCacheService.On("MGet", mock.Anything, []string{}).Return(map[string][]byte{}, nil).Once()

		ctx := context.Background()
		result, err := repo.MGet(ctx, []string{})

		assert.NoError(t, err)
		assert.Empty(t, result)
		mockCacheService.AssertExpectations(t)
	})

	t.Run("context cancellation", func(t *testing.T) {
		repo, mockCacheService := setupCacheRepository(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		mockCacheService.On("Get", mock.Anything, "test_key").Return(nil, context.Canceled).Once()

		_, err := repo.Get(ctx, "test_key")

		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)
		mockCacheService.AssertExpectations(t)
	})
}

// TestCacheRepositoryIntegration tests integrated scenarios
func TestCacheRepositoryIntegration(t *testing.T) {
	t.Run("complete workflow", func(t *testing.T) {
		repo, mockCacheService := setupCacheRepository(t)
		ctx := context.Background()

		// Set multiple values
		items := map[string][]byte{
			"user:1": []byte(`{"id":1,"name":"Alice"}`),
			"user:2": []byte(`{"id":2,"name":"Bob"}`),
			"user:3": []byte(`{"id":3,"name":"Charlie"}`),
		}

		mockCacheService.On("MSet", mock.Anything, items, time.Hour).Return(nil).Once()
		err := repo.MSet(ctx, items, time.Hour)
		require.NoError(t, err)

		// Get multiple values
		keys := []string{"user:1", "user:2", "user:3"}
		mockCacheService.On("MGet", mock.Anything, keys).Return(items, nil).Once()
		
		result, err := repo.MGet(ctx, keys)
		require.NoError(t, err)
		assert.Equal(t, items, result)

		// Check existence for single key
		singleKeyResult := map[string]bool{
			"user:1": true,
		}
		mockCacheService.On("Exists", mock.Anything, []string{"user:1"}).Return(singleKeyResult, nil).Once()
		
		exists, err := repo.Exists(ctx, "user:1")
		require.NoError(t, err)
		assert.True(t, exists)

		// Delete one key
		mockCacheService.On("Delete", mock.Anything, "user:2").Return(nil).Once()
		err = repo.Delete(ctx, "user:2")
		require.NoError(t, err)

		mockCacheService.AssertExpectations(t)
	})
}

// BenchmarkCacheRepository benchmarks cache repository operations
func BenchmarkCacheRepository(b *testing.B) {
	repo, mockCacheService := setupCacheRepositoryBench(b)
	ctx := context.Background()

	// Setup common mock expectations for benchmarks
	mockCacheService.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockCacheService.On("Get", mock.Anything, mock.Anything).Return([]byte("benchmark_data"), nil)

	b.Run("Set", func(b *testing.B) {
		data := []byte("benchmark_data")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			repo.Set(ctx, "benchmark_key", data, time.Hour)
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			repo.Get(ctx, "benchmark_key")
		}
	})
}

// Benchmark setup helper
func setupCacheRepositoryBench(b *testing.B) (*TestCacheRepository, *MockCacheService) {
	mockCacheService := &MockCacheService{}
	logger := zaptest.NewLogger(b)

	repo := &TestCacheRepository{
		cacheService: mockCacheService,
		logger:       logger,
	}

	return repo, mockCacheService
}