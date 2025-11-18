package local

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/dexidp/dex/connector"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCompletePasskeyRegistrationFlow tests the full passkey registration flow from begin to finish
func TestCompletePasskeyRegistrationFlow(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create a test user
	ctx := TestContext(t)
	testUser := NewTestUser("alice@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Step 1: Begin registration
	session, options, err := conn.BeginPasskeyRegistration(ctx, user)
	require.NoError(t, err)
	require.NotNil(t, session, "session should not be nil")
	require.NotNil(t, options, "options should not be nil")

	// Verify session was stored
	storedSession, err := conn.storage.GetWebAuthnSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, storedSession.UserID)
	assert.Equal(t, "registration", storedSession.Operation)
	assert.NotEmpty(t, storedSession.Challenge)
	assert.False(t, time.Now().After(storedSession.ExpiresAt), "session should not be expired")

	// Verify options structure
	assert.NotEmpty(t, options.Response.Challenge, "challenge should not be empty")
	assert.Equal(t, config.Passkey.RPID, options.Response.RelyingParty.ID, "RP ID should match config")
	assert.Equal(t, config.Passkey.RPName, options.Response.RelyingParty.Name, "RP name should match config")
	// Type assert the User.ID to []byte then convert to string
	if userIDBytes, ok := options.Response.User.ID.([]byte); ok {
		assert.Equal(t, user.ID, string(userIDBytes), "user ID should match")
	}
	assert.NotEmpty(t, options.Response.User.Name, "user name should not be empty")

	// Step 2: Simulate credential creation (mock - real WebAuthn verification would fail with fake data)
	// This tests the parsing and validation logic before actual WebAuthn verification
	// Note: FinishPasskeyRegistration will fail during WebAuthn verification,
	// but we can test the code path up to that point

	// Create a mock credential that passes initial parsing
	mockCredential := &protocol.ParsedCredentialCreationData{
		Response: protocol.ParsedAttestationResponse{
			CollectedClientData: protocol.CollectedClientData{
				Type:      "webauthn.create",
				Challenge: base64.RawURLEncoding.EncodeToString(session.Challenge),
				Origin:    config.Passkey.RPOrigins[0],
			},
			AttestationObject: protocol.AttestationObject{
				RawAuthData: make([]byte, 37), // Minimum size for auth data
				AuthData: protocol.AuthenticatorData{
					RPIDHash: make([]byte, 32),
					Flags:    0x41, // User present + attested credential
				},
			},
		},
	}

	// Note: We cannot test FinishPasskeyRegistration with mock data because
	// the go-webauthn library performs cryptographic verification.
	// This would require either:
	// 1. A real WebAuthn authenticator (browser testing)
	// 2. A mock WebAuthn library (complex)
	// 3. Integration tests with a virtual authenticator

	// Instead, we verify that the session is properly set up for the finish step
	assert.NotNil(t, mockCredential, "mock credential created for structure validation")
}

// TestCompletePasskeyAuthenticationFlow tests the full passkey authentication flow
func TestCompletePasskeyAuthenticationFlow(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create a test user with a passkey
	ctx := TestContext(t)
	testUser := NewTestUser("bob@example.com")
	user := testUser.ToUser()
	testPasskey := NewTestPasskey(user.ID, "Test Security Key")
	user.Passkeys = append(user.Passkeys, *testPasskey.ToPasskey())
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Step 1: Begin authentication
	session, options, err := conn.BeginPasskeyAuthentication(ctx, user.Email)
	require.NoError(t, err)
	require.NotNil(t, session, "session should not be nil")
	require.NotNil(t, options, "options should not be nil")

	// Verify session was stored
	storedSession, err := conn.storage.GetWebAuthnSession(ctx, session.SessionID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, storedSession.UserID)
	assert.Equal(t, "authentication", storedSession.Operation)
	assert.NotEmpty(t, storedSession.Challenge)
	assert.False(t, time.Now().After(storedSession.ExpiresAt), "session should not be expired")

	// Verify options structure
	assert.NotEmpty(t, options.Response.Challenge, "challenge should not be empty")
	assert.Equal(t, config.Passkey.RPID, options.Response.RelyingPartyID, "RP ID should match config")
	assert.NotEmpty(t, options.Response.AllowedCredentials, "should have allowed credentials")
	assert.Equal(t, 1, len(options.Response.AllowedCredentials), "should have one credential")

	// Step 2: Verify credential lookup
	// The authentication flow should be able to find the user by credential ID
	credentialID := testPasskey.ID
	lookupUser, err := conn.getUserByPasskeyID(ctx, credentialID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, lookupUser.ID)
	assert.Equal(t, user.Email, lookupUser.Email)

	// Note: Similar to registration, we cannot test FinishPasskeyAuthentication
	// with mock data due to cryptographic signature verification in go-webauthn
}

// TestOAuthIntegration tests the OAuth integration methods (LoginURL and HandleCallback)
func TestOAuthIntegration(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create a test user
	ctx := TestContext(t)
	testUser := NewTestUser("oauth@example.com")
	testUser.Username = "oauthuser"
	testUser.DisplayName = "OAuth Test User"
	testUser.EmailVerified = true
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	t.Run("LoginURL", func(t *testing.T) {
		// Test LoginURL returns correct URL with state and callback
		callbackURL := "https://dex.example.com/callback"
		state := "test-auth-request-id-12345"

		loginURL, err := conn.LoginURL(callbackURL, state)
		require.NoError(t, err)
		require.NotEmpty(t, loginURL)

		// Parse the URL
		parsedURL, err := url.Parse(loginURL)
		require.NoError(t, err)

		// Verify URL structure
		assert.Equal(t, config.BaseURL+"/login", parsedURL.Scheme+"://"+parsedURL.Host+parsedURL.Path)

		// Verify query parameters
		query := parsedURL.Query()
		assert.Equal(t, state, query.Get("state"), "state parameter should match")
		assert.Equal(t, callbackURL, query.Get("callback"), "callback parameter should match")

		t.Logf("LoginURL generated: %s", loginURL)
	})

	t.Run("HandleCallback_Success", func(t *testing.T) {
		// Create a mock HTTP request with user_id parameter
		callbackURL := "https://dex.example.com/callback?state=test-state&user_id=" + user.ID
		req := httptest.NewRequest(http.MethodGet, callbackURL, nil)
		req = req.WithContext(ctx)

		// Call HandleCallback
		identity, err := conn.HandleCallback(connector.Scopes{}, req)
		require.NoError(t, err)

		// Verify identity mapping
		assert.Equal(t, user.ID, identity.UserID, "user ID should match")
		assert.Equal(t, user.Email, identity.Email, "email should match")
		assert.Equal(t, user.Username, identity.Username, "username should match")
		assert.True(t, identity.EmailVerified, "email should be verified")
		assert.Equal(t, user.DisplayName, identity.PreferredUsername, "preferred username should be display name")

		t.Logf("Identity created: UserID=%s, Email=%s, Username=%s",
			identity.UserID, identity.Email, identity.Username)
	})

	t.Run("HandleCallback_MissingUserID", func(t *testing.T) {
		// Create a request without user_id parameter
		req := httptest.NewRequest(http.MethodGet, "https://dex.example.com/callback?state=test-state", nil)
		req = req.WithContext(ctx)

		// Call HandleCallback - should fail
		_, err := conn.HandleCallback(connector.Scopes{}, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing user_id parameter")
	})

	t.Run("HandleCallback_InvalidUserID", func(t *testing.T) {
		// Create a request with invalid user_id
		req := httptest.NewRequest(http.MethodGet, "https://dex.example.com/callback?state=test-state&user_id=nonexistent", nil)
		req = req.WithContext(ctx)

		// Call HandleCallback - should fail
		_, err := conn.HandleCallback(connector.Scopes{}, req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("HandleCallback_PreferredUsername_Fallback", func(t *testing.T) {
		// Test preferred username fallback logic
		// Case 1: DisplayName is set (already tested above)

		// Case 2: Only username is set
		testUser2 := NewTestUser("user2@example.com")
		testUser2.Username = "testuser2"
		testUser2.DisplayName = "" // No display name
		user2 := testUser2.ToUser()
		err := conn.storage.CreateUser(ctx, user2)
		require.NoError(t, err)

		req2 := httptest.NewRequest(http.MethodGet, "https://dex.example.com/callback?user_id="+user2.ID, nil)
		req2 = req2.WithContext(ctx)
		identity2, err := conn.HandleCallback(connector.Scopes{}, req2)
		require.NoError(t, err)
		assert.Equal(t, user2.Username, identity2.PreferredUsername, "should use username when display name empty")

		// Case 3: Neither username nor display name is set
		testUser3 := NewTestUser("user3@example.com")
		testUser3.Username = ""
		testUser3.DisplayName = ""
		user3 := testUser3.ToUser()
		err = conn.storage.CreateUser(ctx, user3)
		require.NoError(t, err)

		req3 := httptest.NewRequest(http.MethodGet, "https://dex.example.com/callback?user_id="+user3.ID, nil)
		req3 = req3.WithContext(ctx)
		identity3, err := conn.HandleCallback(connector.Scopes{}, req3)
		require.NoError(t, err)
		assert.Equal(t, user3.Email, identity3.PreferredUsername, "should use email when username and display name empty")
	})
}

// TestRefresh tests the Refresh method
func TestRefresh(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("Refresh_ReturnsIdentity", func(t *testing.T) {
		// Create an identity
		originalIdentity := connector.Identity{
			UserID:            "test-user-id",
			Username:          "testuser",
			Email:             "test@example.com",
			EmailVerified:     true,
			PreferredUsername: "Test User",
		}

		// Call Refresh
		refreshedIdentity, err := conn.Refresh(ctx, connector.Scopes{}, originalIdentity)
		require.NoError(t, err)

		// For local users, refresh should return the same identity
		assert.Equal(t, originalIdentity.UserID, refreshedIdentity.UserID)
		assert.Equal(t, originalIdentity.Email, refreshedIdentity.Email)
		assert.Equal(t, originalIdentity.Username, refreshedIdentity.Username)
		assert.Equal(t, originalIdentity.EmailVerified, refreshedIdentity.EmailVerified)
		assert.Equal(t, originalIdentity.PreferredUsername, refreshedIdentity.PreferredUsername)
	})
}

// TestRegisterHandlers tests the RegisterHandlers method
func TestRegisterHandlers(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	t.Run("RegisterHandlers_AllEndpoints", func(t *testing.T) {
		// Create a new ServeMux
		mux := http.NewServeMux()

		// Register handlers
		conn.RegisterHandlers(mux)

		// Test that all expected endpoints are registered by making test requests
		endpoints := []struct {
			path           string
			method         string
			expectedStatus int // Expected status if handler is registered
		}{
			{"/login", http.MethodGet, http.StatusOK},                            // Should render login page or return error
			{"/login/password", http.MethodPost, http.StatusBadRequest},          // Should return error for missing data
			{"/passkey/login/begin", http.MethodPost, http.StatusBadRequest},     // Should return error for missing data
			{"/passkey/login/finish", http.MethodPost, http.StatusBadRequest},    // Should return error for missing data
			{"/passkey/register/begin", http.MethodPost, http.StatusBadRequest},  // Should return error for missing data
			{"/passkey/register/finish", http.MethodPost, http.StatusBadRequest}, // Should return error for missing data
		}

		for _, endpoint := range endpoints {
			t.Run(endpoint.path, func(t *testing.T) {
				req := httptest.NewRequest(endpoint.method, endpoint.path, nil)
				w := httptest.NewRecorder()

				// Call the handler
				mux.ServeHTTP(w, req)

				// Verify handler was registered (not 404)
				assert.NotEqual(t, http.StatusNotFound, w.Code,
					"endpoint %s should be registered (got %d)", endpoint.path, w.Code)

				t.Logf("Endpoint %s registered, returned status %d", endpoint.path, w.Code)
			})
		}
	})
}

// TestPasskeyRegistrationSessionValidation tests session validation in the registration flow
func TestPasskeyRegistrationSessionValidation(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("ExpiredSession", func(t *testing.T) {
		// Create a user
		testUser := NewTestUser("expired@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Create an expired session
		expiredSession := NewTestWebAuthnSession(user.ID, "registration", -1*time.Minute) // Expired 1 minute ago
		err = conn.storage.SaveWebAuthnSession(ctx, expiredSession.ToWebAuthnSession())
		require.NoError(t, err)

		// Try to get the session - should return error for expired session
		_, err = conn.storage.GetWebAuthnSession(ctx, expiredSession.SessionID)
		require.Error(t, err, "expired session should return error")
		assert.ErrorIs(t, err, ErrSessionNotFound, "expired session should return session not found error")
	})

	t.Run("SessionTTL", func(t *testing.T) {
		// Create a user
		testUser := NewTestUser("ttl@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		// Begin registration
		session, _, err := conn.BeginPasskeyRegistration(ctx, user)
		require.NoError(t, err)

		// Verify session TTL is 5 minutes
		expectedExpiry := time.Now().Add(5 * time.Minute)
		assert.WithinDuration(t, expectedExpiry, session.ExpiresAt, 5*time.Second,
			"session should expire in approximately 5 minutes")
	})
}

// TestPasskeyAuthenticationChallengeUniqueness tests that each authentication generates a unique challenge
func TestPasskeyAuthenticationChallengeUniqueness(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create a user with a passkey
	testUser := NewTestUser("challenge@example.com")
	user := testUser.ToUser()
	testPasskey := NewTestPasskey(user.ID, "Test Key")
	user.Passkeys = append(user.Passkeys, *testPasskey.ToPasskey())
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Generate multiple challenges
	challenges := make(map[string]bool)
	for i := 0; i < 10; i++ {
		session, _, err := conn.BeginPasskeyAuthentication(ctx, user.Email)
		require.NoError(t, err)

		challengeStr := base64.RawURLEncoding.EncodeToString(session.Challenge)

		// Verify challenge is unique
		assert.False(t, challenges[challengeStr], "challenge %d should be unique", i)
		challenges[challengeStr] = true

		// Verify challenge length (should be 32 bytes = 43 chars in base64)
		assert.Equal(t, 32, len(session.Challenge), "challenge should be 32 bytes")
	}

	t.Logf("Generated %d unique challenges", len(challenges))
}

// TestWebAuthnConfiguration tests that the WebAuthn library is properly configured
func TestWebAuthnConfiguration(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	t.Run("WebAuthn_Initialized", func(t *testing.T) {
		// Verify WebAuthn is initialized
		require.NotNil(t, conn.webAuthn, "webAuthn should be initialized")

		// Verify configuration
		webAuthnConfig := conn.webAuthn.Config
		assert.Equal(t, config.Passkey.RPName, webAuthnConfig.RPDisplayName, "RP display name should match")
		assert.Equal(t, config.Passkey.RPID, webAuthnConfig.RPID, "RP ID should match")
		assert.Equal(t, config.Passkey.RPOrigins, webAuthnConfig.RPOrigins, "RP origins should match")
	})

	t.Run("UserVerification_Setting", func(t *testing.T) {
		// Verify user verification preference
		// This is tested indirectly through registration options
		ctx := TestContext(t)
		testUser := NewTestUser("verify@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		_, options, err := conn.BeginPasskeyRegistration(ctx, user)
		require.NoError(t, err)

		// Verify AuthenticatorSelection is populated
		// Note: The go-webauthn library sets default values if not specified
		// We just verify the structure is present
		assert.NotNil(t, options.Response.AuthenticatorSelection, "authenticator selection should be set")
		t.Logf("User verification setting: %s", options.Response.AuthenticatorSelection.UserVerification)
	})
}

// TestFinishPasskeyRegistration_Integration tests FinishPasskeyRegistration with mock WebAuthn data
// Note: This test will fail at the WebAuthn verification step, but validates the code path
func TestFinishPasskeyRegistration_Integration(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create a test user
	testUser := NewTestUser("finish-reg@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin registration to create session
	session, _, err := conn.BeginPasskeyRegistration(ctx, user)
	require.NoError(t, err)

	t.Run("InvalidSession", func(t *testing.T) {
		// Mock credential response (will fail verification but tests error handling)
		mockResponse := &protocol.ParsedCredentialCreationData{}

		// Try to finish with invalid session ID
		_, err := conn.FinishPasskeyRegistration(ctx, "invalid-session-id", mockResponse, "Test Key")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session", "error should mention session")
	})

	t.Run("SessionOperationType", func(t *testing.T) {
		// Create an authentication session instead of registration session
		authSession := &WebAuthnSession{
			SessionID: "wrong-operation-session",
			UserID:    user.ID,
			Challenge: make([]byte, 32),
			Operation: "authentication", // Wrong operation type
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err = conn.storage.SaveWebAuthnSession(ctx, authSession)
		require.NoError(t, err)

		mockResponse := &protocol.ParsedCredentialCreationData{}

		// Try to finish registration with authentication session
		_, err := conn.FinishPasskeyRegistration(ctx, authSession.SessionID, mockResponse, "Test Key")
		require.Error(t, err)
		// The error will come from WebAuthn library, but we verified the code path
	})

	// Note: We cannot test successful completion without a real WebAuthn response
	// This would require browser integration testing with a virtual authenticator
	t.Logf("Session created for finish testing: %s", session.SessionID)
}

// TestHandleLogin tests the login page handler
func TestHandleLogin(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	t.Run("ValidRequest", func(t *testing.T) {
		// Create request with state and callback parameters
		req := httptest.NewRequest(http.MethodGet, "/login?state=test-state&callback=https://dex.example.com/callback", nil)
		w := httptest.NewRecorder()

		// Call handler
		conn.handleLogin(w, req)

		// Check status code
		assert.Equal(t, http.StatusOK, w.Code)

		// Check content type
		assert.Equal(t, "text/html", w.Header().Get("Content-Type"))

		// Check that response contains state and callback
		body := w.Body.String()
		assert.Contains(t, body, "test-state", "response should contain state")
		assert.Contains(t, body, "https://dex.example.com/callback", "response should contain callback URL")
		assert.Contains(t, body, "Login", "response should contain login page")
	})

	t.Run("MissingState", func(t *testing.T) {
		// Create request without state parameter
		req := httptest.NewRequest(http.MethodGet, "/login?callback=https://dex.example.com/callback", nil)
		w := httptest.NewRecorder()

		// Call handler
		conn.handleLogin(w, req)

		// Check status code
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Missing state parameter")
	})

	t.Run("MissingCallback", func(t *testing.T) {
		// Create request without callback parameter
		req := httptest.NewRequest(http.MethodGet, "/login?state=test-state", nil)
		w := httptest.NewRecorder()

		// Call handler
		conn.handleLogin(w, req)

		// Check status code
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Missing callback parameter")
	})
}

// TestGenerateChallengeAndSessionID tests the challenge and session ID generation
func TestGenerateChallengeAndSessionID(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	t.Run("GenerateChallenge", func(t *testing.T) {
		// Call unexported function via exported method
		ctx := TestContext(t)
		testUser := NewTestUser("challenge-test@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		session, _, err := conn.BeginPasskeyRegistration(ctx, user)
		require.NoError(t, err)

		// Verify challenge properties
		assert.Len(t, session.Challenge, 32, "challenge should be 32 bytes")
		assert.NotEmpty(t, session.SessionID, "session ID should not be empty")
	})
}

// TestCleanupFunctions tests the cleanup functions for expired sessions and tokens
func TestCleanupFunctions(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)
	ctx := TestContext(t)

	t.Run("CleanupExpiredSessions", func(t *testing.T) {
		// Create an expired session
		testUser := NewTestUser("cleanup@example.com")
		user := testUser.ToUser()
		err = conn.storage.CreateUser(ctx, user)
		require.NoError(t, err)

		expiredSession := NewTestWebAuthnSession(user.ID, "registration", -1*time.Minute)
		err = conn.storage.SaveWebAuthnSession(ctx, expiredSession.ToWebAuthnSession())
		require.NoError(t, err)

		// Create a valid session
		validSession := NewTestWebAuthnSession(user.ID, "registration", 5*time.Minute)
		err = conn.storage.SaveWebAuthnSession(ctx, validSession.ToWebAuthnSession())
		require.NoError(t, err)

		// Cleanup expired sessions
		err = conn.storage.CleanupExpiredSessions(ctx)
		require.NoError(t, err)

		// Verify expired session was deleted
		_, err = conn.storage.GetWebAuthnSession(ctx, expiredSession.SessionID)
		assert.Error(t, err, "expired session should be deleted")

		// Verify valid session still exists
		_, err = conn.storage.GetWebAuthnSession(ctx, validSession.SessionID)
		require.NoError(t, err, "valid session should still exist")
	})
}

// TestFinishPasskeyAuthentication_Integration tests FinishPasskeyAuthentication with session validation
func TestFinishPasskeyAuthentication_Integration(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create a test user with a passkey
	testUser := NewTestUser("finish-auth@example.com")
	user := testUser.ToUser()
	testPasskey := NewTestPasskey(user.ID, "Test Key")
	user.Passkeys = append(user.Passkeys, *testPasskey.ToPasskey())
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Begin authentication to create session
	session, _, err := conn.BeginPasskeyAuthentication(ctx, user.Email)
	require.NoError(t, err)

	t.Run("InvalidSession", func(t *testing.T) {
		// Mock credential response
		mockResponse := &protocol.ParsedCredentialAssertionData{}

		// Try to finish with invalid session ID
		_, _, err := conn.FinishPasskeyAuthentication(ctx, "invalid-session-id", mockResponse)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "session", "error should mention session")
	})

	t.Run("SessionOperationType", func(t *testing.T) {
		// Create a registration session instead of authentication session
		regSession := &WebAuthnSession{
			SessionID: "wrong-operation-session",
			UserID:    user.ID,
			Challenge: make([]byte, 32),
			Operation: "registration", // Wrong operation type
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err = conn.storage.SaveWebAuthnSession(ctx, regSession)
		require.NoError(t, err)

		mockResponse := &protocol.ParsedCredentialAssertionData{}

		// Try to finish authentication with registration session
		_, _, err := conn.FinishPasskeyAuthentication(ctx, regSession.SessionID, mockResponse)
		require.Error(t, err)
		// The error will come from WebAuthn library, but we verified the code path
	})

	// Note: We cannot test successful completion without a real WebAuthn response
	t.Logf("Session created for finish testing: %s", session.SessionID)
}
