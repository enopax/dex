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
