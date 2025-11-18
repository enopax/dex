package local

import (
	"fmt"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBegin2FA tests the Begin2FA function
func TestBegin2FA(t *testing.T) {
	t.Run("creates session with correct fields", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		callbackURL := "https://dex.example.com/callback"
		state := "test-state-123"
		primaryMethod := "password"

		session, err := conn.Begin2FA(ctx, user.ID, primaryMethod, callbackURL, state)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.NotEmpty(t, session.SessionID)

		// Retrieve session and verify
		retrievedSession, err := conn.storage.Get2FASession(ctx, session.SessionID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrievedSession.UserID)
		assert.Equal(t, primaryMethod, retrievedSession.PrimaryMethod)
		assert.Equal(t, callbackURL, retrievedSession.CallbackURL)
		assert.Equal(t, state, retrievedSession.State)
		assert.False(t, retrievedSession.Completed)
	})

	t.Run("sets 10-minute expiry", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		session, err := conn.Begin2FA(ctx, user.ID, "password", "https://callback.example.com", "state")
		require.NoError(t, err)

		expectedExpiry := session.CreatedAt.Add(10 * time.Minute)
		assert.WithinDuration(t, expectedExpiry, session.ExpiresAt, time.Second)
	})

	t.Run("stores session in storage", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		session, err := conn.Begin2FA(ctx, user.ID, "password", "https://callback.example.com", "state")
		require.NoError(t, err)

		// Verify session can be retrieved from storage
		retrievedSession, err := conn.storage.Get2FASession(ctx, session.SessionID)
		require.NoError(t, err)
		assert.Equal(t, session.SessionID, retrievedSession.SessionID)
		assert.Equal(t, session.UserID, retrievedSession.UserID)
	})
}

// TestComplete2FA tests the Complete2FA function
func TestComplete2FA(t *testing.T) {
	t.Run("validates and marks session complete", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		session, err := conn.Begin2FA(ctx, user.ID, "password", "https://callback.example.com", "state")
		require.NoError(t, err)

		userID, callbackURL, state, err := conn.Complete2FA(ctx, session.SessionID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, userID)
		assert.Equal(t, "https://callback.example.com", callbackURL)
		assert.Equal(t, "state", state)

		// Session should be marked complete
		retrievedSession, err := conn.storage.Get2FASession(ctx, session.SessionID)
		require.NoError(t, err)
		assert.True(t, retrievedSession.Completed)
	})

	t.Run("returns error for invalid session", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)

		userID, callbackURL, state, err := conn.Complete2FA(ctx, "invalid-session-id")
		assert.Error(t, err)
		assert.Empty(t, userID)
		assert.Empty(t, callbackURL)
		assert.Empty(t, state)
	})

	t.Run("returns error for expired session", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Create expired session
		session := &TwoFactorSession{
			SessionID:     "expired-session",
			UserID:        user.ID,
			PrimaryMethod: "password",
			CreatedAt:     time.Now().Add(-15 * time.Minute),
			ExpiresAt:     time.Now().Add(-5 * time.Minute),
			Completed:     false,
			CallbackURL:   "https://callback.example.com",
			State:         "state",
		}
		err = conn.storage.Save2FASession(ctx, session)
		require.NoError(t, err)

		userID, callbackURL, state, err := conn.Complete2FA(ctx, session.SessionID)
		assert.Error(t, err)
		assert.Empty(t, userID)
		assert.Empty(t, callbackURL)
		assert.Empty(t, state)
		// Session is deleted when expired, so we get "session not found" error
		assert.True(t, err.Error() == "session not found: 2FA session not found" || err.Error() == "2FA session expired")
	})
}

// TestRequire2FAForUser tests the Require2FAForUser function
func TestRequire2FAForUser(t *testing.T) {
	t.Run("returns true when user.Require2FA is true", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.Require2FA = true
		user := testUser.ToUser()

		required := conn.Require2FAForUser(ctx, user)
		assert.True(t, required)
	})

	t.Run("returns true when global TwoFactor.Required is true", func(t *testing.T) {
		config := DefaultTestConfig(t)
		config.TwoFactor.Required = true
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.Require2FA = false
		user := testUser.ToUser()

		required := conn.Require2FAForUser(ctx, user)
		assert.True(t, required)
	})

	t.Run("returns true when user has TOTP enabled", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.Require2FA = false
		testUser.TOTPSecret = "JBSWY3DPEHPK3PXP"
		testUser.TOTPEnabled = true
		user := testUser.ToUser()

		required := conn.Require2FAForUser(ctx, user)
		assert.True(t, required)
	})

	t.Run("returns true when user has password and passkey with global config", func(t *testing.T) {
		config := DefaultTestConfig(t)
		config.TwoFactor.Required = true
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.PasswordHash = "$2a$10$abcdefghijklmnopqrstuv"
		user := testUser.ToUser()

		// Add passkey to user after ToUser()
		testPasskey := NewTestPasskey(user.ID, "Test Passkey")
		user.Passkeys = []Passkey{*testPasskey.ToPasskey()}

		required := conn.Require2FAForUser(ctx, user)
		assert.True(t, required)
	})

	t.Run("returns false for user with only password", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.PasswordHash = "$2a$10$abcdefghijklmnopqrstuv"
		user := testUser.ToUser()

		required := conn.Require2FAForUser(ctx, user)
		assert.False(t, required)
	})

	t.Run("returns false for user with only passkey", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()

		// Add passkey to user after ToUser()
		testPasskey := NewTestPasskey(user.ID, "Test Passkey")
		user.Passkeys = []Passkey{*testPasskey.ToPasskey()}

		required := conn.Require2FAForUser(ctx, user)
		assert.False(t, required)
	})
}

// TestGetAvailable2FAMethods tests the GetAvailable2FAMethods function
func TestGetAvailable2FAMethods(t *testing.T) {
	t.Run("returns totp when user has TOTP enabled", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.TOTPSecret = "JBSWY3DPEHPK3PXP"
		testUser.TOTPEnabled = true
		user := testUser.ToUser()

		methods := conn.GetAvailable2FAMethods(ctx, user, "password")
		assert.Contains(t, methods, "totp")
	})

	t.Run("returns passkey when user has passkeys and passkey not primary", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()

		// Add passkey to user after ToUser()
		testPasskey := NewTestPasskey(user.ID, "Test Passkey")
		user.Passkeys = []Passkey{*testPasskey.ToPasskey()}

		methods := conn.GetAvailable2FAMethods(ctx, user, "password")
		assert.Contains(t, methods, "passkey")
	})

	t.Run("excludes passkey when passkey was primary method", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.TOTPSecret = "JBSWY3DPEHPK3PXP"
		testUser.TOTPEnabled = true
		user := testUser.ToUser()

		// Add passkey to user after ToUser()
		testPasskey := NewTestPasskey(user.ID, "Test Passkey")
		user.Passkeys = []Passkey{*testPasskey.ToPasskey()}

		methods := conn.GetAvailable2FAMethods(ctx, user, "passkey")
		assert.NotContains(t, methods, "passkey")
		assert.Contains(t, methods, "totp") // TOTP should still be available
	})

	t.Run("returns backup_code when user has unused backup codes", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()

		// Add unused backup codes
		for i := 0; i < 10; i++ {
			code := fmt.Sprintf("TEST%04d", i+1)
			hash, _ := hashPassword(code)
			user.BackupCodes = append(user.BackupCodes, BackupCode{
				Code: hash,
				Used: false,
			})
		}

		methods := conn.GetAvailable2FAMethods(ctx, user, "password")
		assert.Contains(t, methods, "backup_code")
	})

	t.Run("excludes backup_code when all codes used", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()

		// Add all used backup codes
		now := time.Now()
		for i := 0; i < 10; i++ {
			code := fmt.Sprintf("TEST%04d", i+1)
			hash, _ := hashPassword(code)
			user.BackupCodes = append(user.BackupCodes, BackupCode{
				Code:   hash,
				Used:   true,
				UsedAt: &now,
			})
		}

		methods := conn.GetAvailable2FAMethods(ctx, user, "password")
		assert.NotContains(t, methods, "backup_code")
	})

	t.Run("returns empty array when no 2FA methods available", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()

		methods := conn.GetAvailable2FAMethods(ctx, user, "password")
		assert.Empty(t, methods)
	})
}

// TestInGracePeriod tests the InGracePeriod function
func TestInGracePeriod(t *testing.T) {
	t.Run("returns true when within grace period", func(t *testing.T) {
		config := DefaultTestConfig(t)
		config.TwoFactor.GracePeriod = 7 * 24 * 60 * 60 // 7 days
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		user.CreatedAt = time.Now().Add(-3 * 24 * time.Hour) // 3 days ago

		inGrace := conn.InGracePeriod(ctx, user)
		assert.True(t, inGrace)
	})

	t.Run("returns false when grace period expired", func(t *testing.T) {
		config := DefaultTestConfig(t)
		config.TwoFactor.GracePeriod = 7 * 24 * 60 * 60 // 7 days
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		user.CreatedAt = time.Now().Add(-10 * 24 * time.Hour) // 10 days ago

		inGrace := conn.InGracePeriod(ctx, user)
		assert.False(t, inGrace)
	})

	t.Run("returns false when user has TOTP enabled", func(t *testing.T) {
		config := DefaultTestConfig(t)
		config.TwoFactor.GracePeriod = 7 * 24 * 60 * 60 // 7 days
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		testUser.TOTPSecret = "JBSWY3DPEHPK3PXP"
		testUser.TOTPEnabled = true
		user := testUser.ToUser()
		user.CreatedAt = time.Now().Add(-3 * 24 * time.Hour) // 3 days ago

		inGrace := conn.InGracePeriod(ctx, user)
		assert.False(t, inGrace)
	})

	t.Run("returns false when user has passkey", func(t *testing.T) {
		config := DefaultTestConfig(t)
		config.TwoFactor.GracePeriod = 7 * 24 * 60 * 60 // 7 days
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		user.CreatedAt = time.Now().Add(-3 * 24 * time.Hour) // 3 days ago

		// Add passkey
		testPasskey := NewTestPasskey(user.ID, "Test Passkey")
		user.Passkeys = []Passkey{*testPasskey.ToPasskey()}

		inGrace := conn.InGracePeriod(ctx, user)
		assert.False(t, inGrace)
	})

	t.Run("edge case: user created exactly at grace period boundary", func(t *testing.T) {
		config := DefaultTestConfig(t)
		config.TwoFactor.GracePeriod = 7 * 24 * 60 * 60 // 7 days
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		user.CreatedAt = time.Now().Add(-7 * 24 * time.Hour) // Exactly 7 days ago

		inGrace := conn.InGracePeriod(ctx, user)
		// Should be false since we've reached the boundary
		assert.False(t, inGrace)
	})
}

// TestValidate2FAMethod tests the Validate2FAMethod function
func TestValidate2FAMethod(t *testing.T) {
	t.Run("validates TOTP code correctly", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Enable TOTP
		secret := "JBSWY3DPEHPK3PXP"

		// Manually enable TOTP
		user.TOTPSecret = &secret
		user.TOTPEnabled = true
		err = conn.storage.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Create 2FA session
		session, err := conn.Begin2FA(ctx, user.ID, "password", "https://callback.example.com", "state")
		require.NoError(t, err)

		// Generate fresh TOTP code for validation
		validCode, err := totp.GenerateCode(secret, time.Now())
		require.NoError(t, err)

		// Validate TOTP
		valid, err := conn.Validate2FAMethod(ctx, session.SessionID, "totp", validCode)
		assert.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("validates backup code correctly", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Add backup codes
		testCode := "TEST0001"
		hash, err := hashPassword(testCode)
		require.NoError(t, err)

		user.BackupCodes = []BackupCode{{Code: hash, Used: false}}
		err = conn.storage.UpdateUser(ctx, user)
		require.NoError(t, err)

		// Create 2FA session
		session, err := conn.Begin2FA(ctx, user.ID, "password", "https://callback.example.com", "state")
		require.NoError(t, err)

		// Validate backup code
		valid, err := conn.Validate2FAMethod(ctx, session.SessionID, "backup_code", testCode)
		assert.NoError(t, err)
		assert.True(t, valid)

		// Verify backup code marked as used
		user, err = conn.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.True(t, user.BackupCodes[0].Used)
	})

	t.Run("returns error for invalid method", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		session, err := conn.Begin2FA(ctx, user.ID, "password", "https://callback.example.com", "state")
		require.NoError(t, err)

		valid, err := conn.Validate2FAMethod(ctx, session.SessionID, "invalid_method", "test")
		assert.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "unknown 2FA method")
	})

	t.Run("returns error for expired session", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		conn, err := New(config, TestLogger(t))
		require.NoError(t, err)

		ctx := TestContext(t)
		testUser := NewTestUser("test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Create expired session
		session := &TwoFactorSession{
			SessionID:     "expired-session",
			UserID:        user.ID,
			PrimaryMethod: "password",
			CreatedAt:     time.Now().Add(-15 * time.Minute),
			ExpiresAt:     time.Now().Add(-5 * time.Minute),
			Completed:     false,
			CallbackURL:   "https://callback.example.com",
			State:         "state",
		}
		err = conn.storage.Save2FASession(ctx, session)
		require.NoError(t, err)

		valid, err := conn.Validate2FAMethod(ctx, session.SessionID, "totp", "123456")
		assert.Error(t, err)
		assert.False(t, valid)
	})
}
