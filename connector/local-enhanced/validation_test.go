package local

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserValidate tests User validation
func TestUserValidate(t *testing.T) {
	t.Run("Valid user with password", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		user := &User{
			ID:            "test-user-id",
			Email:         "alice@example.com",
			Username:      "alice",
			DisplayName:   "Alice",
			EmailVerified: true,
			PasswordHash:  &passwordHash,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		err := user.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid user with passkey", func(t *testing.T) {
		user := &User{
			ID:            "test-user-id",
			Email:         "bob@example.com",
			Username:      "bob",
			DisplayName:   "Bob",
			EmailVerified: true,
			Passkeys: []Passkey{
				{
					ID:        "passkey-1",
					UserID:    "test-user-id",
					PublicKey: []byte("test-public-key"),
					Name:      "My Security Key",
					AAGUID:    make([]byte, 16),
					CreatedAt: time.Now(),
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := user.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid user with magic link", func(t *testing.T) {
		user := &User{
			ID:               "test-user-id",
			Email:            "charlie@example.com",
			Username:         "charlie",
			DisplayName:      "Charlie",
			EmailVerified:    true,
			MagicLinkEnabled: true,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		err := user.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid user with TOTP enabled", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		totpSecret := "JBSWY3DPEHPK3PXP"
		user := &User{
			ID:           "test-user-id",
			Email:        "dave@example.com",
			PasswordHash: &passwordHash,
			TOTPSecret:   &totpSecret,
			TOTPEnabled:  true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		err := user.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid user - empty ID", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		user := &User{
			ID:           "",
			Email:        "alice@example.com",
			PasswordHash: &passwordHash,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		err := user.Validate()
		assert.ErrorIs(t, err, ErrEmptyUserID)
	})

	t.Run("Invalid user - empty email", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		user := &User{
			ID:           "test-user-id",
			Email:        "",
			PasswordHash: &passwordHash,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		err := user.Validate()
		assert.ErrorIs(t, err, ErrEmptyEmail)
	})

	t.Run("Invalid user - invalid email format", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		user := &User{
			ID:           "test-user-id",
			Email:        "not-an-email",
			PasswordHash: &passwordHash,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid user")
	})

	t.Run("Invalid user - no authentication method", func(t *testing.T) {
		user := &User{
			ID:               "test-user-id",
			Email:            "alice@example.com",
			MagicLinkEnabled: false,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one authentication method")
	})

	t.Run("Invalid user - TOTP enabled without secret", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		user := &User{
			ID:           "test-user-id",
			Email:        "alice@example.com",
			PasswordHash: &passwordHash,
			TOTPEnabled:  true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "TOTP secret is empty")
	})

	t.Run("Invalid user - missing created_at", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		user := &User{
			ID:           "test-user-id",
			Email:        "alice@example.com",
			PasswordHash: &passwordHash,
			UpdatedAt:    time.Now(),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "created_at")
	})

	t.Run("Invalid user - missing updated_at", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		user := &User{
			ID:           "test-user-id",
			Email:        "alice@example.com",
			PasswordHash: &passwordHash,
			CreatedAt:    time.Now(),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "updated_at")
	})

	t.Run("Invalid user - updated_at before created_at", func(t *testing.T) {
		passwordHash := "$2a$10$abc123"
		now := time.Now()
		user := &User{
			ID:           "test-user-id",
			Email:        "alice@example.com",
			PasswordHash: &passwordHash,
			CreatedAt:    now,
			UpdatedAt:    now.Add(-1 * time.Hour),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "updated_at cannot be before created_at")
	})

	t.Run("Invalid user - invalid passkey", func(t *testing.T) {
		user := &User{
			ID:            "test-user-id",
			Email:         "alice@example.com",
			EmailVerified: true,
			Passkeys: []Passkey{
				{
					ID:        "", // Invalid: empty ID
					UserID:    "test-user-id",
					PublicKey: []byte("test-key"),
					Name:      "Test Key",
					CreatedAt: time.Now(),
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		err := user.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid passkey")
	})
}

// TestPasskeyValidate tests Passkey validation
func TestPasskeyValidate(t *testing.T) {
	t.Run("Valid passkey", func(t *testing.T) {
		passkey := &Passkey{
			ID:              "passkey-1",
			UserID:          "user-123",
			PublicKey:       []byte("test-public-key"),
			AttestationType: "none",
			AAGUID:          make([]byte, 16),
			SignCount:       0,
			Transports:      []string{"usb", "nfc"},
			Name:            "My YubiKey",
			CreatedAt:       time.Now(),
			BackupEligible:  true,
			BackupState:     false,
		}
		err := passkey.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid passkey - empty ID", func(t *testing.T) {
		passkey := &Passkey{
			ID:        "",
			UserID:    "user-123",
			PublicKey: []byte("test-key"),
			Name:      "Test Key",
			CreatedAt: time.Now(),
		}
		err := passkey.Validate()
		assert.ErrorIs(t, err, ErrEmptyPasskeyID)
	})

	t.Run("Invalid passkey - empty user ID", func(t *testing.T) {
		passkey := &Passkey{
			ID:        "passkey-1",
			UserID:    "",
			PublicKey: []byte("test-key"),
			Name:      "Test Key",
			CreatedAt: time.Now(),
		}
		err := passkey.Validate()
		assert.ErrorIs(t, err, ErrEmptyPasskeyUserID)
	})

	t.Run("Invalid passkey - empty public key", func(t *testing.T) {
		passkey := &Passkey{
			ID:        "passkey-1",
			UserID:    "user-123",
			PublicKey: []byte{},
			Name:      "Test Key",
			CreatedAt: time.Now(),
		}
		err := passkey.Validate()
		assert.ErrorIs(t, err, ErrEmptyPublicKey)
	})

	t.Run("Invalid passkey - empty name", func(t *testing.T) {
		passkey := &Passkey{
			ID:        "passkey-1",
			UserID:    "user-123",
			PublicKey: []byte("test-key"),
			Name:      "",
			CreatedAt: time.Now(),
		}
		err := passkey.Validate()
		assert.ErrorIs(t, err, ErrEmptyPasskeyName)
	})

	t.Run("Invalid passkey - invalid AAGUID length", func(t *testing.T) {
		passkey := &Passkey{
			ID:        "passkey-1",
			UserID:    "user-123",
			PublicKey: []byte("test-key"),
			Name:      "Test Key",
			AAGUID:    make([]byte, 10), // Invalid: not 16 bytes
			CreatedAt: time.Now(),
		}
		err := passkey.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "AAGUID must be 16 bytes")
	})

	t.Run("Invalid passkey - missing created_at", func(t *testing.T) {
		passkey := &Passkey{
			ID:        "passkey-1",
			UserID:    "user-123",
			PublicKey: []byte("test-key"),
			Name:      "Test Key",
		}
		err := passkey.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "created_at")
	})
}

// TestBackupCodeValidate tests BackupCode validation
func TestBackupCodeValidate(t *testing.T) {
	t.Run("Valid unused backup code", func(t *testing.T) {
		code := &BackupCode{
			Code: "ABC123-DEF456",
			Used: false,
		}
		err := code.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid used backup code", func(t *testing.T) {
		usedAt := time.Now()
		code := &BackupCode{
			Code:   "ABC123-DEF456",
			Used:   true,
			UsedAt: &usedAt,
		}
		err := code.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid backup code - empty code", func(t *testing.T) {
		code := &BackupCode{
			Code: "",
			Used: false,
		}
		err := code.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("Invalid backup code - used but no used_at", func(t *testing.T) {
		code := &BackupCode{
			Code: "ABC123-DEF456",
			Used: true,
		}
		err := code.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "used but used_at is nil")
	})

	t.Run("Invalid backup code - not used but has used_at", func(t *testing.T) {
		usedAt := time.Now()
		code := &BackupCode{
			Code:   "ABC123-DEF456",
			Used:   false,
			UsedAt: &usedAt,
		}
		err := code.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not used but used_at is set")
	})
}

// TestWebAuthnSessionValidate tests WebAuthnSession validation
func TestWebAuthnSessionValidate(t *testing.T) {
	t.Run("Valid registration session", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "session-123",
			UserID:    "user-456",
			Challenge: make([]byte, 32),
			Operation: "registration",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err := session.Validate()
		assert.NoError(t, err)
	})

	t.Run("Valid authentication session", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "session-123",
			UserID:    "user-456",
			Challenge: make([]byte, 32),
			Operation: "authentication",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err := session.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid session - empty session ID", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "",
			UserID:    "user-456",
			Challenge: make([]byte, 32),
			Operation: "registration",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err := session.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "session ID cannot be empty")
	})

	t.Run("Invalid session - empty user ID", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "session-123",
			UserID:    "",
			Challenge: make([]byte, 32),
			Operation: "registration",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err := session.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user ID cannot be empty")
	})

	t.Run("Invalid session - empty challenge", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "session-123",
			UserID:    "user-456",
			Challenge: []byte{},
			Operation: "registration",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err := session.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "challenge cannot be empty")
	})

	t.Run("Invalid session - challenge too short", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "session-123",
			UserID:    "user-456",
			Challenge: make([]byte, 10), // Too short
			Operation: "registration",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err := session.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "challenge too short")
	})

	t.Run("Invalid session - invalid operation", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "session-123",
			UserID:    "user-456",
			Challenge: make([]byte, 32),
			Operation: "invalid-operation",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}
		err := session.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid session operation")
	})

	t.Run("Invalid session - expired", func(t *testing.T) {
		session := &WebAuthnSession{
			SessionID: "session-123",
			UserID:    "user-456",
			Challenge: make([]byte, 32),
			Operation: "registration",
			CreatedAt: time.Now().Add(-10 * time.Minute),
			ExpiresAt: time.Now().Add(-5 * time.Minute),
		}
		err := session.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already expired")
	})
}

// TestMagicLinkTokenValidate tests MagicLinkToken validation
func TestMagicLinkTokenValidate(t *testing.T) {
	t.Run("Valid magic link token", func(t *testing.T) {
		token := &MagicLinkToken{
			Token:       "secure-random-token",
			UserID:      "user-123",
			Email:       "alice@example.com",
			CallbackURL: "https://dex.example.com/callback",
			State:       "oauth-state-123",
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			Used:        false,
			IPAddress:   "192.168.1.1",
		}
		err := token.Validate()
		assert.NoError(t, err)
	})

	t.Run("Invalid token - empty token", func(t *testing.T) {
		token := &MagicLinkToken{
			Token:     "",
			UserID:    "user-123",
			Email:     "alice@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		err := token.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is required")
	})

	t.Run("Invalid token - empty user ID", func(t *testing.T) {
		token := &MagicLinkToken{
			Token:     "secure-token",
			UserID:    "",
			Email:     "alice@example.com",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		err := token.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user_id is required")
	})

	t.Run("Invalid token - empty email", func(t *testing.T) {
		token := &MagicLinkToken{
			Token:     "secure-token",
			UserID:    "user-123",
			Email:     "",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		err := token.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email cannot be empty")
	})

	t.Run("Invalid token - invalid email format", func(t *testing.T) {
		token := &MagicLinkToken{
			Token:     "secure-token",
			UserID:    "user-123",
			Email:     "not-an-email",
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}
		err := token.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid email")
	})

	t.Run("Invalid token - expired", func(t *testing.T) {
		token := &MagicLinkToken{
			Token:       "secure-token",
			UserID:      "user-123",
			Email:       "alice@example.com",
			CallbackURL: "https://dex.example.com/callback",
			State:       "oauth-state-123",
			CreatedAt:   time.Now().Add(-15 * time.Minute),
			ExpiresAt:   time.Now().Add(-5 * time.Minute),
		}
		// Note: Validate() doesn't check expiry, only IsExpired() does
		// So this test should pass validation but IsExpired() should return true
		err := token.Validate()
		assert.NoError(t, err)            // Validation passes
		assert.True(t, token.IsExpired()) // But token is expired
	})
}

// TestValidateEmail tests email validation
func TestValidateEmail(t *testing.T) {
	validEmails := []string{
		"alice@example.com",
		"bob.smith@company.co.uk",
		"user+tag@domain.org",
		"test123@test-domain.com",
		"name_123@sub.domain.example.com",
	}

	for _, email := range validEmails {
		t.Run("Valid: "+email, func(t *testing.T) {
			err := ValidateEmail(email)
			assert.NoError(t, err, "Email should be valid: %s", email)
		})
	}

	invalidEmails := map[string]string{
		"":                     "empty email",
		"not-an-email":         "missing @",
		"@example.com":         "missing local part",
		"user@":                "missing domain",
		"user@@example.com":    "double @",
		"user@domain":          "domain without dot",
		"user name@domain.com": "space in local part",
	}

	for email, reason := range invalidEmails {
		t.Run("Invalid: "+reason, func(t *testing.T) {
			err := ValidateEmail(email)
			assert.Error(t, err, "Email should be invalid (%s): %s", reason, email)
		})
	}
}

// TestValidatePassword tests password validation
func TestValidatePassword(t *testing.T) {
	t.Run("Valid passwords", func(t *testing.T) {
		validPasswords := []string{
			"password123",
			"Test1234",
			"MyP@ssw0rd",
			"LongPassword123456",
		}

		for _, password := range validPasswords {
			err := ValidatePassword(password)
			assert.NoError(t, err, "Password should be valid: %s", password)
		}
	})

	t.Run("Invalid - empty password", func(t *testing.T) {
		err := ValidatePassword("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("Invalid - too short", func(t *testing.T) {
		err := ValidatePassword("pass1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 8 characters")
	})

	t.Run("Invalid - too long", func(t *testing.T) {
		longPassword := string(make([]byte, 150)) + "A1"
		err := ValidatePassword(longPassword)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must not exceed 128")
	})

	t.Run("Invalid - no letters", func(t *testing.T) {
		err := ValidatePassword("12345678")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one letter")
	})

	t.Run("Invalid - no numbers", func(t *testing.T) {
		err := ValidatePassword("password")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one number")
	})
}

// TestValidateUsername tests username validation
func TestValidateUsername(t *testing.T) {
	t.Run("Valid usernames", func(t *testing.T) {
		validUsernames := []string{
			"alice",
			"bob_smith",
			"user123",
			"test-user",
			"_underscore",
			"MixedCase123",
		}

		for _, username := range validUsernames {
			err := ValidateUsername(username)
			assert.NoError(t, err, "Username should be valid: %s", username)
		}
	})

	t.Run("Invalid - empty username", func(t *testing.T) {
		err := ValidateUsername("")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
	})

	t.Run("Invalid - too short", func(t *testing.T) {
		err := ValidateUsername("ab")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 3 characters")
	})

	t.Run("Invalid - too long", func(t *testing.T) {
		longUsername := string(make([]byte, 70))
		err := ValidateUsername(longUsername)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must not exceed 64")
	})

	t.Run("Invalid - starts with hyphen", func(t *testing.T) {
		err := ValidateUsername("-user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must start with a letter")
	})

	t.Run("Invalid - starts with number", func(t *testing.T) {
		err := ValidateUsername("123user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must start with a letter")
	})

	t.Run("Invalid - contains special characters", func(t *testing.T) {
		invalidUsernames := []string{
			"user@domain",
			"user name",
			"user.name",
			"user!123",
		}

		for _, username := range invalidUsernames {
			err := ValidateUsername(username)
			assert.Error(t, err, "Username should be invalid: %s", username)
			assert.Contains(t, err.Error(), "letters, numbers, hyphens, and underscores")
		}
	})
}
