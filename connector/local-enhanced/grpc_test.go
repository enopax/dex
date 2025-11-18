package local

import (
	"context"
	"testing"
	"time"

	"github.com/dexidp/dex/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGRPCServer_CreateUser(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	t.Run("create new user", func(t *testing.T) {
		resp, err := server.CreateUser(ctx, &api.CreateUserReq{
			Email:       "alice@example.com",
			Username:    "alice",
			DisplayName: "Alice Smith",
		})
		require.NoError(t, err)
		assert.False(t, resp.AlreadyExists)
		assert.NotNil(t, resp.User)
		assert.Equal(t, "alice@example.com", resp.User.Email)
		assert.Equal(t, "alice", resp.User.Username)
		assert.Equal(t, "Alice Smith", resp.User.DisplayName)
		assert.False(t, resp.User.EmailVerified)
	})

	t.Run("create existing user", func(t *testing.T) {
		resp, err := server.CreateUser(ctx, &api.CreateUserReq{
			Email:       "alice@example.com",
			Username:    "alice2",
			DisplayName: "Alice Smith 2",
		})
		require.NoError(t, err)
		assert.True(t, resp.AlreadyExists)
		assert.NotNil(t, resp.User)
		// Should return existing user, not create new one
		assert.Equal(t, "alice", resp.User.Username) // Original username
	})

	t.Run("invalid email", func(t *testing.T) {
		_, err := server.CreateUser(ctx, &api.CreateUserReq{
			Email:       "invalid-email",
			Username:    "bob",
			DisplayName: "Bob Jones",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid email")
	})

	t.Run("missing email", func(t *testing.T) {
		_, err := server.CreateUser(ctx, &api.CreateUserReq{
			Username:    "charlie",
			DisplayName: "Charlie Brown",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email is required")
	})
}

func TestGRPCServer_GetUser(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user
	user := NewTestUser("alice@example.com")
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("get by user_id", func(t *testing.T) {
		resp, err := server.GetUser(ctx, &api.GetUserReq{
			UserId: user.ID,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)
		assert.NotNil(t, resp.User)
		assert.Equal(t, user.ID, resp.User.Id)
		assert.Equal(t, user.Email, resp.User.Email)
	})

	t.Run("get by email", func(t *testing.T) {
		resp, err := server.GetUser(ctx, &api.GetUserReq{
			Email: user.Email,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)
		assert.NotNil(t, resp.User)
		assert.Equal(t, user.Email, resp.User.Email)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.GetUser(ctx, &api.GetUserReq{
			UserId: "nonexistent",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})

	t.Run("missing both id and email", func(t *testing.T) {
		_, err := server.GetUser(ctx, &api.GetUserReq{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either user_id or email must be provided")
	})
}

func TestGRPCServer_UpdateUser(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user
	user := NewTestUser("alice@example.com")
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("update user", func(t *testing.T) {
		resp, err := server.UpdateUser(ctx, &api.UpdateUserReq{
			UserId:        user.ID,
			Username:      "alice_updated",
			DisplayName:   "Alice Updated",
			EmailVerified: true,
			Require2Fa:    true,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)

		// Verify update
		updated, err := connector.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "alice_updated", updated.Username)
		assert.Equal(t, "Alice Updated", updated.DisplayName)
		assert.True(t, updated.EmailVerified)
		assert.True(t, updated.Require2FA)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.UpdateUser(ctx, &api.UpdateUserReq{
			UserId:   "nonexistent",
			Username: "test",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_DeleteUser(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user
	user := NewTestUser("alice@example.com")
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("delete user", func(t *testing.T) {
		resp, err := server.DeleteUser(ctx, &api.DeleteUserReq{
			UserId: user.ID,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)

		// Verify deletion
		_, err = connector.storage.GetUser(ctx, user.ID)
		assert.Error(t, err)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.DeleteUser(ctx, &api.DeleteUserReq{
			UserId: "nonexistent",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_SetPassword(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user
	user := NewTestUser("alice@example.com")
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("set password", func(t *testing.T) {
		resp, err := server.SetPassword(ctx, &api.SetPasswordReq{
			UserId:   user.ID,
			Password: "SecurePass123",
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)

		// Verify password was set
		updated, err := connector.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.NotNil(t, updated.PasswordHash)
	})

	t.Run("invalid password", func(t *testing.T) {
		_, err := server.SetPassword(ctx, &api.SetPasswordReq{
			UserId:   user.ID,
			Password: "weak",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid password")
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.SetPassword(ctx, &api.SetPasswordReq{
			UserId:   "nonexistent",
			Password: "SecurePass123",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_RemovePassword(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user with password
	user := NewTestUser("alice@example.com")
	require.NoError(t, connector.SetPassword(ctx, user, "SecurePass123"))
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("remove password", func(t *testing.T) {
		resp, err := server.RemovePassword(ctx, &api.RemovePasswordReq{
			UserId: user.ID,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)

		// Verify password was removed
		updated, err := connector.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.Nil(t, updated.PasswordHash)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.RemovePassword(ctx, &api.RemovePasswordReq{
			UserId: "nonexistent",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_EnableTOTP(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user
	user := NewTestUser("alice@example.com")
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("enable TOTP", func(t *testing.T) {
		resp, err := server.EnableTOTP(ctx, &api.EnableTOTPReq{
			UserId: user.ID,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)
		assert.False(t, resp.AlreadyEnabled)
		assert.NotEmpty(t, resp.Secret)
		assert.NotEmpty(t, resp.QrCode)
		assert.NotEmpty(t, resp.OtpauthUrl)
		assert.Len(t, resp.BackupCodes, 10)
	})

	t.Run("TOTP already enabled", func(t *testing.T) {
		// Enable TOTP first
		user.TOTPEnabled = true
		require.NoError(t, connector.storage.SaveUser(ctx, user))

		resp, err := server.EnableTOTP(ctx, &api.EnableTOTPReq{
			UserId: user.ID,
		})
		require.NoError(t, err)
		assert.True(t, resp.AlreadyEnabled)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.EnableTOTP(ctx, &api.EnableTOTPReq{
			UserId: "nonexistent",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_ListPasskeys(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user with passkeys
	user := NewTestUser("alice@example.com")
	passkey1 := NewTestPasskey(user.ID, "MacBook Touch ID")
	passkey2 := NewTestPasskey(user.ID, "Security Key")
	user.Passkeys = []Passkey{passkey1, passkey2}
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("list passkeys", func(t *testing.T) {
		resp, err := server.ListPasskeys(ctx, &api.ListPasskeysReq{
			UserId: user.ID,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)
		assert.Len(t, resp.Passkeys, 2)
		assert.Equal(t, "MacBook Touch ID", resp.Passkeys[0].Name)
		assert.Equal(t, "Security Key", resp.Passkeys[1].Name)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.ListPasskeys(ctx, &api.ListPasskeysReq{
			UserId: "nonexistent",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_RenamePasskey(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user with passkey
	user := NewTestUser("alice@example.com")
	passkey := NewTestPasskey(user.ID, "Old Name")
	user.Passkeys = []Passkey{passkey}
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("rename passkey", func(t *testing.T) {
		resp, err := server.RenamePasskey(ctx, &api.RenamePasskeyReq{
			UserId:    user.ID,
			PasskeyId: passkey.ID,
			NewName:   "New Name",
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)

		// Verify rename
		updated, err := connector.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.Equal(t, "New Name", updated.Passkeys[0].Name)
	})

	t.Run("passkey not found", func(t *testing.T) {
		resp, err := server.RenamePasskey(ctx, &api.RenamePasskeyReq{
			UserId:    user.ID,
			PasskeyId: "nonexistent",
			NewName:   "Test",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_DeletePasskey(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user with passkeys
	user := NewTestUser("alice@example.com")
	passkey1 := NewTestPasskey(user.ID, "Key 1")
	passkey2 := NewTestPasskey(user.ID, "Key 2")
	user.Passkeys = []Passkey{passkey1, passkey2}
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("delete passkey", func(t *testing.T) {
		resp, err := server.DeletePasskey(ctx, &api.DeletePasskeyReq{
			UserId:    user.ID,
			PasskeyId: passkey1.ID,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)

		// Verify deletion
		updated, err := connector.storage.GetUser(ctx, user.ID)
		require.NoError(t, err)
		assert.Len(t, updated.Passkeys, 1)
		assert.Equal(t, passkey2.ID, updated.Passkeys[0].ID)
	})

	t.Run("passkey not found", func(t *testing.T) {
		resp, err := server.DeletePasskey(ctx, &api.DeletePasskeyReq{
			UserId:    user.ID,
			PasskeyId: "nonexistent",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_GetAuthMethods(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Create test user with multiple auth methods
	user := NewTestUser("alice@example.com")
	require.NoError(t, connector.SetPassword(ctx, user, "SecurePass123"))
	passkey := NewTestPasskey(user.ID, "Test Key")
	user.Passkeys = []Passkey{passkey}
	user.TOTPEnabled = true
	require.NoError(t, connector.storage.SaveUser(ctx, user))

	t.Run("get auth methods", func(t *testing.T) {
		resp, err := server.GetAuthMethods(ctx, &api.GetAuthMethodsReq{
			UserId: user.ID,
		})
		require.NoError(t, err)
		assert.False(t, resp.NotFound)
		assert.True(t, resp.HasPassword)
		assert.Equal(t, int32(1), resp.PasskeyCount)
		assert.True(t, resp.TotpEnabled)
		assert.True(t, resp.MagicLinkEnabled)
	})

	t.Run("user not found", func(t *testing.T) {
		resp, err := server.GetAuthMethods(ctx, &api.GetAuthMethodsReq{
			UserId: "nonexistent",
		})
		require.NoError(t, err)
		assert.True(t, resp.NotFound)
	})
}

func TestGRPCServer_Concurrent(t *testing.T) {
	config := DefaultTestConfig(t)
	defer CleanupTestStorage(t, config.DataDir)

	connector := NewTestConnector(t, config)
	server := NewGRPCServer(connector)
	ctx := TestContext(t)

	// Test concurrent user creation
	t.Run("concurrent creates", func(t *testing.T) {
		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func(n int) {
				email := fmt.Sprintf("user%d@example.com", n)
				_, err := server.CreateUser(ctx, &api.CreateUserReq{
					Email:       email,
					Username:    fmt.Sprintf("user%d", n),
					DisplayName: fmt.Sprintf("User %d", n),
				})
				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 5; i++ {
			<-done
		}
	})
}

// NewTestConnector creates a connector for testing (helper function).
func NewTestConnector(t *testing.T, config *Config) *Connector {
	storage := NewFileStorage(config.DataDir, TestLogger(t))
	connector := &Connector{
		config:  config,
		storage: storage,
		logger:  TestLogger(t),
	}
	// Initialize rate limiters
	connector.totpRateLimiter = NewTOTPRateLimiter()
	connector.magicLinkRateLimiter = NewMagicLinkRateLimiter(
		config.MagicLink.RateLimit.PerHour,
		config.MagicLink.RateLimit.PerDay,
	)

	// Start cleanup goroutine
	go connector.CleanupExpiredSessions(context.Background(), time.Minute)

	return connector
}

// fmt import helper
import "fmt"
