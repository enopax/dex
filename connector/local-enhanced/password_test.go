package local

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPasswordRateLimiter_Allow tests the Allow method of PasswordRateLimiter.
func TestPasswordRateLimiter_Allow(t *testing.T) {
	limiter := NewPasswordRateLimiter(5, 5*time.Minute)

	userID := "test-user-1"

	// First 5 attempts should be allowed
	for i := 0; i < 5; i++ {
		allowed := limiter.Allow(userID)
		assert.True(t, allowed, "Attempt %d should be allowed", i+1)
	}

	// 6th attempt should be blocked
	allowed := limiter.Allow(userID)
	assert.False(t, allowed, "6th attempt should be blocked (rate limit exceeded)")

	// 7th attempt should also be blocked
	allowed = limiter.Allow(userID)
	assert.False(t, allowed, "7th attempt should be blocked (rate limit exceeded)")
}

// TestPasswordRateLimiter_Reset tests the Reset method.
func TestPasswordRateLimiter_Reset(t *testing.T) {
	limiter := NewPasswordRateLimiter(5, 5*time.Minute)

	userID := "test-user-2"

	// Use up all attempts
	for i := 0; i < 5; i++ {
		limiter.Allow(userID)
	}

	// Verify blocked
	allowed := limiter.Allow(userID)
	assert.False(t, allowed, "Should be blocked after 5 attempts")

	// Reset
	limiter.Reset(userID)

	// Should be allowed again
	allowed = limiter.Allow(userID)
	assert.True(t, allowed, "Should be allowed after reset")
}

// TestPasswordRateLimiter_Cleanup tests the Cleanup method.
func TestPasswordRateLimiter_Cleanup(t *testing.T) {
	// Use a short window for testing
	limiter := NewPasswordRateLimiter(5, 100*time.Millisecond)

	userID := "test-user-3"

	// Make an attempt
	allowed := limiter.Allow(userID)
	assert.True(t, allowed, "First attempt should be allowed")

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Run cleanup
	limiter.Cleanup()

	// Verify user has been cleaned up
	limiter.mu.RLock()
	_, exists := limiter.attempts[userID]
	limiter.mu.RUnlock()

	assert.False(t, exists, "Expired user should be removed from attempts map")
}

// TestPasswordRateLimiter_PerUserLimit tests that rate limiting is per-user.
func TestPasswordRateLimiter_PerUserLimit(t *testing.T) {
	limiter := NewPasswordRateLimiter(5, 5*time.Minute)

	user1 := "test-user-4"
	user2 := "test-user-5"

	// User 1: use up all attempts
	for i := 0; i < 5; i++ {
		limiter.Allow(user1)
	}

	// User 1 should be blocked
	allowed := limiter.Allow(user1)
	assert.False(t, allowed, "User 1 should be blocked")

	// User 2 should still be allowed (different user)
	allowed = limiter.Allow(user2)
	assert.True(t, allowed, "User 2 should be allowed (different user)")
}

// TestPasswordRateLimiter_SlidingWindow tests sliding window behavior.
func TestPasswordRateLimiter_SlidingWindow(t *testing.T) {
	// Use a short window for testing
	limiter := NewPasswordRateLimiter(3, 200*time.Millisecond)

	userID := "test-user-6"

	// Make 3 attempts (max)
	for i := 0; i < 3; i++ {
		allowed := limiter.Allow(userID)
		assert.True(t, allowed, "Attempt %d should be allowed", i+1)
	}

	// 4th attempt should be blocked
	allowed := limiter.Allow(userID)
	assert.False(t, allowed, "4th attempt should be blocked")

	// Wait for window to expire
	time.Sleep(250 * time.Millisecond)

	// Should be allowed again (sliding window)
	allowed = limiter.Allow(userID)
	assert.True(t, allowed, "Should be allowed after window expires")
}

// TestVerifyPassword_RateLimiting tests rate limiting in VerifyPassword.
func TestVerifyPassword_RateLimiting(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create connector
	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create test user with password
	testUser := NewTestUser("test@example.com")
	user := testUser.ToUser()
	err = connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Set password
	password := "Password123"
	err = connector.SetPassword(ctx, user, password)
	require.NoError(t, err)

	// Get updated user
	user, err = connector.storage.GetUser(ctx, user.ID)
	require.NoError(t, err)

	// First 5 attempts should be processed (even if wrong password)
	wrongPassword := "WrongPassword123"
	for i := 0; i < 5; i++ {
		valid, err := connector.VerifyPassword(ctx, user, wrongPassword)
		assert.False(t, valid, "Wrong password should fail validation")
		assert.NoError(t, err, "Should not return error for wrong password")
	}

	// 6th attempt should be rate limited
	valid, err := connector.VerifyPassword(ctx, user, wrongPassword)
	assert.False(t, valid, "Should not validate when rate limited")
	assert.Error(t, err, "Should return error when rate limited")
	assert.Contains(t, err.Error(), "too many password attempts", "Error should mention rate limit")
}

// TestVerifyPassword_RateLimitReset tests that rate limiter is reset on successful authentication.
func TestVerifyPassword_RateLimitReset(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create connector
	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create test user with password
	testUser := NewTestUser("test2@example.com")
	user := testUser.ToUser()
	err = connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Set password
	password := "Password123"
	err = connector.SetPassword(ctx, user, password)
	require.NoError(t, err)

	// Get updated user
	user, err = connector.storage.GetUser(ctx, user.ID)
	require.NoError(t, err)

	// Make 4 failed attempts
	wrongPassword := "WrongPassword123"
	for i := 0; i < 4; i++ {
		connector.VerifyPassword(ctx, user, wrongPassword)
	}

	// 5th attempt with correct password should succeed AND reset the limiter
	valid, err := connector.VerifyPassword(ctx, user, password)
	assert.True(t, valid, "Correct password should validate")
	assert.NoError(t, err, "Should not return error for correct password")

	// Verify rate limiter was reset - should allow 5 more attempts
	for i := 0; i < 5; i++ {
		valid, err := connector.VerifyPassword(ctx, user, wrongPassword)
		assert.False(t, valid, "Wrong password should fail")
		assert.NoError(t, err, "Should not be rate limited after reset")
	}

	// 6th attempt should now be rate limited again
	valid, err = connector.VerifyPassword(ctx, user, wrongPassword)
	assert.False(t, valid, "Should not validate when rate limited")
	assert.Error(t, err, "Should return error when rate limited")
}

// TestVerifyPassword_NoPassword tests VerifyPassword when user has no password set.
func TestVerifyPassword_NoPassword(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	ctx := TestContext(t)

	// Create connector
	connector, err := New(config, TestLogger(t))
	require.NoError(t, err)

	// Create test user WITHOUT password
	testUser := NewTestUser("nopassword@example.com")
	user := testUser.ToUser()
	user.PasswordHash = nil // No password
	err = connector.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Attempt to verify password
	valid, err := connector.VerifyPassword(ctx, user, "AnyPassword123")
	assert.False(t, valid, "Should not validate when no password set")
	assert.Error(t, err, "Should return error when no password set")
	assert.Contains(t, err.Error(), "no password set", "Error should mention no password")
}
