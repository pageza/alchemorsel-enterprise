// Package security provides encryption services for data protection
package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/scrypt"
	"go.uber.org/zap"
)

// EncryptionService provides encryption and decryption capabilities
type EncryptionService struct {
	logger     *zap.Logger
	masterKey  []byte
	keyDerivation KeyDerivationMethod
}

// KeyDerivationMethod represents different key derivation methods
type KeyDerivationMethod int

const (
	KeyDerivationArgon2 KeyDerivationMethod = iota
	KeyDerivationScrypt
	KeyDerivationPBKDF2
)

// EncryptionAlgorithm represents supported encryption algorithms
type EncryptionAlgorithm int

const (
	AlgorithmAES256GCM EncryptionAlgorithm = iota
	AlgorithmAES256CTR
	AlgorithmChaCha20Poly1305
)

// NewEncryptionService creates a new encryption service
func NewEncryptionService(logger *zap.Logger, masterKey string) *EncryptionService {
	// Derive master key using Argon2
	salt := []byte("alchemorsel-salt-v3") // In production, use random salt per installation
	derivedKey := argon2.IDKey([]byte(masterKey), salt, 1, 64*1024, 4, 32)
	
	return &EncryptionService{
		logger:        logger,
		masterKey:     derivedKey,
		keyDerivation: KeyDerivationArgon2,
	}
}

// EncryptedData represents encrypted data with metadata
type EncryptedData struct {
	Data      []byte              `json:"data"`
	Nonce     []byte              `json:"nonce"`
	Algorithm EncryptionAlgorithm `json:"algorithm"`
	KeyID     string              `json:"key_id"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
}

// EncryptString encrypts a string using AES-256-GCM
func (e *EncryptionService) EncryptString(plaintext string) (*EncryptedData, error) {
	return e.EncryptBytes([]byte(plaintext))
}

// EncryptBytes encrypts byte data using AES-256-GCM
func (e *EncryptionService) EncryptBytes(plaintext []byte) (*EncryptedData, error) {
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	
	return &EncryptedData{
		Data:      ciphertext,
		Nonce:     nonce,
		Algorithm: AlgorithmAES256GCM,
		KeyID:     "master-v1",
	}, nil
}

// DecryptString decrypts encrypted data to string
func (e *EncryptionService) DecryptString(encrypted *EncryptedData) (string, error) {
	bytes, err := e.DecryptBytes(encrypted)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// DecryptBytes decrypts encrypted data to bytes
func (e *EncryptionService) DecryptBytes(encrypted *EncryptedData) ([]byte, error) {
	if encrypted.Algorithm != AlgorithmAES256GCM {
		return nil, fmt.Errorf("unsupported algorithm: %d", encrypted.Algorithm)
	}
	
	block, err := aes.NewCipher(e.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	
	plaintext, err := gcm.Open(nil, encrypted.Nonce, encrypted.Data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	
	return plaintext, nil
}

// EncryptStringToBase64 encrypts and encodes to base64
func (e *EncryptionService) EncryptStringToBase64(plaintext string) (string, error) {
	encrypted, err := e.EncryptString(plaintext)
	if err != nil {
		return "", err
	}
	
	// Combine nonce and ciphertext
	combined := append(encrypted.Nonce, encrypted.Data...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

// DecryptStringFromBase64 decrypts from base64 encoded string
func (e *EncryptionService) DecryptStringFromBase64(encoded string) (string, error) {
	combined, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	
	// AES-GCM nonce size is 12 bytes
	if len(combined) < 12 {
		return "", fmt.Errorf("invalid encrypted data length")
	}
	
	nonce := combined[:12]
	ciphertext := combined[12:]
	
	encrypted := &EncryptedData{
		Data:      ciphertext,
		Nonce:     nonce,
		Algorithm: AlgorithmAES256GCM,
		KeyID:     "master-v1",
	}
	
	return e.DecryptString(encrypted)
}

// PIIEncryptionService provides specialized PII encryption
type PIIEncryptionService struct {
	*EncryptionService
	fieldEncryption map[string]bool
}

// NewPIIEncryptionService creates PII-specific encryption service
func NewPIIEncryptionService(base *EncryptionService) *PIIEncryptionService {
	service := &PIIEncryptionService{
		EncryptionService: base,
		fieldEncryption:   make(map[string]bool),
	}
	
	// Define PII fields that require encryption
	piiFields := []string{
		"email", "phone", "address", "ssn", "credit_card",
		"date_of_birth", "full_name", "passport", "license",
	}
	
	for _, field := range piiFields {
		service.fieldEncryption[field] = true
	}
	
	return service
}

// EncryptPIIField encrypts a PII field with metadata
func (p *PIIEncryptionService) EncryptPIIField(fieldName, value string) (*EncryptedData, error) {
	if !p.fieldEncryption[fieldName] {
		p.logger.Warn("Field not marked as PII", zap.String("field", fieldName))
	}
	
	encrypted, err := p.EncryptString(value)
	if err != nil {
		return nil, err
	}
	
	// Add metadata
	encrypted.Metadata = map[string]string{
		"field_type": "pii",
		"field_name": fieldName,
		"encrypted_at": fmt.Sprintf("%d", time.Now().Unix()),
	}
	
	p.logger.Info("PII field encrypted",
		zap.String("field", fieldName),
		zap.String("key_id", encrypted.KeyID),
	)
	
	return encrypted, nil
}

// KeyRotationService handles encryption key rotation
type KeyRotationService struct {
	logger         *zap.Logger
	currentKeyID   string
	keys           map[string][]byte
	rotationPeriod time.Duration
}

// NewKeyRotationService creates a new key rotation service
func NewKeyRotationService(logger *zap.Logger) *KeyRotationService {
	return &KeyRotationService{
		logger:         logger,
		keys:           make(map[string][]byte),
		rotationPeriod: 90 * 24 * time.Hour, // 90 days
	}
}

// AddKey adds a new encryption key
func (k *KeyRotationService) AddKey(keyID string, key []byte) {
	k.keys[keyID] = key
	if k.currentKeyID == "" {
		k.currentKeyID = keyID
	}
}

// RotateKey creates a new key and sets it as current
func (k *KeyRotationService) RotateKey() (string, error) {
	// Generate new key
	newKey := make([]byte, 32)
	if _, err := rand.Read(newKey); err != nil {
		return "", fmt.Errorf("failed to generate new key: %w", err)
	}
	
	// Create key ID
	keyID := fmt.Sprintf("key-%d", time.Now().Unix())
	
	// Store key
	k.keys[keyID] = newKey
	k.currentKeyID = keyID
	
	k.logger.Info("Key rotated", zap.String("new_key_id", keyID))
	
	return keyID, nil
}

// GetKey retrieves a key by ID
func (k *KeyRotationService) GetKey(keyID string) ([]byte, error) {
	key, exists := k.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}
	return key, nil
}

// DatabaseEncryption provides database field encryption
type DatabaseEncryption struct {
	encryption *EncryptionService
	logger     *zap.Logger
}

// NewDatabaseEncryption creates database encryption service
func NewDatabaseEncryption(encryption *EncryptionService, logger *zap.Logger) *DatabaseEncryption {
	return &DatabaseEncryption{
		encryption: encryption,
		logger:     logger,
	}
}

// EncryptField encrypts a database field
func (d *DatabaseEncryption) EncryptField(tableName, fieldName, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	
	encrypted, err := d.encryption.EncryptString(value)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt field %s.%s: %w", tableName, fieldName, err)
	}
	
	// Store as base64 in database
	combined := append(encrypted.Nonce, encrypted.Data...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

// DecryptField decrypts a database field
func (d *DatabaseEncryption) DecryptField(tableName, fieldName, encryptedValue string) (string, error) {
	if encryptedValue == "" {
		return "", nil
	}
	
	decrypted, err := d.encryption.DecryptStringFromBase64(encryptedValue)
	if err != nil {
		d.logger.Error("Failed to decrypt field",
			zap.String("table", tableName),
			zap.String("field", fieldName),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to decrypt field %s.%s: %w", tableName, fieldName, err)
	}
	
	return decrypted, nil
}

// PasswordHashingService provides secure password hashing
type PasswordHashingService struct {
	logger *zap.Logger
}

// NewPasswordHashingService creates password hashing service
func NewPasswordHashingService(logger *zap.Logger) *PasswordHashingService {
	return &PasswordHashingService{logger: logger}
}

// HashPassword hashes a password using Argon2id
func (p *PasswordHashingService) HashPassword(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}
	
	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	
	// Encode salt and hash
	encoded := base64.StdEncoding.EncodeToString(append(salt, hash...))
	
	return encoded, nil
}

// VerifyPassword verifies a password against its hash
func (p *PasswordHashingService) VerifyPassword(password, hashedPassword string) (bool, error) {
	// Decode hash
	decoded, err := base64.StdEncoding.DecodeString(hashedPassword)
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}
	
	if len(decoded) != 48 { // 16 bytes salt + 32 bytes hash
		return false, fmt.Errorf("invalid hash format")
	}
	
	salt := decoded[:16]
	hash := decoded[16:]
	
	// Hash input password with same salt
	inputHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	
	// Compare hashes
	return subtle.ConstantTimeCompare(hash, inputHash) == 1, nil
}

// DeriveKey derives a key from password using scrypt
func (e *EncryptionService) DeriveKey(password, salt []byte) ([]byte, error) {
	switch e.keyDerivation {
	case KeyDerivationArgon2:
		return argon2.IDKey(password, salt, 1, 64*1024, 4, 32), nil
	case KeyDerivationScrypt:
		return scrypt.Key(password, salt, 32768, 8, 1, 32)
	default:
		return nil, fmt.Errorf("unsupported key derivation method")
	}
}

// SecureRandom generates cryptographically secure random bytes
func SecureRandom(size int) ([]byte, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return bytes, nil
}

// GenerateSecureToken generates a secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes, err := SecureRandom(length)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// HashSHA256 creates SHA-256 hash of input
func HashSHA256(input []byte) []byte {
	hash := sha256.Sum256(input)
	return hash[:]
}

// SecureCompare performs constant-time comparison
func SecureCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}