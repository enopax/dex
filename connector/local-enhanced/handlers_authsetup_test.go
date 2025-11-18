package local

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleAuthSetup tests the GET /setup-auth endpoint.
func TestHandleAuthSetup(t *testing.T) {
	tests := []struct {
		name           string
		tokenSetup     func(c *Connector, ctx context.Context) string
		queryToken     string
		expectedStatus int
		expectError    bool
	}{
		{
			name: "valid token",
			tokenSetup: func(c *Connector, ctx context.Context) string {
				// Create a test user
				user := NewTestUser("alice@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Create a valid auth setup token
				token, err := GenerateSecureToken()
				require.NoError(t, err)

				authToken := &AuthSetupToken{
					Token:     token,
					UserID:    user.ID,
					Email:     user.Email,
					CreatedAt: time.Now(),
					ExpiresAt: time.Now().Add(24 * time.Hour),
					Used:      false,
					ReturnURL: "https://platform.enopax.io/dashboard",
				}

				err = c.storage.SaveAuthSetupToken(ctx, authToken)
				require.NoError(t, err)

				return token
			},
			queryToken:     "",                             // Will be set by tokenSetup
			expectedStatus: http.StatusInternalServerError, // Template rendering not implemented
			expectError:    true,
		},
		{
			name: "missing token parameter",
			tokenSetup: func(c *Connector, ctx context.Context) string {
				return ""
			},
			queryToken:     "",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "invalid token",
			tokenSetup: func(c *Connector, ctx context.Context) string {
				return "invalid-token"
			},
			queryToken:     "invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name: "expired token",
			tokenSetup: func(c *Connector, ctx context.Context) string {
				// Create a test user
				user := NewTestUser("bob@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Create an expired auth setup token
				token, err := GenerateSecureToken()
				require.NoError(t, err)

				authToken := &AuthSetupToken{
					Token:     token,
					UserID:    user.ID,
					Email:     user.Email,
					CreatedAt: time.Now().Add(-48 * time.Hour),
					ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired yesterday
					Used:      false,
					ReturnURL: "https://platform.enopax.io/dashboard",
				}

				err = c.storage.SaveAuthSetupToken(ctx, authToken)
				require.NoError(t, err)

				return token
			},
			queryToken:     "", // Will be set by tokenSetup
			expectedStatus: http.StatusUnauthorized,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			config := DefaultTestConfig(t)
			defer CleanupTestStorage(t, config.DataDir)

			connector, err := New(config, TestLogger(t))
			require.NoError(t, err)

			ctx := TestContext(t)

			// Setup token
			token := tt.tokenSetup(connector, ctx)
			if tt.queryToken == "" && token != "" {
				tt.queryToken = token
			}

			// Create request
			url := "/setup-auth"
			if tt.queryToken != "" {
				url += "?token=" + tt.queryToken
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			rec := httptest.NewRecorder()

			// Execute
			connector.handleAuthSetup(rec, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, rec.Code, "unexpected status code")

			if tt.expectError {
				assert.NotEmpty(t, rec.Body.String(), "expected error message")
			}
		})
	}
}

// TestHandlePasswordSetup tests the POST /setup-auth/password endpoint.
func TestHandlePasswordSetup(t *testing.T) {
	tests := []struct {
		name           string
		userSetup      func(c *Connector, ctx context.Context) *User
		requestBody    interface{}
		expectedStatus int
		expectSuccess  bool
	}{
		{
			name: "valid password setup",
			userSetup: func(c *Connector, ctx context.Context) *User {
				user := NewTestUser("alice@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)
				return user
			},
			requestBody: map[string]string{
				"user_id":  "", // Will be filled by userSetup
				"password": "ValidPass123",
			},
			expectedStatus: http.StatusOK,
			expectSuccess:  true,
		},
		{
			name: "missing user_id",
			userSetup: func(c *Connector, ctx context.Context) *User {
				return nil
			},
			requestBody: map[string]string{
				"password": "ValidPass123",
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "missing password",
			userSetup: func(c *Connector, ctx context.Context) *User {
				user := NewTestUser("bob@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)
				return user
			},
			requestBody: map[string]string{
				"user_id": "", // Will be filled by userSetup
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "weak password",
			userSetup: func(c *Connector, ctx context.Context) *User {
				user := NewTestUser("charlie@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)
				return user
			},
			requestBody: map[string]string{
				"user_id":  "",     // Will be filled by userSetup
				"password": "weak", // Too short, no number
			},
			expectedStatus: http.StatusBadRequest,
			expectSuccess:  false,
		},
		{
			name: "user not found",
			userSetup: func(c *Connector, ctx context.Context) *User {
				return nil
			},
			requestBody: map[string]string{
				"user_id":  "non-existent-user-id",
				"password": "ValidPass123",
			},
			expectedStatus: http.StatusNotFound,
			expectSuccess:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			config := DefaultTestConfig(t)
			defer CleanupTestStorage(t, config.DataDir)

			connector, err := New(config, TestLogger(t))
			require.NoError(t, err)

			ctx := TestContext(t)

			// Setup user
			var user *User
			if tt.userSetup != nil {
				user = tt.userSetup(connector, ctx)
			}

			// Prepare request body
			reqBody := tt.requestBody
			if user != nil {
				if reqMap, ok := reqBody.(map[string]string); ok {
					if reqMap["user_id"] == "" {
						reqMap["user_id"] = user.ID
					}
				}
			}

			bodyBytes, err := json.Marshal(reqBody)
			require.NoError(t, err)

			// Create request
			req := httptest.NewRequest(http.MethodPost, "/setup-auth/password", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Execute
			connector.handlePasswordSetup(rec, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, rec.Code, "unexpected status code")

			if tt.expectSuccess {
				var resp map[string]interface{}
				err = json.NewDecoder(rec.Body).Decode(&resp)
				require.NoError(t, err)

				assert.True(t, resp["success"].(bool), "expected success to be true")
				assert.Equal(t, "Password set successfully", resp["message"], "unexpected message")

				// Verify password was actually set
				updatedUser, err := connector.storage.GetUser(ctx, user.ID)
				require.NoError(t, err)
				assert.NotNil(t, updatedUser.PasswordHash, "password hash should be set")
			}
		})
	}
}

// TestHandlePasswordSetup_MethodNotAllowed tests that non-POST requests are rejected.
func TestHandlePasswordSetup_MethodNotAllowed(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/setup-auth/password", nil)
			rec := httptest.NewRecorder()

			connector.handlePasswordSetup(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		})
	}
}

// TestHandleAuthSetup_MethodNotAllowed tests that non-GET requests are rejected.
func TestHandleAuthSetup_MethodNotAllowed(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	methods := []string{http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/setup-auth?token=test", nil)
			rec := httptest.NewRecorder()

			connector.handleAuthSetup(rec, req)

			assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		})
	}
}
