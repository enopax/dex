package local

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

var (
	// ErrInvalidEmail is returned when an email address is invalid.
	ErrInvalidEmail = errors.New("invalid email address")

	// ErrEmptyUserID is returned when a user ID is empty.
	ErrEmptyUserID = errors.New("user ID cannot be empty")

	// ErrEmptyEmail is returned when an email is empty.
	ErrEmptyEmail = errors.New("email cannot be empty")

	// ErrInvalidPasskey is returned when a passkey is invalid.
	ErrInvalidPasskey = errors.New("invalid passkey")

	// ErrEmptyPasskeyID is returned when a passkey ID is empty.
	ErrEmptyPasskeyID = errors.New("passkey ID cannot be empty")

	// ErrEmptyPasskeyUserID is returned when a passkey user ID is empty.
	ErrEmptyPasskeyUserID = errors.New("passkey user ID cannot be empty")

	// ErrEmptyPublicKey is returned when a passkey public key is empty.
	ErrEmptyPublicKey = errors.New("passkey public key cannot be empty")

	// ErrEmptyPasskeyName is returned when a passkey name is empty.
	ErrEmptyPasskeyName = errors.New("passkey name cannot be empty")

	// ErrInvalidBackupCode is returned when a backup code is invalid.
	ErrInvalidBackupCode = errors.New("invalid backup code")

	// ErrInvalidWebAuthnSession is returned when a WebAuthn session is invalid.
	ErrInvalidWebAuthnSession = errors.New("invalid WebAuthn session")

	// ErrInvalidMagicLinkToken is returned when a magic link token is invalid.
	ErrInvalidMagicLinkToken = errors.New("invalid magic link token")
)

// Validate validates the User struct.
func (u *User) Validate() error {
	// Required fields
	if u.ID == "" {
		return ErrEmptyUserID
	}

	if u.Email == "" {
		return ErrEmptyEmail
	}

	// Validate email format
	if err := ValidateEmail(u.Email); err != nil {
		return fmt.Errorf("invalid user: %w", err)
	}

	// Validate that user has at least one authentication method
	hasAuthMethod := false

	if u.PasswordHash != nil && *u.PasswordHash != "" {
		hasAuthMethod = true
	}

	if len(u.Passkeys) > 0 {
		hasAuthMethod = true
		// Validate each passkey
		for i, passkey := range u.Passkeys {
			if err := passkey.Validate(); err != nil {
				return fmt.Errorf("invalid passkey at index %d: %w", i, err)
			}
		}
	}

	if u.MagicLinkEnabled {
		hasAuthMethod = true
	}

	if !hasAuthMethod {
		return errors.New("user must have at least one authentication method (password, passkey, or magic link)")
	}

	// Validate TOTP if enabled
	if u.TOTPEnabled {
		if u.TOTPSecret == nil || *u.TOTPSecret == "" {
			return errors.New("TOTP is enabled but TOTP secret is empty")
		}
	}

	// Validate backup codes if present
	if len(u.BackupCodes) > 0 {
		for i, code := range u.BackupCodes {
			if err := code.Validate(); err != nil {
				return fmt.Errorf("invalid backup code at index %d: %w", i, err)
			}
		}
	}

	// Validate timestamps
	if u.CreatedAt.IsZero() {
		return errors.New("created_at timestamp is required")
	}

	if u.UpdatedAt.IsZero() {
		return errors.New("updated_at timestamp is required")
	}

	if u.UpdatedAt.Before(u.CreatedAt) {
		return errors.New("updated_at cannot be before created_at")
	}

	if u.LastLoginAt != nil && u.LastLoginAt.Before(u.CreatedAt) {
		return errors.New("last_login_at cannot be before created_at")
	}

	return nil
}

// Validate validates the Passkey struct.
func (p *Passkey) Validate() error {
	if p.ID == "" {
		return ErrEmptyPasskeyID
	}

	if p.UserID == "" {
		return ErrEmptyPasskeyUserID
	}

	if len(p.PublicKey) == 0 {
		return ErrEmptyPublicKey
	}

	if p.Name == "" {
		return ErrEmptyPasskeyName
	}

	// Validate timestamps
	if p.CreatedAt.IsZero() {
		return errors.New("passkey created_at timestamp is required")
	}

	if p.LastUsedAt != nil && p.LastUsedAt.Before(p.CreatedAt) {
		return errors.New("passkey last_used_at cannot be before created_at")
	}

	// Validate AAGUID length (should be 16 bytes)
	if len(p.AAGUID) > 0 && len(p.AAGUID) != 16 {
		return fmt.Errorf("passkey AAGUID must be 16 bytes, got %d", len(p.AAGUID))
	}

	return nil
}

// Validate validates the BackupCode struct.
func (b *BackupCode) Validate() error {
	if b.Code == "" {
		return errors.New("backup code cannot be empty")
	}

	if b.Used && b.UsedAt == nil {
		return errors.New("backup code is marked as used but used_at is nil")
	}

	if !b.Used && b.UsedAt != nil {
		return errors.New("backup code is not used but used_at is set")
	}

	return nil
}

// Validate validates the WebAuthnSession struct.
func (w *WebAuthnSession) Validate() error {
	if w.SessionID == "" {
		return errors.New("session ID cannot be empty")
	}

	if w.UserID == "" {
		return errors.New("session user ID cannot be empty")
	}

	if len(w.Challenge) == 0 {
		return errors.New("session challenge cannot be empty")
	}

	// Validate challenge length (typically 32 bytes)
	if len(w.Challenge) < 16 {
		return fmt.Errorf("session challenge too short: got %d bytes, minimum 16", len(w.Challenge))
	}

	// Validate operation
	if w.Operation != "registration" && w.Operation != "authentication" {
		return fmt.Errorf("invalid session operation: %s (must be 'registration' or 'authentication')", w.Operation)
	}

	// Validate timestamps
	if w.CreatedAt.IsZero() {
		return errors.New("session created_at timestamp is required")
	}

	if w.ExpiresAt.IsZero() {
		return errors.New("session expires_at timestamp is required")
	}

	if w.ExpiresAt.Before(w.CreatedAt) {
		return errors.New("session expires_at cannot be before created_at")
	}

	// Validate that session hasn't already expired
	if time.Now().After(w.ExpiresAt) {
		return errors.New("session has already expired")
	}

	return nil
}

// ValidateEmail validates an email address format.
func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmptyEmail
	}

	// Use net/mail to parse and validate email
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEmail, err)
	}

	// Additional validation: email should have @ and domain
	parts := strings.Split(addr.Address, "@")
	if len(parts) != 2 {
		return ErrInvalidEmail
	}

	if parts[0] == "" || parts[1] == "" {
		return ErrInvalidEmail
	}

	// Domain should have at least one dot
	if !strings.Contains(parts[1], ".") {
		return fmt.Errorf("%w: domain must contain at least one dot", ErrInvalidEmail)
	}

	return nil
}

// ValidatePassword validates a password according to security requirements.
func ValidatePassword(password string) error {
	if password == "" {
		return errors.New("password cannot be empty")
	}

	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	if len(password) > 128 {
		return errors.New("password must not exceed 128 characters")
	}

	// Check for at least one letter and one number (basic requirement)
	hasLetter := false
	hasNumber := false

	for _, char := range password {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			hasLetter = true
		}
		if char >= '0' && char <= '9' {
			hasNumber = true
		}
	}

	if !hasLetter {
		return errors.New("password must contain at least one letter")
	}

	if !hasNumber {
		return errors.New("password must contain at least one number")
	}

	return nil
}

// ValidateUsername validates a username.
func ValidateUsername(username string) error {
	if username == "" {
		return errors.New("username cannot be empty")
	}

	if len(username) < 3 {
		return errors.New("username must be at least 3 characters long")
	}

	if len(username) > 64 {
		return errors.New("username must not exceed 64 characters")
	}

	// Username should only contain alphanumeric characters, hyphens, and underscores
	for _, char := range username {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' ||
			char == '_') {
			return errors.New("username can only contain letters, numbers, hyphens, and underscores")
		}
	}

	// Username cannot start with a hyphen or number
	firstChar := rune(username[0])
	if firstChar == '-' || (firstChar >= '0' && firstChar <= '9') {
		return errors.New("username must start with a letter or underscore")
	}

	return nil
}
