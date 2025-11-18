package local

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	// ErrUserNotFound is returned when a user is not found.
	ErrUserNotFound = errors.New("user not found")

	// ErrUserAlreadyExists is returned when attempting to create a user that already exists.
	ErrUserAlreadyExists = errors.New("user already exists")

	// ErrPasskeyNotFound is returned when a passkey is not found.
	ErrPasskeyNotFound = errors.New("passkey not found")

	// ErrSessionNotFound is returned when a session is not found.
	ErrSessionNotFound = errors.New("session not found")

	// ErrTokenNotFound is returned when a token is not found.
	ErrTokenNotFound = errors.New("token not found")

	// Err2FASessionNotFound is returned when a 2FA session is not found.
	Err2FASessionNotFound = errors.New("2FA session not found")
)

// Storage defines the interface for user and credential storage.
type Storage interface {
	// User operations
	CreateUser(ctx context.Context, user *User) error
	GetUser(ctx context.Context, userID string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, userID string) error
	ListUsers(ctx context.Context) ([]*User, error)

	// Passkey operations
	SavePasskey(ctx context.Context, passkey *Passkey) error
	GetPasskey(ctx context.Context, credentialID string) (*Passkey, error)
	ListPasskeys(ctx context.Context, userID string) ([]*Passkey, error)
	DeletePasskey(ctx context.Context, credentialID string) error

	// WebAuthn session operations
	SaveWebAuthnSession(ctx context.Context, session *WebAuthnSession) error
	GetWebAuthnSession(ctx context.Context, sessionID string) (*WebAuthnSession, error)
	DeleteWebAuthnSession(ctx context.Context, sessionID string) error

	// Magic link token operations
	SaveMagicLinkToken(ctx context.Context, token *MagicLinkToken) error
	GetMagicLinkToken(ctx context.Context, token string) (*MagicLinkToken, error)
	DeleteMagicLinkToken(ctx context.Context, token string) error

	// 2FA session operations
	Save2FASession(ctx context.Context, session *TwoFactorSession) error
	Get2FASession(ctx context.Context, sessionID string) (*TwoFactorSession, error)
	Delete2FASession(ctx context.Context, sessionID string) error

	// Auth setup token operations
	SaveAuthSetupToken(ctx context.Context, token *AuthSetupToken) error
	GetAuthSetupToken(ctx context.Context, token string) (*AuthSetupToken, error)
	DeleteAuthSetupToken(ctx context.Context, token string) error

	// Cleanup operations
	CleanupExpiredSessions(ctx context.Context) error
	CleanupExpiredTokens(ctx context.Context) error
}

// FileStorage implements the Storage interface using file-based storage.
type FileStorage struct {
	dataDir string
	mu      sync.RWMutex
}

// NewFileStorage creates a new file-based storage backend.
func NewFileStorage(dataDir string) (*FileStorage, error) {
	// Create directory structure
	dirs := []string{
		filepath.Join(dataDir, "users"),
		filepath.Join(dataDir, "passkeys"),
		filepath.Join(dataDir, "sessions"),
		filepath.Join(dataDir, "tokens"),
		filepath.Join(dataDir, "2fa-sessions"),
		filepath.Join(dataDir, "auth-setup-tokens"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &FileStorage{
		dataDir: dataDir,
	}, nil
}

// User operations

// CreateUser creates a new user.
func (s *FileStorage) CreateUser(ctx context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if user already exists
	userPath := filepath.Join(s.dataDir, "users", user.ID+".json")
	if _, err := os.Stat(userPath); err == nil {
		return ErrUserAlreadyExists
	}

	// Set timestamps
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	// Write user file
	return s.writeFile(userPath, user)
}

// GetUser retrieves a user by ID.
func (s *FileStorage) GetUser(ctx context.Context, userID string) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userPath := filepath.Join(s.dataDir, "users", userID+".json")
	var user User
	if err := s.readFile(userPath, &user); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByEmail retrieves a user by email address.
func (s *FileStorage) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	// Generate deterministic user ID from email
	userID := generateUserID(email)
	// Call GetUser directly - it will acquire its own lock
	return s.GetUser(ctx, userID)
}

// UpdateUser updates an existing user.
func (s *FileStorage) UpdateUser(ctx context.Context, user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userPath := filepath.Join(s.dataDir, "users", user.ID+".json")

	// Check if user exists
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		return ErrUserNotFound
	}

	// Update timestamp
	user.UpdatedAt = time.Now()

	// Write updated user file
	return s.writeFile(userPath, user)
}

// DeleteUser deletes a user.
func (s *FileStorage) DeleteUser(ctx context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	userPath := filepath.Join(s.dataDir, "users", userID+".json")

	// Check if user exists
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		return ErrUserNotFound
	}

	// Delete user file
	if err := os.Remove(userPath); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// TODO: Delete associated passkeys, sessions, etc.

	return nil
}

// ListUsers returns all users.
func (s *FileStorage) ListUsers(ctx context.Context) ([]*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	usersDir := filepath.Join(s.dataDir, "users")
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read users directory: %w", err)
	}

	var users []*User
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		var user User
		userPath := filepath.Join(usersDir, entry.Name())
		if err := s.readFile(userPath, &user); err != nil {
			// Skip invalid user files
			continue
		}
		users = append(users, &user)
	}

	return users, nil
}

// Passkey operations

// SavePasskey saves a passkey credential.
func (s *FileStorage) SavePasskey(ctx context.Context, passkey *Passkey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	passkeyPath := filepath.Join(s.dataDir, "passkeys", passkey.ID+".json")
	return s.writeFile(passkeyPath, passkey)
}

// GetPasskey retrieves a passkey by credential ID.
func (s *FileStorage) GetPasskey(ctx context.Context, credentialID string) (*Passkey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	passkeyPath := filepath.Join(s.dataDir, "passkeys", credentialID+".json")
	var passkey Passkey
	if err := s.readFile(passkeyPath, &passkey); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPasskeyNotFound
		}
		return nil, err
	}

	return &passkey, nil
}

// ListPasskeys returns all passkeys for a user.
func (s *FileStorage) ListPasskeys(ctx context.Context, userID string) ([]*Passkey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	passkeysDir := filepath.Join(s.dataDir, "passkeys")
	entries, err := os.ReadDir(passkeysDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read passkeys directory: %w", err)
	}

	var passkeys []*Passkey
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		var passkey Passkey
		passkeyPath := filepath.Join(passkeysDir, entry.Name())
		if err := s.readFile(passkeyPath, &passkey); err != nil {
			continue
		}

		if passkey.UserID == userID {
			passkeys = append(passkeys, &passkey)
		}
	}

	return passkeys, nil
}

// DeletePasskey deletes a passkey.
func (s *FileStorage) DeletePasskey(ctx context.Context, credentialID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	passkeyPath := filepath.Join(s.dataDir, "passkeys", credentialID+".json")

	if _, err := os.Stat(passkeyPath); os.IsNotExist(err) {
		return ErrPasskeyNotFound
	}

	return os.Remove(passkeyPath)
}

// WebAuthn session operations

// SaveWebAuthnSession saves a WebAuthn session.
func (s *FileStorage) SaveWebAuthnSession(ctx context.Context, session *WebAuthnSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPath := filepath.Join(s.dataDir, "sessions", session.SessionID+".json")
	return s.writeFile(sessionPath, session)
}

// GetWebAuthnSession retrieves a WebAuthn session.
func (s *FileStorage) GetWebAuthnSession(ctx context.Context, sessionID string) (*WebAuthnSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionPath := filepath.Join(s.dataDir, "sessions", sessionID+".json")
	var session WebAuthnSession
	if err := s.readFile(sessionPath, &session); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, ErrSessionNotFound
	}

	return &session, nil
}

// DeleteWebAuthnSession deletes a WebAuthn session.
func (s *FileStorage) DeleteWebAuthnSession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPath := filepath.Join(s.dataDir, "sessions", sessionID+".json")
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil // Already deleted
	}

	return os.Remove(sessionPath)
}

// Magic link token operations

// SaveMagicLinkToken saves a magic link token.
func (s *FileStorage) SaveMagicLinkToken(ctx context.Context, token *MagicLinkToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenPath := filepath.Join(s.dataDir, "tokens", token.Token+".json")
	return s.writeFile(tokenPath, token)
}

// GetMagicLinkToken retrieves a magic link token.
func (s *FileStorage) GetMagicLinkToken(ctx context.Context, token string) (*MagicLinkToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokenPath := filepath.Join(s.dataDir, "tokens", token+".json")
	var magicToken MagicLinkToken
	if err := s.readFile(tokenPath, &magicToken); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	// Check if token is expired
	if time.Now().After(magicToken.ExpiresAt) {
		return nil, ErrTokenNotFound
	}

	return &magicToken, nil
}

// DeleteMagicLinkToken deletes a magic link token.
func (s *FileStorage) DeleteMagicLinkToken(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenPath := filepath.Join(s.dataDir, "tokens", token+".json")
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		return nil // Already deleted
	}

	return os.Remove(tokenPath)
}

// 2FA session operations

// Save2FASession saves a 2FA session.
func (s *FileStorage) Save2FASession(ctx context.Context, session *TwoFactorSession) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPath := filepath.Join(s.dataDir, "2fa-sessions", session.SessionID+".json")
	return s.writeFile(sessionPath, session)
}

// Get2FASession retrieves a 2FA session.
func (s *FileStorage) Get2FASession(ctx context.Context, sessionID string) (*TwoFactorSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionPath := filepath.Join(s.dataDir, "2fa-sessions", sessionID+".json")
	var session TwoFactorSession
	if err := s.readFile(sessionPath, &session); err != nil {
		if os.IsNotExist(err) {
			return nil, Err2FASessionNotFound
		}
		return nil, err
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, Err2FASessionNotFound
	}

	return &session, nil
}

// Delete2FASession deletes a 2FA session.
func (s *FileStorage) Delete2FASession(ctx context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionPath := filepath.Join(s.dataDir, "2fa-sessions", sessionID+".json")
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil // Already deleted
	}

	return os.Remove(sessionPath)
}

// Cleanup operations

// CleanupExpiredSessions removes expired WebAuthn sessions and 2FA sessions.
func (s *FileStorage) CleanupExpiredSessions(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Clean up WebAuthn sessions
	sessionsDir := filepath.Join(s.dataDir, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return fmt.Errorf("failed to read sessions directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(sessionsDir, entry.Name())
		var session WebAuthnSession
		if err := s.readFile(sessionPath, &session); err != nil {
			continue
		}

		if now.After(session.ExpiresAt) {
			os.Remove(sessionPath)
		}
	}

	// Clean up 2FA sessions
	twoFASessionsDir := filepath.Join(s.dataDir, "2fa-sessions")
	entries2FA, err := os.ReadDir(twoFASessionsDir)
	if err != nil {
		return fmt.Errorf("failed to read 2fa-sessions directory: %w", err)
	}

	for _, entry := range entries2FA {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionPath := filepath.Join(twoFASessionsDir, entry.Name())
		var session TwoFactorSession
		if err := s.readFile(sessionPath, &session); err != nil {
			continue
		}

		if now.After(session.ExpiresAt) {
			os.Remove(sessionPath)
		}
	}

	return nil
}

// CleanupExpiredTokens removes expired magic link tokens.
func (s *FileStorage) CleanupExpiredTokens(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokensDir := filepath.Join(s.dataDir, "tokens")
	entries, err := os.ReadDir(tokensDir)
	if err != nil {
		return fmt.Errorf("failed to read tokens directory: %w", err)
	}

	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		tokenPath := filepath.Join(tokensDir, entry.Name())
		var token MagicLinkToken
		if err := s.readFile(tokenPath, &token); err != nil {
			continue
		}

		if now.After(token.ExpiresAt) {
			os.Remove(tokenPath)
		}
	}

	return nil
}

// Helper functions

// writeFile writes data to a file with proper locking.
func (s *FileStorage) writeFile(path string, data interface{}) error {
	// Marshal data to JSON
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	// Create temporary file
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Lock file for exclusive access
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to lock file: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Write data
	if _, err := f.Write(jsonData); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Sync to disk
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Close file
	f.Close()

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// readFile reads data from a file.
func (s *FileStorage) readFile(path string, data interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Lock file for shared access
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_SH); err != nil {
		return fmt.Errorf("failed to lock file: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Decode JSON
	if err := json.NewDecoder(f).Decode(data); err != nil {
		return fmt.Errorf("failed to decode file: %w", err)
	}

	return nil
}

// generateUserID generates a deterministic user ID from an email address.
func generateUserID(email string) string {
	hash := sha256.Sum256([]byte(email))
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
}

// GenerateSecureToken generates a cryptographically secure random token.
func GenerateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateSessionID generates a cryptographically secure session ID.
func GenerateSessionID() (string, error) {
	return GenerateSecureToken()
}

// Auth setup token operations

// SaveAuthSetupToken saves an auth setup token.
func (s *FileStorage) SaveAuthSetupToken(ctx context.Context, token *AuthSetupToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenPath := filepath.Join(s.dataDir, "auth-setup-tokens", token.Token+".json")
	return s.writeFile(tokenPath, token)
}

// GetAuthSetupToken retrieves an auth setup token.
func (s *FileStorage) GetAuthSetupToken(ctx context.Context, token string) (*AuthSetupToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tokenPath := filepath.Join(s.dataDir, "auth-setup-tokens", token+".json")
	var setupToken AuthSetupToken
	if err := s.readFile(tokenPath, &setupToken); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTokenNotFound
		}
		return nil, err
	}

	return &setupToken, nil
}

// DeleteAuthSetupToken deletes an auth setup token.
func (s *FileStorage) DeleteAuthSetupToken(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokenPath := filepath.Join(s.dataDir, "auth-setup-tokens", token+".json")
	if err := os.Remove(tokenPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
