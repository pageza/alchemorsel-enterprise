// Package security provides Multi-Factor Authentication (MFA) capabilities
package security

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// MFAService provides multi-factor authentication services
type MFAService struct {
	logger      *zap.Logger
	redisClient *redis.Client
	issuer      string
}

// NewMFAService creates a new MFA service
func NewMFAService(logger *zap.Logger, redisClient *redis.Client, issuer string) *MFAService {
	return &MFAService{
		logger:      logger,
		redisClient: redisClient,
		issuer:      issuer,
	}
}

// MFAMethod represents different MFA methods
type MFAMethod string

const (
	MFAMethodTOTP    MFAMethod = "totp"
	MFAMethodSMS     MFAMethod = "sms"
	MFAMethodEmail   MFAMethod = "email"
	MFAMethodBackup  MFAMethod = "backup"
)

// TOTPSetup represents TOTP setup information
type TOTPSetup struct {
	Secret    string `json:"secret"`
	QRCode    string `json:"qr_code"`
	BackupCodes []string `json:"backup_codes"`
	URL       string `json:"url"`
}

// MFAChallenge represents an MFA challenge
type MFAChallenge struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Method    MFAMethod `json:"method"`
	Code      string    `json:"code,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	Attempts  int       `json:"attempts"`
	MaxAttempts int     `json:"max_attempts"`
	Verified  bool      `json:"verified"`
}

// MFAConfig represents user's MFA configuration
type MFAConfig struct {
	UserID      string      `json:"user_id"`
	Enabled     bool        `json:"enabled"`
	Methods     []MFAMethod `json:"methods"`
	TOTPSecret  string      `json:"totp_secret,omitempty"`
	PhoneNumber string      `json:"phone_number,omitempty"`
	BackupCodes []string    `json:"backup_codes,omitempty"`
	LastUsed    time.Time   `json:"last_used"`
}

// SetupTOTP generates TOTP secret and QR code for user
func (m *MFAService) SetupTOTP(userID, userEmail string) (*TOTPSetup, error) {
	// Generate secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      m.issuer,
		AccountName: userEmail,
		SecretSize:  32,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// Generate backup codes
	backupCodes, err := m.generateBackupCodes(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	setup := &TOTPSetup{
		Secret:      key.Secret(),
		QRCode:      key.URL(),
		BackupCodes: backupCodes,
		URL:         key.URL(),
	}

	return setup, nil
}

// EnableMFA enables MFA for a user with TOTP
func (m *MFAService) EnableMFA(userID, totpSecret string, verificationCode string, backupCodes []string) error {
	// Verify the TOTP code first
	if !m.verifyTOTP(totpSecret, verificationCode) {
		return fmt.Errorf("invalid verification code")
	}

	// Hash backup codes
	hashedBackupCodes := make([]string, len(backupCodes))
	for i, code := range backupCodes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash backup code: %w", err)
		}
		hashedBackupCodes[i] = string(hash)
	}

	// Store MFA configuration
	config := &MFAConfig{
		UserID:      userID,
		Enabled:     true,
		Methods:     []MFAMethod{MFAMethodTOTP, MFAMethodBackup},
		TOTPSecret:  totpSecret,
		BackupCodes: hashedBackupCodes,
		LastUsed:    time.Now(),
	}

	return m.storeMFAConfig(config)
}

// DisableMFA disables MFA for a user
func (m *MFAService) DisableMFA(userID string) error {
	config := &MFAConfig{
		UserID:  userID,
		Enabled: false,
		Methods: []MFAMethod{},
	}

	return m.storeMFAConfig(config)
}

// CreateChallenge creates an MFA challenge for user
func (m *MFAService) CreateChallenge(userID string, method MFAMethod) (*MFAChallenge, error) {
	// Get user's MFA config
	config, err := m.getMFAConfig(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA config: %w", err)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("MFA not enabled for user")
	}

	// Check if method is available for user
	methodAvailable := false
	for _, availableMethod := range config.Methods {
		if availableMethod == method {
			methodAvailable = true
			break
		}
	}

	if !methodAvailable {
		return nil, fmt.Errorf("MFA method not available for user: %s", method)
	}

	challenge := &MFAChallenge{
		ID:          m.generateChallengeID(),
		UserID:      userID,
		Method:      method,
		ExpiresAt:   time.Now().Add(5 * time.Minute), // 5-minute expiration
		Attempts:    0,
		MaxAttempts: 3,
		Verified:    false,
	}

	// Generate code for SMS/Email methods
	if method == MFAMethodSMS || method == MFAMethodEmail {
		challenge.Code = m.generateVerificationCode()
		// TODO: Send code via SMS/Email service
	}

	// Store challenge
	if err := m.storeChallenge(challenge); err != nil {
		return nil, fmt.Errorf("failed to store challenge: %w", err)
	}

	return challenge, nil
}

// VerifyChallenge verifies an MFA challenge
func (m *MFAService) VerifyChallenge(challengeID, code string) (*MFAChallenge, error) {
	// Get challenge
	challenge, err := m.getChallenge(challengeID)
	if err != nil {
		return nil, fmt.Errorf("challenge not found: %w", err)
	}

	// Check if challenge is expired
	if time.Now().After(challenge.ExpiresAt) {
		return nil, fmt.Errorf("challenge expired")
	}

	// Check if challenge is already verified
	if challenge.Verified {
		return nil, fmt.Errorf("challenge already verified")
	}

	// Check attempts limit
	if challenge.Attempts >= challenge.MaxAttempts {
		return nil, fmt.Errorf("maximum attempts exceeded")
	}

	// Increment attempts
	challenge.Attempts++

	// Get user's MFA config
	config, err := m.getMFAConfig(challenge.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA config: %w", err)
	}

	// Verify based on method
	var verified bool
	switch challenge.Method {
	case MFAMethodTOTP:
		verified = m.verifyTOTP(config.TOTPSecret, code)
	case MFAMethodSMS, MFAMethodEmail:
		verified = challenge.Code == code
	case MFAMethodBackup:
		verified = m.verifyBackupCode(config.BackupCodes, code)
	default:
		return nil, fmt.Errorf("unsupported MFA method: %s", challenge.Method)
	}

	if verified {
		challenge.Verified = true
		config.LastUsed = time.Now()
		m.storeMFAConfig(config)
		
		m.logger.Info("MFA challenge verified",
			zap.String("user_id", challenge.UserID),
			zap.String("method", string(challenge.Method)),
			zap.String("challenge_id", challengeID),
		)
	} else {
		m.logger.Warn("MFA challenge verification failed",
			zap.String("user_id", challenge.UserID),
			zap.String("method", string(challenge.Method)),
			zap.String("challenge_id", challengeID),
			zap.Int("attempts", challenge.Attempts),
		)
	}

	// Update challenge
	m.storeChallenge(challenge)

	if !verified {
		return nil, fmt.Errorf("invalid verification code")
	}

	return challenge, nil
}

// RequireMFA middleware that enforces MFA for sensitive operations
func (m *MFAService) RequireMFA() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Check if user has MFA enabled
		config, err := m.getMFAConfig(userID)
		if err != nil {
			m.logger.Error("Failed to get MFA config", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "MFA check failed"})
			c.Abort()
			return
		}

		if !config.Enabled {
			// MFA not enabled, require setup for sensitive operations
			c.JSON(http.StatusPreconditionRequired, gin.H{
				"error": "MFA required for this operation",
				"mfa_setup_required": true,
			})
			c.Abort()
			return
		}

		// Check if MFA was recently verified (grace period)
		gracePeriod := 30 * time.Minute
		if time.Since(config.LastUsed) < gracePeriod {
			c.Next()
			return
		}

		// Check for MFA verification in headers
		mfaChallengeID := c.GetHeader("X-MFA-Challenge")
		if mfaChallengeID == "" {
			c.JSON(http.StatusPreconditionRequired, gin.H{
				"error": "MFA verification required",
				"mfa_challenge_required": true,
				"available_methods": config.Methods,
			})
			c.Abort()
			return
		}

		// Verify MFA challenge
		challenge, err := m.getChallenge(mfaChallengeID)
		if err != nil || !challenge.Verified || challenge.UserID != userID {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid MFA verification"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// verifyTOTP verifies a TOTP code
func (m *MFAService) verifyTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// verifyBackupCode verifies a backup code
func (m *MFAService) verifyBackupCode(hashedCodes []string, code string) bool {
	for _, hashedCode := range hashedCodes {
		if bcrypt.CompareHashAndPassword([]byte(hashedCode), []byte(code)) == nil {
			return true
		}
	}
	return false
}

// generateBackupCodes generates random backup codes
func (m *MFAService) generateBackupCodes(count int) ([]string, error) {
	codes := make([]string, count)
	
	for i := 0; i < count; i++ {
		// Generate 8-character alphanumeric code
		bytes := make([]byte, 5)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		
		// Convert to base32 and take first 8 characters
		code := base32.StdEncoding.EncodeToString(bytes)[:8]
		codes[i] = strings.ToUpper(code)
	}
	
	return codes, nil
}

// generateVerificationCode generates a 6-digit verification code
func (m *MFAService) generateVerificationCode() string {
	bytes := make([]byte, 3)
	rand.Read(bytes)
	
	// Convert to 6-digit number
	num := int(bytes[0])<<16 + int(bytes[1])<<8 + int(bytes[2])
	return fmt.Sprintf("%06d", num%1000000)
}

// generateChallengeID generates a unique challenge ID
func (m *MFAService) generateChallengeID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

// storeMFAConfig stores MFA configuration in Redis
func (m *MFAService) storeMFAConfig(config *MFAConfig) error {
	// Implementation would store in Redis with proper serialization
	// This is a placeholder for the actual Redis storage logic
	return nil
}

// getMFAConfig retrieves MFA configuration from Redis
func (m *MFAService) getMFAConfig(userID string) (*MFAConfig, error) {
	// Implementation would retrieve from Redis
	// This is a placeholder returning a default config
	return &MFAConfig{
		UserID:  userID,
		Enabled: false,
		Methods: []MFAMethod{},
	}, nil
}

// storeChallenge stores MFA challenge in Redis
func (m *MFAService) storeChallenge(challenge *MFAChallenge) error {
	// Implementation would store in Redis with TTL
	// This is a placeholder for the actual Redis storage logic
	return nil
}

// getChallenge retrieves MFA challenge from Redis
func (m *MFAService) getChallenge(challengeID string) (*MFAChallenge, error) {
	// Implementation would retrieve from Redis
	// This is a placeholder returning a mock challenge
	return &MFAChallenge{
		ID:          challengeID,
		UserID:      "mock-user",
		Method:      MFAMethodTOTP,
		ExpiresAt:   time.Now().Add(5 * time.Minute),
		Attempts:    0,
		MaxAttempts: 3,
		Verified:    false,
	}, nil
}

// GetMFAStatus returns MFA status for a user
func (m *MFAService) GetMFAStatus(userID string) (*MFAConfig, error) {
	return m.getMFAConfig(userID)
}

// RegenerateTOTPSecret generates a new TOTP secret for user
func (m *MFAService) RegenerateTOTPSecret(userID, userEmail string) (*TOTPSetup, error) {
	// Disable current MFA
	if err := m.DisableMFA(userID); err != nil {
		return nil, fmt.Errorf("failed to disable current MFA: %w", err)
	}

	// Generate new setup
	return m.SetupTOTP(userID, userEmail)
}

// GenerateNewBackupCodes generates new backup codes for user
func (m *MFAService) GenerateNewBackupCodes(userID string) ([]string, error) {
	config, err := m.getMFAConfig(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA config: %w", err)
	}

	if !config.Enabled {
		return nil, fmt.Errorf("MFA not enabled for user")
	}

	// Generate new backup codes
	backupCodes, err := m.generateBackupCodes(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// Hash and store them
	hashedBackupCodes := make([]string, len(backupCodes))
	for i, code := range backupCodes {
		hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash backup code: %w", err)
		}
		hashedBackupCodes[i] = string(hash)
	}

	config.BackupCodes = hashedBackupCodes
	if err := m.storeMFAConfig(config); err != nil {
		return nil, fmt.Errorf("failed to store new backup codes: %w", err)
	}

	return backupCodes, nil
}