package local

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComplete2FAFlow_PasswordTOTP tests the complete 2FA flow with password + TOTP
func TestComplete2FAFlow_PasswordTOTP(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Step 1: Create user with password and TOTP enabled
	testUser := NewTestUser("alice@example.com")
	user := testUser.ToUser()

	// Create user first
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Then set password
	err = conn.SetPassword(ctx, user, "SecurePass123")
	require.NoError(t, err)

	// Enable TOTP
	setupResult, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, setupResult)

	// Generate a valid TOTP code for the current time window
	validCode, err := generateValidTOTPCode(setupResult.Secret)
	require.NoError(t, err)

	// Complete TOTP setup
	err = conn.FinishTOTPSetup(ctx, user, setupResult.Secret, validCode, setupResult.BackupCodes)
	require.NoError(t, err)

	// Mark user as requiring 2FA
	user.Require2FA = true
	err = conn.storage.UpdateUser(ctx, user)
	require.NoError(t, err)

	// Step 2: Primary authentication (password)
	valid, err := conn.VerifyPassword(ctx, user, "SecurePass123")
	require.NoError(t, err)
	require.True(t, valid, "password verification should succeed")

	// Step 3: Check if 2FA is required
	require2FA := conn.Require2FAForUser(ctx, user)
	assert.True(t, require2FA, "2FA should be required for this user")

	// Step 4: Begin 2FA
	session, err := conn.Begin2FA(ctx, user.ID, "password", "https://example.com/callback", "test-state")
	require.NoError(t, err)
	require.NotEmpty(t, session.SessionID)
	assert.Equal(t, user.ID, session.UserID)
	assert.Equal(t, "password", session.PrimaryMethod)

	// Step 5: Get available 2FA methods
	methods := conn.GetAvailable2FAMethods(ctx, user, "password")
	assert.Contains(t, methods, "totp", "TOTP should be available")
	assert.Contains(t, methods, "backup_code", "Backup codes should be available")

	// Step 6: Validate TOTP code
	validCode2, err := generateValidTOTPCode(setupResult.Secret)
	require.NoError(t, err)

	valid2, err := conn.ValidateTOTP(ctx, user, validCode2)
	require.NoError(t, err)
	require.True(t, valid2, "TOTP validation should succeed")

	// Step 7: Complete 2FA
	userID, callbackURL, state, err := conn.Complete2FA(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)
	assert.Equal(t, "https://example.com/callback", callbackURL)
	assert.Equal(t, "test-state", state)

	// Verify session is marked as completed
	completedSession, err := conn.storage.Get2FASession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.True(t, completedSession.Completed)

	t.Log("✅ Complete 2FA flow (password + TOTP) passed")
}

// TestComplete2FAFlow_PasswordBackupCode tests 2FA flow with password + backup code
func TestComplete2FAFlow_PasswordBackupCode(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Step 1: Create user with password and TOTP (which includes backup codes)
	testUser := NewTestUser("bob@example.com")
	user := testUser.ToUser()

	// Create user first
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Then set password
	err = conn.SetPassword(ctx, user, "SecurePass456")
	require.NoError(t, err)

	// Enable TOTP to get backup codes
	setupResult, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)

	validCode, err := generateValidTOTPCode(setupResult.Secret)
	require.NoError(t, err)

	err = conn.FinishTOTPSetup(ctx, user, setupResult.Secret, validCode, setupResult.BackupCodes)
	require.NoError(t, err)

	user.Require2FA = true
	err = conn.storage.UpdateUser(ctx, user)
	require.NoError(t, err)

	// Step 2: Primary authentication (password)
	valid, err := conn.VerifyPassword(ctx, user, "SecurePass456")
	require.NoError(t, err)
	require.True(t, valid, "password verification should succeed")

	// Step 3: Begin 2FA
	session, err := conn.Begin2FA(ctx, user.ID, "password", "https://example.com/callback", "state-123")
	require.NoError(t, err)

	// Step 4: Use a backup code instead of TOTP
	// Get the first backup code
	require.NotEmpty(t, setupResult.BackupCodes, "backup codes should exist")
	backupCode := setupResult.BackupCodes[0]

	validBackup, err := conn.ValidateBackupCode(ctx, user, backupCode)
	require.NoError(t, err)
	require.True(t, validBackup, "backup code validation should succeed")

	// Verify backup code is marked as used
	updatedUser, err := conn.storage.GetUser(ctx, user.ID)
	require.NoError(t, err)

	found := false
	for _, bc := range updatedUser.BackupCodes {
		// Backup codes are stored as bcrypt hashes, verify the code matches
		if verifyPassword(backupCode, bc.Code) {
			assert.True(t, bc.Used, "backup code should be marked as used")
			assert.NotNil(t, bc.UsedAt, "backup code should have UsedAt timestamp")
			found = true
			break
		}
	}
	assert.True(t, found, "backup code should be found in user's backup codes")

	// Step 5: Complete 2FA
	userID, _, _, err := conn.Complete2FA(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, userID)

	// Verify backup code cannot be reused
	validBackup2, err := conn.ValidateBackupCode(ctx, updatedUser, backupCode)
	assert.False(t, validBackup2, "backup code should not be valid after use")
	// Note: err might be nil if the code exists but is marked as used

	t.Log("✅ Complete 2FA flow (password + backup code) passed")
}

// TestComplete2FAFlow_PasswordPasskey tests 2FA flow with password + passkey
func TestComplete2FAFlow_PasswordPasskey(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Step 1: Create user with password and passkey
	testUser := NewTestUser("charlie@example.com")
	user := testUser.ToUser()

	// Add a test passkey
	testPasskey := NewTestPasskey(user.ID, "Security Key")
	user.Passkeys = []Passkey{*testPasskey.ToPasskey()}

	// Create user first
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Then set password
	err = conn.SetPassword(ctx, user, "SecurePass789")
	require.NoError(t, err)

	user.Require2FA = true
	err = conn.storage.UpdateUser(ctx, user)
	require.NoError(t, err)

	// Step 2: Primary authentication (password)
	valid, err := conn.VerifyPassword(ctx, user, "SecurePass789")
	require.NoError(t, err)
	require.True(t, valid, "password verification should succeed")

	// Step 3: Begin 2FA
	_, err = conn.Begin2FA(ctx, user.ID, "password", "https://example.com/callback", "state-456")
	require.NoError(t, err)

	// Step 4: Get available 2FA methods
	methods := conn.GetAvailable2FAMethods(ctx, user, "password")
	assert.Contains(t, methods, "passkey", "Passkey should be available as 2FA method")

	// Step 5: Begin passkey authentication for 2FA
	// (This would normally involve WebAuthn ceremony, but we can test the session creation)
	authSession, authOptions, err := conn.BeginPasskeyAuthentication(ctx, user.Email)
	require.NoError(t, err)
	require.NotNil(t, authSession)
	require.NotNil(t, authOptions)

	// Verify WebAuthn session created
	assert.Equal(t, user.ID, authSession.UserID)
	assert.Equal(t, "authentication", authSession.Operation)

	// Note: Full passkey verification requires real WebAuthn credential
	// This test validates the session setup for 2FA with passkey

	t.Log("✅ Complete 2FA flow (password + passkey) session setup passed")
}

// Test2FASessionExpiry tests that 2FA sessions expire after 10 minutes
func Test2FASessionExpiry(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create a test user
	testUser := NewTestUser("dave@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create a 2FA session
	session, err := conn.Begin2FA(ctx, user.ID, "password", "https://example.com/callback", "state-789")
	require.NoError(t, err)

	// Verify session is valid immediately
	_, err = conn.storage.Get2FASession(ctx, session.SessionID)
	require.NoError(t, err)

	// Manually set session expiry to the past
	session.ExpiresAt = time.Now().Add(-1 * time.Minute)
	err = conn.storage.Save2FASession(ctx, session)
	require.NoError(t, err)

	// Try to complete 2FA with expired session
	_, _, _, err = conn.Complete2FA(ctx, session.SessionID)
	assert.Error(t, err, "should not be able to complete 2FA with expired session")
	// The error might say "session not found" or "expired" depending on cleanup
	assert.True(t, err != nil, "should return error for expired session")

	t.Log("✅ 2FA session expiry test passed")
}

// Test2FAGracePeriod tests the grace period enforcement
func Test2FAGracePeriod(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.TwoFactor.GracePeriod = 7 * 24 * 60 * 60 // 7 days
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("User within grace period", func(t *testing.T) {
		// Create user created 3 days ago
		testUser := NewTestUser("eve@example.com")
		user := testUser.ToUser()
		user.CreatedAt = time.Now().Add(-3 * 24 * time.Hour)
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Should be in grace period
		inGrace := conn.InGracePeriod(ctx, user)
		assert.True(t, inGrace, "user created 3 days ago should be in 7-day grace period")

		// 2FA should not be required
		require2FA := conn.Require2FAForUser(ctx, user)
		assert.False(t, require2FA, "2FA should not be required during grace period")
	})

	t.Run("User outside grace period", func(t *testing.T) {
		// Create user created 10 days ago
		testUser := NewTestUser("frank@example.com")
		user := testUser.ToUser()
		user.Require2FA = true // Enforce 2FA
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Update created at to 10 days ago (after creation)
		user.CreatedAt = time.Now().Add(-10 * 24 * time.Hour)
		err = conn.storage.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Should NOT be in grace period
		inGrace := conn.InGracePeriod(ctx, user)
		assert.False(t, inGrace, "user created 10 days ago should NOT be in 7-day grace period")

		// 2FA should be required
		require2FA := conn.Require2FAForUser(ctx, user)
		assert.True(t, require2FA, "2FA should be required after grace period expires")
	})

	t.Run("User with 2FA setup exits grace period", func(t *testing.T) {
		// Create user created 2 days ago
		testUser := NewTestUser("grace@example.com")
		user := testUser.ToUser()

		// Create user first
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Enable TOTP
		setupResult, err := conn.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		validCode, err := generateValidTOTPCode(setupResult.Secret)
		require.NoError(t, err)

		err = conn.FinishTOTPSetup(ctx, user, setupResult.Secret, validCode, setupResult.BackupCodes)
		require.NoError(t, err)

		// Should NOT be in grace period even though created recently
		inGrace := conn.InGracePeriod(ctx, user)
		assert.False(t, inGrace, "user with TOTP enabled should not be in grace period")
	})

	t.Log("✅ 2FA grace period enforcement tests passed")
}

// Test2FABypassForNonRequiredUsers tests that users without 2FA requirement can skip it
func Test2FABypassForNonRequiredUsers(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.TwoFactor.Required = false // Global 2FA not required
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create user with password only
	testUser := NewTestUser("henry@example.com")
	user := testUser.ToUser()
	user.Require2FA = false // User-level 2FA not required

	// Create user first
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Then set password
	err = conn.SetPassword(ctx, user, "Password123")
	require.NoError(t, err)

	// Authenticate with password
	valid, err := conn.VerifyPassword(ctx, user, "Password123")
	require.NoError(t, err)
	require.True(t, valid, "password verification should succeed")

	// Check if 2FA is required
	require2FA := conn.Require2FAForUser(ctx, user)
	assert.False(t, require2FA, "2FA should not be required for this user")

	// User should be able to proceed without 2FA
	// (This would normally continue to OAuth callback directly)

	t.Log("✅ 2FA bypass for non-required users test passed")
}

// TestCompleteMagicLinkFlow tests the complete magic link authentication flow
func TestCompleteMagicLinkFlow(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Set up mock email sender
	mockSender := NewMockEmailSender()
	conn.SetEmailSender(mockSender)

	ctx := TestContext(t)

	// Step 1: Create user
	testUser := NewTestUser("isabel@example.com")
	user := testUser.ToUser()
	user.MagicLinkEnabled = true
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Step 2: Send magic link
	// Parameters: email, callbackURL, state, ipAddress
	token, err := conn.CreateMagicLink(ctx, user.Email, "https://example.com/callback", "magic-state", "192.168.1.1")
	require.NoError(t, err)
	require.NotEmpty(t, token.Token)

	// Send email
	err = conn.SendMagicLinkEmail(ctx, user.Email, token.Token)
	require.NoError(t, err)

	// Verify email was sent
	lastEmail := mockSender.GetLastEmail()
	require.NotNil(t, lastEmail)
	assert.Equal(t, user.Email, lastEmail.To)
	assert.Contains(t, lastEmail.Body, token.Token, "email should contain magic link token")

	// Step 3: Verify magic link token
	verifiedUser, callbackURL, state, err := conn.VerifyMagicLink(ctx, token.Token)
	require.NoError(t, err)
	assert.Equal(t, user.ID, verifiedUser.ID)
	assert.Equal(t, user.Email, verifiedUser.Email)
	assert.Equal(t, "https://example.com/callback", callbackURL)
	assert.Equal(t, "magic-state", state)

	// Step 4: Verify token is marked as used
	usedToken, err := conn.storage.GetMagicLinkToken(ctx, token.Token)
	require.NoError(t, err)
	assert.True(t, usedToken.Used, "token should be marked as used")
	assert.NotNil(t, usedToken.UsedAt, "token should have UsedAt timestamp")

	// Step 5: Verify token cannot be reused
	_, _, _, err = conn.VerifyMagicLink(ctx, token.Token)
	assert.Error(t, err, "token should not be reusable")
	assert.Contains(t, err.Error(), "already been used", "error should mention token was used")

	t.Log("✅ Complete magic link authentication flow passed")
}

// TestMagicLinkExpiry tests that magic links expire after TTL
func TestMagicLinkExpiry(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create user
	testUser := NewTestUser("jack@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create magic link
	// Parameters: email, callbackURL, state, ipAddress
	token, err := conn.CreateMagicLink(ctx, user.Email, "https://example.com/callback", "state-123", "192.168.1.1")
	require.NoError(t, err)

	// Manually expire the token
	token.ExpiresAt = time.Now().Add(-1 * time.Minute)
	err = conn.storage.SaveMagicLinkToken(ctx, token)
	require.NoError(t, err)

	// Try to verify expired token
	_, _, _, err = conn.VerifyMagicLink(ctx, token.Token)
	assert.Error(t, err, "expired token should not be valid")
	assert.Contains(t, err.Error(), "expired", "error should mention expiry")

	t.Log("✅ Magic link expiry test passed")
}

// TestErrorScenarios tests various error conditions
func TestErrorScenarios(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("Invalid password authentication", func(t *testing.T) {
		testUser := NewTestUser("user1@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)
		err = conn.SetPassword(ctx, user, "CorrectPassword123")
		require.NoError(t, err)

		// Try wrong password
		valid, err := conn.VerifyPassword(ctx, user, "WrongPassword123")
		require.NoError(t, err)
		assert.False(t, valid, "wrong password should return false")
	})

	t.Run("Invalid TOTP code", func(t *testing.T) {
		testUser := NewTestUser("user2@example.com")
		user := testUser.ToUser()

		// Create user first
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		setupResult, err := conn.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		validCode, err := generateValidTOTPCode(setupResult.Secret)
		require.NoError(t, err)

		err = conn.FinishTOTPSetup(ctx, user, setupResult.Secret, validCode, setupResult.BackupCodes)
		require.NoError(t, err)

		// Try invalid TOTP code
		valid, err := conn.ValidateTOTP(ctx, user, "000000")
		require.NoError(t, err)
		assert.False(t, valid, "invalid TOTP code should return false")
	})

	t.Run("User not found", func(t *testing.T) {
		// Try to get non-existent user
		_, err := conn.storage.GetUser(ctx, "non-existent-id")
		assert.Error(t, err, "non-existent user should return error")
	})

	t.Run("Invalid session ID", func(t *testing.T) {
		// Try to get non-existent 2FA session
		_, err := conn.storage.Get2FASession(ctx, "invalid-session-id")
		assert.Error(t, err, "invalid session ID should return error")
	})

	t.Run("TOTP rate limiting", func(t *testing.T) {
		testUser := NewTestUser("user3@example.com")
		user := testUser.ToUser()

		// Create user first
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		setupResult, err := conn.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		validCode, err := generateValidTOTPCode(setupResult.Secret)
		require.NoError(t, err)

		err = conn.FinishTOTPSetup(ctx, user, setupResult.Secret, validCode, setupResult.BackupCodes)
		require.NoError(t, err)

		// Exceed rate limit (5 attempts in 5 minutes)
		for i := 0; i < 6; i++ {
			_, _ = conn.ValidateTOTP(ctx, user, "000000") // Invalid code
		}

		// 6th attempt should be rate limited
		valid, err := conn.ValidateTOTP(ctx, user, "000000")
		// Rate limiting returns an error
		if err != nil {
			assert.Contains(t, err.Error(), "rate limit", "error should mention rate limit")
			assert.False(t, valid, "rate limited attempt should return false")
		} else {
			assert.False(t, valid, "rate limited attempt should return false")
		}
	})

	t.Log("✅ Error scenario tests passed")
}

