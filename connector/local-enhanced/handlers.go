package local

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
)

// HTTP handler stubs - to be implemented in subsequent phases

// handleLogin displays the login page.
func (c *Connector) handleLogin(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 2
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handlePasswordLogin handles password-based login.
func (c *Connector) handlePasswordLogin(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 1 (Week 3-4)
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// PasskeyAuthenticateBeginRequest represents the request body for beginning passkey authentication.
type PasskeyAuthenticateBeginRequest struct {
	// Email is the email address of the user (optional for discoverable credentials)
	Email string `json:"email,omitempty"`
}

// PasskeyAuthenticateBeginResponse represents the response from beginning passkey authentication.
type PasskeyAuthenticateBeginResponse struct {
	// SessionID is the session identifier to be used in the finish call
	SessionID string `json:"session_id"`

	// Options contains the PublicKeyCredentialRequestOptions to be passed
	// to navigator.credentials.get() in the browser
	Options interface{} `json:"options"`
}

// handlePasskeyLoginBegin begins passkey authentication.
//
// This endpoint:
// 1. Validates the request and checks if passkeys are enabled
// 2. Retrieves the user by email (or allows discoverable credentials if no email provided)
// 3. Calls BeginPasskeyAuthentication to generate challenge and options
// 4. Creates a WebAuthn session with 5-minute TTL
// 5. Returns the session ID and PublicKeyCredentialRequestOptions
//
// Request body:
//
//	{
//	  "email": "user@example.com"  // Optional - omit for discoverable credentials
//	}
//
// Response:
//
//	{
//	  "session_id": "base64-session-id",
//	  "options": {
//	    "publicKey": {
//	      "challenge": "base64-challenge",
//	      "timeout": 60000,
//	      "rpId": "auth.enopax.io",
//	      "allowCredentials": [...],  // Empty for discoverable credentials
//	      "userVerification": "preferred"
//	    }
//	  }
//	}
func (c *Connector) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if passkeys are enabled
	if !c.config.Passkey.Enabled {
		c.logger.Warn("passkey authentication attempted but passkeys are disabled")
		http.Error(w, "Passkeys are not enabled", http.StatusForbidden)
		return
	}

	// Parse request body
	var req PasskeyAuthenticateBeginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.logger.Errorf("failed to parse request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Begin passkey authentication (email is optional for discoverable credentials)
	ctx := r.Context()
	session, options, err := c.BeginPasskeyAuthentication(ctx, req.Email)
	if err != nil {
		c.logger.Errorf("failed to begin passkey authentication: %v", err)

		// Provide more specific error messages
		if err.Error() == "user not found: user not found" {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to begin authentication", http.StatusInternalServerError)
		}
		return
	}

	// Prepare response
	resp := PasskeyAuthenticateBeginResponse{
		SessionID: session.SessionID,
		Options:   options,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Errorf("failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	c.logger.Infof("passkey authentication begin successful (session: %s)", session.SessionID)
}

// PasskeyAuthenticateFinishRequest represents the request body for completing passkey authentication.
type PasskeyAuthenticateFinishRequest struct {
	// SessionID is the session identifier from the begin call
	SessionID string `json:"session_id"`

	// Credential contains the credential assertion response from navigator.credentials.get()
	Credential json.RawMessage `json:"credential"`
}

// PasskeyAuthenticateFinishResponse represents the response from completing passkey authentication.
type PasskeyAuthenticateFinishResponse struct {
	// Success indicates whether the authentication was successful
	Success bool `json:"success"`

	// UserID is the ID of the authenticated user
	UserID string `json:"user_id,omitempty"`

	// Email is the email of the authenticated user
	Email string `json:"email,omitempty"`

	// Message provides additional information
	Message string `json:"message,omitempty"`
}

// handlePasskeyLoginFinish completes passkey authentication.
//
// This endpoint:
// 1. Validates the request and session
// 2. Parses the credential assertion response from the browser
// 3. Calls FinishPasskeyAuthentication to verify signature and authenticate
// 4. Returns success with user information
//
// Request body:
//
//	{
//	  "session_id": "base64-session-id",
//	  "credential": { ... PublicKeyCredential assertion object ... }
//	}
//
// Response:
//
//	{
//	  "success": true,
//	  "user_id": "user-id",
//	  "email": "user@example.com",
//	  "message": "Authentication successful"
//	}
func (c *Connector) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if passkeys are enabled
	if !c.config.Passkey.Enabled {
		c.logger.Warn("passkey authentication finish attempted but passkeys are disabled")
		http.Error(w, "Passkeys are not enabled", http.StatusForbidden)
		return
	}

	// Parse request body
	var req PasskeyAuthenticateFinishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.logger.Errorf("failed to parse request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if len(req.Credential) == 0 {
		http.Error(w, "credential is required", http.StatusBadRequest)
		return
	}

	// Parse credential assertion response
	ctx := r.Context()
	parsedResponse, err := c.parseCredentialAssertionResponse(req.Credential)
	if err != nil {
		c.logger.Errorf("failed to parse credential assertion: %v", err)
		http.Error(w, "Invalid credential format", http.StatusBadRequest)
		return
	}

	// Finish passkey authentication
	user, passkey, err := c.FinishPasskeyAuthentication(ctx, req.SessionID, parsedResponse)
	if err != nil {
		c.logger.Errorf("failed to finish passkey authentication: %v", err)

		// Provide more specific error messages
		if err.Error() == "invalid session" || err.Error() == "session expired" {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		} else if err.Error() == "authenticator clone detected" {
			http.Error(w, "Security error: authenticator clone detected", http.StatusForbidden)
		} else if err.Error() == "credential not found: passkey not found" {
			http.Error(w, "Credential not found", http.StatusNotFound)
		} else {
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
		}
		return
	}

	// Prepare response
	resp := PasskeyAuthenticateFinishResponse{
		Success: true,
		UserID:  user.ID,
		Email:   user.Email,
		Message: "Authentication successful",
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Errorf("failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	c.logger.Infof("passkey authentication complete for user %s (passkey: %s)", user.ID, passkey.ID)
}

// PasskeyRegisterBeginRequest represents the request body for beginning passkey registration.
type PasskeyRegisterBeginRequest struct {
	// UserID is the ID of the user registering the passkey
	UserID string `json:"user_id"`
}

// PasskeyRegisterBeginResponse represents the response from beginning passkey registration.
type PasskeyRegisterBeginResponse struct {
	// SessionID is the session identifier to be used in the finish call
	SessionID string `json:"session_id"`

	// Options contains the PublicKeyCredentialCreationOptions to be passed
	// to navigator.credentials.create() in the browser
	Options interface{} `json:"options"`
}

// handlePasskeyRegisterBegin begins passkey registration.
//
// This endpoint:
// 1. Validates the request and checks if passkeys are enabled
// 2. Retrieves the user from storage
// 3. Calls BeginPasskeyRegistration to generate challenge and options
// 4. Creates a WebAuthn session with 5-minute TTL
// 5. Returns the session ID and PublicKeyCredentialCreationOptions
//
// Request body:
//
//	{
//	  "user_id": "user-uuid"
//	}
//
// Response:
//
//	{
//	  "session_id": "base64-session-id",
//	  "options": {
//	    "publicKey": {
//	      "challenge": "base64-challenge",
//	      "rp": { "name": "Enopax", "id": "auth.enopax.io" },
//	      "user": { "id": "base64-user-id", "name": "user@example.com", "displayName": "User" },
//	      "pubKeyCredParams": [...],
//	      "timeout": 60000,
//	      "authenticatorSelection": {...},
//	      "attestation": "none"
//	    }
//	  }
//	}
func (c *Connector) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if passkeys are enabled
	if !c.config.Passkey.Enabled {
		c.logger.Warn("passkey registration attempted but passkeys are disabled")
		http.Error(w, "Passkeys are not enabled", http.StatusForbidden)
		return
	}

	// Parse request body
	var req PasskeyRegisterBeginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.logger.Errorf("failed to parse request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate user ID
	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	// Get user from storage
	ctx := r.Context()
	user, err := c.storage.GetUser(ctx, req.UserID)
	if err != nil {
		c.logger.Errorf("failed to get user %s: %v", req.UserID, err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Begin passkey registration
	session, options, err := c.BeginPasskeyRegistration(ctx, user)
	if err != nil {
		c.logger.Errorf("failed to begin passkey registration for user %s: %v", user.ID, err)
		http.Error(w, "Failed to begin registration", http.StatusInternalServerError)
		return
	}

	// Prepare response
	resp := PasskeyRegisterBeginResponse{
		SessionID: session.SessionID,
		Options:   options,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Errorf("failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	c.logger.Infof("passkey registration begin successful for user %s (session: %s)", user.ID, session.SessionID)
}

// PasskeyRegisterFinishRequest represents the request body for completing passkey registration.
type PasskeyRegisterFinishRequest struct {
	// SessionID is the session identifier from the begin call
	SessionID string `json:"session_id"`

	// Credential contains the credential creation response from navigator.credentials.create()
	Credential json.RawMessage `json:"credential"`

	// PasskeyName is the user-friendly name for this passkey (e.g., "MacBook Touch ID")
	PasskeyName string `json:"passkey_name"`
}

// PasskeyRegisterFinishResponse represents the response from completing passkey registration.
type PasskeyRegisterFinishResponse struct {
	// Success indicates whether the registration was successful
	Success bool `json:"success"`

	// PasskeyID is the ID of the newly created passkey
	PasskeyID string `json:"passkey_id,omitempty"`

	// Message provides additional information
	Message string `json:"message,omitempty"`
}

// handlePasskeyRegisterFinish completes passkey registration.
//
// This endpoint:
// 1. Validates the request and session
// 2. Parses the credential creation response from the browser
// 3. Calls FinishPasskeyRegistration to verify and store the credential
// 4. Returns success with the passkey ID
//
// Request body:
//
//	{
//	  "session_id": "base64-session-id",
//	  "credential": { ... PublicKeyCredential object ... },
//	  "passkey_name": "MacBook Touch ID"
//	}
//
// Response:
//
//	{
//	  "success": true,
//	  "passkey_id": "passkey-id",
//	  "message": "Passkey registered successfully"
//	}
func (c *Connector) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if passkeys are enabled
	if !c.config.Passkey.Enabled {
		c.logger.Warn("passkey registration finish attempted but passkeys are disabled")
		http.Error(w, "Passkeys are not enabled", http.StatusForbidden)
		return
	}

	// Parse request body
	var req PasskeyRegisterFinishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.logger.Errorf("failed to parse request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.SessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	if len(req.Credential) == 0 {
		http.Error(w, "credential is required", http.StatusBadRequest)
		return
	}
	if req.PasskeyName == "" {
		http.Error(w, "passkey_name is required", http.StatusBadRequest)
		return
	}

	// Parse credential from JSON - we need to import the protocol package
	// The credential comes from the browser as a PublicKeyCredential object
	// which we need to parse into the format expected by go-webauthn
	ctx := r.Context()
	parsedResponse, err := c.parseCredentialCreationResponse(req.Credential)
	if err != nil {
		c.logger.Errorf("failed to parse credential: %v", err)
		http.Error(w, "Invalid credential format", http.StatusBadRequest)
		return
	}

	// Finish passkey registration
	passkey, err := c.FinishPasskeyRegistration(ctx, req.SessionID, parsedResponse, req.PasskeyName)
	if err != nil {
		c.logger.Errorf("failed to finish passkey registration: %v", err)

		// Provide more specific error messages
		if err.Error() == "invalid session" || err.Error() == "session expired" {
			http.Error(w, err.Error(), http.StatusUnauthorized)
		} else if err.Error() == "credential verification failed" {
			http.Error(w, "Credential verification failed", http.StatusBadRequest)
		} else {
			http.Error(w, "Failed to complete registration", http.StatusInternalServerError)
		}
		return
	}

	// Prepare response
	resp := PasskeyRegisterFinishResponse{
		Success:   true,
		PasskeyID: passkey.ID,
		Message:   "Passkey registered successfully",
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Errorf("failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	c.logger.Infof("passkey registration complete for passkey %s (name: %s)", passkey.ID, passkey.Name)
}

// handleTOTPVerify verifies a TOTP code.
func (c *Connector) handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 3 Week 8
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleTOTPEnable enables TOTP for a user.
func (c *Connector) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 3 Week 8
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleTOTPValidate validates a TOTP code during login.
func (c *Connector) handleTOTPValidate(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 3 Week 9
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleMagicLinkSend sends a magic link to the user's email.
func (c *Connector) handleMagicLinkSend(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 4 Week 10
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleMagicLinkVerify verifies a magic link token.
func (c *Connector) handleMagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 4 Week 10
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleAuthSetup handles the auth setup flow.
func (c *Connector) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 6 Week 12
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// Helper functions

// parseCredentialCreationResponse parses the credential creation response from the browser.
//
// The browser sends a PublicKeyCredential object which contains the attestation response.
// This function uses the go-webauthn protocol package to parse it into the expected format.
func (c *Connector) parseCredentialCreationResponse(credentialJSON json.RawMessage) (*protocol.ParsedCredentialCreationData, error) {
	// Parse the credential using the protocol package
	// The protocol package provides a ParseCredentialCreationResponse function
	// that handles all the base64 decoding and validation
	// We need to convert json.RawMessage to io.Reader first
	reader := bytes.NewReader(credentialJSON)
	ccr, err := protocol.ParseCredentialCreationResponseBody(reader)
	if err != nil {
		return nil, err
	}

	return ccr, nil
}

// parseCredentialAssertionResponse parses the credential assertion response from the browser.
//
// The browser sends a PublicKeyCredential object which contains the assertion response.
// This function uses the go-webauthn protocol package to parse it into the expected format.
func (c *Connector) parseCredentialAssertionResponse(credentialJSON json.RawMessage) (*protocol.ParsedCredentialAssertionData, error) {
	// Parse the credential using the protocol package
	// The protocol package provides a ParseCredentialRequestResponse function
	// that handles all the base64 decoding and validation
	reader := bytes.NewReader(credentialJSON)
	car, err := protocol.ParseCredentialRequestResponseBody(reader)
	if err != nil {
		return nil, err
	}

	return car, nil
}
