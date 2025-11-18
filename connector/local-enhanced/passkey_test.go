package local

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create a Connector for testing
func setupTestConnector(t *testing.T) (*Connector, *TestConfig) {
	testConfig := DefaultTestConfig(t)

	config := &Config{
		DataDir: testConfig.DataDir,
		Passkey: PasskeyConfig{
			Enabled:          true,
			RPID:             testConfig.RPID,
			RPName:           testConfig.RPDisplayName,
			RPOrigins:        testConfig.RPOrigins,
			UserVerification: testConfig.UserVerification,
		},
	}

	logger := TestLogger(t)
	connector, err := New(config, logger)
	require.NoError(t, err)

	return connector, testConfig
}

// Helper function to create a test user
func createTestUser(email, username, displayName string) *User {
	return &User{
		ID:            generateUserID(email),
		Email:         email,
		Username:      username,
		DisplayName:   displayName,
		EmailVerified: true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

// TestUserWebAuthnInterface tests the WebAuthn user interface implementation.
func TestUserWebAuthnInterface(t *testing.T) {
	tests := []struct {
		name     string
		user     *User
		wantID   string
		wantName string
		wantDisp string
	}{
		{
			name: "user with username and display name",
			user: &User{
				ID:          "user-123",
				Email:       "alice@example.com",
				Username:    "alice",
				DisplayName: "Alice Smith",
			},
			wantID:   "user-123",
			wantName: "alice",
			wantDisp: "Alice Smith",
		},
		{
			name: "user with username only",
			user: &User{
				ID:       "user-456",
				Email:    "bob@example.com",
				Username: "bob",
			},
			wantID:   "user-456",
			wantName: "bob",
			wantDisp: "bob",
		},
		{
			name: "user with email only",
			user: &User{
				ID:    "user-789",
				Email: "charlie@example.com",
			},
			wantID:   "user-789",
			wantName: "charlie@example.com",
			wantDisp: "charlie@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, []byte(tt.wantID), tt.user.WebAuthnID())
			assert.Equal(t, tt.wantName, tt.user.WebAuthnName())
			assert.Equal(t, tt.wantDisp, tt.user.WebAuthnDisplayName())
			assert.Equal(t, "", tt.user.WebAuthnIcon())
		})
	}
}

// TestUserWebAuthnCredentials tests the WebAuthnCredentials method.
func TestUserWebAuthnCredentials(t *testing.T) {
	now := time.Now()
	user := &User{
		ID:    "user-123",
		Email: "alice@example.com",
		Passkeys: []Passkey{
			{
				ID:              "credential-1",
				UserID:          "user-123",
				PublicKey:       []byte("public-key-1"),
				AttestationType: "none",
				AAGUID:          make([]byte, 16),
				SignCount:       5,
				Transports:      []string{"usb", "nfc"},
				Name:            "Security Key",
				CreatedAt:       now,
				BackupEligible:  true,
				BackupState:     false,
			},
			{
				ID:              "credential-2",
				UserID:          "user-123",
				PublicKey:       []byte("public-key-2"),
				AttestationType: "packed",
				AAGUID:          make([]byte, 16),
				SignCount:       10,
				Transports:      []string{"internal"},
				Name:            "Touch ID",
				CreatedAt:       now,
				BackupEligible:  true,
				BackupState:     true,
			},
		},
	}

	credentials := user.WebAuthnCredentials()

	require.Len(t, credentials, 2)

	// Check first credential
	assert.Equal(t, []byte("credential-1"), credentials[0].ID)
	assert.Equal(t, []byte("public-key-1"), credentials[0].PublicKey)
	assert.Equal(t, "none", credentials[0].AttestationType)
	assert.Equal(t, uint32(5), credentials[0].Authenticator.SignCount)
	assert.True(t, credentials[0].Flags.BackupEligible)
	assert.False(t, credentials[0].Flags.BackupState)

	// Check second credential
	assert.Equal(t, []byte("credential-2"), credentials[1].ID)
	assert.Equal(t, []byte("public-key-2"), credentials[1].PublicKey)
	assert.Equal(t, "packed", credentials[1].AttestationType)
	assert.Equal(t, uint32(10), credentials[1].Authenticator.SignCount)
	assert.True(t, credentials[1].Flags.BackupEligible)
	assert.True(t, credentials[1].Flags.BackupState)
}

// TestGenerateChallenge tests the challenge generation function.
func TestGenerateChallenge(t *testing.T) {
	// Generate multiple challenges and verify they're unique
	challenges := make(map[string]bool)

	for i := 0; i < 100; i++ {
		challenge, err := generateChallenge()
		require.NoError(t, err)
		require.Len(t, challenge, 32, "challenge should be 32 bytes")

		// Convert to string for uniqueness check
		challengeStr := string(challenge)
		assert.False(t, challenges[challengeStr], "challenges should be unique")
		challenges[challengeStr] = true
	}
}

// TestPasskeyGenerateSessionID tests the session ID generation function.
func TestPasskeyGenerateSessionID(t *testing.T) {
	// Generate multiple session IDs and verify they're unique
	sessionIDs := make(map[string]bool)

	for i := 0; i < 100; i++ {
		sessionID, err := generateSessionID()
		require.NoError(t, err)
		require.NotEmpty(t, sessionID)

		// Verify it's valid base64
		_, err = base64.URLEncoding.DecodeString(sessionID)
		assert.NoError(t, err, "session ID should be valid base64")

		assert.False(t, sessionIDs[sessionID], "session IDs should be unique")
		sessionIDs[sessionID] = true
	}
}

// TestBeginPasskeyRegistration tests the passkey registration begin flow.
func TestBeginPasskeyRegistration(t *testing.T) {
	// Setup
	connector, testConfig := setupTestConnector(t)
	defer CleanupTestStorage(t, testConfig.DataDir)

	ctx := TestContext(t)

	// Create a test user with magic link enabled (to pass validation)
	user := createTestUser("alice@example.com", "alice", "Alice Smith")
	user.MagicLinkEnabled = true

	// Save user
	err := connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin registration
	session, options, err := connector.BeginPasskeyRegistration(ctx, user)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, options)

	// Check session
	assert.NotEmpty(t, session.SessionID)
	assert.Equal(t, user.ID, session.UserID)
	assert.Len(t, session.Challenge, 32)
	assert.Equal(t, "registration", session.Operation)
	assert.True(t, session.ExpiresAt.After(time.Now()))
	assert.True(t, session.CreatedAt.Before(time.Now().Add(time.Second)))

	// Check options
	assert.NotEmpty(t, options.Response.Challenge)
	assert.Equal(t, testConfig.RPID, options.Response.RelyingParty.ID)
	assert.Equal(t, testConfig.RPDisplayName, options.Response.RelyingParty.Name)
	// User ID is returned as URLEncodedBase64 ([]byte)
	assert.Equal(t, []byte(user.ID), []byte(options.Response.User.ID.(protocol.URLEncodedBase64)))
	assert.Equal(t, "alice", options.Response.User.Name)
	assert.Equal(t, "Alice Smith", options.Response.User.DisplayName)

	// Verify session was saved
	savedSession, err := connector.storage.GetWebAuthnSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, session.SessionID, savedSession.SessionID)
}

// TestBeginPasskeyRegistrationInvalidUser tests registration with invalid user.
func TestBeginPasskeyRegistrationInvalidUser(t *testing.T) {
	connector, testConfig := setupTestConnector(t)
	defer CleanupTestStorage(t, testConfig.DataDir)

	ctx := TestContext(t)

	// Invalid user (no email)
	invalidUser := &User{
		ID: "user-123",
	}

	session, options, err := connector.BeginPasskeyRegistration(ctx, invalidUser)

	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Nil(t, options)
}

// TestBeginPasskeyAuthentication tests the passkey authentication begin flow.
func TestBeginPasskeyAuthentication(t *testing.T) {
	// Setup
	connector, testConfig := setupTestConnector(t)
	defer CleanupTestStorage(t, testConfig.DataDir)

	ctx := TestContext(t)

	// Create a test user with a passkey
	user := createTestUser("alice@example.com", "alice", "Alice Smith")
	passkey := Passkey{
		ID:              "test-credential-id",
		UserID:          user.ID,
		PublicKey:       []byte("test-public-key"),
		AttestationType: "none",
		AAGUID:          make([]byte, 16),
		SignCount:       0,
		Name:            "Security Key",
		CreatedAt:       time.Now(),
	}
	user.Passkeys = []Passkey{passkey}

	err := connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin authentication
	session, options, err := connector.BeginPasskeyAuthentication(ctx, user.Email)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, session)
	require.NotNil(t, options)

	// Check session
	assert.NotEmpty(t, session.SessionID)
	assert.Equal(t, user.ID, session.UserID)
	assert.Len(t, session.Challenge, 32)
	assert.Equal(t, "authentication", session.Operation)
	assert.True(t, session.ExpiresAt.After(time.Now()))

	// Check options
	assert.NotEmpty(t, options.Response.Challenge)
	assert.Equal(t, testConfig.RPID, options.Response.RelyingPartyID)
	assert.NotEmpty(t, options.Response.AllowedCredentials)

	// Verify session was saved
	savedSession, err := connector.storage.GetWebAuthnSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, session.SessionID, savedSession.SessionID)
}

// TestBeginPasskeyAuthenticationDiscoverable tests authentication with discoverable credentials.
func TestBeginPasskeyAuthenticationDiscoverable(t *testing.T) {
	connector, testConfig := setupTestConnector(t)
	defer CleanupTestStorage(t, testConfig.DataDir)

	ctx := TestContext(t)

	// Note: Discoverable credentials (resident keys) would work if there's a user with passkeys.
	// For now, we just test that it attempts to create options (even if it fails without credentials).
	// In a real scenario, the browser would prompt for a security key, and the key would provide
	// the user information.

	// This test validates that we attempt to create authentication options
	// without requiring an email address (allowing discoverable credentials)
	session, options, err := connector.BeginPasskeyAuthentication(ctx, "")

	// Expected to fail with empty user (no credentials)
	// In production, this would work with resident keys on the authenticator
	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Nil(t, options)
}

// TestBeginPasskeyAuthenticationUserNotFound tests authentication with non-existent user.
func TestBeginPasskeyAuthenticationUserNotFound(t *testing.T) {
	connector, testConfig := setupTestConnector(t)
	defer CleanupTestStorage(t, testConfig.DataDir)

	ctx := TestContext(t)

	session, options, err := connector.BeginPasskeyAuthentication(ctx, "nonexistent@example.com")

	assert.Error(t, err)
	assert.Nil(t, session)
	assert.Nil(t, options)
}

// TestConvertTransports tests transport conversion.
func TestConvertTransports(t *testing.T) {
	tests := []struct {
		name       string
		input      []string
		wantLength int
	}{
		{
			name:       "usb and nfc",
			input:      []string{"usb", "nfc"},
			wantLength: 2,
		},
		{
			name:       "internal only",
			input:      []string{"internal"},
			wantLength: 1,
		},
		{
			name:       "empty",
			input:      []string{},
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertTransports(tt.input)
			assert.Len(t, result, tt.wantLength)

			for i, transport := range tt.input {
				assert.Equal(t, protocol.AuthenticatorTransport(transport), result[i])
			}
		})
	}
}

// TestTransportStrings tests transport to string conversion.
func TestTransportStrings(t *testing.T) {
	transports := []protocol.AuthenticatorTransport{
		protocol.USB,
		protocol.NFC,
		protocol.Internal,
	}

	result := transportStrings(transports)

	require.Len(t, result, 3)
	assert.Equal(t, "usb", result[0])
	assert.Equal(t, "nfc", result[1])
	assert.Equal(t, "internal", result[2])
}

// TestGetUserByPasskeyID tests finding users by passkey ID.
func TestGetUserByPasskeyID(t *testing.T) {
	connector, testConfig := setupTestConnector(t)
	defer CleanupTestStorage(t, testConfig.DataDir)

	ctx := TestContext(t)

	// Create test users with passkeys
	user1 := createTestUser("alice@example.com", "alice", "Alice Smith")
	user1.Passkeys = []Passkey{
		{
			ID:              "credential-alice",
			UserID:          user1.ID,
			PublicKey:       []byte("public-key-alice"),
			AttestationType: "none",
			AAGUID:          make([]byte, 16),
			SignCount:       0,
			Name:            "Alice's Key",
			CreatedAt:       time.Now(),
		},
	}

	user2 := createTestUser("bob@example.com", "bob", "Bob Johnson")
	user2.Passkeys = []Passkey{
		{
			ID:              "credential-bob",
			UserID:          user2.ID,
			PublicKey:       []byte("public-key-bob"),
			AttestationType: "none",
			AAGUID:          make([]byte, 16),
			SignCount:       0,
			Name:            "Bob's Key",
			CreatedAt:       time.Now(),
		},
	}

	err := connector.storage.CreateUser(ctx, user1)
	require.NoError(t, err)
	err = connector.storage.CreateUser(ctx, user2)
	require.NoError(t, err)

	// Test finding user by passkey ID
	foundUser, err := connector.getUserByPasskeyID(ctx, "credential-alice")
	require.NoError(t, err)
	assert.Equal(t, user1.ID, foundUser.ID)
	assert.Equal(t, user1.Email, foundUser.Email)

	foundUser, err = connector.getUserByPasskeyID(ctx, "credential-bob")
	require.NoError(t, err)
	assert.Equal(t, user2.ID, foundUser.ID)
	assert.Equal(t, user2.Email, foundUser.Email)

	// Test not found
	_, err = connector.getUserByPasskeyID(ctx, "nonexistent-credential")
	assert.ErrorIs(t, err, ErrPasskeyNotFound)
}
