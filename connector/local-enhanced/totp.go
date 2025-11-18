package local

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"image/png"
	"sync"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTPRateLimiter implements a simple in-memory rate limiter for TOTP validation.
type TOTPRateLimiter struct {
	mu       sync.RWMutex
	attempts map[string][]time.Time // userID -> list of attempt timestamps

	// MaxAttempts is the maximum number of TOTP validation attempts allowed per window
	MaxAttempts int

	// Window is the time window for rate limiting
	Window time.Duration
}

// NewTOTPRateLimiter creates a new TOTP rate limiter.
func NewTOTPRateLimiter(maxAttempts int, window time.Duration) *TOTPRateLimiter {
	return &TOTPRateLimiter{
		attempts:    make(map[string][]time.Time),
		MaxAttempts: maxAttempts,
		Window:      window,
	}
}

// Allow checks if a TOTP validation attempt is allowed for the given user.
// It returns true if the attempt is allowed, false if rate limit is exceeded.
func (rl *TOTPRateLimiter) Allow(userID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.Window)

	// Get attempts for this user
	attempts, exists := rl.attempts[userID]
	if !exists {
		// First attempt - allow it
		rl.attempts[userID] = []time.Time{now}
		return true
	}

	// Filter out attempts outside the window
	validAttempts := make([]time.Time, 0)
	for _, attempt := range attempts {
		if attempt.After(windowStart) {
			validAttempts = append(validAttempts, attempt)
		}
	}

	// Check if rate limit exceeded
	if len(validAttempts) >= rl.MaxAttempts {
		return false
	}

	// Add this attempt
	validAttempts = append(validAttempts, now)
	rl.attempts[userID] = validAttempts

	return true
}

// Reset removes all rate limit data for the given user.
// This should be called after a successful authentication.
func (rl *TOTPRateLimiter) Reset(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, userID)
}

// Cleanup removes expired attempts from the rate limiter.
// This should be called periodically to prevent memory leaks.
func (rl *TOTPRateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-rl.Window)

	for userID, attempts := range rl.attempts {
		validAttempts := make([]time.Time, 0)
		for _, attempt := range attempts {
			if attempt.After(windowStart) {
				validAttempts = append(validAttempts, attempt)
			}
		}

		if len(validAttempts) == 0 {
			delete(rl.attempts, userID)
		} else {
			rl.attempts[userID] = validAttempts
		}
	}
}

// TOTPSetupResult contains the result of TOTP setup including the secret and QR code.
type TOTPSetupResult struct {
	// Secret is the TOTP secret (base32 encoded)
	Secret string `json:"secret"`

	// QRCodeDataURL is the data URL for the QR code image (data:image/png;base64,...)
	QRCodeDataURL string `json:"qr_code_data_url"`

	// BackupCodes is the list of generated backup codes
	BackupCodes []string `json:"backup_codes"`

	// URL is the otpauth:// URL for manual entry
	URL string `json:"url"`
}

// BeginTOTPSetup generates a new TOTP secret and QR code for the user.
//
// This initiates the TOTP enrollment process. The user should scan the QR code
// with their authenticator app and then call FinishTOTPSetup with a valid TOTP
// code to confirm enrollment.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User who is setting up TOTP
//
// Returns:
//   - setup: TOTP setup information including secret and QR code
//   - error: Any error that occurred
func (c *Connector) BeginTOTPSetup(ctx context.Context, user *User) (*TOTPSetupResult, error) {
	c.logger.Infof("BeginTOTPSetup: user_id=%s", user.ID)

	// Generate TOTP secret
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      c.config.Passkey.RPName, // Reuse RP name as issuer
		AccountName: user.Email,
		Period:      30,    // 30 seconds
		SecretSize:  32,    // 256 bits
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		c.logger.Errorf("BeginTOTPSetup: failed to generate TOTP key: %v", err)
		return nil, fmt.Errorf("failed to generate TOTP key: %w", err)
	}

	// Generate QR code image
	img, err := key.Image(256, 256)
	if err != nil {
		c.logger.Errorf("BeginTOTPSetup: failed to generate QR code: %v", err)
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Convert image to PNG data URL
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		c.logger.Errorf("BeginTOTPSetup: failed to encode QR code: %v", err)
		return nil, fmt.Errorf("failed to encode QR code: %w", err)
	}

	qrCodeDataURL := fmt.Sprintf("data:image/png;base64,%s",
		base64.StdEncoding.EncodeToString(buf.Bytes()))

	// Generate backup codes
	backupCodes, err := c.generateBackupCodes(10)
	if err != nil {
		c.logger.Errorf("BeginTOTPSetup: failed to generate backup codes: %v", err)
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	result := &TOTPSetupResult{
		Secret:        key.Secret(),
		QRCodeDataURL: qrCodeDataURL,
		BackupCodes:   backupCodes,
		URL:           key.URL(),
	}

	c.logger.Infof("BeginTOTPSetup: TOTP setup initiated for user_id=%s", user.ID)
	return result, nil
}

// FinishTOTPSetup completes TOTP enrollment by verifying the user's TOTP code.
//
// This confirms that the user has successfully scanned the QR code and can
// generate valid TOTP codes. The TOTP secret and backup codes are stored
// in the user's account.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User who is completing TOTP setup
//   - secret: TOTP secret from BeginTOTPSetup
//   - code: TOTP code from the user's authenticator app
//   - backupCodes: Backup codes from BeginTOTPSetup (to be hashed and stored)
//
// Returns:
//   - error: Any error that occurred (nil if successful)
func (c *Connector) FinishTOTPSetup(ctx context.Context, user *User, secret, code string, backupCodes []string) error {
	c.logger.Infof("FinishTOTPSetup: user_id=%s", user.ID)

	// Validate TOTP code
	valid := totp.Validate(code, secret)
	if !valid {
		c.logger.Warnf("FinishTOTPSetup: invalid TOTP code for user_id=%s", user.ID)
		return fmt.Errorf("invalid TOTP code")
	}

	// Hash and store backup codes
	hashedBackupCodes := make([]BackupCode, len(backupCodes))
	for i, code := range backupCodes {
		hash, err := hashPassword(code)
		if err != nil {
			c.logger.Errorf("FinishTOTPSetup: failed to hash backup code: %v", err)
			return fmt.Errorf("failed to hash backup code: %w", err)
		}
		hashedBackupCodes[i] = BackupCode{
			Code:  hash,
			Used:  false,
			UsedAt: nil,
		}
	}

	// Update user record
	user.TOTPSecret = &secret
	user.TOTPEnabled = true
	user.BackupCodes = hashedBackupCodes
	user.UpdatedAt = time.Now()

	// Save user
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("FinishTOTPSetup: failed to update user: %v", err)
		return fmt.Errorf("failed to update user: %w", err)
	}

	c.logger.Infof("FinishTOTPSetup: TOTP enabled for user_id=%s", user.ID)
	return nil
}

// ValidateTOTP validates a TOTP code for the given user.
//
// This is used during login to verify the second factor. It checks the TOTP code
// against the user's stored secret with a time window of ±1 period (30 seconds).
//
// Rate limiting is applied to prevent brute force attacks (5 attempts per 5 minutes).
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose TOTP code is being validated
//   - code: TOTP code from the user
//
// Returns:
//   - valid: Whether the TOTP code is valid
//   - error: Any error that occurred
func (c *Connector) ValidateTOTP(ctx context.Context, user *User, code string) (bool, error) {
	c.logger.Infof("ValidateTOTP: user_id=%s", user.ID)

	// Check if TOTP is enabled
	if !user.TOTPEnabled || user.TOTPSecret == nil {
		c.logger.Warnf("ValidateTOTP: TOTP not enabled for user_id=%s", user.ID)
		return false, fmt.Errorf("TOTP not enabled for this user")
	}

	// Check rate limit
	if !c.totpRateLimiter.Allow(user.ID) {
		c.logger.Warnf("ValidateTOTP: rate limit exceeded for user_id=%s", user.ID)
		return false, fmt.Errorf("rate limit exceeded, please try again later")
	}

	// Validate TOTP code with ±1 period window
	valid := totp.Validate(code, *user.TOTPSecret)

	if valid {
		// Reset rate limiter on successful authentication
		c.totpRateLimiter.Reset(user.ID)
		c.logger.Infof("ValidateTOTP: valid TOTP code for user_id=%s", user.ID)
	} else {
		c.logger.Warnf("ValidateTOTP: invalid TOTP code for user_id=%s", user.ID)
	}

	return valid, nil
}

// ValidateBackupCode validates a backup code for the given user.
//
// This is used during login when the user doesn't have access to their
// authenticator app. Each backup code can only be used once.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose backup code is being validated
//   - code: Backup code from the user
//
// Returns:
//   - valid: Whether the backup code is valid and unused
//   - error: Any error that occurred
func (c *Connector) ValidateBackupCode(ctx context.Context, user *User, code string) (bool, error) {
	c.logger.Infof("ValidateBackupCode: user_id=%s", user.ID)

	// Check if user has backup codes
	if len(user.BackupCodes) == 0 {
		c.logger.Warnf("ValidateBackupCode: no backup codes for user_id=%s", user.ID)
		return false, fmt.Errorf("no backup codes configured")
	}

	// Find matching backup code
	for i := range user.BackupCodes {
		backupCode := &user.BackupCodes[i]

		// Skip already used codes
		if backupCode.Used {
			continue
		}

		// Verify code against hash
		valid := verifyPassword(code, backupCode.Code)
		if valid {
			// Mark code as used
			now := time.Now()
			backupCode.Used = true
			backupCode.UsedAt = &now
			user.UpdatedAt = time.Now()

			// Save user
			if err := c.storage.UpdateUser(ctx, user); err != nil {
				c.logger.Errorf("ValidateBackupCode: failed to update user: %v", err)
				return false, fmt.Errorf("failed to update user: %w", err)
			}

			c.logger.Infof("ValidateBackupCode: valid backup code for user_id=%s", user.ID)
			return true, nil
		}
	}

	c.logger.Warnf("ValidateBackupCode: invalid or already used backup code for user_id=%s", user.ID)
	return false, nil
}

// DisableTOTP disables TOTP for the given user.
//
// This removes the TOTP secret and all backup codes from the user's account.
// The user must provide a valid TOTP code to confirm the action.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose TOTP is being disabled
//   - code: TOTP code to confirm the action
//
// Returns:
//   - error: Any error that occurred (nil if successful)
func (c *Connector) DisableTOTP(ctx context.Context, user *User, code string) error {
	c.logger.Infof("DisableTOTP: user_id=%s", user.ID)

	// Verify TOTP code before disabling
	valid, err := c.ValidateTOTP(ctx, user, code)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("invalid TOTP code")
	}

	// Clear TOTP data
	user.TOTPSecret = nil
	user.TOTPEnabled = false
	user.BackupCodes = nil
	user.UpdatedAt = time.Now()

	// Save user
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("DisableTOTP: failed to update user: %v", err)
		return fmt.Errorf("failed to update user: %w", err)
	}

	c.logger.Infof("DisableTOTP: TOTP disabled for user_id=%s", user.ID)
	return nil
}

// RegenerateBackupCodes generates new backup codes for the user.
//
// This replaces all existing backup codes with new ones. The user must
// provide a valid TOTP code to confirm the action.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose backup codes are being regenerated
//   - code: TOTP code to confirm the action
//
// Returns:
//   - backupCodes: The new backup codes (plaintext, to be displayed once)
//   - error: Any error that occurred
func (c *Connector) RegenerateBackupCodes(ctx context.Context, user *User, code string) ([]string, error) {
	c.logger.Infof("RegenerateBackupCodes: user_id=%s", user.ID)

	// Verify TOTP code before regenerating
	valid, err := c.ValidateTOTP(ctx, user, code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("invalid TOTP code")
	}

	// Generate new backup codes
	backupCodes, err := c.generateBackupCodes(10)
	if err != nil {
		c.logger.Errorf("RegenerateBackupCodes: failed to generate backup codes: %v", err)
		return nil, fmt.Errorf("failed to generate backup codes: %w", err)
	}

	// Hash and store backup codes
	hashedBackupCodes := make([]BackupCode, len(backupCodes))
	for i, code := range backupCodes {
		hash, err := hashPassword(code)
		if err != nil {
			c.logger.Errorf("RegenerateBackupCodes: failed to hash backup code: %v", err)
			return nil, fmt.Errorf("failed to hash backup code: %w", err)
		}
		hashedBackupCodes[i] = BackupCode{
			Code:  hash,
			Used:  false,
			UsedAt: nil,
		}
	}

	// Update user record
	user.BackupCodes = hashedBackupCodes
	user.UpdatedAt = time.Now()

	// Save user
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("RegenerateBackupCodes: failed to update user: %v", err)
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	c.logger.Infof("RegenerateBackupCodes: backup codes regenerated for user_id=%s", user.ID)
	return backupCodes, nil
}

// generateBackupCodes generates cryptographically secure backup codes.
//
// Each backup code is 8 characters (uppercase alphanumeric without ambiguous characters).
//
// Parameters:
//   - count: Number of backup codes to generate
//
// Returns:
//   - codes: Generated backup codes
//   - error: Any error that occurred
func (c *Connector) generateBackupCodes(count int) ([]string, error) {
	// Character set without ambiguous characters (0, O, 1, I, L)
	const charset = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
	const codeLength = 8

	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code := make([]byte, codeLength)
		randomBytes := make([]byte, codeLength)

		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate random bytes: %w", err)
		}

		for j := 0; j < codeLength; j++ {
			code[j] = charset[int(randomBytes[j])%len(charset)]
		}

		codes[i] = string(code)
	}

	return codes, nil
}

// generateTOTPSecret generates a cryptographically secure TOTP secret.
//
// The secret is 32 bytes (256 bits) and base32 encoded.
//
// Returns:
//   - secret: Base32 encoded TOTP secret
//   - error: Any error that occurred
func generateTOTPSecret() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}

	return base32.StdEncoding.EncodeToString(secret), nil
}
