package local

import (
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestUserForTOTP creates a test User (not TestUser) for TOTP testing
func createTestUserForTOTP(t *testing.T, email string) *User {
	t.Helper()

	return &User{
		ID:               generateUserID(email),
		Email:            email,
		Username:         email,
		DisplayName:      email,
		EmailVerified:    true,
		PasswordHash:     nil,
		Passkeys:         []Passkey{},
		TOTPSecret:       nil,
		TOTPEnabled:      false,
		BackupCodes:      []BackupCode{},
		MagicLinkEnabled: true,
		Require2FA:       false,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		LastLoginAt:      nil,
	}
}

func TestBeginTOTPSetup(t *testing.T) {
	conn, config := setupTestConnector(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create test user
	user := createTestUserForTOTP(t, "alice@example.com")
	err := conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin TOTP setup
	setup, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, setup)

	// Verify setup result
	assert.NotEmpty(t, setup.Secret, "secret should not be empty")
	assert.NotEmpty(t, setup.QRCodeDataURL, "QR code URL should not be empty")
	assert.Len(t, setup.BackupCodes, 10, "should generate 10 backup codes")
	assert.NotEmpty(t, setup.URL, "otpauth URL should not be empty")

	// Verify QR code is a valid data URL
	assert.True(t, strings.HasPrefix(setup.QRCodeDataURL, "data:image/png;base64,"),
		"QR code should be a PNG data URL")

	// Verify otpauth URL contains user email
	assert.Contains(t, setup.URL, user.Email, "URL should contain user email")

	// Verify backup codes are 8 characters each
	for i, code := range setup.BackupCodes {
		assert.Len(t, code, 8, "backup code %d should be 8 characters", i)
		// Verify backup codes don't contain ambiguous characters
		for _, char := range "01OIL" {
			assert.NotContains(t, code, string(char),
				"backup code %d should not contain ambiguous character %c", i, char)
		}
	}

	// Verify secret can generate valid TOTP codes
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)
	assert.Len(t, code, 6, "TOTP code should be 6 digits")
}

func TestFinishTOTPSetup(t *testing.T) {
	conn, config := setupTestConnector(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create test user
	user := createTestUserForTOTP(t, "alice@example.com")
	err := conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin TOTP setup
	setup, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)

	// Generate valid TOTP code
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)

	t.Run("successful setup", func(t *testing.T) {
		// Finish TOTP setup
		err := conn.FinishTOTPSetup(ctx, user, setup.Secret, code, setup.BackupCodes)
		require.NoError(t, err)

		// Verify user has TOTP enabled
		assert.True(t, user.TOTPEnabled)
		assert.NotNil(t, user.TOTPSecret)
		assert.Equal(t, setup.Secret, *user.TOTPSecret)
		assert.Len(t, user.BackupCodes, 10, "should have 10 backup codes")

		// Verify backup codes are hashed
		for i, bc := range user.BackupCodes {
			assert.NotEqual(t, setup.BackupCodes[i], bc.Code, "backup code should be hashed")
			assert.False(t, bc.Used, "backup code should not be used")
			assert.Nil(t, bc.UsedAt, "backup code should not have used timestamp")
		}

		// Verify user was updated in storage
		updatedUser, err := conn.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, updatedUser.TOTPEnabled)
	})

	t.Run("invalid TOTP code", func(t *testing.T) {
		// Create new user
		user2 := createTestUserForTOTP(t, "bob@example.com")
		err := conn.storage.CreateUser(ctx, user2)
		require.NoError(t, err)

		// Begin TOTP setup
		setup2, err := conn.BeginTOTPSetup(ctx, user2)
		require.NoError(t, err)

		// Try to finish with invalid code
		err = conn.FinishTOTPSetup(ctx, user2, setup2.Secret, "000000", setup2.BackupCodes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid TOTP code")
		assert.False(t, user2.TOTPEnabled, "TOTP should not be enabled")
	})
}

func TestValidateTOTP(t *testing.T) {
	conn, config := setupTestConnector(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create user with TOTP enabled
	user := createTestUserForTOTP(t, "alice@example.com")
	err := conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin and finish TOTP setup
	setup, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)

	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)

	err = conn.FinishTOTPSetup(ctx, user, setup.Secret, code, setup.BackupCodes)
	require.NoError(t, err)

	t.Run("valid TOTP code", func(t *testing.T) {
		// Generate current TOTP code
		validCode, err := totp.GenerateCode(*user.TOTPSecret, time.Now())
		require.NoError(t, err)

		// Validate TOTP code
		valid, err := conn.ValidateTOTP(ctx, user, validCode)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("invalid TOTP code", func(t *testing.T) {
		// Use an invalid code
		valid, err := conn.ValidateTOTP(ctx, user, "000000")
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("TOTP not enabled", func(t *testing.T) {
		// Create user without TOTP
		user2 := createTestUserForTOTP(t, "bob@example.com")
		err := conn.storage.CreateUser(ctx, user2)
		require.NoError(t, err)

		// Try to validate TOTP
		valid, err := conn.ValidateTOTP(ctx, user2, "123456")
		assert.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "TOTP not enabled")
	})

	t.Run("rate limiting", func(t *testing.T) {
		// Create new user
		user3 := createTestUserForTOTP(t, "charlie@example.com")
		err := conn.storage.CreateUser(ctx, user3)
		require.NoError(t, err)

		// Enable TOTP
		setup3, err := conn.BeginTOTPSetup(ctx, user3)
		require.NoError(t, err)

		code3, err := totp.GenerateCode(setup3.Secret, time.Now())
		require.NoError(t, err)

		err = conn.FinishTOTPSetup(ctx, user3, setup3.Secret, code3, setup3.BackupCodes)
		require.NoError(t, err)

		// Make 5 invalid attempts (should all be allowed)
		for i := 0; i < 5; i++ {
			valid, err := conn.ValidateTOTP(ctx, user3, "000000")
			require.NoError(t, err)
			assert.False(t, valid)
		}

		// 6th attempt should be rate limited
		valid, err := conn.ValidateTOTP(ctx, user3, "000000")
		assert.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "rate limit exceeded")
	})
}

func TestValidateBackupCode(t *testing.T) {
	conn, config := setupTestConnector(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create user with TOTP enabled
	user := createTestUserForTOTP(t, "alice@example.com")
	err := conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin and finish TOTP setup
	setup, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)

	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)

	err = conn.FinishTOTPSetup(ctx, user, setup.Secret, code, setup.BackupCodes)
	require.NoError(t, err)

	t.Run("valid backup code", func(t *testing.T) {
		// Use first backup code
		validCode := setup.BackupCodes[0]

		// Validate backup code
		valid, err := conn.ValidateBackupCode(ctx, user, validCode)
		require.NoError(t, err)
		assert.True(t, valid)

		// Verify backup code is marked as used
		updatedUser, err := conn.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, updatedUser.BackupCodes[0].Used)
		assert.NotNil(t, updatedUser.BackupCodes[0].UsedAt)
	})

	t.Run("already used backup code", func(t *testing.T) {
		// Try to use the same backup code again
		validCode := setup.BackupCodes[0]

		// Should fail because it's already used
		valid, err := conn.ValidateBackupCode(ctx, user, validCode)
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("invalid backup code", func(t *testing.T) {
		// Use an invalid code
		valid, err := conn.ValidateBackupCode(ctx, user, "INVALID1")
		require.NoError(t, err)
		assert.False(t, valid)
	})

	t.Run("no backup codes", func(t *testing.T) {
		// Create user without backup codes
		user2 := createTestUserForTOTP(t, "bob@example.com")
		err := conn.storage.CreateUser(ctx, user2)
		require.NoError(t, err)

		// Try to validate backup code
		valid, err := conn.ValidateBackupCode(ctx, user2, "CODE1234")
		assert.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "no backup codes")
	})
}

func TestDisableTOTP(t *testing.T) {
	conn, config := setupTestConnector(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create user with TOTP enabled
	user := createTestUserForTOTP(t, "alice@example.com")
	err := conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Enable TOTP
	setup, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)

	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)

	err = conn.FinishTOTPSetup(ctx, user, setup.Secret, code, setup.BackupCodes)
	require.NoError(t, err)

	t.Run("successful disable", func(t *testing.T) {
		// Generate current TOTP code
		validCode, err := totp.GenerateCode(*user.TOTPSecret, time.Now())
		require.NoError(t, err)

		// Disable TOTP
		err = conn.DisableTOTP(ctx, user, validCode)
		require.NoError(t, err)

		// Verify TOTP is disabled
		assert.False(t, user.TOTPEnabled)
		assert.Nil(t, user.TOTPSecret)
		assert.Nil(t, user.BackupCodes)

		// Verify user was updated in storage
		updatedUser, err := conn.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.False(t, updatedUser.TOTPEnabled)
	})

	t.Run("invalid TOTP code", func(t *testing.T) {
		// Create and enable TOTP for new user
		user2 := createTestUserForTOTP(t, "bob@example.com")
		err := conn.storage.CreateUser(ctx, user2)
		require.NoError(t, err)

		setup2, err := conn.BeginTOTPSetup(ctx, user2)
		require.NoError(t, err)

		code2, err := totp.GenerateCode(setup2.Secret, time.Now())
		require.NoError(t, err)

		err = conn.FinishTOTPSetup(ctx, user2, setup2.Secret, code2, setup2.BackupCodes)
		require.NoError(t, err)

		// Try to disable with invalid code
		err = conn.DisableTOTP(ctx, user2, "000000")
		assert.Error(t, err)
		assert.True(t, user2.TOTPEnabled, "TOTP should still be enabled")
	})
}

func TestRegenerateBackupCodes(t *testing.T) {
	conn, config := setupTestConnector(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create user with TOTP enabled
	user := createTestUserForTOTP(t, "alice@example.com")
	err := conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Enable TOTP
	setup, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)

	code, err := totp.GenerateCode(setup.Secret, time.Now())
	require.NoError(t, err)

	err = conn.FinishTOTPSetup(ctx, user, setup.Secret, code, setup.BackupCodes)
	require.NoError(t, err)

	oldBackupCodes := make([]string, len(user.BackupCodes))
	for i, bc := range user.BackupCodes {
		oldBackupCodes[i] = bc.Code
	}

	t.Run("successful regeneration", func(t *testing.T) {
		// Generate current TOTP code
		validCode, err := totp.GenerateCode(*user.TOTPSecret, time.Now())
		require.NoError(t, err)

		// Regenerate backup codes
		newCodes, err := conn.RegenerateBackupCodes(ctx, user, validCode)
		require.NoError(t, err)
		require.Len(t, newCodes, 10)

		// Verify new codes are different
		for i, newCode := range newCodes {
			// The plaintext codes should be different
			assert.NotContains(t, setup.BackupCodes, newCode)

			// The hashed codes should be different
			assert.NotEqual(t, oldBackupCodes[i], user.BackupCodes[i].Code)

			// New codes should not be marked as used
			assert.False(t, user.BackupCodes[i].Used)
		}

		// Verify user was updated in storage
		updatedUser, err := conn.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, updatedUser.BackupCodes, 10)
	})

	t.Run("invalid TOTP code", func(t *testing.T) {
		// Try to regenerate with invalid code
		newCodes, err := conn.RegenerateBackupCodes(ctx, user, "000000")
		assert.Error(t, err)
		assert.Nil(t, newCodes)
	})
}

func TestTOTPRateLimiter(t *testing.T) {
	t.Run("allow first attempt", func(t *testing.T) {
		limiter := NewTOTPRateLimiter(5, 5*time.Minute)
		allowed := limiter.Allow("user1")
		assert.True(t, allowed)
	})

	t.Run("allow multiple attempts within limit", func(t *testing.T) {
		limiter := NewTOTPRateLimiter(5, 5*time.Minute)

		// Make 5 attempts
		for i := 0; i < 5; i++ {
			allowed := limiter.Allow("user1")
			assert.True(t, allowed, "attempt %d should be allowed", i+1)
		}

		// 6th attempt should be blocked
		allowed := limiter.Allow("user1")
		assert.False(t, allowed)
	})

	t.Run("reset clears attempts", func(t *testing.T) {
		limiter := NewTOTPRateLimiter(5, 5*time.Minute)

		// Make 5 attempts
		for i := 0; i < 5; i++ {
			limiter.Allow("user1")
		}

		// Reset
		limiter.Reset("user1")

		// Next attempt should be allowed
		allowed := limiter.Allow("user1")
		assert.True(t, allowed)
	})

	t.Run("different users have separate limits", func(t *testing.T) {
		limiter := NewTOTPRateLimiter(5, 5*time.Minute)

		// Make 5 attempts for user1
		for i := 0; i < 5; i++ {
			limiter.Allow("user1")
		}

		// user1's next attempt should be blocked
		allowed := limiter.Allow("user1")
		assert.False(t, allowed)

		// user2's attempt should be allowed
		allowed = limiter.Allow("user2")
		assert.True(t, allowed)
	})

	t.Run("cleanup removes expired attempts", func(t *testing.T) {
		limiter := NewTOTPRateLimiter(5, 100*time.Millisecond)

		// Make 3 attempts
		for i := 0; i < 3; i++ {
			limiter.Allow("user1")
		}

		// Wait for attempts to expire
		time.Sleep(150 * time.Millisecond)

		// Cleanup
		limiter.Cleanup()

		// Next attempt should be allowed (counter reset)
		allowed := limiter.Allow("user1")
		assert.True(t, allowed)
	})
}

func TestGenerateBackupCodes(t *testing.T) {
	conn, config := setupTestConnector(t)
	defer CleanupTestStorage(t, config.DataDir)

	t.Run("generates correct number of codes", func(t *testing.T) {
		codes, err := conn.generateBackupCodes(10)
		require.NoError(t, err)
		assert.Len(t, codes, 10)
	})

	t.Run("codes are 8 characters", func(t *testing.T) {
		codes, err := conn.generateBackupCodes(5)
		require.NoError(t, err)

		for i, code := range codes {
			assert.Len(t, code, 8, "code %d should be 8 characters", i)
		}
	})

	t.Run("codes don't contain ambiguous characters", func(t *testing.T) {
		codes, err := conn.generateBackupCodes(20)
		require.NoError(t, err)

		for i, code := range codes {
			for _, char := range "01OIL" {
				assert.NotContains(t, code, string(char),
					"code %d should not contain ambiguous character %c", i, char)
			}
		}
	})

	t.Run("codes are unique", func(t *testing.T) {
		codes, err := conn.generateBackupCodes(100)
		require.NoError(t, err)

		// Check for duplicates
		seen := make(map[string]bool)
		for _, code := range codes {
			assert.False(t, seen[code], "code %s should be unique", code)
			seen[code] = true
		}
	})
}
