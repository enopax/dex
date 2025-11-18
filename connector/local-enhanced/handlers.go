package local

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/go-webauthn/webauthn/protocol"
)

// HTTP handler stubs - to be implemented in subsequent phases

// handleLogin displays the login page with passkey and password options.
//
// This handler is called when Dex redirects the user to our connector for authentication.
// The state parameter contains the auth request ID from Dex, which must be preserved
// throughout the authentication flow.
//
// The login page should:
// 1. Display passkey login button (if enabled)
// 2. Display password login form (if enabled)
// 3. Call the passkey/password authentication endpoints via JavaScript
// 4. After successful authentication, redirect to the callback URL with user_id
func (c *Connector) handleLogin(w http.ResponseWriter, r *http.Request) {
	// Get state and callback URL from query parameters
	state := r.URL.Query().Get("state")
	callbackURL := r.URL.Query().Get("callback")

	if state == "" {
		c.logger.Error("handleLogin: missing state parameter")
		http.Error(w, "Missing state parameter", http.StatusBadRequest)
		return
	}

	if callbackURL == "" {
		c.logger.Error("handleLogin: missing callback parameter")
		http.Error(w, "Missing callback parameter", http.StatusBadRequest)
		return
	}

	// For now, render a simple login page
	// In Phase 2 Week 7, this will use the actual HTML templates
	c.logger.Infof("handleLogin: displaying login page (state: %s)", state)

	// TODO: Render the actual login template from templates/login.html
	// For now, just show a placeholder
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
	<title>Login - Enopax</title>
</head>
<body>
	<h1>Login to Enopax</h1>
	<p>This page will display passkey and password login options.</p>
	<p>State: ` + state + `</p>
	<p>Callback: ` + callbackURL + `</p>
	<p>Implementation in progress...</p>
</body>
</html>`))
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

// TOTPEnableBeginRequest represents the request body for beginning TOTP setup.
type TOTPEnableBeginRequest struct {
	// UserID is the ID of the user setting up TOTP
	UserID string `json:"user_id"`
}

// TOTPEnableBeginResponse represents the response from beginning TOTP setup.
type TOTPEnableBeginResponse struct {
	// Secret is the TOTP secret (base32 encoded)
	Secret string `json:"secret"`

	// QRCodeDataURL is the data URL for the QR code image
	QRCodeDataURL string `json:"qr_code_data_url"`

	// BackupCodes is the list of backup codes (shown only once)
	BackupCodes []string `json:"backup_codes"`

	// URL is the otpauth:// URL for manual entry
	URL string `json:"url"`
}

// handleTOTPEnable begins TOTP setup for a user.
//
// This endpoint:
// 1. Validates the request
// 2. Retrieves the user
// 3. Calls BeginTOTPSetup to generate secret and QR code
// 4. Returns the setup information
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
//	  "secret": "base32-encoded-secret",
//	  "qr_code_data_url": "data:image/png;base64,...",
//	  "backup_codes": ["CODE1234", "CODE5678", ...],
//	  "url": "otpauth://totp/Enopax:user@example.com?secret=..."
//	}
func (c *Connector) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req TOTPEnableBeginRequest
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

	// Get user
	ctx := r.Context()
	user, err := c.storage.GetUser(ctx, req.UserID)
	if err != nil {
		c.logger.Errorf("failed to get user: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Check if TOTP is already enabled
	if user.TOTPEnabled {
		http.Error(w, "TOTP is already enabled for this user", http.StatusBadRequest)
		return
	}

	// Begin TOTP setup
	setup, err := c.BeginTOTPSetup(ctx, user)
	if err != nil {
		c.logger.Errorf("failed to begin TOTP setup: %v", err)
		http.Error(w, "Failed to begin TOTP setup", http.StatusInternalServerError)
		return
	}

	// Prepare response
	resp := TOTPEnableBeginResponse{
		Secret:        setup.Secret,
		QRCodeDataURL: setup.QRCodeDataURL,
		BackupCodes:   setup.BackupCodes,
		URL:           setup.URL,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Errorf("failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	c.logger.Infof("TOTP setup initiated for user %s", user.ID)
}

// TOTPVerifyRequest represents the request body for verifying TOTP setup.
type TOTPVerifyRequest struct {
	// UserID is the ID of the user completing TOTP setup
	UserID string `json:"user_id"`

	// Secret is the TOTP secret from the enable endpoint
	Secret string `json:"secret"`

	// Code is the TOTP code from the user's authenticator app
	Code string `json:"code"`

	// BackupCodes is the list of backup codes from the enable endpoint
	BackupCodes []string `json:"backup_codes"`
}

// TOTPVerifyResponse represents the response from verifying TOTP setup.
type TOTPVerifyResponse struct {
	// Success indicates whether TOTP was enabled successfully
	Success bool `json:"success"`

	// Message provides additional information
	Message string `json:"message,omitempty"`
}

// handleTOTPVerify completes TOTP setup by verifying the user's TOTP code.
//
// This endpoint:
// 1. Validates the request
// 2. Retrieves the user
// 3. Calls FinishTOTPSetup to verify the code and enable TOTP
// 4. Returns success
//
// Request body:
//
//	{
//	  "user_id": "user-uuid",
//	  "secret": "base32-encoded-secret",
//	  "code": "123456",
//	  "backup_codes": ["CODE1234", "CODE5678", ...]
//	}
//
// Response:
//
//	{
//	  "success": true,
//	  "message": "TOTP enabled successfully"
//	}
func (c *Connector) handleTOTPVerify(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req TOTPVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.logger.Errorf("failed to parse request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if req.Secret == "" {
		http.Error(w, "secret is required", http.StatusBadRequest)
		return
	}
	if req.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}
	if len(req.BackupCodes) == 0 {
		http.Error(w, "backup_codes is required", http.StatusBadRequest)
		return
	}

	// Get user
	ctx := r.Context()
	user, err := c.storage.GetUser(ctx, req.UserID)
	if err != nil {
		c.logger.Errorf("failed to get user: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Finish TOTP setup
	if err := c.FinishTOTPSetup(ctx, user, req.Secret, req.Code, req.BackupCodes); err != nil {
		c.logger.Errorf("failed to finish TOTP setup: %v", err)

		// Provide specific error messages
		if err.Error() == "invalid TOTP code" {
			http.Error(w, "Invalid TOTP code", http.StatusBadRequest)
		} else {
			http.Error(w, "Failed to enable TOTP", http.StatusInternalServerError)
		}
		return
	}

	// Prepare response
	resp := TOTPVerifyResponse{
		Success: true,
		Message: "TOTP enabled successfully",
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Errorf("failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	c.logger.Infof("TOTP enabled for user %s", user.ID)
}

// TOTPValidateRequest represents the request body for validating a TOTP code during login.
type TOTPValidateRequest struct {
	// UserID is the ID of the user being authenticated
	UserID string `json:"user_id"`

	// Code is the TOTP code from the user's authenticator app or a backup code
	Code string `json:"code"`
}

// TOTPValidateResponse represents the response from validating a TOTP code.
type TOTPValidateResponse struct {
	// Valid indicates whether the TOTP code is valid
	Valid bool `json:"valid"`

	// Message provides additional information
	Message string `json:"message,omitempty"`
}

// handleTOTPValidate validates a TOTP code during login (2FA flow).
//
// This endpoint:
// 1. Validates the request
// 2. Retrieves the user
// 3. Attempts to validate as TOTP code first
// 4. If TOTP fails, attempts to validate as backup code
// 5. Returns validation result
//
// Request body:
//
//	{
//	  "user_id": "user-uuid",
//	  "code": "123456"  // or backup code
//	}
//
// Response:
//
//	{
//	  "valid": true,
//	  "message": "TOTP code verified"
//	}
func (c *Connector) handleTOTPValidate(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req TOTPValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		c.logger.Errorf("failed to parse request body: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.UserID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}
	if req.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	// Get user
	ctx := r.Context()
	user, err := c.storage.GetUser(ctx, req.UserID)
	if err != nil {
		c.logger.Errorf("failed to get user: %v", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	var valid bool
	var message string

	// Try TOTP validation first
	valid, err = c.ValidateTOTP(ctx, user, req.Code)
	if err == nil && valid {
		message = "TOTP code verified"
	} else {
		// If TOTP validation fails, try backup code
		valid, err = c.ValidateBackupCode(ctx, user, req.Code)
		if err == nil && valid {
			message = "Backup code verified"
		} else {
			message = "Invalid code"
		}
	}

	// Prepare response
	resp := TOTPValidateResponse{
		Valid:   valid,
		Message: message,
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		c.logger.Errorf("failed to encode response: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	if valid {
		c.logger.Infof("TOTP validation successful for user %s", user.ID)
	} else {
		c.logger.Warnf("TOTP validation failed for user %s", user.ID)
	}
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
