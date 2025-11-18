package local

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// TwoFactorSession represents a 2FA session during login flow.
type TwoFactorSession struct {
	// SessionID is the unique session identifier
	SessionID string `json:"session_id"`

	// UserID is the ID of the user who completed primary authentication
	UserID string `json:"user_id"`

	// PrimaryMethod is the authentication method used in step 1 (password, passkey, magic_link)
	PrimaryMethod string `json:"primary_method"`

	// CreatedAt is when this session was created
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when this session expires (typically 10 minutes)
	ExpiresAt time.Time `json:"expires_at"`

	// Completed indicates whether the 2FA challenge was completed
	Completed bool `json:"completed"`

	// CallbackURL is the OAuth callback URL to redirect to after 2FA
	CallbackURL string `json:"callback_url"`

	// State is the OAuth state parameter
	State string `json:"state"`
}

// Begin2FA creates a 2FA session after successful primary authentication.
// This is called after password or passwordless (passkey/magic link) authentication succeeds.
func (c *Connector) Begin2FA(ctx context.Context, userID, primaryMethod, callbackURL, state string) (*TwoFactorSession, error) {
	// Generate session ID
	sessionID, err := generate2FASessionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}

	// Create session
	session := &TwoFactorSession{
		SessionID:     sessionID,
		UserID:        userID,
		PrimaryMethod: primaryMethod,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(10 * time.Minute),
		Completed:     false,
		CallbackURL:   callbackURL,
		State:         state,
	}

	// Store session
	if err := c.storage.Save2FASession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to store 2FA session: %w", err)
	}

	c.logger.Infof("Begin2FA: created session for user %s (primary method: %s)", userID, primaryMethod)
	return session, nil
}

// Complete2FA completes a 2FA challenge and returns the user ID for OAuth flow.
// This is called after successful TOTP/passkey verification in step 2.
func (c *Connector) Complete2FA(ctx context.Context, sessionID string) (userID, callbackURL, state string, err error) {
	// Get session
	session, err := c.storage.Get2FASession(ctx, sessionID)
	if err != nil {
		return "", "", "", fmt.Errorf("session not found: %w", err)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		c.logger.Warnf("Complete2FA: session %s expired", sessionID)
		_ = c.storage.Delete2FASession(ctx, sessionID)
		return "", "", "", fmt.Errorf("2FA session expired")
	}

	// Check if already completed
	if session.Completed {
		c.logger.Warnf("Complete2FA: session %s already completed", sessionID)
		return "", "", "", fmt.Errorf("2FA session already used")
	}

	// Mark session as completed
	session.Completed = true
	if err := c.storage.Save2FASession(ctx, session); err != nil {
		return "", "", "", fmt.Errorf("failed to update session: %w", err)
	}

	c.logger.Infof("Complete2FA: session %s completed for user %s", sessionID, session.UserID)

	// Clean up session after 1 minute (allow time for redirect)
	go func() {
		time.Sleep(1 * time.Minute)
		_ = c.storage.Delete2FASession(context.Background(), sessionID)
	}()

	return session.UserID, session.CallbackURL, session.State, nil
}

// Require2FAForUser checks if a user requires 2FA based on configuration and user settings.
func (c *Connector) Require2FAForUser(ctx context.Context, user *User) bool {
	// Check user-level 2FA requirement
	if user.Require2FA {
		c.logger.Infof("Require2FAForUser: user %s has Require2FA flag set", user.ID)
		return true
	}

	// Check global 2FA requirement
	if c.config.TwoFactor.Required {
		c.logger.Infof("Require2FAForUser: global 2FA required by config")
		return true
	}

	// Check if user has 2FA enabled (TOTP or multiple auth methods)
	if user.TOTPEnabled {
		c.logger.Infof("Require2FAForUser: user %s has TOTP enabled", user.ID)
		return true
	}

	// If user has both password and passkey, consider it 2FA-ready
	// but don't require it unless explicitly configured
	if user.PasswordHash != nil && len(user.Passkeys) > 0 {
		c.logger.Debugf("Require2FAForUser: user %s has both password and passkey (2FA-capable)", user.ID)
		// Only require if global config says so
		return c.config.TwoFactor.Required
	}

	c.logger.Debugf("Require2FAForUser: 2FA not required for user %s", user.ID)
	return false
}

// GetAvailable2FAMethods returns the list of 2FA methods available for a user.
func (c *Connector) GetAvailable2FAMethods(ctx context.Context, user *User, primaryMethod string) []string {
	var methods []string

	// TOTP is always available if enabled
	if user.TOTPEnabled && contains(c.config.TwoFactor.Methods, "totp") {
		methods = append(methods, "totp")
	}

	// Passkey is available if:
	// 1. User has at least one passkey registered
	// 2. Passkey is in allowed 2FA methods
	// 3. Passkey was not used as the primary authentication method
	if len(user.Passkeys) > 0 &&
	   contains(c.config.TwoFactor.Methods, "passkey") &&
	   primaryMethod != "passkey" {
		methods = append(methods, "passkey")
	}

	// Backup codes are always available if user has unused codes
	hasUnusedBackupCode := false
	for _, code := range user.BackupCodes {
		if !code.Used {
			hasUnusedBackupCode = true
			break
		}
	}
	if hasUnusedBackupCode {
		methods = append(methods, "backup_code")
	}

	c.logger.Debugf("GetAvailable2FAMethods: user %s (primary: %s) has methods: %v",
		user.ID, primaryMethod, methods)
	return methods
}

// Validate2FAMethod validates a 2FA method for a user session.
// Returns true if the validation succeeded.
func (c *Connector) Validate2FAMethod(ctx context.Context, sessionID, method, value string) (bool, error) {
	// Get 2FA session
	session, err := c.storage.Get2FASession(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("session not found: %w", err)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		c.logger.Warnf("Validate2FAMethod: session %s expired", sessionID)
		_ = c.storage.Delete2FASession(ctx, sessionID)
		return false, fmt.Errorf("2FA session expired")
	}

	// Get user
	user, err := c.storage.GetUser(ctx, session.UserID)
	if err != nil {
		return false, fmt.Errorf("user not found: %w", err)
	}

	switch method {
	case "totp":
		// Validate TOTP code
		valid, err := c.ValidateTOTP(ctx, user, value)
		if err != nil {
			c.logger.Errorf("Validate2FAMethod: TOTP validation failed: %v", err)
			return false, err
		}
		return valid, nil

	case "passkey":
		// Passkey validation is handled separately via WebAuthn flow
		// This method is not used for passkeys (they have their own begin/finish endpoints)
		return false, fmt.Errorf("passkey validation requires WebAuthn flow")

	case "backup_code":
		// Validate backup code
		valid, err := c.ValidateBackupCode(ctx, user, value)
		if err != nil {
			c.logger.Errorf("Validate2FAMethod: backup code validation failed: %v", err)
			return false, err
		}
		return valid, nil

	default:
		return false, fmt.Errorf("unknown 2FA method: %s", method)
	}
}

// InGracePeriod checks if a user is still within the 2FA setup grace period.
// Returns true if the user has time to set up 2FA before being forced to.
func (c *Connector) InGracePeriod(ctx context.Context, user *User) bool {
	// If grace period is 0, no grace period
	if c.config.TwoFactor.GracePeriod <= 0 {
		return false
	}

	// If user has any 2FA method set up, grace period doesn't apply
	if user.TOTPEnabled || len(user.Passkeys) > 0 {
		return false
	}

	// Check if account age is within grace period
	accountAge := time.Since(user.CreatedAt)
	gracePeriod := time.Duration(c.config.TwoFactor.GracePeriod) * time.Second

	inGrace := accountAge < gracePeriod
	c.logger.Debugf("InGracePeriod: user %s account age %v, grace period %v, in grace: %v",
		user.ID, accountAge, gracePeriod, inGrace)
	return inGrace
}

// generate2FASessionID generates a cryptographically secure random session ID.
func generate2FASessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// contains checks if a string slice contains a value.
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
