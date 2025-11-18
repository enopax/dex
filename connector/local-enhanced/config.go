package local

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// Config holds the configuration for the enhanced local connector.
type Config struct {
	// BaseURL is the base URL for the Dex server (e.g., "https://auth.enopax.io")
	BaseURL string `json:"baseURL"`

	// DataDir is the directory where user data is stored
	DataDir string `json:"dataDir"`

	// TemplateDir is the directory containing HTML templates
	TemplateDir string `json:"templateDir"`

	// Passkey configuration
	Passkey PasskeyConfig `json:"passkey"`

	// TwoFactor configuration
	TwoFactor TwoFactorConfig `json:"twoFactor"`

	// MagicLink configuration
	MagicLink MagicLinkConfig `json:"magicLink"`

	// Email configuration
	Email EmailConfig `json:"email"`

	// GRPC configuration
	GRPC GRPCConfig `json:"grpc"`
}

// PasskeyConfig holds WebAuthn/passkey configuration.
type PasskeyConfig struct {
	// Enabled indicates whether passkey authentication is enabled
	Enabled bool `json:"enabled"`

	// RPID is the Relying Party ID (typically the domain)
	RPID string `json:"rpID"`

	// RPName is the Relying Party display name
	RPName string `json:"rpName"`

	// RPOrigins is the list of allowed origins for WebAuthn
	RPOrigins []string `json:"rpOrigins"`

	// UserVerification is the user verification requirement
	// Options: "required", "preferred", "discouraged"
	UserVerification string `json:"userVerification"`

	// AttestationPreference is the attestation conveyance preference
	// Options: "none", "indirect", "direct", "enterprise"
	AttestationPreference string `json:"attestationPreference"`

	// AuthenticatorSelection criteria
	AuthenticatorSelection AuthenticatorSelectionConfig `json:"authenticatorSelection"`
}

// AuthenticatorSelectionConfig holds authenticator selection criteria.
type AuthenticatorSelectionConfig struct {
	// AuthenticatorAttachment filters authenticators by attachment
	// Options: "platform", "cross-platform", "" (no preference)
	AuthenticatorAttachment string `json:"authenticatorAttachment"`

	// RequireResidentKey indicates whether resident keys are required
	RequireResidentKey bool `json:"requireResidentKey"`

	// ResidentKey is the resident key requirement
	// Options: "discouraged", "preferred", "required"
	ResidentKey string `json:"residentKey"`

	// UserVerification is the user verification requirement
	// Options: "required", "preferred", "discouraged"
	UserVerification string `json:"userVerification"`
}

// TwoFactorConfig holds 2FA configuration.
type TwoFactorConfig struct {
	// Required indicates whether 2FA is required globally
	Required bool `json:"required"`

	// Methods is the list of allowed 2FA methods
	// Options: "totp", "passkey"
	Methods []string `json:"methods"`

	// GracePeriod is the grace period in seconds for users to set up 2FA
	GracePeriod int `json:"gracePeriod"`
}

// MagicLinkConfig holds magic link configuration.
type MagicLinkConfig struct {
	// Enabled indicates whether magic link authentication is enabled
	Enabled bool `json:"enabled"`

	// TTL is the time-to-live in seconds for magic link tokens
	TTL int `json:"ttl"`

	// RateLimit configuration for magic link sending
	RateLimit RateLimitConfig `json:"rateLimit"`
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	// PerHour is the maximum number of requests per hour
	PerHour int `json:"perHour"`

	// PerDay is the maximum number of requests per day
	PerDay int `json:"perDay"`
}

// EmailConfig holds email/SMTP configuration.
type EmailConfig struct {
	// SMTP configuration
	SMTP SMTPConfig `json:"smtp"`

	// From is the sender email address
	From string `json:"from"`

	// FromName is the sender display name
	FromName string `json:"fromName"`
}

// SMTPConfig holds SMTP server configuration.
type SMTPConfig struct {
	// Host is the SMTP server hostname
	Host string `json:"host"`

	// Port is the SMTP server port
	Port int `json:"port"`

	// Username is the SMTP authentication username
	Username string `json:"username"`

	// Password is the SMTP authentication password
	Password string `json:"password"`

	// TLS indicates whether to use TLS
	TLS bool `json:"tls"`
}

// GRPCConfig holds gRPC API configuration.
type GRPCConfig struct {
	// Enabled indicates whether gRPC API authentication is enabled
	Enabled bool `json:"enabled"`

	// APIKeys is a list of valid API keys for gRPC authentication
	// Each key should be a long random string (32+ characters)
	APIKeys []string `json:"apiKeys"`

	// RequireAuthentication indicates whether authentication is required for all gRPC calls
	// If false, API will be accessible without authentication (INSECURE - only for development)
	RequireAuthentication bool `json:"requireAuthentication"`
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.BaseURL == "" {
		return errors.New("baseURL is required")
	}

	// Validate baseURL uses HTTPS
	if !strings.HasPrefix(c.BaseURL, "https://") {
		return errors.New("baseURL must use HTTPS (required for WebAuthn and magic links)")
	}

	if c.DataDir == "" {
		return errors.New("dataDir is required")
	}

	// Validate passkey config
	if c.Passkey.Enabled {
		if c.Passkey.RPID == "" {
			return errors.New("passkey.rpID is required when passkey is enabled")
		}
		if c.Passkey.RPName == "" {
			return errors.New("passkey.rpName is required when passkey is enabled")
		}
		if len(c.Passkey.RPOrigins) == 0 {
			return errors.New("passkey.rpOrigins must contain at least one origin")
		}

		// Validate all RPOrigins use HTTPS (required for WebAuthn)
		for i, origin := range c.Passkey.RPOrigins {
			if !strings.HasPrefix(origin, "https://") && !strings.HasPrefix(origin, "http://localhost") {
				return fmt.Errorf("passkey.rpOrigins[%d] must use HTTPS (got: %s). Only localhost is allowed with HTTP for development", i, origin)
			}
		}
	}

	// Validate magic link config
	if c.MagicLink.Enabled {
		if c.MagicLink.TTL <= 0 {
			return errors.New("magicLink.ttl must be greater than 0")
		}
		if c.Email.SMTP.Host == "" {
			return errors.New("email.smtp.host is required when magic link is enabled")
		}
		if c.Email.From == "" {
			return errors.New("email.from is required when magic link is enabled")
		}
	}

	// Validate gRPC config
	if c.GRPC.Enabled && c.GRPC.RequireAuthentication {
		if len(c.GRPC.APIKeys) == 0 {
			return errors.New("grpc.apiKeys must contain at least one API key when authentication is required")
		}
		// Validate API key length (should be 32+ characters)
		for i, key := range c.GRPC.APIKeys {
			if len(key) < 32 {
				return fmt.Errorf("grpc.apiKeys[%d] must be at least 32 characters (got %d)", i, len(key))
			}
		}
	}

	return nil
}

// DefaultConfig returns a default configuration suitable for development.
func DefaultConfig() *Config {
	return &Config{
		BaseURL:     "https://auth.enopax.io",
		DataDir:     "./data",
		TemplateDir: "./templates",
		Passkey: PasskeyConfig{
			Enabled: true,
			RPID:    "auth.enopax.io",
			RPName:  "Enopax",
			RPOrigins: []string{
				"https://auth.enopax.io",
			},
			UserVerification:      "preferred",
			AttestationPreference: "none",
			AuthenticatorSelection: AuthenticatorSelectionConfig{
				AuthenticatorAttachment: "",
				RequireResidentKey:      false,
				ResidentKey:             "preferred",
				UserVerification:        "preferred",
			},
		},
		TwoFactor: TwoFactorConfig{
			Required:    false,
			Methods:     []string{"totp", "passkey"},
			GracePeriod: 86400 * 7, // 7 days
		},
		MagicLink: MagicLinkConfig{
			Enabled: true,
			TTL:     600, // 10 minutes
			RateLimit: RateLimitConfig{
				PerHour: 3,
				PerDay:  10,
			},
		},
		Email: EmailConfig{
			SMTP: SMTPConfig{
				Host: "smtp.example.com",
				Port: 587,
				TLS:  true,
			},
			From:     "noreply@enopax.io",
			FromName: "Enopax Authentication",
		},
		GRPC: GRPCConfig{
			Enabled:               false, // Disabled by default for development
			APIKeys:               []string{},
			RequireAuthentication: false, // No auth for development
		},
	}
}

// Open creates a new connector from the configuration.
func (c *Config) Open(id string, logger interface{}) (interface{}, error) {
	// Validate configuration
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Convert logger to logrus.FieldLogger
	log, ok := logger.(logrus.FieldLogger)
	if !ok {
		return nil, errors.New("invalid logger type: expected logrus.FieldLogger")
	}

	// Create a scoped logger with connector ID
	scopedLogger := log.WithField("connector", id)

	// Create connector
	conn, err := New(c, scopedLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create connector: %w", err)
	}

	return conn, nil
}
