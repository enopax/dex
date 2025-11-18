package local

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// PasswordRateLimiter prevents brute force password attacks by limiting the number
// of password verification attempts per user within a time window.
type PasswordRateLimiter struct {
	mu       sync.RWMutex
	attempts map[string][]time.Time // userID -> list of attempt timestamps

	// MaxAttempts is the maximum number of password verification attempts allowed per window
	MaxAttempts int

	// Window is the time window for rate limiting
	Window time.Duration
}

// NewPasswordRateLimiter creates a new password rate limiter.
//
// Parameters:
//   - maxAttempts: Maximum number of password attempts allowed within the window
//   - window: Time window for rate limiting (e.g., 5 minutes)
//
// Returns:
//   - *PasswordRateLimiter: The new rate limiter instance
//
// Example:
//   limiter := NewPasswordRateLimiter(5, 5*time.Minute)  // 5 attempts per 5 minutes
func NewPasswordRateLimiter(maxAttempts int, window time.Duration) *PasswordRateLimiter {
	return &PasswordRateLimiter{
		attempts:    make(map[string][]time.Time),
		MaxAttempts: maxAttempts,
		Window:      window,
	}
}

// Allow checks if a password verification attempt is allowed for the given user.
//
// This implements a sliding window rate limiter. Attempts older than the window
// are automatically discarded.
//
// Parameters:
//   - userID: The user ID to check
//
// Returns:
//   - bool: true if the attempt is allowed, false if rate limit is exceeded
func (rl *PasswordRateLimiter) Allow(userID string) bool {
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
//
// This should be called after a successful authentication to reset the counter
// for that user.
//
// Parameters:
//   - userID: The user ID to reset
func (rl *PasswordRateLimiter) Reset(userID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, userID)
}

// Cleanup removes expired attempts from the rate limiter.
//
// This should be called periodically to prevent memory leaks from users who
// never successfully authenticate.
func (rl *PasswordRateLimiter) Cleanup() {
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
// This is used during password-based login. It implements rate limiting to prevent
// brute force attacks.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User whose password is being verified
//   - password: The plaintext password to verify
//
// Returns:
//   - valid: Whether the password is correct
//   - error: Any error that occurred (including rate limit exceeded)
func (c *Connector) VerifyPassword(ctx context.Context, user *User, password string) (bool, error) {
	c.logger.Infof("VerifyPassword: user_id=%s", user.ID)

	// Check rate limit
	if !c.passwordRateLimiter.Allow(user.ID) {
		c.logger.Warnf("VerifyPassword: rate limit exceeded for user_id=%s", user.ID)
		return false, fmt.Errorf("too many password attempts, please try again later")
	}

	// Check if user has a password set
	if user.PasswordHash == nil {
		c.logger.Warnf("VerifyPassword: no password set for user_id=%s", user.ID)
		return false, fmt.Errorf("no password set for this user")
	}

	// Verify password
	valid := verifyPassword(password, *user.PasswordHash)

	if valid {
		c.logger.Infof("VerifyPassword: valid password for user_id=%s", user.ID)
		// Reset rate limiter on successful authentication
		c.passwordRateLimiter.Reset(user.ID)
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
