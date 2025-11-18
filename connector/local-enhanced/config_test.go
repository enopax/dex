package local

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultConfig tests the default configuration.
func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "https://auth.enopax.io", config.BaseURL)
	assert.Equal(t, "./data", config.DataDir)
	assert.Equal(t, "./templates", config.TemplateDir)

	// Passkey config
	assert.True(t, config.Passkey.Enabled)
	assert.Equal(t, "auth.enopax.io", config.Passkey.RPID)
	assert.Equal(t, "Enopax", config.Passkey.RPName)
	assert.Contains(t, config.Passkey.RPOrigins, "https://auth.enopax.io")
	assert.Equal(t, "preferred", config.Passkey.UserVerification)
	assert.Equal(t, "none", config.Passkey.AttestationPreference)

	// 2FA config
	assert.False(t, config.TwoFactor.Required)
	assert.Contains(t, config.TwoFactor.Methods, "totp")
	assert.Contains(t, config.TwoFactor.Methods, "passkey")
	assert.Equal(t, 86400*7, config.TwoFactor.GracePeriod) // 7 days

	// Magic link config
	assert.True(t, config.MagicLink.Enabled)
	assert.Equal(t, 600, config.MagicLink.TTL) // 10 minutes
	assert.Equal(t, 3, config.MagicLink.RateLimit.PerHour)
	assert.Equal(t, 10, config.MagicLink.RateLimit.PerDay)

	// Email config
	assert.Equal(t, "smtp.example.com", config.Email.SMTP.Host)
	assert.Equal(t, 587, config.Email.SMTP.Port)
	assert.True(t, config.Email.SMTP.TLS)
	assert.Equal(t, "noreply@enopax.io", config.Email.From)
	assert.Equal(t, "Enopax Authentication", config.Email.FromName)
}

// TestConfigValidation tests configuration validation.
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		modifyFunc  func(*Config)
		expectedErr string
	}{
		{
			name: "valid config",
			modifyFunc: func(c *Config) {
				// No modification - use default valid config
			},
			expectedErr: "",
		},
		{
			name: "missing baseURL",
			modifyFunc: func(c *Config) {
				c.BaseURL = ""
			},
			expectedErr: "baseURL is required",
		},
		{
			name: "missing dataDir",
			modifyFunc: func(c *Config) {
				c.DataDir = ""
			},
			expectedErr: "dataDir is required",
		},
		{
			name: "passkey enabled without RPID",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = true
				c.Passkey.RPID = ""
			},
			expectedErr: "passkey.rpID is required",
		},
		{
			name: "passkey enabled without RPName",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = true
				c.Passkey.RPName = ""
			},
			expectedErr: "passkey.rpName is required",
		},
		{
			name: "passkey enabled without RPOrigins",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = true
				c.Passkey.RPOrigins = []string{}
			},
			expectedErr: "passkey.rpOrigins must contain at least one origin",
		},
		{
			name: "magic link enabled without TTL",
			modifyFunc: func(c *Config) {
				c.MagicLink.Enabled = true
				c.MagicLink.TTL = 0
			},
			expectedErr: "magicLink.ttl must be greater than 0",
		},
		{
			name: "magic link enabled without SMTP host",
			modifyFunc: func(c *Config) {
				c.MagicLink.Enabled = true
				c.Email.SMTP.Host = ""
			},
			expectedErr: "email.smtp.host is required",
		},
		{
			name: "magic link enabled without from email",
			modifyFunc: func(c *Config) {
				c.MagicLink.Enabled = true
				c.Email.From = ""
			},
			expectedErr: "email.from is required",
		},
		{
			name: "passkey disabled is valid",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = false
				c.Passkey.RPID = ""
				c.Passkey.RPName = ""
				c.Passkey.RPOrigins = []string{}
			},
			expectedErr: "",
		},
		{
			name: "magic link disabled is valid",
			modifyFunc: func(c *Config) {
				c.MagicLink.Enabled = false
				c.Email.SMTP.Host = ""
				c.Email.From = ""
			},
			expectedErr: "",
		},
		{
			name: "baseURL with HTTP instead of HTTPS",
			modifyFunc: func(c *Config) {
				c.BaseURL = "http://auth.enopax.io"
			},
			expectedErr: "baseURL must use HTTPS",
		},
		{
			name: "passkey RPOrigin with HTTP (not localhost)",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = true
				c.Passkey.RPOrigins = []string{"http://auth.enopax.io"}
			},
			expectedErr: "passkey.rpOrigins[0] must use HTTPS",
		},
		{
			name: "passkey RPOrigin with HTTP localhost is allowed",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = true
				c.Passkey.RPOrigins = []string{"http://localhost:3000"}
			},
			expectedErr: "",
		},
		{
			name: "passkey RPOrigin with HTTPS is valid",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = true
				c.Passkey.RPOrigins = []string{"https://auth.enopax.io"}
			},
			expectedErr: "",
		},
		{
			name: "multiple passkey RPOrigins with one HTTP (not localhost)",
			modifyFunc: func(c *Config) {
				c.Passkey.Enabled = true
				c.Passkey.RPOrigins = []string{"https://auth.enopax.io", "http://bad.example.com"}
			},
			expectedErr: "passkey.rpOrigins[1] must use HTTPS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			tt.modifyFunc(config)

			err := config.Validate()

			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
		})
	}
}

// TestConfigOpen tests the Config.Open method.
func TestConfigOpen(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	config := &Config{
		BaseURL:     "https://auth.test.local",
		DataDir:     dataDir,
		TemplateDir: dataDir, // Use dataDir as template dir for testing
		Passkey: PasskeyConfig{
			Enabled: true,
			RPID:    "auth.test.local",
			RPName:  "Test",
			RPOrigins: []string{
				"https://auth.test.local",
			},
			UserVerification:      "preferred",
			AttestationPreference: "none",
		},
		TwoFactor: TwoFactorConfig{
			Required:    false,
			Methods:     []string{"totp"},
			GracePeriod: 3600,
		},
		MagicLink: MagicLinkConfig{
			Enabled: false, // Disable to avoid email config requirement
		},
	}

	logger := TestLogger(t)

	// Test successful connector creation
	conn, err := config.Open("test-connector", logger)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	// Verify it's a Connector
	connector, ok := conn.(*Connector)
	require.True(t, ok)
	assert.NotNil(t, connector)
	assert.Equal(t, config, connector.config)
}

// TestConfigOpenInvalidConfig tests Config.Open with invalid configuration.
func TestConfigOpenInvalidConfig(t *testing.T) {
	config := &Config{
		BaseURL: "", // Invalid - missing baseURL
		DataDir: "/tmp/test",
	}

	logger := TestLogger(t)

	_, err := config.Open("test-connector", logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
	assert.Contains(t, err.Error(), "baseURL is required")
}

// TestConfigOpenInvalidLogger tests Config.Open with invalid logger type.
func TestConfigOpenInvalidLogger(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	config := DefaultConfig()
	config.DataDir = dataDir
	config.TemplateDir = dataDir
	config.MagicLink.Enabled = false // Disable to avoid email config requirement

	// Pass a string instead of logger
	_, err := config.Open("test-connector", "invalid-logger")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid logger type")
}

// TestPasskeyConfigValidation tests passkey configuration options.
func TestPasskeyConfigValidation(t *testing.T) {
	tests := []struct {
		name             string
		rpID             string
		rpName           string
		rpOrigins        []string
		userVerification string
		shouldFail       bool
	}{
		{
			name:             "valid config",
			rpID:             "auth.enopax.io",
			rpName:           "Enopax",
			rpOrigins:        []string{"https://auth.enopax.io"},
			userVerification: "preferred",
			shouldFail:       false,
		},
		{
			name:             "multiple origins",
			rpID:             "auth.enopax.io",
			rpName:           "Enopax",
			rpOrigins:        []string{"https://auth.enopax.io", "https://app.enopax.io"},
			userVerification: "required",
			shouldFail:       false,
		},
		{
			name:             "localhost for development",
			rpID:             "localhost",
			rpName:           "Local Dev",
			rpOrigins:        []string{"http://localhost:3000"},
			userVerification: "discouraged",
			shouldFail:       false,
		},
		{
			name:             "HTTP origin (not localhost) should fail",
			rpID:             "auth.enopax.io",
			rpName:           "Enopax",
			rpOrigins:        []string{"http://auth.enopax.io"},
			userVerification: "preferred",
			shouldFail:       true,
		},
		{
			name:             "mixed HTTPS and HTTP localhost is valid",
			rpID:             "auth.enopax.io",
			rpName:           "Enopax",
			rpOrigins:        []string{"https://auth.enopax.io", "http://localhost:5556"},
			userVerification: "preferred",
			shouldFail:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				BaseURL:     "https://auth.test.local",
				DataDir:     "/tmp/test",
				TemplateDir: "/tmp/templates",
				Passkey: PasskeyConfig{
					Enabled:          true,
					RPID:             tt.rpID,
					RPName:           tt.rpName,
					RPOrigins:        tt.rpOrigins,
					UserVerification: tt.userVerification,
				},
				MagicLink: MagicLinkConfig{
					Enabled: false,
				},
			}

			err := config.Validate()

			if tt.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestTwoFactorConfigValidation tests 2FA configuration options.
func TestTwoFactorConfigValidation(t *testing.T) {
	config := DefaultConfig()
	config.DataDir = "/tmp/test"
	config.MagicLink.Enabled = false

	tests := []struct {
		name       string
		required   bool
		methods    []string
		shouldFail bool
	}{
		{
			name:       "TOTP only",
			required:   false,
			methods:    []string{"totp"},
			shouldFail: false,
		},
		{
			name:       "Passkey only",
			required:   true,
			methods:    []string{"passkey"},
			shouldFail: false,
		},
		{
			name:       "Both TOTP and Passkey",
			required:   true,
			methods:    []string{"totp", "passkey"},
			shouldFail: false,
		},
		{
			name:       "Empty methods but not required",
			required:   false,
			methods:    []string{},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.TwoFactor.Required = tt.required
			config.TwoFactor.Methods = tt.methods

			err := config.Validate()

			if tt.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestMagicLinkConfigValidation tests magic link configuration options.
func TestMagicLinkConfigValidation(t *testing.T) {
	baseConfig := DefaultConfig()
	baseConfig.DataDir = "/tmp/test"

	tests := []struct {
		name       string
		ttl        int
		perHour    int
		perDay     int
		shouldFail bool
	}{
		{
			name:       "valid rate limits",
			ttl:        600,
			perHour:    3,
			perDay:     10,
			shouldFail: false,
		},
		{
			name:       "higher rate limits",
			ttl:        300,
			perHour:    10,
			perDay:     50,
			shouldFail: false,
		},
		{
			name:       "zero TTL",
			ttl:        0,
			perHour:    3,
			perDay:     10,
			shouldFail: true,
		},
		{
			name:       "negative TTL",
			ttl:        -100,
			perHour:    3,
			perDay:     10,
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.DataDir = "/tmp/test"
			config.MagicLink.Enabled = true
			config.MagicLink.TTL = tt.ttl
			config.MagicLink.RateLimit.PerHour = tt.perHour
			config.MagicLink.RateLimit.PerDay = tt.perDay

			err := config.Validate()

			if tt.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestEmailConfigValidation tests email/SMTP configuration.
func TestEmailConfigValidation(t *testing.T) {
	tests := []struct {
		name       string
		smtpHost   string
		smtpPort   int
		from       string
		shouldFail bool
	}{
		{
			name:       "valid SMTP config",
			smtpHost:   "smtp.example.com",
			smtpPort:   587,
			from:       "noreply@enopax.io",
			shouldFail: false,
		},
		{
			name:       "missing SMTP host",
			smtpHost:   "",
			smtpPort:   587,
			from:       "noreply@enopax.io",
			shouldFail: true,
		},
		{
			name:       "missing from email",
			smtpHost:   "smtp.example.com",
			smtpPort:   587,
			from:       "",
			shouldFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultConfig()
			config.DataDir = "/tmp/test"
			config.MagicLink.Enabled = true
			config.Email.SMTP.Host = tt.smtpHost
			config.Email.SMTP.Port = tt.smtpPort
			config.Email.From = tt.from

			err := config.Validate()

			if tt.shouldFail {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestConfigWithFieldLogger tests that Config.Open works with logrus.FieldLogger.
func TestConfigWithFieldLogger(t *testing.T) {
	dataDir := SetupTestStorage(t)
	defer CleanupTestStorage(t, dataDir)

	config := DefaultConfig()
	config.DataDir = dataDir
	config.TemplateDir = dataDir
	config.MagicLink.Enabled = false

	// Create a logrus logger
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create a field logger
	fieldLogger := logger.WithField("test", "config")

	// Test with logrus.FieldLogger
	conn, err := config.Open("test-connector", fieldLogger)
	require.NoError(t, err)
	assert.NotNil(t, conn)

	connector, ok := conn.(*Connector)
	require.True(t, ok)
	assert.NotNil(t, connector.logger)
}
