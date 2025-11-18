package local

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultTestConfig verifies the default test configuration
func TestDefaultTestConfig(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	assert.NotEmpty(t, config.DataDir, "DataDir should be set")
	assert.Equal(t, "Enopax Test", config.RPDisplayName)
	assert.Equal(t, "localhost", config.RPID)
	assert.Equal(t, []string{"http://localhost:5556"}, config.RPOrigins)
	assert.Equal(t, "preferred", config.UserVerification)
	assert.Equal(t, "Enopax Test", config.TOTPIssuer)
	assert.Equal(t, 600, config.MagicLinkTTL)
	assert.Equal(t, 300, config.SessionTTL)
	assert.True(t, config.EnableMagicLink)
	assert.False(t, config.Enable2FA)

	// Verify temp directory exists
	info, err := os.Stat(config.DataDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

// TestNewTestUser verifies test user creation
func TestNewTestUser(t *testing.T) {
	email := "test@example.com"
	user := NewTestUser(email)

	assert.Equal(t, email, user.Email)
	assert.Equal(t, email, user.Username)
	assert.Equal(t, email, user.DisplayName)
	assert.NotEmpty(t, user.ID)
	assert.NotEmpty(t, user.Password)
	assert.True(t, user.EmailVerified)
	assert.True(t, user.MagicLinkEnabled)
	assert.False(t, user.Require2FA)
	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

// TestNewTestPasskey verifies test passkey creation
func TestNewTestPasskey(t *testing.T) {
	userID := "test-user-123"
	name := "My Security Key"
	passkey := NewTestPasskey(userID, name)

	assert.Equal(t, userID, passkey.UserID)
	assert.Equal(t, name, passkey.Name)
	assert.NotEmpty(t, passkey.ID)
	assert.NotEmpty(t, passkey.PublicKey)
	assert.NotEmpty(t, passkey.AAGUID)
	assert.Equal(t, "none", passkey.AttestationType)
	assert.Equal(t, uint32(0), passkey.SignCount)
	assert.Contains(t, passkey.Transports, "internal")
	assert.Contains(t, passkey.Transports, "hybrid")
	assert.True(t, passkey.BackupEligible)
	assert.True(t, passkey.BackupState)
	assert.False(t, passkey.CreatedAt.IsZero())
	assert.False(t, passkey.LastUsedAt.IsZero())
}

// TestNewTestWebAuthnSession verifies test WebAuthn session creation
func TestNewTestWebAuthnSession(t *testing.T) {
	userID := "test-user-123"
	operation := "registration"
	ttl := 5 * time.Minute

	session := NewTestWebAuthnSession(userID, operation, ttl)

	assert.Equal(t, userID, session.UserID)
	assert.Equal(t, operation, session.Operation)
	assert.NotEmpty(t, session.SessionID)
	assert.NotEmpty(t, session.Challenge)
	assert.Len(t, session.Challenge, 32, "challenge should be 32 bytes")
	assert.True(t, session.ExpiresAt.After(time.Now()))
	assert.True(t, session.ExpiresAt.Before(time.Now().Add(6*time.Minute)))
}

// TestNewTestMagicLinkToken verifies test magic link token creation
func TestNewTestMagicLinkToken(t *testing.T) {
	userID := "test-user-123"
	email := "test@example.com"
	ttl := 10 * time.Minute

	token := NewTestMagicLinkToken(userID, email, ttl)

	assert.Equal(t, userID, token.UserID)
	assert.Equal(t, email, token.Email)
	assert.NotEmpty(t, token.Token)
	assert.False(t, token.Used)
	assert.Equal(t, "127.0.0.1", token.IPAddress)
	assert.False(t, token.CreatedAt.IsZero())
	assert.True(t, token.ExpiresAt.After(time.Now()))
	assert.True(t, token.ExpiresAt.Before(time.Now().Add(11*time.Minute)))
}

// TestGenerateTestBackupCodes verifies backup code generation
func TestGenerateTestBackupCodes(t *testing.T) {
	count := 10
	codes := GenerateTestBackupCodes(count)

	assert.Len(t, codes, count)

	// Verify all codes are unique
	seen := make(map[string]bool)
	for _, code := range codes {
		assert.NotEmpty(t, code.Code)
		assert.False(t, code.Used)
		assert.Nil(t, code.UsedAt)
		assert.False(t, seen[code.Code], "duplicate backup code: %s", code.Code)
		seen[code.Code] = true
	}
}

// TestSetupTestStorage verifies test storage setup
func TestSetupTestStorage(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	// Verify main directory exists
	AssertFileExists(t, dataDir)
	AssertDirPermissions(t, dataDir, 0700)

	// Verify all subdirectories exist
	subdirs := []string{
		"users",
		"passkeys",
		"totp",
		"magic-link-tokens",
		"webauthn-sessions",
		"backup-codes",
	}

	for _, subdir := range subdirs {
		subdirPath := filepath.Join(dataDir, subdir)
		AssertFileExists(t, subdirPath)
		AssertDirPermissions(t, subdirPath, 0700)
	}
}

// TestCleanupTestStorage verifies test storage cleanup
func TestCleanupTestStorage(t *testing.T) {
	dataDir := SetupTestStorage(t)

	// Verify directory exists
	AssertFileExists(t, dataDir)

	// Clean up
	CleanupTestStorage(t, dataDir)

	// Verify directory is removed
	AssertFileNotExists(t, dataDir)
}

// TestTestContext verifies test context creation
func TestTestContext(t *testing.T) {
	ctx := TestContext(t)

	assert.NotNil(t, ctx)

	// Context should have a deadline
	deadline, ok := ctx.Deadline()
	assert.True(t, ok, "context should have a deadline")
	assert.True(t, deadline.After(time.Now()))
}

// TestTestContextWithDeadline verifies custom deadline context
func TestTestContextWithDeadline(t *testing.T) {
	timeout := 5 * time.Second
	ctx := TestContextWithDeadline(t, timeout)

	assert.NotNil(t, ctx)

	deadline, ok := ctx.Deadline()
	assert.True(t, ok)
	assert.True(t, deadline.Before(time.Now().Add(6*time.Second)))
	assert.True(t, deadline.After(time.Now().Add(4*time.Second)))
}

// TestWithTestTimeout verifies timeout handling
func TestWithTestTimeout(t *testing.T) {
	t.Run("CompletesSuccessfully", func(t *testing.T) {
		completed := false
		WithTestTimeout(t, 1*time.Second, func() {
			completed = true
		})
		assert.True(t, completed)
	})

	t.Run("SlowOperation", func(t *testing.T) {
		// This test should complete successfully even though
		// it takes time (but less than timeout)
		completed := false
		WithTestTimeout(t, 2*time.Second, func() {
			time.Sleep(100 * time.Millisecond)
			completed = true
		})
		assert.True(t, completed)
	})
}

// TestMockEmailSender verifies mock email sender
func TestMockEmailSender(t *testing.T) {
	sender := NewMockEmailSender()

	assert.Empty(t, sender.SentEmails)
	assert.Nil(t, sender.GetLastEmail())

	// Send first email
	err := sender.SendEmail("user1@example.com", "Welcome", "Welcome to Enopax!")
	assert.NoError(t, err)
	assert.Len(t, sender.SentEmails, 1)

	lastEmail := sender.GetLastEmail()
	assert.NotNil(t, lastEmail)
	assert.Equal(t, "user1@example.com", lastEmail.To)
	assert.Equal(t, "Welcome", lastEmail.Subject)
	assert.Equal(t, "Welcome to Enopax!", lastEmail.Body)
	assert.False(t, lastEmail.SentAt.IsZero())

	// Send second email
	err = sender.SendEmail("user2@example.com", "Magic Link", "Click here to login")
	assert.NoError(t, err)
	assert.Len(t, sender.SentEmails, 2)

	lastEmail = sender.GetLastEmail()
	assert.Equal(t, "user2@example.com", lastEmail.To)

	// Reset
	sender.Reset()
	assert.Empty(t, sender.SentEmails)
	assert.Nil(t, sender.GetLastEmail())
}

// TestAssertFilePermissions verifies file permission assertions
func TestAssertFilePermissions(t *testing.T) {
	tempDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, tempDir)

	// Create a test file with specific permissions
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test"), 0600)
	require.NoError(t, err)

	// Verify permissions
	AssertFilePermissions(t, testFile, 0600)
}

// TestAssertDirPermissions verifies directory permission assertions
func TestAssertDirPermissions(t *testing.T) {
	tempDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, tempDir)

	// Verify directory has correct permissions
	AssertDirPermissions(t, tempDir, 0700)
}
