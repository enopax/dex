package local

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// hashPassword hashes a password using bcrypt.
//
// Parameters:
//   - password: The plaintext password to hash
//
// Returns:
//   - hash: The bcrypt hash of the password
//   - error: Any error that occurred
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// verifyPassword verifies a password against a bcrypt hash.
//
// Parameters:
//   - password: The plaintext password to verify
//   - hash: The bcrypt hash to verify against
//
// Returns:
//   - valid: Whether the password matches the hash
func verifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// SetPassword sets or updates a user's password.
//
// This hashes the password and stores it in the user record.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose password is being set
//   - password: The new plaintext password
//
// Returns:
//   - error: Any error that occurred (nil if successful)
func (c *Connector) SetPassword(ctx context.Context, user *User, password string) error {
	c.logger.Infof("SetPassword: user_id=%s", user.ID)

	// Validate password (minimum 8 characters)
	if err := ValidatePassword(password); err != nil {
		return err
	}

	// Hash password
	hash, err := hashPassword(password)
	if err != nil {
		c.logger.Errorf("SetPassword: failed to hash password: %v", err)
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user record
	user.PasswordHash = &hash

	// Save user
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("SetPassword: failed to update user: %v", err)
		return fmt.Errorf("failed to update user: %w", err)
	}

	c.logger.Infof("SetPassword: password set for user_id=%s", user.ID)
	return nil
}

// VerifyPassword verifies a password for a user.
//
// This is used during password-based login.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose password is being verified
//   - password: The plaintext password to verify
//
// Returns:
//   - valid: Whether the password is correct
//   - error: Any error that occurred
func (c *Connector) VerifyPassword(ctx context.Context, user *User, password string) (bool, error) {
	c.logger.Infof("VerifyPassword: user_id=%s", user.ID)

	// Check if user has a password set
	if user.PasswordHash == nil {
		c.logger.Warnf("VerifyPassword: no password set for user_id=%s", user.ID)
		return false, fmt.Errorf("no password set for this user")
	}

	// Verify password
	valid := verifyPassword(password, *user.PasswordHash)

	if valid {
		c.logger.Infof("VerifyPassword: valid password for user_id=%s", user.ID)
	} else {
		c.logger.Warnf("VerifyPassword: invalid password for user_id=%s", user.ID)
	}

	return valid, nil
}

// RemovePassword removes a user's password.
//
// This allows users to switch to passwordless authentication.
// The user must have at least one other authentication method enabled.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose password is being removed
//
// Returns:
//   - error: Any error that occurred (nil if successful)
func (c *Connector) RemovePassword(ctx context.Context, user *User) error {
	c.logger.Infof("RemovePassword: user_id=%s", user.ID)

	// Ensure user has at least one other auth method
	if len(user.Passkeys) == 0 && !user.MagicLinkEnabled {
		return fmt.Errorf("cannot remove password: user must have at least one other authentication method")
	}

	// Clear password hash
	user.PasswordHash = nil

	// Save user
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("RemovePassword: failed to update user: %v", err)
		return fmt.Errorf("failed to update user: %w", err)
	}

	c.logger.Infof("RemovePassword: password removed for user_id=%s", user.ID)
	return nil
}
