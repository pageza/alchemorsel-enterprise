package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserPasswordHandling tests the critical password hashing and verification logic
func TestUserPasswordHandling(t *testing.T) {
	t.Run("NewUser_ShouldHashPassword", func(t *testing.T) {
		// Arrange
		email := "test@example.com"
		name := "Test User"
		password := "securepassword123"

		// Act
		user, err := NewUser(email, name, password)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, user)

		// Password should be hashed, not stored in plaintext
		assert.NotEqual(t, password, user.passwordHash)
		assert.NotEmpty(t, user.passwordHash)

		// Hash should be bcrypt format (starts with $2)
		assert.Contains(t, user.passwordHash, "$2")
	})

	t.Run("CheckPassword_ValidPassword_ShouldSucceed", func(t *testing.T) {
		// Arrange
		password := "mysecretpassword"
		user, _ := NewUser("test@example.com", "Test User", password)

		// Act
		err := user.CheckPassword(password)

		// Assert
		assert.NoError(t, err, "Valid password should pass verification")
	})

	t.Run("CheckPassword_InvalidPassword_ShouldFail", func(t *testing.T) {
		// Arrange
		correctPassword := "mysecretpassword"
		wrongPassword := "wrongpassword"
		user, _ := NewUser("test@example.com", "Test User", correctPassword)

		// Act
		err := user.CheckPassword(wrongPassword)

		// Assert
		assert.Error(t, err, "Invalid password should fail verification")
	})

	t.Run("CheckPassword_EmptyPassword_ShouldFail", func(t *testing.T) {
		// Arrange
		user, _ := NewUser("test@example.com", "Test User", "validpassword")

		// Act
		err := user.CheckPassword("")

		// Assert
		assert.Error(t, err, "Empty password should fail verification")
	})

	t.Run("UpdatePassword_ShouldHashNewPassword", func(t *testing.T) {
		// Arrange
		originalPassword := "originalpassword"
		newPassword := "newpassword123"
		user, _ := NewUser("test@example.com", "Test User", originalPassword)
		oldHash := user.passwordHash

		// Act
		err := user.UpdatePassword(newPassword)

		// Assert
		require.NoError(t, err)

		// Hash should have changed
		assert.NotEqual(t, oldHash, user.passwordHash)

		// New password should work
		assert.NoError(t, user.CheckPassword(newPassword))

		// Old password should not work
		assert.Error(t, user.CheckPassword(originalPassword))
	})
}

// TestUserValidation tests user creation validation
func TestUserValidation(t *testing.T) {
	t.Run("NewUser_InvalidEmail_ShouldFail", func(t *testing.T) {
		// Act & Assert
		_, err := NewUser("invalid-email", "Test User", "password123")
		assert.Error(t, err)
	})

	t.Run("NewUser_ShortPassword_ShouldFail", func(t *testing.T) {
		// Act & Assert
		_, err := NewUser("test@example.com", "Test User", "short")
		assert.Error(t, err)
	})

	t.Run("NewUser_EmptyName_ShouldFail", func(t *testing.T) {
		// Act & Assert
		_, err := NewUser("test@example.com", "", "password123")
		assert.Error(t, err)
	})

	t.Run("NewUser_ValidData_ShouldSucceed", func(t *testing.T) {
		// Act
		user, err := NewUser("test@example.com", "Test User", "password123")

		// Assert
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", user.Email())
		assert.Equal(t, "Test User", user.Name())
		assert.True(t, user.IsActive())
		assert.False(t, user.IsVerified())
		assert.Equal(t, UserRoleUser, user.Role())
	})
}
