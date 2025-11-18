package local

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlePasskeyRegisterBegin(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create a test user
	ctx := TestContext(t)
	testUser := NewTestUser("alice@example.com")
	user := testUser.ToUser()
	err = connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	tests := []struct {
		name           string
		method         string
		requestBody    interface{}
		passkeyEnabled bool
		expectedStatus int
		validateResp   func(t *testing.T, resp *PasskeyRegisterBeginResponse)
	}{
		{
			name:           "successful registration begin",
			method:         http.MethodPost,
			requestBody:    PasskeyRegisterBeginRequest{UserID: user.ID},
			passkeyEnabled: true,
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp *PasskeyRegisterBeginResponse) {
				assert.NotEmpty(t, resp.SessionID, "session ID should not be empty")
				assert.NotNil(t, resp.Options, "options should not be nil")
			},
		},
		{
			name:           "method not allowed",
			method:         http.MethodGet,
			requestBody:    PasskeyRegisterBeginRequest{UserID: user.ID},
			passkeyEnabled: true,
			expectedStatus: http.StatusMethodNotAllowed,
			validateResp:   nil,
		},
		{
			name:           "passkeys disabled",
			method:         http.MethodPost,
			requestBody:    PasskeyRegisterBeginRequest{UserID: user.ID},
			passkeyEnabled: false,
			expectedStatus: http.StatusForbidden,
			validateResp:   nil,
		},
		{
			name:           "invalid request body",
			method:         http.MethodPost,
			requestBody:    "invalid json",
			passkeyEnabled: true,
			expectedStatus: http.StatusBadRequest,
			validateResp:   nil,
		},
		{
			name:           "missing user ID",
			method:         http.MethodPost,
			requestBody:    PasskeyRegisterBeginRequest{UserID: ""},
			passkeyEnabled: true,
			expectedStatus: http.StatusBadRequest,
			validateResp:   nil,
		},
		{
			name:           "user not found",
			method:         http.MethodPost,
			requestBody:    PasskeyRegisterBeginRequest{UserID: "nonexistent-user-id"},
			passkeyEnabled: true,
			expectedStatus: http.StatusNotFound,
			validateResp:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set passkey enabled state
			connector.config.Passkey.Enabled = tt.passkeyEnabled

			// Create request
			var body []byte
			if str, ok := tt.requestBody.(string); ok {
				body = []byte(str)
			} else {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, "/passkey/register/begin", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Create response recorder
			w := httptest.NewRecorder()

			// Call handler
			connector.handlePasskeyRegisterBegin(w, req)

			// Check status code
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Validate response if needed
			if tt.validateResp != nil && w.Code == http.StatusOK {
				var resp PasskeyRegisterBeginResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				tt.validateResp(t, &resp)
			}
		})
	}
}

func TestHandlePasskeyRegisterBeginSessionCreation(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create a test user
	ctx := TestContext(t)
	testUser := NewTestUser("bob@example.com")
	user := testUser.ToUser()
	err = connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Create request
	reqBody := PasskeyRegisterBeginRequest{UserID: user.ID}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/passkey/register/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Create response recorder
	w := httptest.NewRecorder()

	// Call handler
	connector.handlePasskeyRegisterBegin(w, req)

	// Check response
	require.Equal(t, http.StatusOK, w.Code)

	var resp PasskeyRegisterBeginResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	// Verify session was created and stored
	session, err := connector.storage.GetWebAuthnSession(context.Background(), resp.SessionID)
	require.NoError(t, err)
	assert.Equal(t, user.ID, session.UserID)
	assert.Equal(t, "registration", session.Operation)
	assert.NotEmpty(t, session.Challenge)
}

func TestHandlePasskeyRegisterBeginConcurrent(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create test users
	ctx := TestContext(t)
	testUser1 := NewTestUser("user1@example.com")
	testUser2 := NewTestUser("user2@example.com")
	user1 := testUser1.ToUser()
	user2 := testUser2.ToUser()
	err = connector.storage.CreateUser(ctx, user1)
	require.NoError(t, err)
	err = connector.storage.CreateUser(ctx, user2)
	require.NoError(t, err)

	// Test concurrent registration begin
	done := make(chan bool, 2)

	// User 1 registration
	go func() {
		reqBody := PasskeyRegisterBeginRequest{UserID: user1.ID}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/passkey/register/begin", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		connector.handlePasskeyRegisterBegin(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		done <- true
	}()

	// User 2 registration
	go func() {
		reqBody := PasskeyRegisterBeginRequest{UserID: user2.ID}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/passkey/register/begin", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		connector.handlePasskeyRegisterBegin(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		done <- true
	}()

	// Wait for both to complete
	<-done
	<-done
}
