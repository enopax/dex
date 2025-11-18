package local

import (
	"context"
	"fmt"
	"testing"

	"github.com/dexidp/dex/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TestGRPCAuth_Disabled tests that auth is skipped when disabled
func TestGRPCAuth_Disabled(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.GRPC.Enabled = false
	config.GRPC.RequireAuthentication = false
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	grpcServer := NewGRPCServer(conn)
	ctx := TestContext(t)

	// Test: Create user without API key should succeed
	resp, err := grpcServer.CreateUser(ctx, &api.CreateUserReq{
		Email:    "test@example.com",
		Username: "testuser",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "test@example.com", resp.User.Email)
}

// TestGRPCAuth_EnabledButNotRequired tests that auth is skipped when enabled but not required
func TestGRPCAuth_EnabledButNotRequired(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.GRPC.Enabled = true
	config.GRPC.RequireAuthentication = false // Not required
	config.GRPC.APIKeys = []string{"test-api-key-12345678901234567890"}
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	grpcServer := NewGRPCServer(conn)
	ctx := TestContext(t)

	// Test: Create user without API key should succeed
	resp, err := grpcServer.CreateUser(ctx, &api.CreateUserReq{
		Email:    "test@example.com",
		Username: "testuser",
	})

	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// TestGRPCAuth_ValidAPIKey tests successful authentication with valid API key
func TestGRPCAuth_ValidAPIKey(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.GRPC.Enabled = true
	config.GRPC.RequireAuthentication = true
	config.GRPC.APIKeys = []string{"valid-api-key-12345678901234567890"}
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	grpcServer := NewGRPCServer(conn)
	interceptor := grpcServer.UnaryServerInterceptor()

	// Create context with valid API key in metadata
	md := metadata.New(map[string]string{
		"authorization": "valid-api-key-12345678901234567890",
	})
	ctx := metadata.NewIncomingContext(TestContext(t), md)

	// Mock handler
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return grpcServer.CreateUser(ctx, req.(*api.CreateUserReq))
	}

	// Test: Request with valid API key should succeed
	resp, err := interceptor(ctx, &api.CreateUserReq{
		Email:    "test@example.com",
		Username: "testuser",
	}, &grpc.UnaryServerInfo{FullMethod: "/api.EnhancedLocalConnector/CreateUser"}, handler)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.True(t, handlerCalled, "Handler should be called")
}

// TestGRPCAuth_InvalidAPIKey tests authentication failure with invalid API key
func TestGRPCAuth_InvalidAPIKey(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.GRPC.Enabled = true
	config.GRPC.RequireAuthentication = true
	config.GRPC.APIKeys = []string{"valid-api-key-12345678901234567890"}
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	grpcServer := NewGRPCServer(conn)
	interceptor := grpcServer.UnaryServerInterceptor()

	// Create context with invalid API key in metadata
	md := metadata.New(map[string]string{
		"authorization": "invalid-api-key",
	})
	ctx := metadata.NewIncomingContext(TestContext(t), md)

	// Mock handler
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	// Test: Request with invalid API key should fail
	resp, err := interceptor(ctx, &api.CreateUserReq{
		Email:    "test@example.com",
		Username: "testuser",
	}, &grpc.UnaryServerInfo{FullMethod: "/api.EnhancedLocalConnector/CreateUser"}, handler)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.False(t, handlerCalled, "Handler should not be called")

	// Check error is Unauthenticated
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "invalid API key")
}

// TestGRPCAuth_MissingAPIKey tests authentication failure when API key is missing
func TestGRPCAuth_MissingAPIKey(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.GRPC.Enabled = true
	config.GRPC.RequireAuthentication = true
	config.GRPC.APIKeys = []string{"valid-api-key-12345678901234567890"}
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	grpcServer := NewGRPCServer(conn)
	interceptor := grpcServer.UnaryServerInterceptor()

	// Create context WITHOUT API key
	ctx := TestContext(t)

	// Mock handler
	handlerCalled := false
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		handlerCalled = true
		return nil, nil
	}

	// Test: Request without metadata should fail
	resp, err := interceptor(ctx, &api.CreateUserReq{
		Email:    "test@example.com",
		Username: "testuser",
	}, &grpc.UnaryServerInfo{FullMethod: "/api.EnhancedLocalConnector/CreateUser"}, handler)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.False(t, handlerCalled, "Handler should not be called")

	// Check error is Unauthenticated
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing metadata")
}

// TestGRPCAuth_MultipleAPIKeys tests authentication with multiple API keys
func TestGRPCAuth_MultipleAPIKeys(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.GRPC.Enabled = true
	config.GRPC.RequireAuthentication = true
	config.GRPC.APIKeys = []string{
		"api-key-1-12345678901234567890",
		"api-key-2-12345678901234567890",
		"api-key-3-12345678901234567890",
	}
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	grpcServer := NewGRPCServer(conn)
	interceptor := grpcServer.UnaryServerInterceptor()

	// Mock handler
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return grpcServer.CreateUser(ctx, req.(*api.CreateUserReq))
	}

	// Test with each API key
	for i, apiKey := range config.GRPC.APIKeys {
		t.Run(fmt.Sprintf("APIKey%d", i+1), func(t *testing.T) {
			md := metadata.New(map[string]string{
				"authorization": apiKey,
			})
			ctx := metadata.NewIncomingContext(TestContext(t), md)

			resp, err := interceptor(ctx, &api.CreateUserReq{
				Email:    fmt.Sprintf("test%d@example.com", i+1),
				Username: fmt.Sprintf("testuser%d", i+1),
			}, &grpc.UnaryServerInfo{FullMethod: "/api.EnhancedLocalConnector/CreateUser"}, handler)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
		})
	}
}

// TestGRPCAuth_ConstantTimeComparison tests that API key comparison is constant-time
func TestGRPCAuth_ConstantTimeComparison(t *testing.T) {
	// Setup
	config := DefaultTestConfig(t)
	config.GRPC.Enabled = true
	config.GRPC.RequireAuthentication = true
	config.GRPC.APIKeys = []string{"correct-api-key-12345678901234567890"}
	defer CleanupTestStorage(t, config.DataDir)

	conn, err := New(config, TestLogger(t))
	require.NoError(t, err)

	grpcServer := NewGRPCServer(conn)

	// Test: validateAPIKey should use constant-time comparison
	// We can't directly test timing, but we can verify it works correctly
	assert.True(t, grpcServer.validateAPIKey("correct-api-key-12345678901234567890"))
	assert.False(t, grpcServer.validateAPIKey("wrong-api-key-1234567890123456789"))
	assert.False(t, grpcServer.validateAPIKey(""))
	assert.False(t, grpcServer.validateAPIKey("short"))
}

// TestGRPCAuth_ConfigValidation tests configuration validation for gRPC auth
func TestGRPCAuth_ConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		config    GRPCConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "Auth disabled - no validation",
			config: GRPCConfig{
				Enabled:               false,
				RequireAuthentication: false,
				APIKeys:               []string{},
			},
			wantError: false,
		},
		{
			name: "Auth enabled but not required - no validation",
			config: GRPCConfig{
				Enabled:               true,
				RequireAuthentication: false,
				APIKeys:               []string{},
			},
			wantError: false,
		},
		{
			name: "Auth required with valid API keys",
			config: GRPCConfig{
				Enabled:               true,
				RequireAuthentication: true,
				APIKeys:               []string{"valid-api-key-12345678901234567890"},
			},
			wantError: false,
		},
		{
			name: "Auth required without API keys - error",
			config: GRPCConfig{
				Enabled:               true,
				RequireAuthentication: true,
				APIKeys:               []string{},
			},
			wantError: true,
			errorMsg:  "grpc.apiKeys must contain at least one API key",
		},
		{
			name: "Auth required with short API key - error",
			config: GRPCConfig{
				Enabled:               true,
				RequireAuthentication: true,
				APIKeys:               []string{"short-key"},
			},
			wantError: true,
			errorMsg:  "grpc.apiKeys[0] must be at least 32 characters",
		},
		{
			name: "Multiple API keys with one short - error",
			config: GRPCConfig{
				Enabled:               true,
				RequireAuthentication: true,
				APIKeys: []string{
					"valid-api-key-12345678901234567890",
					"short",
				},
			},
			wantError: true,
			errorMsg:  "grpc.apiKeys[1] must be at least 32 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultTestConfig(t)
			config.GRPC = tt.config
			// Set a valid HTTPS base URL to avoid validation errors from other fields
			config.BaseURL = "https://auth.example.com"

			err := config.Validate()

			if tt.wantError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
