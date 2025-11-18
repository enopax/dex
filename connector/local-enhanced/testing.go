package local

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestConfig provides configuration for testing the enhanced local connector
type TestConfig struct {
	// DataDir is the temporary directory for test data
	DataDir string

	// RPDisplayName is the WebAuthn relying party display name
	RPDisplayName string

	// RPID is the WebAuthn relying party ID
	RPID string

	// RPOrigins are the allowed WebAuthn origins
	RPOrigins []string

	// UserVerification specifies the WebAuthn user verification requirement
	UserVerification string

	// TOTPIssuer is the issuer name for TOTP tokens
	TOTPIssuer string

	// MagicLinkTTL is the time-to-live for magic link tokens (in seconds)
	MagicLinkTTL int

	// SessionTTL is the time-to-live for WebAuthn sessions (in seconds)
	SessionTTL int

	// EnableMagicLink enables magic link authentication
	EnableMagicLink bool

	// Enable2FA enables two-factor authentication
	Enable2FA bool
}

// DefaultTestConfig returns a default configuration for testing
func DefaultTestConfig(t *testing.T) *Config {
	tempDir, err := os.MkdirTemp("", "dex-local-enhanced-test-*")
	require.NoError(t, err, "failed to create temp directory")

	return &Config{
		BaseURL:     "http://localhost:5556",
		DataDir:     tempDir,
		TemplateDir: "", // Empty for tests - templates not needed
		Passkey: PasskeyConfig{
			Enabled:          true,
			RPID:             "localhost",
			RPName:           "Enopax Test",
			RPOrigins:        []string{"http://localhost:5556"},
			UserVerification: "preferred",
		},
		TwoFactor: TwoFactorConfig{
			Required:    false,
			Methods:     []string{"totp", "passkey"},
			GracePeriod: 86400 * 7, // 7 days
		},
		MagicLink: MagicLinkConfig{
			Enabled: true,
			TTL:     600, // 10 minutes
			RateLimit: RateLimitConfig{
				PerHour: 3,
				PerDay:  10,
			},
		},
		Email: EmailConfig{
			SMTP: SMTPConfig{
				Host: "localhost",
				Port: 587,
				TLS:  false,
			},
			From:     "test@example.com",
			FromName: "Test",
		},
	}
}

// TestLogger creates a logrus logger suitable for testing
func TestLogger(t *testing.T) *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	return logger
}

// TestUser represents a test user with various authentication methods
type TestUser struct {
	ID              string
	Email           string
	Username        string
	DisplayName     string
	Password        string // Plaintext password for testing
	PasswordHash    string // Bcrypt hash
	EmailVerified   bool
	TOTPSecret      string
	TOTPEnabled     bool
	MagicLinkEnabled bool
	Require2FA      bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastLoginAt     time.Time
}

// NewTestUser creates a new test user with default values
func NewTestUser(email string) *TestUser {
	return &TestUser{
		ID:               generateTestUserID(email),
		Email:            email,
		Username:         email,
		DisplayName:      email,
		Password:         "test-password-123",
		EmailVerified:    true,
		MagicLinkEnabled: true,
		Require2FA:       false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// generateTestUserID generates a deterministic user ID for testing
// This must match the production generateUserID function in storage.go
func generateTestUserID(email string) string {
	// Use the same logic as production
	return generateUserID(email)
}

// TestPasskey represents a test WebAuthn passkey credential
type TestPasskey struct {
	ID              string
	UserID          string
	PublicKey       []byte
	AttestationType string
	AAGUID          []byte
	SignCount       uint32
	Transports      []string
	Name            string
	CreatedAt       time.Time
	LastUsedAt      time.Time
	BackupEligible  bool
	BackupState     bool
}

// NewTestPasskey creates a new test passkey
func NewTestPasskey(userID string, name string) *TestPasskey {
	credID := make([]byte, 32)
	rand.Read(credID)

	publicKey := make([]byte, 65) // Mock public key
	rand.Read(publicKey)

	aaguid := make([]byte, 16)
	rand.Read(aaguid)

	return &TestPasskey{
		ID:              base64.RawURLEncoding.EncodeToString(credID),
		UserID:          userID,
		PublicKey:       publicKey,
		AttestationType: "none",
		AAGUID:          aaguid,
		SignCount:       0,
		Transports:      []string{"internal", "hybrid"},
		Name:            name,
		CreatedAt:       time.Now(),
		LastUsedAt:      time.Now(),
		BackupEligible:  true,
		BackupState:     true,
	}
}

// TestWebAuthnSession represents a test WebAuthn session
type TestWebAuthnSession struct {
	SessionID string
	UserID    string
	Challenge []byte
	Operation string // "registration" or "authentication"
	ExpiresAt time.Time
}

// NewTestWebAuthnSession creates a new test WebAuthn session
func NewTestWebAuthnSession(userID string, operation string, ttl time.Duration) *TestWebAuthnSession {
	challenge := make([]byte, 32)
	rand.Read(challenge)

	sessionID := make([]byte, 16)
	rand.Read(sessionID)

	return &TestWebAuthnSession{
		SessionID: base64.RawURLEncoding.EncodeToString(sessionID),
		UserID:    userID,
		Challenge: challenge,
		Operation: operation,
		ExpiresAt: time.Now().Add(ttl),
	}
}

// TestMagicLinkToken represents a test magic link token
type TestMagicLinkToken struct {
	Token     string
	UserID    string
	Email     string
	CreatedAt time.Time
	ExpiresAt time.Time
	Used      bool
	IPAddress string
}

// NewTestMagicLinkToken creates a new test magic link token
func NewTestMagicLinkToken(userID string, email string, ttl time.Duration) *TestMagicLinkToken {
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)

	return &TestMagicLinkToken{
		Token:     base64.RawURLEncoding.EncodeToString(tokenBytes),
		UserID:    userID,
		Email:     email,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Used:      false,
		IPAddress: "127.0.0.1",
	}
}

// TestBackupCode represents a test backup code
type TestBackupCode struct {
	Code   string
	Used   bool
	UsedAt *time.Time
}

// NewTestBackupCode creates a new test backup code
func NewTestBackupCode(code string) *TestBackupCode {
	return &TestBackupCode{
		Code: code,
		Used: false,
	}
}

// GenerateTestBackupCodes generates a set of test backup codes
func GenerateTestBackupCodes(count int) []*TestBackupCode {
	codes := make([]*TestBackupCode, count)
	for i := 0; i < count; i++ {
		codeBytes := make([]byte, 8)
		rand.Read(codeBytes)
		code := fmt.Sprintf("%s-%s",
			base64.RawURLEncoding.EncodeToString(codeBytes[:4]),
			base64.RawURLEncoding.EncodeToString(codeBytes[4:]))
		codes[i] = NewTestBackupCode(code)
	}
	return codes
}

// SetupTestStorage creates a temporary storage directory for testing
func SetupTestStorage(t *testing.T) string {
	tempDir, err := os.MkdirTemp("", "dex-local-enhanced-storage-test-*")
	require.NoError(t, err, "failed to create temp directory")

	// Create subdirectories
	subdirs := []string{
		"users",
		"passkeys",
		"totp",
		"magic-link-tokens",
		"webauthn-sessions",
		"backup-codes",
	}

	for _, subdir := range subdirs {
		dirPath := filepath.Join(tempDir, subdir)
		err := os.MkdirAll(dirPath, 0700)
		require.NoError(t, err, "failed to create subdirectory: %s", subdir)
	}

	return tempDir
}

// CleanupTestStorage removes the temporary storage directory
func CleanupTestStorage(t *testing.T, dataDir string) {
	err := os.RemoveAll(dataDir)
	require.NoError(t, err, "failed to remove temp directory")
}

// TestContext creates a test context with timeout
func TestContext(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return ctx
}

// TestContextWithDeadline creates a test context with custom deadline
func TestContextWithDeadline(t *testing.T, timeout time.Duration) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	return ctx
}

// AssertFileExists checks if a file exists at the given path
func AssertFileExists(t *testing.T, path string) {
	_, err := os.Stat(path)
	require.NoError(t, err, "file should exist: %s", path)
}

// AssertFileNotExists checks if a file does not exist at the given path
func AssertFileNotExists(t *testing.T, path string) {
	_, err := os.Stat(path)
	require.True(t, os.IsNotExist(err), "file should not exist: %s", path)
}

// AssertFilePermissions checks if a file has the expected permissions
func AssertFilePermissions(t *testing.T, path string, expectedPerm os.FileMode) {
	info, err := os.Stat(path)
	require.NoError(t, err, "failed to stat file: %s", path)
	require.Equal(t, expectedPerm, info.Mode().Perm(), "incorrect file permissions for: %s", path)
}

// AssertDirPermissions checks if a directory has the expected permissions
func AssertDirPermissions(t *testing.T, path string, expectedPerm os.FileMode) {
	info, err := os.Stat(path)
	require.NoError(t, err, "failed to stat directory: %s", path)
	require.True(t, info.IsDir(), "path is not a directory: %s", path)
	require.Equal(t, expectedPerm, info.Mode().Perm(), "incorrect directory permissions for: %s", path)
}

// WithTestTimeout runs a test function with a timeout to detect deadlocks
func WithTestTimeout(t *testing.T, timeout time.Duration, f func()) {
	done := make(chan struct{})
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("test panicked: %v", r)
			}
			close(done)
		}()
		f()
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(timeout):
		t.Fatal("test timed out - possible deadlock")
	}
}

// MockEmailSender is a mock email sender for testing
type MockEmailSender struct {
	SentEmails []MockEmail
}

// MockEmail represents a sent email
type MockEmail struct {
	To      string
	Subject string
	Body    string
	SentAt  time.Time
}

// SendEmail sends a mock email
func (m *MockEmailSender) SendEmail(to, subject, body string) error {
	m.SentEmails = append(m.SentEmails, MockEmail{
		To:      to,
		Subject: subject,
		Body:    body,
		SentAt:  time.Now(),
	})
	return nil
}

// GetLastEmail returns the last sent email
func (m *MockEmailSender) GetLastEmail() *MockEmail {
	if len(m.SentEmails) == 0 {
		return nil
	}
	return &m.SentEmails[len(m.SentEmails)-1]
}

// Reset clears all sent emails
func (m *MockEmailSender) Reset() {
	m.SentEmails = []MockEmail{}
}

// NewMockEmailSender creates a new mock email sender
func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{
		SentEmails: []MockEmail{},
	}
}

// ToUser converts a TestUser to a User (for storage operations)
func (tu *TestUser) ToUser() *User {
	user := &User{
		ID:               tu.ID,
		Email:            tu.Email,
		Username:         tu.Username,
		DisplayName:      tu.DisplayName,
		EmailVerified:    tu.EmailVerified,
		TOTPEnabled:      tu.TOTPEnabled,
		MagicLinkEnabled: tu.MagicLinkEnabled,
		Require2FA:       tu.Require2FA,
		CreatedAt:        tu.CreatedAt,
		UpdatedAt:        tu.UpdatedAt,
	}

	if tu.PasswordHash != "" {
		user.PasswordHash = &tu.PasswordHash
	}

	if tu.TOTPSecret != "" {
		user.TOTPSecret = &tu.TOTPSecret
	}

	if !tu.LastLoginAt.IsZero() {
		user.LastLoginAt = &tu.LastLoginAt
	}

	return user
}

// ToPasskey converts a TestPasskey to a Passkey (for storage operations)
func (tp *TestPasskey) ToPasskey() *Passkey {
	passkey := &Passkey{
		ID:              tp.ID,
		UserID:          tp.UserID,
		PublicKey:       tp.PublicKey,
		AttestationType: tp.AttestationType,
		AAGUID:          tp.AAGUID,
		SignCount:       tp.SignCount,
		Transports:      tp.Transports,
		Name:            tp.Name,
		CreatedAt:       tp.CreatedAt,
		BackupEligible:  tp.BackupEligible,
		BackupState:     tp.BackupState,
	}

	if !tp.LastUsedAt.IsZero() {
		passkey.LastUsedAt = &tp.LastUsedAt
	}

	return passkey
}

// ToWebAuthnSession converts a TestWebAuthnSession to a WebAuthnSession
func (ts *TestWebAuthnSession) ToWebAuthnSession() *WebAuthnSession {
	return &WebAuthnSession{
		SessionID: ts.SessionID,
		UserID:    ts.UserID,
		Challenge: ts.Challenge,
		Operation: ts.Operation,
		ExpiresAt: ts.ExpiresAt,
		CreatedAt: time.Now(),
	}
}

// ToMagicLinkToken converts a TestMagicLinkToken to a MagicLinkToken
func (tm *TestMagicLinkToken) ToMagicLinkToken() *MagicLinkToken {
	return &MagicLinkToken{
		Token:     tm.Token,
		UserID:    tm.UserID,
		Email:     tm.Email,
		CreatedAt: tm.CreatedAt,
		ExpiresAt: tm.ExpiresAt,
		Used:      tm.Used,
		IPAddress: tm.IPAddress,
	}
}

// stringPtr returns a pointer to a string value
func stringPtr(s string) *string {
	return &s
}
