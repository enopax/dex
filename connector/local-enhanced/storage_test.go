package local

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileStorageCreation tests creating a new file storage backend.
func TestFileStorageCreation(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)
	assert.NotNil(t, storage)

	// Verify directories were created
	AssertDirPermissions(t, filepath.Join(dataDir, "users"), 0700)
	AssertDirPermissions(t, filepath.Join(dataDir, "passkeys"), 0700)
	AssertDirPermissions(t, filepath.Join(dataDir, "sessions"), 0700)
	AssertDirPermissions(t, filepath.Join(dataDir, "tokens"), 0700)
}

// TestUserCRUD tests user create, read, update, delete operations.
func TestUserCRUD(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create user
	testUser := NewTestUser("alice@example.com")
	user := testUser.ToUser()
	err = storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Verify user file was created
	userPath := filepath.Join(dataDir, "users", user.ID+".json")
	AssertFileExists(t, userPath)
	AssertFilePermissions(t, userPath, 0600)

	// Read user by ID
	retrievedUser, err := storage.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Email, retrievedUser.Email)
	assert.Equal(t, user.Username, retrievedUser.Username)
	assert.False(t, retrievedUser.CreatedAt.IsZero())
	assert.False(t, retrievedUser.UpdatedAt.IsZero())

	// Read user by email
	retrievedUser2, err := storage.GetUserByEmail(ctx, "alice@example.com")
	require.NoError(t, err)
	assert.Equal(t, user.ID, retrievedUser2.ID)

	// Update user
	retrievedUser.Username = "alice_updated"
	retrievedUser.EmailVerified = true
	err = storage.UpdateUser(ctx, retrievedUser)
	require.NoError(t, err)

	// Verify update
	updatedUser, err := storage.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "alice_updated", updatedUser.Username)
	assert.True(t, updatedUser.EmailVerified)
	assert.True(t, updatedUser.UpdatedAt.After(updatedUser.CreatedAt))

	// Delete user
	err = storage.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify user was deleted
	AssertFileNotExists(t, userPath)
	_, err = storage.GetUser(ctx, user.ID)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestUserAlreadyExists tests creating a user that already exists.
func TestUserAlreadyExists(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)
	testUser := NewTestUser("bob@example.com")
	user := testUser.ToUser()

	// Create user first time - should succeed
	err = storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create same user again - should fail
	err = storage.CreateUser(ctx, user)
	assert.ErrorIs(t, err, ErrUserAlreadyExists)
}

// TestUserNotFound tests reading a non-existent user.
func TestUserNotFound(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Try to get non-existent user
	_, err = storage.GetUser(ctx, "non-existent-id")
	assert.ErrorIs(t, err, ErrUserNotFound)

	// Try to get user by non-existent email
	_, err = storage.GetUserByEmail(ctx, "nonexistent@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound)

	// Try to update non-existent user
	testUser := NewTestUser("test@example.com")
	user := testUser.ToUser()
	err = storage.UpdateUser(ctx, user)
	assert.ErrorIs(t, err, ErrUserNotFound)

	// Try to delete non-existent user
	err = storage.DeleteUser(ctx, "non-existent-id")
	assert.ErrorIs(t, err, ErrUserNotFound)
}

// TestListUsers tests listing all users.
func TestListUsers(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Initially no users
	users, err := storage.ListUsers(ctx)
	require.NoError(t, err)
	assert.Empty(t, users)

	// Create multiple users
	testUser1 := NewTestUser("alice@example.com")
	testUser2 := NewTestUser("bob@example.com")
	testUser3 := NewTestUser("charlie@example.com")

	require.NoError(t, storage.CreateUser(ctx, testUser1.ToUser()))
	require.NoError(t, storage.CreateUser(ctx, testUser2.ToUser()))
	require.NoError(t, storage.CreateUser(ctx, testUser3.ToUser()))

	// List users
	users, err = storage.ListUsers(ctx)
	require.NoError(t, err)
	assert.Len(t, users, 3)

	// Verify all users are in the list
	emails := make(map[string]bool)
	for _, u := range users {
		emails[u.Email] = true
	}
	assert.True(t, emails["alice@example.com"])
	assert.True(t, emails["bob@example.com"])
	assert.True(t, emails["charlie@example.com"])
}

// TestPasskeyCRUD tests passkey create, read, list, delete operations.
func TestPasskeyCRUD(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create user first
	testUser := NewTestUser("alice@example.com")
	user := testUser.ToUser()
	require.NoError(t, storage.CreateUser(ctx, user))

	// Create passkey
	testPasskey := NewTestPasskey(user.ID, "My Security Key")
	passkey := testPasskey.ToPasskey()
	err = storage.SavePasskey(ctx, passkey)
	require.NoError(t, err)

	// Verify passkey file was created
	passkeyPath := filepath.Join(dataDir, "passkeys", passkey.ID+".json")
	AssertFileExists(t, passkeyPath)
	AssertFilePermissions(t, passkeyPath, 0600)

	// Read passkey
	retrievedPasskey, err := storage.GetPasskey(ctx, passkey.ID)
	require.NoError(t, err)
	assert.Equal(t, passkey.UserID, retrievedPasskey.UserID)
	assert.Equal(t, passkey.Name, retrievedPasskey.Name)
	assert.Equal(t, passkey.PublicKey, retrievedPasskey.PublicKey)

	// List passkeys for user
	passkeys, err := storage.ListPasskeys(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, passkeys, 1)
	assert.Equal(t, passkey.ID, passkeys[0].ID)

	// Delete passkey
	err = storage.DeletePasskey(ctx, passkey.ID)
	require.NoError(t, err)

	// Verify passkey was deleted
	AssertFileNotExists(t, passkeyPath)
	_, err = storage.GetPasskey(ctx, passkey.ID)
	assert.ErrorIs(t, err, ErrPasskeyNotFound)
}

// TestListPasskeysMultipleUsers tests listing passkeys filters by user.
func TestListPasskeysMultipleUsers(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create two users
	testUser1 := NewTestUser("alice@example.com")
	testUser2 := NewTestUser("bob@example.com")
	user1 := testUser1.ToUser()
	user2 := testUser2.ToUser()
	require.NoError(t, storage.CreateUser(ctx, user1))
	require.NoError(t, storage.CreateUser(ctx, user2))

	// Create passkeys for user1
	testPasskey1a := NewTestPasskey(user1.ID, "Alice's YubiKey")
	testPasskey1b := NewTestPasskey(user1.ID, "Alice's Phone")
	require.NoError(t, storage.SavePasskey(ctx, testPasskey1a.ToPasskey()))
	require.NoError(t, storage.SavePasskey(ctx, testPasskey1b.ToPasskey()))

	// Create passkey for user2
	testPasskey2 := NewTestPasskey(user2.ID, "Bob's Security Key")
	require.NoError(t, storage.SavePasskey(ctx, testPasskey2.ToPasskey()))

	// List passkeys for user1
	passkeys1, err := storage.ListPasskeys(ctx, user1.ID)
	require.NoError(t, err)
	assert.Len(t, passkeys1, 2)

	// List passkeys for user2
	passkeys2, err := storage.ListPasskeys(ctx, user2.ID)
	require.NoError(t, err)
	assert.Len(t, passkeys2, 1)
	assert.Equal(t, "Bob's Security Key", passkeys2[0].Name)
}

// TestWebAuthnSessionCRUD tests WebAuthn session operations.
func TestWebAuthnSessionCRUD(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create session
	testSession := NewTestWebAuthnSession("user-123", "registration", 5*time.Minute)
	session := testSession.ToWebAuthnSession()
	err = storage.SaveWebAuthnSession(ctx, session)
	require.NoError(t, err)

	// Read session
	retrievedSession, err := storage.GetWebAuthnSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, session.UserID, retrievedSession.UserID)
	assert.Equal(t, session.Operation, retrievedSession.Operation)
	assert.Equal(t, session.Challenge, retrievedSession.Challenge)

	// Delete session
	err = storage.DeleteWebAuthnSession(ctx, session.SessionID)
	require.NoError(t, err)

	// Verify session was deleted
	_, err = storage.GetWebAuthnSession(ctx, session.SessionID)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

// TestWebAuthnSessionExpiry tests that expired sessions cannot be retrieved.
func TestWebAuthnSessionExpiry(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create session that expires in the past
	testSession := NewTestWebAuthnSession("user-123", "authentication", -1*time.Minute)
	session := testSession.ToWebAuthnSession()
	err = storage.SaveWebAuthnSession(ctx, session)
	require.NoError(t, err)

	// Try to read expired session - should fail
	_, err = storage.GetWebAuthnSession(ctx, session.SessionID)
	assert.ErrorIs(t, err, ErrSessionNotFound)
}

// TestMagicLinkTokenCRUD tests magic link token operations.
func TestMagicLinkTokenCRUD(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create token
	testToken := NewTestMagicLinkToken("user-123", "alice@example.com", 10*time.Minute)
	token := testToken.ToMagicLinkToken()
	err = storage.SaveMagicLinkToken(ctx, token)
	require.NoError(t, err)

	// Read token
	retrievedToken, err := storage.GetMagicLinkToken(ctx, token.Token)
	require.NoError(t, err)
	assert.Equal(t, token.UserID, retrievedToken.UserID)
	assert.Equal(t, token.Email, retrievedToken.Email)
	assert.False(t, retrievedToken.Used)

	// Delete token
	err = storage.DeleteMagicLinkToken(ctx, token.Token)
	require.NoError(t, err)

	// Verify token was deleted
	_, err = storage.GetMagicLinkToken(ctx, token.Token)
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

// TestMagicLinkTokenExpiry tests that expired tokens cannot be retrieved.
func TestMagicLinkTokenExpiry(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create token that expires in the past
	testToken := NewTestMagicLinkToken("user-123", "alice@example.com", -1*time.Minute)
	token := testToken.ToMagicLinkToken()
	err = storage.SaveMagicLinkToken(ctx, token)
	require.NoError(t, err)

	// Try to read expired token - should fail
	_, err = storage.GetMagicLinkToken(ctx, token.Token)
	assert.ErrorIs(t, err, ErrTokenNotFound)
}

// TestCleanupExpiredSessions tests session cleanup.
func TestCleanupExpiredSessions(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create expired session
	testExpiredSession := NewTestWebAuthnSession("user-1", "registration", -1*time.Minute)
	expiredSession := testExpiredSession.ToWebAuthnSession()
	require.NoError(t, storage.SaveWebAuthnSession(ctx, expiredSession))

	// Create valid session
	testValidSession := NewTestWebAuthnSession("user-2", "authentication", 5*time.Minute)
	validSession := testValidSession.ToWebAuthnSession()
	require.NoError(t, storage.SaveWebAuthnSession(ctx, validSession))

	// Run cleanup
	err = storage.CleanupExpiredSessions(ctx)
	require.NoError(t, err)

	// Verify expired session was deleted
	expiredPath := filepath.Join(dataDir, "sessions", expiredSession.SessionID+".json")
	AssertFileNotExists(t, expiredPath)

	// Verify valid session still exists
	validPath := filepath.Join(dataDir, "sessions", validSession.SessionID+".json")
	AssertFileExists(t, validPath)
}

// TestCleanupExpiredTokens tests token cleanup.
func TestCleanupExpiredTokens(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create expired token
	testExpiredToken := NewTestMagicLinkToken("user-1", "alice@example.com", -1*time.Minute)
	expiredToken := testExpiredToken.ToMagicLinkToken()
	require.NoError(t, storage.SaveMagicLinkToken(ctx, expiredToken))

	// Create valid token
	testValidToken := NewTestMagicLinkToken("user-2", "bob@example.com", 10*time.Minute)
	validToken := testValidToken.ToMagicLinkToken()
	require.NoError(t, storage.SaveMagicLinkToken(ctx, validToken))

	// Run cleanup
	err = storage.CleanupExpiredTokens(ctx)
	require.NoError(t, err)

	// Verify expired token was deleted
	expiredPath := filepath.Join(dataDir, "tokens", expiredToken.Token+".json")
	AssertFileNotExists(t, expiredPath)

	// Verify valid token still exists
	validPath := filepath.Join(dataDir, "tokens", validToken.Token+".json")
	AssertFileExists(t, validPath)
}

// TestGenerateUserID tests deterministic user ID generation.
func TestGenerateUserID(t *testing.T) {
	// Same email should always generate same ID
	id1 := generateUserID("alice@example.com")
	id2 := generateUserID("alice@example.com")
	assert.Equal(t, id1, id2)

	// Different emails should generate different IDs
	id3 := generateUserID("bob@example.com")
	assert.NotEqual(t, id1, id3)

	// ID should be in UUID format (8-4-4-4-12 hex digits)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id1)
}

// TestGenerateSecureToken tests secure token generation.
func TestGenerateSecureToken(t *testing.T) {
	token1, err := GenerateSecureToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token1)

	// Tokens should be unique
	token2, err := GenerateSecureToken()
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2)

	// Token should be 64 hex characters (32 bytes)
	assert.Len(t, token1, 64)
	assert.Regexp(t, `^[0-9a-f]{64}$`, token1)
}

// TestGenerateSessionID tests session ID generation.
func TestGenerateSessionID(t *testing.T) {
	sessionID1, err := GenerateSessionID()
	require.NoError(t, err)
	assert.NotEmpty(t, sessionID1)

	// Session IDs should be unique
	sessionID2, err := GenerateSessionID()
	require.NoError(t, err)
	assert.NotEqual(t, sessionID1, sessionID2)

	// Session ID should be 64 hex characters (32 bytes)
	assert.Len(t, sessionID1, 64)
}

// TestSave2FASession tests saving 2FA sessions to storage.
func TestSave2FASession(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create test 2FA session
	session := &TwoFactorSession{
		SessionID:     "test-session-id",
		UserID:        "test-user-id",
		PrimaryMethod: "password",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Completed:     false,
		CallbackURL:   "https://example.com/callback",
		State:         "test-state",
	}

	// Save session
	err = storage.Save2FASession(ctx, session)
	require.NoError(t, err)

	// Verify file was created
	sessionPath := filepath.Join(dataDir, "2fa-sessions", session.SessionID+".json")
	AssertFileExists(t, sessionPath)
	AssertFilePermissions(t, sessionPath, 0600)

	// Test concurrent saves
	t.Run("concurrent saves work correctly", func(t *testing.T) {
		done := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			go func(id int) {
				session := &TwoFactorSession{
					SessionID:     GenerateTestID(),
					UserID:        "test-user-id",
					PrimaryMethod: "password",
					CreatedAt:     time.Now(),
					ExpiresAt:     time.Now().Add(10 * time.Minute),
					Completed:     false,
					CallbackURL:   "https://example.com/callback",
					State:         "test-state",
				}
				err := storage.Save2FASession(ctx, session)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 5; i++ {
			<-done
		}
	})
}

// TestGet2FASession tests retrieving 2FA sessions from storage.
func TestGet2FASession(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create and save test session
	session := &TwoFactorSession{
		SessionID:     "test-session-id",
		UserID:        "test-user-id",
		PrimaryMethod: "password",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Completed:     false,
		CallbackURL:   "https://example.com/callback",
		State:         "test-state",
	}
	err = storage.Save2FASession(ctx, session)
	require.NoError(t, err)

	t.Run("retrieves session correctly", func(t *testing.T) {
		retrieved, err := storage.Get2FASession(ctx, session.SessionID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, session.SessionID, retrieved.SessionID)
		assert.Equal(t, session.UserID, retrieved.UserID)
		assert.Equal(t, session.PrimaryMethod, retrieved.PrimaryMethod)
		assert.Equal(t, session.CallbackURL, retrieved.CallbackURL)
		assert.Equal(t, session.State, retrieved.State)
		assert.False(t, retrieved.Completed)
	})

	t.Run("returns error for non-existent session", func(t *testing.T) {
		_, err := storage.Get2FASession(ctx, "nonexistent")
		require.Error(t, err)
	})

	t.Run("returns error for expired session", func(t *testing.T) {
		// Create expired session
		expiredSession := &TwoFactorSession{
			SessionID:     "expired-session",
			UserID:        "test-user-id",
			PrimaryMethod: "password",
			CreatedAt:     time.Now().Add(-20 * time.Minute),
			ExpiresAt:     time.Now().Add(-1 * time.Second), // Expired 1 second ago
			Completed:     false,
			CallbackURL:   "https://example.com/callback",
			State:         "test-state",
		}
		err = storage.Save2FASession(ctx, expiredSession)
		require.NoError(t, err)

		// Get2FASession returns Err2FASessionNotFound for expired sessions
		_, err = storage.Get2FASession(ctx, expiredSession.SessionID)
		require.Error(t, err)
		assert.Equal(t, Err2FASessionNotFound, err)
	})

	t.Run("validates session structure", func(t *testing.T) {
		retrieved, err := storage.Get2FASession(ctx, session.SessionID)
		require.NoError(t, err)

		// Verify all required fields are present
		assert.NotEmpty(t, retrieved.SessionID)
		assert.NotEmpty(t, retrieved.UserID)
		assert.NotEmpty(t, retrieved.PrimaryMethod)
		assert.NotZero(t, retrieved.CreatedAt)
		assert.NotZero(t, retrieved.ExpiresAt)
	})
}

// TestDelete2FASession tests deleting 2FA sessions from storage.
func TestDelete2FASession(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create and save test session
	session := &TwoFactorSession{
		SessionID:     "test-session-id",
		UserID:        "test-user-id",
		PrimaryMethod: "password",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Completed:     false,
		CallbackURL:   "https://example.com/callback",
		State:         "test-state",
	}
	err = storage.Save2FASession(ctx, session)
	require.NoError(t, err)

	t.Run("removes session file", func(t *testing.T) {
		// Verify session exists
		sessionPath := filepath.Join(dataDir, "2fa-sessions", session.SessionID+".json")
		AssertFileExists(t, sessionPath)

		// Delete session
		err = storage.Delete2FASession(ctx, session.SessionID)
		require.NoError(t, err)

		// Verify file was removed
		AssertFileNotExists(t, sessionPath)
	})

	t.Run("no error if file doesn't exist", func(t *testing.T) {
		// Delete non-existent session - should not error
		err = storage.Delete2FASession(ctx, "nonexistent")
		require.NoError(t, err)
	})

	t.Run("subsequent Get returns error", func(t *testing.T) {
		// Create session
		session2 := &TwoFactorSession{
			SessionID:     "test-session-2",
			UserID:        "test-user-id",
			PrimaryMethod: "password",
			CreatedAt:     time.Now(),
			ExpiresAt:     time.Now().Add(10 * time.Minute),
			Completed:     false,
			CallbackURL:   "https://example.com/callback",
			State:         "test-state",
		}
		err = storage.Save2FASession(ctx, session2)
		require.NoError(t, err)

		// Delete it
		err = storage.Delete2FASession(ctx, session2.SessionID)
		require.NoError(t, err)

		// Try to get it - should error
		_, err = storage.Get2FASession(ctx, session2.SessionID)
		require.Error(t, err)
	})
}

// TestCleanupExpired2FASessions tests cleanup of expired 2FA sessions.
func TestCleanupExpired2FASessions(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create expired session
	expiredSession := &TwoFactorSession{
		SessionID:     "expired-session",
		UserID:        "test-user-id",
		PrimaryMethod: "password",
		CreatedAt:     time.Now().Add(-20 * time.Minute),
		ExpiresAt:     time.Now().Add(-1 * time.Minute), // Expired 1 minute ago
		Completed:     false,
		CallbackURL:   "https://example.com/callback",
		State:         "test-state",
	}
	err = storage.Save2FASession(ctx, expiredSession)
	require.NoError(t, err)

	// Create valid session
	validSession := &TwoFactorSession{
		SessionID:     "valid-session",
		UserID:        "test-user-id",
		PrimaryMethod: "password",
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Completed:     false,
		CallbackURL:   "https://example.com/callback",
		State:         "test-state",
	}
	err = storage.Save2FASession(ctx, validSession)
	require.NoError(t, err)

	t.Run("removes expired sessions", func(t *testing.T) {
		// Run cleanup
		err = storage.CleanupExpiredSessions(ctx)
		require.NoError(t, err)

		// Expired session should be gone
		expiredPath := filepath.Join(dataDir, "2fa-sessions", expiredSession.SessionID+".json")
		AssertFileNotExists(t, expiredPath)
	})

	t.Run("keeps non-expired sessions", func(t *testing.T) {
		// Valid session should still exist
		validPath := filepath.Join(dataDir, "2fa-sessions", validSession.SessionID+".json")
		AssertFileExists(t, validPath)

		// Should be able to retrieve it
		retrieved, err := storage.Get2FASession(ctx, validSession.SessionID)
		require.NoError(t, err)
		assert.Equal(t, validSession.SessionID, retrieved.SessionID)
	})

	t.Run("works with concurrent sessions", func(t *testing.T) {
		// Create multiple sessions with different expiry times
		for i := 0; i < 10; i++ {
			var expiresAt time.Time
			if i%2 == 0 {
				// Even: expired
				expiresAt = time.Now().Add(-1 * time.Minute)
			} else {
				// Odd: valid
				expiresAt = time.Now().Add(10 * time.Minute)
			}

			session := &TwoFactorSession{
				SessionID:     GenerateTestID(),
				UserID:        "test-user-id",
				PrimaryMethod: "password",
				CreatedAt:     time.Now(),
				ExpiresAt:     expiresAt,
				Completed:     false,
				CallbackURL:   "https://example.com/callback",
				State:         "test-state",
			}
			err = storage.Save2FASession(ctx, session)
			require.NoError(t, err)
		}

		// Cleanup
		err = storage.CleanupExpiredSessions(ctx)
		require.NoError(t, err)

		// Count remaining sessions (should be ~5 valid ones plus the original valid session)
		sessionsDir := filepath.Join(dataDir, "2fa-sessions")
		files, err := filepath.Glob(filepath.Join(sessionsDir, "*.json"))
		require.NoError(t, err)
		// We expect at least 5-6 files (5 odd-numbered + 1 original valid session)
		assert.GreaterOrEqual(t, len(files), 5)
		assert.LessOrEqual(t, len(files), 6)
	})
}

// TestSaveAuthSetupToken tests saving auth setup tokens.
func TestSaveAuthSetupToken(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("creates file in auth-setup-tokens directory", func(t *testing.T) {
		token := &AuthSetupToken{
			Token:     "test-token-abc123",
			UserID:    "user-id-123",
			Email:     "user@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Used:      false,
			ReturnURL: "https://platform.example.com/dashboard",
		}

		err = storage.SaveAuthSetupToken(ctx, token)
		require.NoError(t, err)

		// Verify file was created
		tokenPath := filepath.Join(dataDir, "auth-setup-tokens", token.Token+".json")
		AssertFileExists(t, tokenPath)
		AssertFilePermissions(t, tokenPath, 0600)
	})

	t.Run("concurrent saves work correctly", func(t *testing.T) {
		// Create multiple tokens concurrently
		tokenCount := 5
		done := make(chan bool, tokenCount)

		for i := 0; i < tokenCount; i++ {
			go func(index int) {
				token := &AuthSetupToken{
					Token:     GenerateTestID(),
					UserID:    "user-id-" + GenerateTestID(),
					Email:     "user" + GenerateTestID() + "@example.com",
					CreatedAt: time.Now(),
					ExpiresAt: time.Now().Add(24 * time.Hour),
					Used:      false,
					ReturnURL: "https://platform.example.com/dashboard",
				}
				err := storage.SaveAuthSetupToken(ctx, token)
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < tokenCount; i++ {
			<-done
		}

		// Verify all files were created
		tokensDir := filepath.Join(dataDir, "auth-setup-tokens")
		files, err := filepath.Glob(filepath.Join(tokensDir, "*.json"))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(files), tokenCount)
	})
}

// TestGetAuthSetupToken tests retrieving auth setup tokens.
func TestGetAuthSetupToken(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("retrieves token correctly", func(t *testing.T) {
		token := &AuthSetupToken{
			Token:     "test-token-retrieve",
			UserID:    "user-id-123",
			Email:     "user@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Used:      false,
			ReturnURL: "https://platform.example.com/dashboard",
		}

		err = storage.SaveAuthSetupToken(ctx, token)
		require.NoError(t, err)

		retrieved, err := storage.GetAuthSetupToken(ctx, token.Token)
		require.NoError(t, err)
		assert.Equal(t, token.Token, retrieved.Token)
		assert.Equal(t, token.UserID, retrieved.UserID)
		assert.Equal(t, token.Email, retrieved.Email)
		assert.Equal(t, token.ReturnURL, retrieved.ReturnURL)
		assert.False(t, retrieved.Used)
	})

	t.Run("returns error for non-existent token", func(t *testing.T) {
		_, err := storage.GetAuthSetupToken(ctx, "non-existent-token")
		assert.Error(t, err)
	})

	t.Run("retrieves expired token", func(t *testing.T) {
		expiredToken := &AuthSetupToken{
			Token:     "expired-token",
			UserID:    "user-id-456",
			Email:     "user2@example.com",
			CreatedAt: time.Now().Add(-25 * time.Hour),
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
			Used:      false,
			ReturnURL: "https://platform.example.com/dashboard",
		}

		err = storage.SaveAuthSetupToken(ctx, expiredToken)
		require.NoError(t, err)

		// Should be able to retrieve, but application logic should check expiry
		retrieved, err := storage.GetAuthSetupToken(ctx, expiredToken.Token)
		require.NoError(t, err)
		assert.Equal(t, expiredToken.Token, retrieved.Token)
		// Verify it's expired
		assert.True(t, time.Now().After(retrieved.ExpiresAt))
	})

	t.Run("validates token structure", func(t *testing.T) {
		token := &AuthSetupToken{
			Token:     "test-token-structure",
			UserID:    "user-id-789",
			Email:     "user3@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Used:      false,
			ReturnURL: "https://platform.example.com/dashboard",
		}

		err = storage.SaveAuthSetupToken(ctx, token)
		require.NoError(t, err)

		retrieved, err := storage.GetAuthSetupToken(ctx, token.Token)
		require.NoError(t, err)

		// Verify all fields
		assert.NotEmpty(t, retrieved.Token)
		assert.NotEmpty(t, retrieved.UserID)
		assert.NotEmpty(t, retrieved.Email)
		assert.NotEmpty(t, retrieved.ReturnURL)
		assert.False(t, retrieved.CreatedAt.IsZero())
		assert.False(t, retrieved.ExpiresAt.IsZero())
		assert.Nil(t, retrieved.UsedAt) // Not used yet
	})
}

// TestDeleteAuthSetupToken tests deleting auth setup tokens.
func TestDeleteAuthSetupToken(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("removes token file", func(t *testing.T) {
		token := &AuthSetupToken{
			Token:     "test-token-delete",
			UserID:    "user-id-123",
			Email:     "user@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Used:      false,
			ReturnURL: "https://platform.example.com/dashboard",
		}

		err = storage.SaveAuthSetupToken(ctx, token)
		require.NoError(t, err)

		// Verify file exists
		tokenPath := filepath.Join(dataDir, "auth-setup-tokens", token.Token+".json")
		AssertFileExists(t, tokenPath)

		// Delete token
		err = storage.DeleteAuthSetupToken(ctx, token.Token)
		require.NoError(t, err)

		// Verify file was removed
		AssertFileNotExists(t, tokenPath)
	})

	t.Run("delete is idempotent", func(t *testing.T) {
		token := &AuthSetupToken{
			Token:     "test-token-idempotent",
			UserID:    "user-id-456",
			Email:     "user2@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Used:      false,
			ReturnURL: "https://platform.example.com/dashboard",
		}

		err = storage.SaveAuthSetupToken(ctx, token)
		require.NoError(t, err)

		// Delete once
		err = storage.DeleteAuthSetupToken(ctx, token.Token)
		require.NoError(t, err)

		// Delete again - should not error
		err = storage.DeleteAuthSetupToken(ctx, token.Token)
		assert.NoError(t, err)
	})

	t.Run("subsequent Get returns error", func(t *testing.T) {
		token := &AuthSetupToken{
			Token:     "test-token-get-after-delete",
			UserID:    "user-id-789",
			Email:     "user3@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(24 * time.Hour),
			Used:      false,
			ReturnURL: "https://platform.example.com/dashboard",
		}

		err = storage.SaveAuthSetupToken(ctx, token)
		require.NoError(t, err)

		// Delete token
		err = storage.DeleteAuthSetupToken(ctx, token.Token)
		require.NoError(t, err)

		// Try to retrieve - should error
		_, err = storage.GetAuthSetupToken(ctx, token.Token)
		assert.Error(t, err)
	})
}

// TestCleanupExpiredAuthSetupTokens tests cleanup of expired auth setup tokens.
func TestCleanupExpiredAuthSetupTokens(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	storage, err := NewFileStorage(dataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create expired token
	expiredToken := &AuthSetupToken{
		Token:     "expired-token",
		UserID:    "user-id-123",
		Email:     "user@example.com",
		CreatedAt: time.Now().Add(-25 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
		Used:      false,
		ReturnURL: "https://platform.example.com/dashboard",
	}
	err = storage.SaveAuthSetupToken(ctx, expiredToken)
	require.NoError(t, err)

	// Create valid token
	validToken := &AuthSetupToken{
		Token:     "valid-token",
		UserID:    "user-id-456",
		Email:     "user2@example.com",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Used:      false,
		ReturnURL: "https://platform.example.com/dashboard",
	}
	err = storage.SaveAuthSetupToken(ctx, validToken)
	require.NoError(t, err)

	t.Run("removes expired tokens", func(t *testing.T) {
		// Run cleanup
		err = storage.CleanupExpiredTokens(ctx)
		require.NoError(t, err)

		// Expired token should be gone
		expiredPath := filepath.Join(dataDir, "auth-setup-tokens", expiredToken.Token+".json")
		AssertFileNotExists(t, expiredPath)
	})

	t.Run("keeps non-expired tokens", func(t *testing.T) {
		// Valid token should still exist
		validPath := filepath.Join(dataDir, "auth-setup-tokens", validToken.Token+".json")
		AssertFileExists(t, validPath)

		// Should be able to retrieve it
		retrieved, err := storage.GetAuthSetupToken(ctx, validToken.Token)
		require.NoError(t, err)
		assert.Equal(t, validToken.Token, retrieved.Token)
	})

	t.Run("works with concurrent tokens", func(t *testing.T) {
		// Create multiple tokens with different expiry times
		for i := 0; i < 10; i++ {
			var expiresAt time.Time
			if i%2 == 0 {
				// Even: expired
				expiresAt = time.Now().Add(-1 * time.Hour)
			} else {
				// Odd: valid
				expiresAt = time.Now().Add(24 * time.Hour)
			}

			token := &AuthSetupToken{
				Token:     GenerateTestID(),
				UserID:    "user-id-" + GenerateTestID(),
				Email:     "user" + GenerateTestID() + "@example.com",
				CreatedAt: time.Now(),
				ExpiresAt: expiresAt,
				Used:      false,
				ReturnURL: "https://platform.example.com/dashboard",
			}
			err = storage.SaveAuthSetupToken(ctx, token)
			require.NoError(t, err)
		}

		// Cleanup
		err = storage.CleanupExpiredTokens(ctx)
		require.NoError(t, err)

		// Count remaining tokens (should be ~5 valid ones plus the original valid token)
		tokensDir := filepath.Join(dataDir, "auth-setup-tokens")
		files, err := filepath.Glob(filepath.Join(tokensDir, "*.json"))
		require.NoError(t, err)
		// We expect at least 5-6 files (5 odd-numbered + 1 original valid token)
		assert.GreaterOrEqual(t, len(files), 5)
		assert.LessOrEqual(t, len(files), 6)
	})
}
