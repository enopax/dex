// Package local provides WebAuthn passkey support for the enhanced local connector.
package local

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

// WebAuthn User Interface Implementation
// These methods are required by the go-webauthn library

// WebAuthnID returns the user's ID in a format suitable for WebAuthn.
// This should be a stable, unique identifier that doesn't contain PII.
func (u *User) WebAuthnID() []byte {
	return []byte(u.ID)
}

// WebAuthnName returns the user's username for WebAuthn.
// This is displayed during registration/authentication.
func (u *User) WebAuthnName() string {
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}

// WebAuthnDisplayName returns the user's display name for WebAuthn.
// This is a user-friendly name displayed in authenticator UIs.
func (u *User) WebAuthnDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Username != "" {
		return u.Username
	}
	return u.Email
}

// WebAuthnIcon returns a URL to the user's icon/avatar.
// Returns empty string if no icon is set.
func (u *User) WebAuthnIcon() string {
	// No icon support for now
	return ""
}

// WebAuthnCredentials returns all WebAuthn credentials for the user.
// This is used during authentication to get the list of valid credentials.
func (u *User) WebAuthnCredentials() []webauthn.Credential {
	credentials := make([]webauthn.Credential, len(u.Passkeys))
	for i, passkey := range u.Passkeys {
		credentials[i] = webauthn.Credential{
			ID:              []byte(passkey.ID),
			PublicKey:       passkey.PublicKey,
			AttestationType: passkey.AttestationType,
			Authenticator: webauthn.Authenticator{
				AAGUID:       passkey.AAGUID,
				SignCount:    passkey.SignCount,
				CloneWarning: false,
			},
			Transport: convertTransports(passkey.Transports),
			Flags: webauthn.CredentialFlags{
				BackupEligible: passkey.BackupEligible,
				BackupState:    passkey.BackupState,
			},
		}
	}
	return credentials
}

// convertTransports converts string transport names to protocol.AuthenticatorTransport.
func convertTransports(transports []string) []protocol.AuthenticatorTransport {
	result := make([]protocol.AuthenticatorTransport, len(transports))
	for i, t := range transports {
		result[i] = protocol.AuthenticatorTransport(t)
	}
	return result
}

// Challenge Generation

// generateChallenge generates a cryptographically secure random challenge
// for WebAuthn registration or authentication.
func generateChallenge() ([]byte, error) {
	challenge := make([]byte, 32) // 32 bytes = 256 bits
	if _, err := rand.Read(challenge); err != nil {
		return nil, fmt.Errorf("failed to generate challenge: %w", err)
	}
	return challenge, nil
}

// generateSessionID generates a unique session ID for WebAuthn sessions.
func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate session ID: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// Passkey Registration

// BeginPasskeyRegistration starts the WebAuthn registration ceremony.
//
// It generates a challenge and registration options for the client,
// creates a session to track the registration flow, and returns the
// PublicKeyCredentialCreationOptions that should be passed to
// navigator.credentials.create() in the browser.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User who is registering the passkey
//
// Returns:
//   - session: WebAuthn session (must be validated in FinishPasskeyRegistration)
//   - options: Options to pass to Web Authentication API
//   - error: Any error that occurred
func (c *Connector) BeginPasskeyRegistration(ctx context.Context, user *User) (*WebAuthnSession, *protocol.CredentialCreation, error) {
	// Validate user
	if err := user.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid user: %w", err)
	}

	// Generate registration options
	options, sessionData, err := c.webAuthn.BeginRegistration(user)
	if err != nil {
		c.logger.Errorf("failed to begin passkey registration: %v", err)
		return nil, nil, fmt.Errorf("failed to begin registration: %w", err)
	}

	// Create session
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, nil, err
	}

	// Store the challenge (sessionData.Challenge is base64 URL encoded string)
	// We need to decode it to bytes for storage
	challengeBytes, err := base64.RawURLEncoding.DecodeString(sessionData.Challenge)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode challenge: %w", err)
	}

	session := &WebAuthnSession{
		SessionID: sessionID,
		UserID:    user.ID,
		Challenge: challengeBytes,
		Operation: "registration",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CreatedAt: time.Now(),
	}

	// Store session
	if err := c.storage.SaveWebAuthnSession(ctx, session); err != nil {
		c.logger.Errorf("failed to save WebAuthn session: %v", err)
		return nil, nil, fmt.Errorf("failed to save session: %w", err)
	}

	c.logger.Infof("began passkey registration for user %s (session: %s)", user.ID, sessionID)
	return session, options, nil
}

// FinishPasskeyRegistration completes the WebAuthn registration ceremony.
//
// It validates the credential creation response from the browser,
// verifies the attestation, and stores the new passkey credential.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - sessionID: The session ID from BeginPasskeyRegistration
//   - response: The credential creation response from the browser
//   - passkeyName: User-friendly name for the passkey (e.g., "MacBook Touch ID")
//
// Returns:
//   - passkey: The newly registered passkey
//   - error: Any error that occurred
func (c *Connector) FinishPasskeyRegistration(ctx context.Context, sessionID string, response *protocol.ParsedCredentialCreationData, passkeyName string) (*Passkey, error) {
	// Get session
	session, err := c.storage.GetWebAuthnSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session: %w", err)
	}

	// Validate session
	if session.Operation != "registration" {
		return nil, fmt.Errorf("invalid session operation: expected registration, got %s", session.Operation)
	}
	if time.Now().After(session.ExpiresAt) {
		c.storage.DeleteWebAuthnSession(ctx, sessionID)
		return nil, fmt.Errorf("session expired")
	}

	// Get user
	user, err := c.storage.GetUser(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Create session data for verification
	sessionData := webauthn.SessionData{
		Challenge:        base64.RawURLEncoding.EncodeToString(session.Challenge),
		UserID:           user.WebAuthnID(),
		UserVerification: protocol.VerificationPreferred,
	}

	// Verify credential
	credential, err := c.webAuthn.CreateCredential(user, sessionData, response)
	if err != nil {
		c.logger.Errorf("failed to verify credential: %v", err)
		return nil, fmt.Errorf("credential verification failed: %w", err)
	}

	// Create passkey record
	passkey := &Passkey{
		ID:              base64.URLEncoding.EncodeToString(credential.ID),
		UserID:          user.ID,
		PublicKey:       credential.PublicKey,
		AttestationType: credential.AttestationType,
		AAGUID:          credential.Authenticator.AAGUID,
		SignCount:       credential.Authenticator.SignCount,
		Transports:      transportStrings(credential.Transport),
		Name:            passkeyName,
		CreatedAt:       time.Now(),
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}

	// Validate passkey
	if err := passkey.Validate(); err != nil {
		return nil, fmt.Errorf("invalid passkey: %w", err)
	}

	// Add passkey to user
	user.Passkeys = append(user.Passkeys, *passkey)
	user.UpdatedAt = time.Now()

	// Save user
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("failed to save passkey: %v", err)
		return nil, fmt.Errorf("failed to save passkey: %w", err)
	}

	// Delete session
	c.storage.DeleteWebAuthnSession(ctx, sessionID)

	c.logger.Infof("user %s registered passkey: %s (name: %s)", user.ID, passkey.ID, passkey.Name)
	return passkey, nil
}

// transportStrings converts protocol.AuthenticatorTransport to string slice.
func transportStrings(transports []protocol.AuthenticatorTransport) []string {
	result := make([]string, len(transports))
	for i, t := range transports {
		result[i] = string(t)
	}
	return result
}

// Passkey Authentication

// BeginPasskeyAuthentication starts the WebAuthn authentication ceremony.
//
// It generates a challenge and authentication options for the client,
// creates a session to track the authentication flow, and returns the
// PublicKeyCredentialRequestOptions that should be passed to
// navigator.credentials.get() in the browser.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - email: Email address of the user (or empty for discoverable credentials)
//
// Returns:
//   - session: WebAuthn session (must be validated in FinishPasskeyAuthentication)
//   - options: Options to pass to Web Authentication API
//   - error: Any error that occurred
func (c *Connector) BeginPasskeyAuthentication(ctx context.Context, email string) (*WebAuthnSession, *protocol.CredentialAssertion, error) {
	var user *User
	var err error

	if email != "" {
		// Get user by email
		user, err = c.storage.GetUserByEmail(ctx, email)
		if err != nil {
			return nil, nil, fmt.Errorf("user not found: %w", err)
		}
	} else {
		// Allow discoverable credentials (resident keys)
		// Create a temporary user for the session
		user = &User{
			ID:    "", // Will be determined after authentication
			Email: "",
		}
	}

	// Generate authentication options
	options, sessionData, err := c.webAuthn.BeginLogin(user)
	if err != nil {
		c.logger.Errorf("failed to begin passkey authentication: %v", err)
		return nil, nil, fmt.Errorf("failed to begin authentication: %w", err)
	}

	// Create session
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, nil, err
	}

	// Store the challenge (sessionData.Challenge is base64 URL encoded string)
	// We need to decode it to bytes for storage
	challengeBytes, err := base64.RawURLEncoding.DecodeString(sessionData.Challenge)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode challenge: %w", err)
	}

	session := &WebAuthnSession{
		SessionID: sessionID,
		UserID:    user.ID,
		Challenge: challengeBytes,
		Operation: "authentication",
		ExpiresAt: time.Now().Add(5 * time.Minute),
		CreatedAt: time.Now(),
	}

	// Store session
	if err := c.storage.SaveWebAuthnSession(ctx, session); err != nil {
		c.logger.Errorf("failed to save WebAuthn session: %v", err)
		return nil, nil, fmt.Errorf("failed to save session: %w", err)
	}

	c.logger.Infof("began passkey authentication (session: %s)", sessionID)
	return session, options, nil
}

// FinishPasskeyAuthentication completes the WebAuthn authentication ceremony.
//
// It validates the credential assertion response from the browser,
// verifies the signature, updates the sign counter, and returns the
// authenticated user identity.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - sessionID: The session ID from BeginPasskeyAuthentication
//   - response: The credential assertion response from the browser
//
// Returns:
//   - user: The authenticated user
//   - passkey: The passkey that was used for authentication
//   - error: Any error that occurred
func (c *Connector) FinishPasskeyAuthentication(ctx context.Context, sessionID string, response *protocol.ParsedCredentialAssertionData) (*User, *Passkey, error) {
	// Get session
	session, err := c.storage.GetWebAuthnSession(ctx, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid session: %w", err)
	}

	// Validate session
	if session.Operation != "authentication" {
		return nil, nil, fmt.Errorf("invalid session operation: expected authentication, got %s", session.Operation)
	}
	if time.Now().After(session.ExpiresAt) {
		c.storage.DeleteWebAuthnSession(ctx, sessionID)
		return nil, nil, fmt.Errorf("session expired")
	}

	// Get credential ID from response
	credentialID := base64.URLEncoding.EncodeToString(response.RawID)

	// Find user by credential ID
	user, err := c.getUserByPasskeyID(ctx, credentialID)
	if err != nil {
		return nil, nil, fmt.Errorf("credential not found: %w", err)
	}

	// Create session data for verification
	sessionData := webauthn.SessionData{
		Challenge:        base64.RawURLEncoding.EncodeToString(session.Challenge),
		UserID:           user.WebAuthnID(),
		UserVerification: protocol.VerificationPreferred,
	}

	// Verify assertion
	credential, err := c.webAuthn.ValidateLogin(user, sessionData, response)
	if err != nil {
		c.logger.Errorf("failed to verify assertion: %v", err)
		return nil, nil, fmt.Errorf("authentication failed: %w", err)
	}

	// Find and update the passkey
	var authenticatedPasskey *Passkey
	for i := range user.Passkeys {
		if user.Passkeys[i].ID == credentialID {
			// Update sign count (clone detection)
			if credential.Authenticator.SignCount > 0 && credential.Authenticator.SignCount <= user.Passkeys[i].SignCount {
				c.logger.Warnf("possible cloned authenticator detected for user %s (credential %s)", user.ID, credentialID)
				return nil, nil, fmt.Errorf("authenticator clone detected")
			}

			user.Passkeys[i].SignCount = credential.Authenticator.SignCount
			now := time.Now()
			user.Passkeys[i].LastUsedAt = &now
			authenticatedPasskey = &user.Passkeys[i]
			break
		}
	}

	if authenticatedPasskey == nil {
		return nil, nil, fmt.Errorf("passkey not found in user record")
	}

	// Update user last login time
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now

	// Save updated user
	if err := c.storage.UpdateUser(ctx, user); err != nil {
		c.logger.Errorf("failed to update user: %v", err)
		return nil, nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Delete session
	c.storage.DeleteWebAuthnSession(ctx, sessionID)

	c.logger.Infof("user %s authenticated with passkey: %s", user.ID, credentialID)
	return user, authenticatedPasskey, nil
}

// Helper Functions

// getUserByPasskeyID finds a user by their passkey credential ID.
// This is used during authentication to find which user owns the credential.
func (c *Connector) getUserByPasskeyID(ctx context.Context, credentialID string) (*User, error) {
	// List all users
	users, err := c.storage.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// Search for the passkey in all users
	for _, user := range users {
		for _, passkey := range user.Passkeys {
			if passkey.ID == credentialID {
				return user, nil
			}
		}
	}

	return nil, ErrPasskeyNotFound
}
