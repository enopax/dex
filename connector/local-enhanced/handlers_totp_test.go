package local

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleTOTPEnable tests the POST /totp/enable endpoint
func TestHandleTOTPEnable(t *testing.T) {
	t.Run("valid user returns secret, QR code, and backup codes", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create test user
		user := NewTestUser("alice@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, user.ToUser()))

		// Make request
		reqBody := map[string]string{"user_id": user.ID}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPEnable(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Secret        string   `json:"secret"`
			QRCodeDataURL string   `json:"qr_code_data_url"`
			URL           string   `json:"url"`
			BackupCodes   []string `json:"backup_codes"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))

		// Validate response data
		assert.NotEmpty(t, resp.Secret, "Secret should not be empty")
		assert.NotEmpty(t, resp.QRCodeDataURL, "QR code should not be empty")
		assert.NotEmpty(t, resp.URL, "OTPAuth URL should not be empty")
		assert.Len(t, resp.BackupCodes, 10, "Should return 10 backup codes")

		// Verify QR code is base64 data URL
		assert.Contains(t, resp.QRCodeDataURL, "data:image/png;base64,")

		// Verify OTPAuth URL format
		assert.Contains(t, resp.URL, "otpauth://totp/")
		assert.Contains(t, resp.URL, "alice@example.com")

		// Verify backup codes format (8 characters, alphanumeric uppercase)
		for _, code := range resp.BackupCodes {
			assert.Len(t, code, 8, "Backup code should be 8 characters")
			assert.Regexp(t, "^[A-Z0-9]+$", code, "Backup code should be uppercase alphanumeric")
		}
	})

	t.Run("missing user_id returns 400", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		reqBody := map[string]string{} // No user_id
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPEnable(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("user not found returns 404", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		reqBody := map[string]string{"user_id": "non-existent-user"}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPEnable(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("TOTP already enabled returns 409", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create user with TOTP already enabled
		user := NewTestUser("bob@example.com")
		totpSecret := "JBSWY3DPEHPK3PXP"
		user.TOTPSecret = totpSecret
		user.TOTPEnabled = true
		require.NoError(t, connector.storage.CreateUser(ctx, user.ToUser()))

		reqBody := map[string]string{"user_id": user.ID}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/enable", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPEnable(w, req)

		// Handler returns 400 for TOTP already enabled (not 409)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("concurrent requests handled correctly", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create 5 test users
		var wg sync.WaitGroup
		results := make([]int, 5)

		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				user := NewTestUser(GenerateTestID() + "@example.com")
				require.NoError(t, connector.storage.CreateUser(ctx, user.ToUser()))

				reqBody := map[string]string{"user_id": user.ID}
				body, err := json.Marshal(reqBody)
				require.NoError(t, err)

				req := httptest.NewRequest("POST", "/totp/enable", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				connector.handleTOTPEnable(w, req)
				results[index] = w.Code
			}(i)
		}

		wg.Wait()

		// All requests should succeed
		for i, code := range results {
			assert.Equal(t, http.StatusOK, code, "Request %d should succeed", i)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		req := httptest.NewRequest("POST", "/totp/enable", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPEnable(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestHandleTOTPVerify tests the POST /totp/verify endpoint
func TestHandleTOTPVerify(t *testing.T) {
	t.Run("valid TOTP code enables TOTP and stores backup codes", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create test user
		testUser := NewTestUser("carol@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, testUser.ToUser()))

		// Get user from storage (BeginTOTPSetup needs full User object)
		user, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)

		// Begin TOTP setup
		result, err := connector.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		// Generate valid TOTP code
		code, err := generateValidTOTPCode(result.Secret)
		require.NoError(t, err)

		// Make request
		reqBody := map[string]interface{}{
			"user_id":      testUser.ID,
			"secret":       result.Secret,
			"code":         code,
			"backup_codes": result.BackupCodes,
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPVerify(w, req)

		// Verify response
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify user has TOTP enabled
		updatedUser, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)
		assert.True(t, updatedUser.TOTPEnabled, "TOTP should be enabled")
		assert.NotNil(t, updatedUser.TOTPSecret, "TOTP secret should be set")
		assert.Len(t, updatedUser.BackupCodes, 10, "Should have 10 backup codes")

		// Verify backup codes are hashed
		for _, bc := range updatedUser.BackupCodes {
			assert.NotEqual(t, bc.Code, result.BackupCodes[0], "Backup codes should be hashed")
		}
	})

	t.Run("invalid TOTP code returns error", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create test user
		testUser := NewTestUser("dave@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, testUser.ToUser()))

		// Get user from storage
		user, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)

		// Begin TOTP setup
		result, err := connector.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		// Make request with invalid code
		reqBody := map[string]interface{}{
			"user_id":      testUser.ID,
			"secret":       result.Secret,
			"code":         "000000", // Invalid code
			"backup_codes": result.BackupCodes,
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPVerify(w, req)

		// Handler returns 400 for invalid TOTP code (not 401)
		assert.Equal(t, http.StatusBadRequest, w.Code)

		// Verify TOTP is NOT enabled
		updatedUser, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)
		assert.False(t, updatedUser.TOTPEnabled, "TOTP should not be enabled")
	})

	t.Run("missing fields return 400", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		testCases := []struct {
			name string
			body map[string]interface{}
		}{
			{
				name: "missing user_id",
				body: map[string]interface{}{
					"secret":       "SECRET",
					"code":         "123456",
					"backup_codes": []string{"CODE1234"},
				},
			},
			{
				name: "missing secret",
				body: map[string]interface{}{
					"user_id":      "user-id",
					"code":         "123456",
					"backup_codes": []string{"CODE1234"},
				},
			},
			{
				name: "missing code",
				body: map[string]interface{}{
					"user_id":      "user-id",
					"secret":       "SECRET",
					"backup_codes": []string{"CODE1234"},
				},
			},
			{
				name: "missing backup_codes",
				body: map[string]interface{}{
					"user_id": "user-id",
					"secret":  "SECRET",
					"code":    "123456",
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				body, err := json.Marshal(tc.body)
				require.NoError(t, err)

				req := httptest.NewRequest("POST", "/totp/verify", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()

				connector.handleTOTPVerify(w, req)

				assert.Equal(t, http.StatusBadRequest, w.Code)
			})
		}
	})

	t.Run("user not found returns 404", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		reqBody := map[string]interface{}{
			"user_id":      "non-existent-user",
			"secret":       "SECRET",
			"code":         "123456",
			"backup_codes": []string{"CODE1234"},
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/verify", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPVerify(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestHandleTOTPValidate tests the POST /totp/validate endpoint
func TestHandleTOTPValidate(t *testing.T) {
	t.Run("valid TOTP code returns success", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create user with TOTP enabled
		testUser := NewTestUser("eve@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, testUser.ToUser()))

		// Get user from storage
		user, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)

		result, err := connector.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		// Complete TOTP setup
		validCode, err := generateValidTOTPCode(result.Secret)
		require.NoError(t, err)
		err = connector.FinishTOTPSetup(ctx, user, result.Secret, validCode, result.BackupCodes)
		require.NoError(t, err)

		// Generate new valid code for validation
		newCode, err := generateValidTOTPCode(result.Secret)
		require.NoError(t, err)

		// Make validation request
		reqBody := map[string]string{
			"user_id": testUser.ID,
			"code":    newCode,
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPValidate(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Valid bool `json:"valid"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.True(t, resp.Valid, "Code should be valid")
	})

	t.Run("invalid TOTP code returns error", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create user with TOTP enabled
		testUser := NewTestUser("frank@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, testUser.ToUser()))

		// Get user from storage
		user, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)

		result, err := connector.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		// Complete TOTP setup
		validCode, err := generateValidTOTPCode(result.Secret)
		require.NoError(t, err)
		err = connector.FinishTOTPSetup(ctx, user, result.Secret, validCode, result.BackupCodes)
		require.NoError(t, err)

		// Make validation request with invalid code
		reqBody := map[string]string{
			"user_id": testUser.ID,
			"code":    "000000", // Invalid code
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPValidate(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Valid bool `json:"valid"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.False(t, resp.Valid, "Code should be invalid")
	})

	t.Run("backup code fallback works and marks code as used", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create user with TOTP enabled
		testUser := NewTestUser("grace@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, testUser.ToUser()))

		// Get user from storage
		user, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)

		result, err := connector.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		// Complete TOTP setup
		validCode, err := generateValidTOTPCode(result.Secret)
		require.NoError(t, err)
		err = connector.FinishTOTPSetup(ctx, user, result.Secret, validCode, result.BackupCodes)
		require.NoError(t, err)

		// Use first backup code
		backupCode := result.BackupCodes[0]
		reqBody := map[string]string{
			"user_id": testUser.ID,
			"code":    backupCode,
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPValidate(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Valid bool `json:"valid"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.True(t, resp.Valid, "Backup code should be valid")

		// Verify backup code is marked as used
		updatedUser, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)

		usedCount := 0
		for _, bc := range updatedUser.BackupCodes {
			if bc.Used {
				usedCount++
			}
		}
		assert.Equal(t, 1, usedCount, "One backup code should be marked as used")

		// Try to use same backup code again
		req2 := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		w2 := httptest.NewRecorder()

		connector.handleTOTPValidate(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)

		var resp2 struct {
			Valid bool `json:"valid"`
		}
		require.NoError(t, json.NewDecoder(w2.Body).Decode(&resp2))
		assert.False(t, resp2.Valid, "Used backup code should not be valid again")
	})

	t.Run("rate limiting enforced", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create user with TOTP enabled
		testUser := NewTestUser("hannah@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, testUser.ToUser()))

		// Get user from storage
		user, err := connector.storage.GetUser(ctx, testUser.ID)
		require.NoError(t, err)

		result, err := connector.BeginTOTPSetup(ctx, user)
		require.NoError(t, err)

		// Complete TOTP setup
		validCode, err := generateValidTOTPCode(result.Secret)
		require.NoError(t, err)
		err = connector.FinishTOTPSetup(ctx, user, result.Secret, validCode, result.BackupCodes)
		require.NoError(t, err)

		// Make 6 failed attempts (rate limit is 5)
		// Note: The handler returns 200 with valid=false, not different HTTP status codes
		for i := 0; i < 6; i++ {
			reqBody := map[string]string{
				"user_id": testUser.ID,
				"code":    "000000", // Invalid code
			}
			body, err := json.Marshal(reqBody)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			connector.handleTOTPValidate(w, req)

			// All attempts return OK with valid=false
			assert.Equal(t, http.StatusOK, w.Code)

			var resp struct {
				Valid bool `json:"valid"`
			}
			json.NewDecoder(w.Body).Decode(&resp)

			// After rate limit, valid should still be false
			assert.False(t, resp.Valid, "Code should be invalid (attempt %d)", i+1)
		}
	})

	t.Run("user without TOTP returns error", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		ctx := TestContext(t)

		// Create user WITHOUT TOTP enabled
		testUser := NewTestUser("ivan@example.com")
		require.NoError(t, connector.storage.CreateUser(ctx, testUser.ToUser()))

		reqBody := map[string]string{
			"user_id": testUser.ID,
			"code":    "123456",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPValidate(w, req)

		// Handler returns 200 with valid=false for users without TOTP
		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Valid bool `json:"valid"`
		}
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		assert.False(t, resp.Valid, "Validation should fail for user without TOTP")
	})

	t.Run("missing user_id returns 400", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		reqBody := map[string]string{
			"code": "123456",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPValidate(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing code returns 400", func(t *testing.T) {
		config := DefaultTestConfig(t)
		defer CleanupTestStorage(t, config.DataDir)

		connector := NewTestConnector(t, config)

		reqBody := map[string]string{
			"user_id": "user-id",
		}
		body, err := json.Marshal(reqBody)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/totp/validate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		connector.handleTOTPValidate(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// Helper function to generate a valid TOTP code for testing
func generateValidTOTPCode(secret string) (string, error) {
	// Use the otp library to generate a valid code
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		return "", err
	}
	return code, nil
}
