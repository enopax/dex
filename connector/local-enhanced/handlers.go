package local

import (
	"encoding/json"
	"net/http"
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

// handlePasskeyLoginBegin begins passkey authentication.
func (c *Connector) handlePasskeyLoginBegin(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 2 Week 7
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handlePasskeyLoginFinish completes passkey authentication.
func (c *Connector) handlePasskeyLoginFinish(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 2 Week 7
	http.Error(w, "Not implemented", http.StatusNotImplemented)
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

// handlePasskeyRegisterFinish completes passkey registration.
func (c *Connector) handlePasskeyRegisterFinish(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 2 Week 6
	http.Error(w, "Not implemented", http.StatusNotImplemented)
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
