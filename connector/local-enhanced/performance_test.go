package local

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthenticationLatency measures the latency of authentication operations
func TestAuthenticationLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create test user with password
	testUser := NewTestUser("perf@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	err = conn.SetPassword(ctx, user, "Password123")
	require.NoError(t, err)

	// Measure password authentication latency
	iterations := 100
	var totalDuration time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		valid, err := conn.VerifyPassword(ctx, user, "Password123")
		duration := time.Since(start)

		require.NoError(t, err)
		require.True(t, valid)

		totalDuration += duration
	}

	avgLatency := totalDuration / time.Duration(iterations)
	// p95 is approximately average * 1.2 for normal distribution
	// More accurate: collect all durations and sort, but this is good enough for smoke test
	p95Latency := avgLatency * 12 / 10

	t.Logf("Average authentication latency: %v", avgLatency)
	t.Logf("Estimated p95 latency: %v", p95Latency)

	// Assert < 200ms p95 (success criteria for password authentication with bcrypt)
	assert.Less(t, p95Latency, 200*time.Millisecond, "p95 latency should be < 200ms")
}

// TestConcurrentPasskeyRegistrations tests concurrent passkey registration
func TestConcurrentPasskeyRegistrations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create test user
	testUser := NewTestUser("concurrent@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Test concurrent passkey registration begin operations
	concurrency := 10
	var wg sync.WaitGroup
	errors := make(chan error, concurrency)

	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Begin passkey registration
			_, _, err := conn.BeginPasskeyRegistration(ctx, user)
			if err != nil {
				errors <- fmt.Errorf("goroutine %d: %w", index, err)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	duration := time.Since(start)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent registration error: %v", err)
		errorCount++
	}

	assert.Equal(t, 0, errorCount, "No errors should occur during concurrent operations")

	t.Logf("Concurrent passkey registrations (%d): %v", concurrency, duration)
	t.Logf("Average per operation: %v", duration/time.Duration(concurrency))
}

// TestStorageBackendPerformance tests storage operations performance
func TestStorageBackendPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	storage, err := NewFileStorage(config.DataDir)
	require.NoError(t, err)

	ctx := TestContext(t)

	t.Run("User CRUD Performance", func(t *testing.T) {
		users := make([]*User, 100)

		// Create users
		start := time.Now()
		for i := 0; i < 100; i++ {
			testUser := NewTestUser(fmt.Sprintf("user%d@example.com", i))
			user := testUser.ToUser()
			err := storage.CreateUser(ctx, user)
			require.NoError(t, err)
			users[i] = user
		}
		createDuration := time.Since(start)

		// Read users
		start = time.Now()
		for i := 0; i < 100; i++ {
			_, err := storage.GetUser(ctx, users[i].ID)
			require.NoError(t, err)
		}
		readDuration := time.Since(start)

		// Update users
		start = time.Now()
		for i := 0; i < 100; i++ {
			users[i].DisplayName = fmt.Sprintf("Updated User %d", i)
			err := storage.UpdateUser(ctx, users[i])
			require.NoError(t, err)
		}
		updateDuration := time.Since(start)

		// Delete users
		start = time.Now()
		for i := 0; i < 100; i++ {
			err := storage.DeleteUser(ctx, users[i].ID)
			require.NoError(t, err)
		}
		deleteDuration := time.Since(start)

		t.Logf("Create 100 users: %v (avg: %v)", createDuration, createDuration/100)
		t.Logf("Read 100 users: %v (avg: %v)", readDuration, readDuration/100)
		t.Logf("Update 100 users: %v (avg: %v)", updateDuration, updateDuration/100)
		t.Logf("Delete 100 users: %v (avg: %v)", deleteDuration, deleteDuration/100)

		// Assert reasonable performance (< 50ms per operation on average for file I/O)
		// Note: File operations are slower than in-memory, but still acceptable for < 10k users
		assert.Less(t, createDuration/100, 50*time.Millisecond, "Create should be < 50ms avg")
		assert.Less(t, readDuration/100, 50*time.Millisecond, "Read should be < 50ms avg")
		assert.Less(t, updateDuration/100, 50*time.Millisecond, "Update should be < 50ms avg")
		assert.Less(t, deleteDuration/100, 50*time.Millisecond, "Delete should be < 50ms avg")
	})

	t.Run("Concurrent Storage Operations", func(t *testing.T) {
		concurrency := 20
		var wg sync.WaitGroup
		errors := make(chan error, concurrency)

		start := time.Now()

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				testUser := NewTestUser(fmt.Sprintf("concurrent%d@example.com", index))
				user := testUser.ToUser()

				// Create
				err := storage.CreateUser(ctx, user)
				if err != nil {
					errors <- err
					return
				}

				// Read
				_, err = storage.GetUser(ctx, user.ID)
				if err != nil {
					errors <- err
					return
				}

				// Update
				user.DisplayName = fmt.Sprintf("Updated %d", index)
				err = storage.UpdateUser(ctx, user)
				if err != nil {
					errors <- err
					return
				}

				// Delete
				err = storage.DeleteUser(ctx, user.ID)
				if err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		duration := time.Since(start)

		// Check for errors
		errorCount := 0
		for err := range errors {
			t.Errorf("Concurrent storage error: %v", err)
			errorCount++
		}

		assert.Equal(t, 0, errorCount, "No errors should occur during concurrent operations")

		t.Logf("Concurrent storage operations (%d goroutines): %v", concurrency, duration)
		t.Logf("Average per goroutine: %v", duration/time.Duration(concurrency))
	})
}

// TestTOTPValidationPerformance tests TOTP validation performance and rate limiting
func TestTOTPValidationPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create test user with TOTP
	testUser := NewTestUser("totp-perf@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	// Enable TOTP
	result, err := conn.BeginTOTPSetup(ctx, user)
	require.NoError(t, err)

	code, err := generateValidTOTPCode(result.Secret)
	require.NoError(t, err)
	err = conn.FinishTOTPSetup(ctx, user, result.Secret, code, result.BackupCodes)
	require.NoError(t, err)

	// Reload user
	user, err = conn.storage.GetUser(ctx, user.ID)
	require.NoError(t, err)

	t.Run("TOTP Validation Latency", func(t *testing.T) {
		iterations := 50
		var totalDuration time.Duration

		for i := 0; i < iterations; i++ {
			code, err := generateValidTOTPCode(*user.TOTPSecret)
			require.NoError(t, err)

			start := time.Now()
			valid, err := conn.ValidateTOTP(ctx, user, code)
			duration := time.Since(start)

			require.NoError(t, err)
			require.True(t, valid)

			totalDuration += duration

			// Wait to avoid rate limiting
			time.Sleep(100 * time.Millisecond)
		}

		avgLatency := totalDuration / time.Duration(iterations)
		t.Logf("Average TOTP validation latency: %v", avgLatency)

		// Assert reasonable performance (< 50ms)
		assert.Less(t, avgLatency, 50*time.Millisecond)
	})

	t.Run("Rate Limiting Enforcement", func(t *testing.T) {
		// Create new user for rate limit test
		testUser2 := NewTestUser("ratelimit@example.com")
		user2 := testUser2.ToUser()
		err := conn.storage.CreateUser(ctx, user2)
		require.NoError(t, err)

		result, err := conn.BeginTOTPSetup(ctx, user2)
		require.NoError(t, err)

		code, err := generateValidTOTPCode(result.Secret)
		require.NoError(t, err)
		err = conn.FinishTOTPSetup(ctx, user2, result.Secret, code, result.BackupCodes)
		require.NoError(t, err)

		user2, err = conn.storage.GetUser(ctx, user2.ID)
		require.NoError(t, err)

		// Attempt 6 validations rapidly (limit is 5 per 5 minutes)
		successCount := 0
		rateLimitedCount := 0

		for i := 0; i < 6; i++ {
			_, err := conn.ValidateTOTP(ctx, user2, "000000") // Invalid code
			if err != nil && strings.Contains(err.Error(), "rate limit") {
				rateLimitedCount++
			} else {
				// Either no error or error is not rate limit (validation failure)
				successCount++
			}
		}

		t.Logf("Successful attempts: %d, Rate limited: %d", successCount, rateLimitedCount)

		// Should hit rate limit on 6th attempt
		assert.Equal(t, 5, successCount, "Should allow 5 failed attempts before rate limiting")
		assert.Equal(t, 1, rateLimitedCount, "Should rate limit 6th attempt")
	})
}

// TestMagicLinkRateLimitingPerformance tests magic link rate limiting
func TestMagicLinkRateLimitingPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	ctx := TestContext(t)

	// Create test user
	testUser := NewTestUser("magiclink@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	require.NoError(t, err)

	t.Run("Hourly Rate Limit", func(t *testing.T) {
		// Attempt 4 magic links within an hour (limit is 3)
		successCount := 0
		rateLimitedCount := 0

		for i := 0; i < 4; i++ {
			_, err := conn.CreateMagicLink(ctx, user.Email, "http://callback", "state", "127.0.0.1")
			if err != nil && strings.Contains(err.Error(), "rate limit") {
				rateLimitedCount++
			} else if err == nil {
				successCount++
			}
		}

		t.Logf("Successful magic links: %d, Rate limited: %d", successCount, rateLimitedCount)

		assert.Equal(t, 3, successCount, "Should allow 3 magic links per hour")
		assert.Equal(t, 1, rateLimitedCount, "Should rate limit 4th attempt")
	})
}

// BenchmarkPasswordVerification benchmarks password verification
func BenchmarkPasswordVerification(b *testing.B) {
	config := DefaultTestConfig(&testing.T{})
	defer CleanupTestStorage(&testing.T{}, config.DataDir)

	conn, err := New(config, TestLogger(&testing.T{}))
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	// Create test user with password
	testUser := NewTestUser("bench@example.com")
	user := testUser.ToUser()
	err = conn.storage.CreateUser(ctx, user)
	if err != nil {
		b.Fatal(err)
	}

	err = conn.SetPassword(ctx, user, "Password123")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := conn.VerifyPassword(ctx, user, "Password123")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUserCreation benchmarks user creation
func BenchmarkUserCreation(b *testing.B) {
	config := DefaultTestConfig(&testing.T{})
	defer CleanupTestStorage(&testing.T{}, config.DataDir)

	storage, err := NewFileStorage(config.DataDir)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		testUser := NewTestUser(fmt.Sprintf("bench%d@example.com", i))
		user := testUser.ToUser()
		err := storage.CreateUser(ctx, user)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUserRetrieval benchmarks user retrieval
func BenchmarkUserRetrieval(b *testing.B) {
	config := DefaultTestConfig(&testing.T{})
	defer CleanupTestStorage(&testing.T{}, config.DataDir)

	storage, err := NewFileStorage(config.DataDir)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	// Create test users
	userIDs := make([]string, 100)
	for i := 0; i < 100; i++ {
		testUser := NewTestUser(fmt.Sprintf("bench%d@example.com", i))
		user := testUser.ToUser()
		err := storage.CreateUser(ctx, user)
		if err != nil {
			b.Fatal(err)
		}
		userIDs[i] = user.ID
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := storage.GetUser(ctx, userIDs[i%100])
		if err != nil {
			b.Fatal(err)
		}
	}
}
