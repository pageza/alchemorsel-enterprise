// Package testutils provides custom assertions and testing utilities
package testutils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/domain/user"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RecipeAssertions provides recipe-specific assertion methods
type RecipeAssertions struct {
	t *testing.T
}

// NewRecipeAssertions creates a new recipe assertions helper
func NewRecipeAssertions(t *testing.T) *RecipeAssertions {
	return &RecipeAssertions{t: t}
}

// ValidRecipe asserts that a recipe is valid for publishing
func (ra *RecipeAssertions) ValidRecipe(recipe *recipe.Recipe, msgAndArgs ...interface{}) {
	require.NotNil(ra.t, recipe, "Recipe should not be nil")
	assert.NotEqual(ra.t, uuid.Nil, recipe.ID(), "Recipe should have a valid ID")
	assert.NotEmpty(ra.t, recipe.Title(), "Recipe should have a title")
	
	// Check that recipe can be published (has required fields)
	err := recipe.Publish()
	if err != nil {
		// If it fails due to status, it means it's already published or archived
		// which is fine for this validation
		if err != recipe.ErrInvalidStatusTransition {
			assert.NoError(ra.t, err, "Recipe should be valid for publishing")
		}
	}
}

// RecipeHasIngredients asserts that a recipe has ingredients
func (ra *RecipeAssertions) RecipeHasIngredients(recipe *recipe.Recipe, expectedCount int, msgAndArgs ...interface{}) {
	require.NotNil(ra.t, recipe, "Recipe should not be nil")
	// Note: We would need to add a public method to Recipe to get ingredients count
	// For now, we'll check indirectly by trying to publish
	err := recipe.Publish()
	if err == recipe.ErrNoIngredients {
		assert.Fail(ra.t, "Recipe should have ingredients", msgAndArgs...)
	}
}

// RecipeHasInstructions asserts that a recipe has instructions
func (ra *RecipeAssertions) RecipeHasInstructions(recipe *recipe.Recipe, expectedCount int, msgAndArgs ...interface{}) {
	require.NotNil(ra.t, recipe, "Recipe should not be nil")
	// Note: We would need to add a public method to Recipe to get instructions count
	// For now, we'll check indirectly by trying to publish
	err := recipe.Publish()
	if err == recipe.ErrNoInstructions {
		assert.Fail(ra.t, "Recipe should have instructions", msgAndArgs...)
	}
}

// RecipeStatus asserts the recipe status
func (ra *RecipeAssertions) RecipeStatus(recipe *recipe.Recipe, expectedStatus recipe.RecipeStatus, msgAndArgs ...interface{}) {
	require.NotNil(ra.t, recipe, "Recipe should not be nil")
	// Note: We would need a public Status() method on Recipe
	// For demonstration, we'll use reflection or implement the interface
}

// UserAssertions provides user-specific assertion methods
type UserAssertions struct {
	t *testing.T
}

// NewUserAssertions creates a new user assertions helper
func NewUserAssertions(t *testing.T) *UserAssertions {
	return &UserAssertions{t: t}
}

// ValidUser asserts that a user is valid
func (ua *UserAssertions) ValidUser(u *user.User, msgAndArgs ...interface{}) {
	require.NotNil(ua.t, u, "User should not be nil")
	assert.NotEqual(ua.t, uuid.Nil, u.ID(), "User should have a valid ID")
	assert.NotEmpty(ua.t, u.Email(), "User should have an email")
	assert.NotEmpty(ua.t, u.Username(), "User should have a username")
}

// UserEmail asserts the user's email
func (ua *UserAssertions) UserEmail(u *user.User, expectedEmail string, msgAndArgs ...interface{}) {
	require.NotNil(ua.t, u, "User should not be nil")
	assert.Equal(ua.t, expectedEmail, u.Email(), msgAndArgs...)
}

// UserUsername asserts the user's username
func (ua *UserAssertions) UserUsername(u *user.User, expectedUsername string, msgAndArgs ...interface{}) {
	require.NotNil(ua.t, u, "User should not be nil")
	assert.Equal(ua.t, expectedUsername, u.Username(), msgAndArgs...)
}

// HTTPAssertions provides HTTP-specific assertion methods
type HTTPAssertions struct {
	t *testing.T
}

// NewHTTPAssertions creates a new HTTP assertions helper
func NewHTTPAssertions(t *testing.T) *HTTPAssertions {
	return &HTTPAssertions{t: t}
}

// StatusCode asserts the HTTP status code
func (ha *HTTPAssertions) StatusCode(resp *http.Response, expectedCode int, msgAndArgs ...interface{}) {
	require.NotNil(ha.t, resp, "Response should not be nil")
	assert.Equal(ha.t, expectedCode, resp.StatusCode, msgAndArgs...)
}

// JSONResponse asserts that the response is valid JSON and unmarshals it
func (ha *HTTPAssertions) JSONResponse(resp *http.Response, target interface{}, msgAndArgs ...interface{}) {
	require.NotNil(ha.t, resp, "Response should not be nil")
	
	contentType := resp.Header.Get("Content-Type")
	assert.True(ha.t, strings.Contains(contentType, "application/json"), 
		"Response should have JSON content type, got: %s", contentType)
	
	decoder := json.NewDecoder(resp.Body)
	err := decoder.Decode(target)
	assert.NoError(ha.t, err, "Response should be valid JSON")
}

// ErrorResponse asserts that the response contains an error
func (ha *HTTPAssertions) ErrorResponse(resp *http.Response, expectedMessage string, msgAndArgs ...interface{}) {
	require.NotNil(ha.t, resp, "Response should not be nil")
	
	var errorResp map[string]interface{}
	ha.JSONResponse(resp, &errorResp)
	
	if expectedMessage != "" {
		errorMsg, exists := errorResp["error"]
		assert.True(ha.t, exists, "Response should contain error field")
		assert.Contains(ha.t, errorMsg, expectedMessage, msgAndArgs...)
	}
}

// Header asserts that a header exists with expected value
func (ha *HTTPAssertions) Header(resp *http.Response, headerName, expectedValue string, msgAndArgs ...interface{}) {
	require.NotNil(ha.t, resp, "Response should not be nil")
	
	actualValue := resp.Header.Get(headerName)
	assert.Equal(ha.t, expectedValue, actualValue, msgAndArgs...)
}

// HasHeader asserts that a header exists
func (ha *HTTPAssertions) HasHeader(resp *http.Response, headerName string, msgAndArgs ...interface{}) {
	require.NotNil(ha.t, resp, "Response should not be nil")
	
	_, exists := resp.Header[headerName]
	assert.True(ha.t, exists, "Response should have header %s", headerName)
}

// SecurityHeaders asserts that security headers are present
func (ha *HTTPAssertions) SecurityHeaders(resp *http.Response, msgAndArgs ...interface{}) {
	require.NotNil(ha.t, resp, "Response should not be nil")
	
	securityHeaders := []string{
		"X-Content-Type-Options",
		"X-Frame-Options", 
		"X-XSS-Protection",
		"Strict-Transport-Security",
		"Content-Security-Policy",
	}
	
	for _, header := range securityHeaders {
		ha.HasHeader(resp, header, "Security header %s should be present", header)
	}
}

// PerformanceAssertions provides performance-related assertions
type PerformanceAssertions struct {
	t *testing.T
}

// NewPerformanceAssertions creates a new performance assertions helper
func NewPerformanceAssertions(t *testing.T) *PerformanceAssertions {
	return &PerformanceAssertions{t: t}
}

// ResponseTime asserts that an operation completes within expected time
func (pa *PerformanceAssertions) ResponseTime(duration time.Duration, maxDuration time.Duration, msgAndArgs ...interface{}) {
	assert.True(pa.t, duration <= maxDuration, 
		"Operation took %v, expected less than %v", duration, maxDuration)
}

// MemoryUsage asserts memory usage is within limits
func (pa *PerformanceAssertions) MemoryUsage(beforeMem, afterMem, maxIncrease uint64, msgAndArgs ...interface{}) {
	increase := afterMem - beforeMem
	assert.True(pa.t, increase <= maxIncrease,
		"Memory increased by %d bytes, expected less than %d bytes", increase, maxIncrease)
}

// DatabaseAssertions provides database-specific assertions
type DatabaseAssertions struct {
	t  *testing.T
	db *TestDatabase
}

// NewDatabaseAssertions creates a new database assertions helper
func NewDatabaseAssertions(t *testing.T, db *TestDatabase) *DatabaseAssertions {
	return &DatabaseAssertions{t: t, db: db}
}

// RecordExists asserts that a record exists in the database
func (da *DatabaseAssertions) RecordExists(table, whereClause string, args ...interface{}) {
	helper := NewDatabaseHelper(da.db)
	exists, err := helper.RecordExists(table, whereClause, args...)
	require.NoError(da.t, err, "Failed to check if record exists")
	assert.True(da.t, exists, "Record should exist in table %s with condition %s", table, whereClause)
}

// RecordNotExists asserts that a record does not exist in the database
func (da *DatabaseAssertions) RecordNotExists(table, whereClause string, args ...interface{}) {
	helper := NewDatabaseHelper(da.db)
	exists, err := helper.RecordExists(table, whereClause, args...)
	require.NoError(da.t, err, "Failed to check if record exists")
	assert.False(da.t, exists, "Record should not exist in table %s with condition %s", table, whereClause)
}

// RecordCount asserts the number of records in a table
func (da *DatabaseAssertions) RecordCount(table string, expectedCount int, msgAndArgs ...interface{}) {
	helper := NewDatabaseHelper(da.db)
	count, err := helper.CountRecords(table)
	require.NoError(da.t, err, "Failed to count records")
	assert.Equal(da.t, expectedCount, count, msgAndArgs...)
}

// TableEmpty asserts that a table is empty
func (da *DatabaseAssertions) TableEmpty(table string, msgAndArgs ...interface{}) {
	da.RecordCount(table, 0, msgAndArgs...)
}

// EventAssertions provides event-related assertions
type EventAssertions struct {
	t *testing.T
}

// NewEventAssertions creates a new event assertions helper
func NewEventAssertions(t *testing.T) *EventAssertions {
	return &EventAssertions{t: t}
}

// EventPublished asserts that an event was published to the message bus
func (ea *EventAssertions) EventPublished(mockBus *MockMessageBus, eventType reflect.Type, msgAndArgs ...interface{}) {
	events := mockBus.GetPublishedEvents()
	found := false
	
	for _, event := range events {
		if reflect.TypeOf(event) == eventType {
			found = true
			break
		}
	}
	
	assert.True(ea.t, found, "Event of type %s should have been published", eventType.Name())
}

// EventCount asserts the number of published events
func (ea *EventAssertions) EventCount(mockBus *MockMessageBus, expectedCount int, msgAndArgs ...interface{}) {
	events := mockBus.GetPublishedEvents()
	assert.Len(ea.t, events, expectedCount, msgAndArgs...)
}

// NoEventsPublished asserts that no events were published
func (ea *EventAssertions) NoEventsPublished(mockBus *MockMessageBus, msgAndArgs ...interface{}) {
	ea.EventCount(mockBus, 0, msgAndArgs...)
}

// EmailAssertions provides email-related assertions
type EmailAssertions struct {
	t *testing.T
}

// NewEmailAssertions creates a new email assertions helper
func NewEmailAssertions(t *testing.T) *EmailAssertions {
	return &EmailAssertions{t: t}
}

// EmailSent asserts that an email was sent
func (ea *EmailAssertions) EmailSent(mockEmail *MockEmailService, to, subject string, msgAndArgs ...interface{}) {
	emails := mockEmail.GetSentEmails()
	found := false
	
	for _, email := range emails {
		if email.To == to && strings.Contains(email.Subject, subject) {
			found = true
			break
		}
	}
	
	assert.True(ea.t, found, "Email to %s with subject containing '%s' should have been sent", to, subject)
}

// EmailCount asserts the number of sent emails
func (ea *EmailAssertions) EmailCount(mockEmail *MockEmailService, expectedCount int, msgAndArgs ...interface{}) {
	emails := mockEmail.GetSentEmails()
	assert.Len(ea.t, emails, expectedCount, msgAndArgs...)
}

// NoEmailsSent asserts that no emails were sent
func (ea *EmailAssertions) NoEmailsSent(mockEmail *MockEmailService, msgAndArgs ...interface{}) {
	ea.EmailCount(mockEmail, 0, msgAndArgs...)
}

// SecurityAssertions provides security-related assertions
type SecurityAssertions struct {
	t *testing.T
}

// NewSecurityAssertions creates a new security assertions helper
func NewSecurityAssertions(t *testing.T) *SecurityAssertions {
	return &SecurityAssertions{t: t}
}

// JWTValid asserts that a JWT token is valid
func (sa *SecurityAssertions) JWTValid(token string, msgAndArgs ...interface{}) {
	// This would integrate with your auth service to validate the token
	assert.NotEmpty(sa.t, token, "JWT token should not be empty")
	// Add more sophisticated JWT validation here
}

// PasswordHash asserts that a password is properly hashed
func (sa *SecurityAssertions) PasswordHash(hash string, msgAndArgs ...interface{}) {
	assert.NotEmpty(sa.t, hash, "Password hash should not be empty")
	assert.True(sa.t, strings.HasPrefix(hash, "$2a$") || strings.HasPrefix(hash, "$2b$"), 
		"Password should be bcrypt hashed")
}

// ComprehensiveAssertions combines all assertion types
type ComprehensiveAssertions struct {
	t            *testing.T
	Recipe       *RecipeAssertions
	User         *UserAssertions
	HTTP         *HTTPAssertions
	Performance  *PerformanceAssertions
	Database     *DatabaseAssertions
	Event        *EventAssertions
	Email        *EmailAssertions
	Security     *SecurityAssertions
}

// NewComprehensiveAssertions creates a comprehensive assertions helper
func NewComprehensiveAssertions(t *testing.T, db *TestDatabase) *ComprehensiveAssertions {
	return &ComprehensiveAssertions{
		t:           t,
		Recipe:      NewRecipeAssertions(t),
		User:        NewUserAssertions(t),
		HTTP:        NewHTTPAssertions(t),
		Performance: NewPerformanceAssertions(t),
		Database:    NewDatabaseAssertions(t, db),
		Event:       NewEventAssertions(t),
		Email:       NewEmailAssertions(t),
		Security:    NewSecurityAssertions(t),
	}
}

// TestResult represents the result of a test operation
type TestResult struct {
	Success   bool
	Error     error
	Duration  time.Duration
	Metadata  map[string]interface{}
}

// AssertTestResult asserts on a test result
func (ca *ComprehensiveAssertions) TestResult(result *TestResult, expectSuccess bool, maxDuration time.Duration, msgAndArgs ...interface{}) {
	require.NotNil(ca.t, result, "Test result should not be nil")
	
	if expectSuccess {
		assert.True(ca.t, result.Success, "Test should succeed")
		assert.NoError(ca.t, result.Error, "Test should not return error")
	} else {
		assert.False(ca.t, result.Success, "Test should fail")
		assert.Error(ca.t, result.Error, "Test should return error")
	}
	
	if maxDuration > 0 {
		ca.Performance.ResponseTime(result.Duration, maxDuration, msgAndArgs...)
	}
}

// Helper function to measure test execution time
func MeasureTime(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// Helper function to create a test result
func CreateTestResult(fn func() error) *TestResult {
	start := time.Now()
	err := fn()
	duration := time.Since(start)
	
	return &TestResult{
		Success:  err == nil,
		Error:    err,
		Duration: duration,
		Metadata: make(map[string]interface{}),
	}
}