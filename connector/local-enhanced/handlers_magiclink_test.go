package local

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleMagicLinkSend tests the handleMagicLinkSend handler.
func TestHandleMagicLinkSend(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		enableMagic    bool
		requestBody    map[string]interface{}
		setupMock      func(*Connector)
		expectedStatus int
		expectEmail    bool
		checkResponse  func(t *testing.T, resp map[string]interface{})
	}{
		{
			name:        "valid request sends email",
			method:      http.MethodPost,
			enableMagic: true,
			requestBody: map[string]interface{}{
				"email":    "test@example.com",
				"callback": "https://dex.example.com/callback",
				"state":    "test-state-123",
			},
			setupMock: func(c *Connector) {
				// Create user first
				user := NewTestUser("test@example.com").ToUser()
				ctx := TestContext(t)
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Set up mock email sender
				mockSender := NewMockEmailSender()
				c.SetEmailSender(mockSender)
			},
			expectedStatus: http.StatusOK,
			expectEmail:    true,
			checkResponse: func(t *testing.T, resp map[string]interface{}) {
				assert.Equal(t, true, resp["success"])
				assert.Contains(t, resp["message"], "Magic link sent")
			},
		},
		{
			name:   "method not allowed",
			method: http.MethodGet,
			requestBody: map[string]interface{}{
				"email": "test@example.com",
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "magic links disabled",
			method:         http.MethodPost,
			enableMagic:    false,
			requestBody:    map[string]interface{}{"email": "test@example.com"},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:   "missing email",
			method: http.MethodPost,
			requestBody: map[string]interface{}{
				"callback": "https://dex.example.com/callback",
				"state":    "test-state",
			},
			enableMagic:    true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "missing callback",
			method: http.MethodPost,
			requestBody: map[string]interface{}{
				"email": "test@example.com",
				"state": "test-state",
			},
			enableMagic:    true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "missing state",
			method: http.MethodPost,
			requestBody: map[string]interface{}{
				"email":    "test@example.com",
				"callback": "https://dex.example.com/callback",
			},
			enableMagic:    true,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "invalid email format",
			method:      http.MethodPost,
			enableMagic: true,
			requestBody: map[string]interface{}{
				"email":    "not-an-email",
				"callback": "https://dex.example.com/callback",
				"state":    "test-state",
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:        "user not found",
			method:      http.MethodPost,
			enableMagic: true,
			requestBody: map[string]interface{}{
				"email":    "nonexistent@example.com",
				"callback": "https://dex.example.com/callback",
				"state":    "test-state",
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "rate limit exceeded",
			method:      http.MethodPost,
			enableMagic: true,
			requestBody: map[string]interface{}{
				"email":    "ratelimit@example.com",
				"callback": "https://dex.example.com/callback",
				"state":    "test-state",
			},
			setupMock: func(c *Connector) {
				// Create user
				user := NewTestUser("ratelimit@example.com").ToUser()
				ctx := TestContext(t)
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Exhaust rate limit (3 per hour)
				for i := 0; i < 3; i++ {
					c.magicLinkRateLimiter.Allow("ratelimit@example.com")
				}
			},
			expectedStatus: http.StatusTooManyRequests,
		},
		{
			name:        "email sending failure",
			method:      http.MethodPost,
			enableMagic: true,
			requestBody: map[string]interface{}{
				"email":    "emailfail@example.com",
				"callback": "https://dex.example.com/callback",
				"state":    "test-state",
			},
			setupMock: func(c *Connector) {
				// Create user
				user := NewTestUser("emailfail@example.com").ToUser()
				ctx := TestContext(t)
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Set up failing email sender
				mockSender := &FailingEmailSender{}
				c.SetEmailSender(mockSender)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "invalid JSON body",
			method:      http.MethodPost,
			enableMagic: true,
			requestBody: nil, // Will send invalid JSON
			setupMock: func(c *Connector) {
				// We'll override the body in the test
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			config := DefaultTestConfig(t)
			config.MagicLink.Enabled = tt.enableMagic
			defer CleanupTestStorage(t, config.DataDir)

			connector := NewTestConnector(t, config)

			if tt.setupMock != nil {
				tt.setupMock(connector)
			}

			// Create request
			var body []byte
			var err error
			if tt.name == "invalid JSON body" {
				body = []byte("{invalid json")
			} else if tt.requestBody != nil {
				body, err = json.Marshal(tt.requestBody)
				require.NoError(t, err)
			}

			req := httptest.NewRequest(tt.method, "/magic-link/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			// Execute
			connector.handleMagicLinkSend(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK {
				var resp map[string]interface{}
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)

				if tt.checkResponse != nil {
					tt.checkResponse(t, resp)
				}

				// Check that email was sent if expected
				if tt.expectEmail && connector.emailSender != nil {
					if mockSender, ok := connector.emailSender.(*MockEmailSender); ok {
						lastEmail := mockSender.GetLastEmail()
						assert.NotNil(t, lastEmail)
						if tt.requestBody != nil {
							assert.Equal(t, tt.requestBody["email"], lastEmail.To)
						}
						assert.Contains(t, lastEmail.Body, "login link")
					}
				}
			}
		})
	}
}

// TestHandleMagicLinkVerify tests the handleMagicLinkVerify handler.
func TestHandleMagicLinkVerify(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		enableMagic    bool
		setupFunc      func(*Connector) string // Returns token
		token          string
		expectedStatus int
		expectRedirect bool
		checkRedirect  func(t *testing.T, location string)
	}{
		{
			name:        "valid token redirects to callback",
			method:      http.MethodGet,
			enableMagic: true,
			setupFunc: func(c *Connector) string {
				ctx := TestContext(t)

				// Create user
				user := NewTestUser("verify@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Create magic link token
				token, err := c.CreateMagicLink(ctx, "verify@example.com", "https://dex.example.com/callback", "test-state", "127.0.0.1")
				require.NoError(t, err)

				return token.Token
			},
			expectedStatus: http.StatusSeeOther,
			expectRedirect: true,
			checkRedirect: func(t *testing.T, location string) {
				assert.Contains(t, location, "https://dex.example.com/callback")
				assert.Contains(t, location, "state=test-state")
				assert.Contains(t, location, "user_id=")
			},
		},
		{
			name:           "method not allowed",
			method:         http.MethodPost,
			enableMagic:    true,
			token:          "dummy-token",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "magic links disabled",
			method:         http.MethodGet,
			enableMagic:    false,
			token:          "dummy-token",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "missing token parameter",
			method:         http.MethodGet,
			enableMagic:    true,
			token:          "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid token",
			method:         http.MethodGet,
			enableMagic:    true,
			token:          "invalid-token-123",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:        "expired token",
			method:      http.MethodGet,
			enableMagic: true,
			setupFunc: func(c *Connector) string {
				ctx := TestContext(t)

				// Create user
				user := NewTestUser("expired@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Create expired token
				now := time.Now()
				token := &MagicLinkToken{
					Token:       "expired-token-123",
					UserID:      user.ID,
					Email:       "expired@example.com",
					CreatedAt:   now.Add(-20 * time.Minute),
					ExpiresAt:   now.Add(-10 * time.Minute), // Expired
					Used:        false,
					CallbackURL: "https://dex.example.com/callback",
					State:       "test-state",
				}
				err = c.storage.SaveMagicLinkToken(ctx, token)
				require.NoError(t, err)

				return token.Token
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:        "already used token",
			method:      http.MethodGet,
			enableMagic: true,
			setupFunc: func(c *Connector) string {
				ctx := TestContext(t)

				// Create user
				user := NewTestUser("used@example.com").ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Create used token
				now := time.Now()
				token := &MagicLinkToken{
					Token:       "used-token-123",
					UserID:      user.ID,
					Email:       "used@example.com",
					CreatedAt:   now.Add(-5 * time.Minute),
					ExpiresAt:   now.Add(5 * time.Minute),
					Used:        true, // Already used
					UsedAt:      &now,
					CallbackURL: "https://dex.example.com/callback",
					State:       "test-state",
				}
				err = c.storage.SaveMagicLinkToken(ctx, token)
				require.NoError(t, err)

				return token.Token
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:        "2FA required redirects to 2FA prompt",
			method:      http.MethodGet,
			enableMagic: true,
			setupFunc: func(c *Connector) string {
				ctx := TestContext(t)

				// Create user with 2FA required
				testUser := NewTestUser("2fa@example.com")
				testUser.Require2FA = true
				user := testUser.ToUser()
				err := c.storage.CreateUser(ctx, user)
				require.NoError(t, err)

				// Create magic link token
				token, err := c.CreateMagicLink(ctx, "2fa@example.com", "https://dex.example.com/callback", "test-state", "127.0.0.1")
				require.NoError(t, err)

				return token.Token
			},
			expectedStatus: http.StatusSeeOther,
			expectRedirect: true,
			checkRedirect: func(t *testing.T, location string) {
				assert.Contains(t, location, "/2fa/prompt")
				assert.Contains(t, location, "session_id=")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			config := DefaultTestConfig(t)
			config.MagicLink.Enabled = tt.enableMagic
			defer CleanupTestStorage(t, config.DataDir)

			connector := NewTestConnector(t, config)

			token := tt.token
			if tt.setupFunc != nil {
				token = tt.setupFunc(connector)
			}

			// Create request
			url := "/magic-link/verify"
			if token != "" {
				url = fmt.Sprintf("%s?token=%s", url, token)
			}
			req := httptest.NewRequest(tt.method, url, nil)
			w := httptest.NewRecorder()

			// Execute
			connector.handleMagicLinkVerify(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectRedirect {
				location := w.Header().Get("Location")
				assert.NotEmpty(t, location)
				if tt.checkRedirect != nil {
					tt.checkRedirect(t, location)
				}
			}
		})
	}
}

// TestHandleMagicLinkSend_Concurrent tests concurrent requests to handleMagicLinkSend.
func TestHandleMagicLinkSend_Concurrent(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.MagicLink.Enabled = true
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)

	// Create user
	user := NewTestUser("concurrent@example.com").ToUser()
	ctx := TestContext(t)
	err := connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Set up mock email sender
	mockSender := NewMockEmailSender()
	connector.SetEmailSender(mockSender)

	// Send 3 concurrent requests (within rate limit)
	numRequests := 3
	results := make(chan int, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			requestBody := map[string]interface{}{
				"email":    "concurrent@example.com",
				"callback": "https://dex.example.com/callback",
				"state":    "test-state",
			}
			body, _ := json.Marshal(requestBody)
			req := httptest.NewRequest(http.MethodPost, "/magic-link/send", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			connector.handleMagicLinkSend(w, req)
			results <- w.Code
		}()
	}

	// Collect results
	successCount := 0
	for i := 0; i < numRequests; i++ {
		statusCode := <-results
		if statusCode == http.StatusOK {
			successCount++
		}
	}

	// All 3 should succeed (within rate limit)
	assert.Equal(t, 3, successCount)
}

// TestHandleMagicLinkVerify_RateLimiterReset tests that the rate limiter is reset after successful auth.
func TestHandleMagicLinkVerify_RateLimiterReset(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.MagicLink.Enabled = true
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)

	ctx := TestContext(t)

	// Create user
	user := NewTestUser("reset@example.com").ToUser()
	err := connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Exhaust rate limit
	for i := 0; i < 3; i++ {
		connector.magicLinkRateLimiter.Allow("reset@example.com")
	}

	// Verify rate limit is exceeded
	assert.False(t, connector.magicLinkRateLimiter.Allow("reset@example.com"))

	// Create and verify magic link token
	token, err := connector.CreateMagicLink(ctx, "reset@example.com", "https://dex.example.com/callback", "test-state", "127.0.0.1")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/magic-link/verify?token=%s", token.Token), nil)
	w := httptest.NewRecorder()
	connector.handleMagicLinkVerify(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)

	// Verify rate limiter was reset
	assert.True(t, connector.magicLinkRateLimiter.Allow("reset@example.com"))
}

// FailingEmailSender is a mock email sender that always fails.
type FailingEmailSender struct{}

func (f *FailingEmailSender) SendEmail(to, subject, body string) error {
	return fmt.Errorf("email sending failed")
}
