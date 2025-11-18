// Package local provides an enhanced local authentication connector
// supporting multiple authentication methods including passwords,
// passkeys (WebAuthn), TOTP, and magic links.
//
// This connector allows Enopax Platform to manage users with flexible
// authentication policies and true 2FA support.
package local

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/sirupsen/logrus"

	"github.com/dexidp/dex/connector"
)

// Connector implements the connector.Connector and connector.PasswordConnector interfaces.
type Connector struct {
	config          *Config
	storage         Storage
	webAuthn        *webauthn.WebAuthn
	logger          logrus.FieldLogger
	templates       *Templates
	totpRateLimiter *TOTPRateLimiter
}

// New creates a new enhanced local connector.
func New(config *Config, logger logrus.FieldLogger) (*Connector, error) {
	// Initialize storage
	storage, err := NewFileStorage(config.DataDir)
	if err != nil {
		return nil, err
	}

	// Initialize WebAuthn
	webAuthn, err := webauthn.New(&webauthn.Config{
		RPDisplayName: config.Passkey.RPName,
		RPID:          config.Passkey.RPID,
		RPOrigins:     config.Passkey.RPOrigins,
	})
	if err != nil {
		return nil, err
	}

	// Load templates
	templates, err := LoadTemplates(config.TemplateDir)
	if err != nil {
		return nil, err
	}

	// Initialize TOTP rate limiter (5 attempts per 5 minutes)
	totpRateLimiter := NewTOTPRateLimiter(5, 5*time.Minute)

	// Start cleanup goroutine for rate limiter
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			totpRateLimiter.Cleanup()
		}
	}()

	return &Connector{
		config:          config,
		storage:         storage,
		webAuthn:        webAuthn,
		logger:          logger,
		templates:       templates,
		totpRateLimiter: totpRateLimiter,
	}, nil
}

// LoginURL returns the URL to redirect the user to for authentication.
// This is called by Dex's OAuth flow to initiate the login.
func (c *Connector) LoginURL(callbackURL, state string) (string, error) {
	// Build URL to our login page with state parameter
	// The state parameter is the auth request ID from Dex
	loginURL := c.config.BaseURL + "/login?state=" + state + "&callback=" + callbackURL
	c.logger.Infof("LoginURL: redirecting to %s", loginURL)
	return loginURL, nil
}

// HandleCallback handles the callback from the login flow.
// This is called after the user successfully authenticates via passkey or password.
func (c *Connector) HandleCallback(s connector.Scopes, r *http.Request) (connector.Identity, error) {
	// The callback should include the user_id from our login page
	// After successful passkey/password authentication, our login page
	// redirects back to Dex's callback URL with the user_id parameter

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		c.logger.Error("HandleCallback: missing user_id parameter")
		return connector.Identity{}, fmt.Errorf("missing user_id parameter")
	}

	// Get user from storage
	ctx := r.Context()
	user, err := c.storage.GetUser(ctx, userID)
	if err != nil {
		c.logger.Errorf("HandleCallback: failed to get user %s: %v", userID, err)
		return connector.Identity{}, fmt.Errorf("user not found: %w", err)
	}

	// Build connector identity
	identity := connector.Identity{
		UserID:        user.ID,
		Username:      user.Username,
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
	}

	// Add preferred username if set
	if user.DisplayName != "" {
		identity.PreferredUsername = user.DisplayName
	} else if user.Username != "" {
		identity.PreferredUsername = user.Username
	} else {
		identity.PreferredUsername = user.Email
	}

	c.logger.Infof("HandleCallback: authenticated user %s (%s)", user.ID, user.Email)
	return identity, nil
}

// Refresh is called when a client attempts to claim a refresh token.
func (c *Connector) Refresh(ctx context.Context, s connector.Scopes, identity connector.Identity) (connector.Identity, error) {
	// For local users, we just return the same identity
	return identity, nil
}

// RegisterHandlers registers HTTP handlers for the connector.
func (c *Connector) RegisterHandlers(mux *http.ServeMux) {
	// Password authentication
	mux.HandleFunc("/login", c.handleLogin)
	mux.HandleFunc("/login/password", c.handlePasswordLogin)

	// Passkey authentication
	mux.HandleFunc("/passkey/login/begin", c.handlePasskeyLoginBegin)
	mux.HandleFunc("/passkey/login/finish", c.handlePasskeyLoginFinish)
	mux.HandleFunc("/passkey/register/begin", c.handlePasskeyRegisterBegin)
	mux.HandleFunc("/passkey/register/finish", c.handlePasskeyRegisterFinish)

	// TOTP 2FA
	mux.HandleFunc("/totp/verify", c.handleTOTPVerify)
	mux.HandleFunc("/totp/enable", c.handleTOTPEnable)
	mux.HandleFunc("/totp/validate", c.handleTOTPValidate)

	// Magic link
	mux.HandleFunc("/magic-link/send", c.handleMagicLinkSend)
	mux.HandleFunc("/magic-link/verify", c.handleMagicLinkVerify)

	// Auth setup
	mux.HandleFunc("/setup-auth", c.handleAuthSetup)
}

// User represents an enhanced user account with support for multiple
// authentication methods.
type User struct {
	// ID is a deterministic UUID derived from the user's email address
	ID string `json:"id"`

	// Email is the user's primary email address (required, unique)
	Email string `json:"email"`

	// Username is the user's username (optional)
	Username string `json:"username,omitempty"`

	// DisplayName is the user's display name
	DisplayName string `json:"display_name,omitempty"`

	// EmailVerified indicates whether the user's email has been verified
	EmailVerified bool `json:"email_verified"`

	// PasswordHash is the bcrypt hash of the user's password.
	// Nil if user has not set a password (passwordless account).
	PasswordHash *string `json:"password_hash,omitempty"`

	// Passkeys is the list of WebAuthn credentials registered by the user
	Passkeys []Passkey `json:"passkeys,omitempty"`

	// TOTPSecret is the user's TOTP secret (base32 encoded)
	TOTPSecret *string `json:"totp_secret,omitempty"`

	// TOTPEnabled indicates whether TOTP is enabled for this user
	TOTPEnabled bool `json:"totp_enabled"`

	// BackupCodes is the list of backup codes for 2FA recovery
	BackupCodes []BackupCode `json:"backup_codes,omitempty"`

	// MagicLinkEnabled indicates whether magic link authentication is enabled
	MagicLinkEnabled bool `json:"magic_link_enabled"`

	// Require2FA indicates whether this user must use 2FA
	Require2FA bool `json:"require_2fa"`

	// CreatedAt is the timestamp when the user was created
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is the timestamp when the user was last updated
	UpdatedAt time.Time `json:"updated_at"`

	// LastLoginAt is the timestamp of the user's last login
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

// Passkey represents a WebAuthn credential.
type Passkey struct {
	// ID is the credential ID (base64 encoded)
	ID string `json:"id"`

	// UserID is the ID of the user who owns this passkey
	UserID string `json:"user_id"`

	// PublicKey is the credential's public key
	PublicKey []byte `json:"public_key"`

	// AttestationType is the attestation type used during registration
	AttestationType string `json:"attestation_type"`

	// AAGUID is the authenticator's AAGUID
	AAGUID []byte `json:"aaguid"`

	// SignCount is the signature counter for clone detection
	SignCount uint32 `json:"sign_count"`

	// Transports indicates the supported transports
	Transports []string `json:"transports,omitempty"`

	// Name is a user-friendly name for this passkey
	Name string `json:"name"`

	// CreatedAt is the timestamp when the passkey was registered
	CreatedAt time.Time `json:"created_at"`

	// LastUsedAt is the timestamp of the last authentication
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`

	// BackupEligible indicates if the credential can be backed up
	BackupEligible bool `json:"backup_eligible"`

	// BackupState indicates if the credential is currently backed up
	BackupState bool `json:"backup_state"`
}

// BackupCode represents a 2FA backup code.
type BackupCode struct {
	// Code is the backup code (hashed)
	Code string `json:"code"`

	// Used indicates whether this code has been used
	Used bool `json:"used"`

	// UsedAt is the timestamp when the code was used
	UsedAt *time.Time `json:"used_at,omitempty"`
}

// WebAuthnSession represents a WebAuthn challenge session.
type WebAuthnSession struct {
	// SessionID is the unique session identifier
	SessionID string `json:"session_id"`

	// UserID is the ID of the user this session belongs to
	UserID string `json:"user_id"`

	// Challenge is the WebAuthn challenge
	Challenge []byte `json:"challenge"`

	// Operation is either "registration" or "authentication"
	Operation string `json:"operation"`

	// ExpiresAt is when this session expires (typically 5 minutes)
	ExpiresAt time.Time `json:"expires_at"`

	// CreatedAt is when this session was created
	CreatedAt time.Time `json:"created_at"`
}

// MagicLinkToken represents a magic link authentication token.
type MagicLinkToken struct {
	// Token is the unique token (secure random)
	Token string `json:"token"`

	// UserID is the ID of the user this token is for
	UserID string `json:"user_id"`

	// Email is the email address the link was sent to
	Email string `json:"email"`

	// CreatedAt is when this token was created
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is when this token expires (typically 10 minutes)
	ExpiresAt time.Time `json:"expires_at"`

	// Used indicates whether this token has been used
	Used bool `json:"used"`

	// IPAddress is the IP address that requested the magic link
	IPAddress string `json:"ip_address,omitempty"`
}

// Templates holds parsed HTML templates.
type Templates struct {
	Login            string
	SetupAuth        string
	ManageCredential string
}

// LoadTemplates loads HTML templates from the template directory.
func LoadTemplates(templateDir string) (*Templates, error) {
	// If template directory is empty (for tests), return empty templates
	if templateDir == "" {
		return &Templates{}, nil
	}

	// TODO: Implement actual template loading from files
	return &Templates{}, nil
}
