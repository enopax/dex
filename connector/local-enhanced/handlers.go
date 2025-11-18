package local

import (
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

// handlePasskeyRegisterBegin begins passkey registration.
func (c *Connector) handlePasskeyRegisterBegin(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement in Phase 2 Week 6
	http.Error(w, "Not implemented", http.StatusNotImplemented)
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
