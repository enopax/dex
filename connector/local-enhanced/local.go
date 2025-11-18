// Package local provides an enhanced local authentication connector
// supporting multiple authentication methods including passwords,
// passkeys (WebAuthn), TOTP, and magic links.
//
// This connector allows Enopax Platform to manage users with flexible
// authentication policies and true 2FA support.
package local

import (
	"context"
	"net/http"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/sirupsen/logrus"

	"github.com/dexidp/dex/connector"
)

// Connector implements the connector.Connector and connector.PasswordConnector interfaces.
type Connector struct {
	config    *Config
	storage   Storage
	webAuthn  *webauthn.WebAuthn
	logger    logrus.FieldLogger
	templates *Templates
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

	return &Connector{
		config:    config,
		storage:   storage,
		webAuthn:  webAuthn,
		logger:    logger,
		templates: templates,
	}, nil
}

// LoginURL returns the URL to redirect the user to for authentication.
func (c *Connector) LoginURL(callbackURL, state string) (string, error) {
	// Return URL to our login page
	return c.config.BaseURL + "/login?state=" + state + "&callback=" + callbackURL, nil
}

// HandleCallback handles the callback from the login flow.
func (c *Connector) HandleCallback(s connector.Scopes, r *http.Request) (connector.Identity, error) {
	// This will be implemented in the password.go and passkey.go files
	// For now, return a basic implementation
	return connector.Identity{}, nil
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
	// TODO: Implement template loading
	return &Templates{}, nil
}
