# Coding Standards and Conventions

**Project**: Dex Enhanced Local Connector
**Last Updated**: 2025-11-18
**Version**: 1.0

---

## Table of Contents

1. [General Principles](#general-principles)
2. [Go Code Style](#go-code-style)
3. [File Organization](#file-organization)
4. [Naming Conventions](#naming-conventions)
5. [Error Handling](#error-handling)
6. [Logging](#logging)
7. [Testing Standards](#testing-standards)
8. [Security Guidelines](#security-guidelines)
9. [Documentation Requirements](#documentation-requirements)
10. [Commit Message Guidelines](#commit-message-guidelines)

---

## General Principles

### Code Quality Goals

1. **Readability First** - Code is read more often than it's written
2. **Simplicity** - Prefer simple solutions over clever ones
3. **Consistency** - Follow established patterns in the codebase
4. **Testability** - Write code that's easy to test
5. **Security** - Security is not optional, it's required

### Go Proverbs to Live By

```
- Clear is better than clever
- Don't panic
- Errors are values
- A little copying is better than a little dependency
- The bigger the interface, the weaker the abstraction
- Make the zero value useful
- interface{} says nothing
- Gofmt's style is no one's favorite, yet gofmt is everyone's favorite
```

Source: [Go Proverbs](https://go-proverbs.github.io/)

---

## Go Code Style

### 1. Code Formatting

**Always use `gofmt`**:
```bash
# Format all Go files
go fmt ./...

# Or use make target
make fmt
```

**Linting**:
```bash
# Run golangci-lint
make lint

# Auto-fix issues
make fix
```

### 2. Import Organization

**Order**: Standard library → External packages → Internal packages

```go
import (
    // Standard library (alphabetical order)
    "context"
    "crypto/rand"
    "encoding/json"
    "fmt"
    "time"

    // External dependencies (alphabetical order)
    "github.com/go-webauthn/webauthn/webauthn"
    "github.com/go-webauthn/webauthn/protocol"
    "github.com/golang-jwt/jwt/v5"
    "github.com/pquerna/otp/totp"

    // Internal packages (alphabetical order)
    "github.com/dexidp/dex/connector"
    "github.com/dexidp/dex/pkg/log"
    "github.com/dexidp/dex/storage"
)
```

**Formatting**:
- Group imports with blank lines
- No dot imports (except for tests with Ginkgo/Gomega if used)
- No underscore imports unless necessary for side effects

### 3. Line Length

- **Soft limit**: 100 characters
- **Hard limit**: 120 characters
- Break long lines at logical points

```go
// Good - breaks at logical points
func (c *Connector) BeginPasskeyRegistration(
    ctx context.Context,
    user *User,
) (*WebAuthnSession, *protocol.CredentialCreation, error) {
    // Implementation
}

// Bad - exceeds 120 characters
func (c *Connector) BeginPasskeyRegistration(ctx context.Context, user *User) (*WebAuthnSession, *protocol.CredentialCreation, error) {
    // Implementation
}
```

### 4. Function Length

- **Target**: < 50 lines per function
- **Maximum**: 100 lines (rare exceptions)
- If a function is too long, extract helper functions

```go
// Good - focused function
func (c *Connector) HandleLogin(w http.ResponseWriter, r *http.Request) {
    creds, err := c.parseCredentials(r)
    if err != nil {
        c.handleError(w, err)
        return
    }

    user, err := c.authenticate(ctx, creds)
    if err != nil {
        c.handleError(w, err)
        return
    }

    c.createSession(w, user)
}

// Bad - doing too much in one function
func (c *Connector) HandleLogin(w http.ResponseWriter, r *http.Request) {
    // 150 lines of mixed parsing, validation, authentication, and response logic
}
```

### 5. Variable Declaration

**Use short variable declarations when possible**:
```go
// Good
user := &User{ID: "123"}
count := len(users)

// Acceptable (when type clarity is needed)
var timeout time.Duration = 5 * time.Minute
```

**Zero values**:
```go
// Good - relies on zero value
var (
    users []User
    count int
)

// Bad - unnecessary initialization
var (
    users []User = []User{}
    count int = 0
)
```

**Declare variables close to usage**:
```go
// Good
func process() error {
    data, err := fetchData()
    if err != nil {
        return err
    }

    // Use data here
    result := transform(data)
    return save(result)
}

// Bad
func process() error {
    var data Data
    var result Result
    var err error

    data, err = fetchData()
    // ...
}
```

---

## File Organization

### 1. Package Structure

```
connector/local-enhanced/
├── local.go              # Main connector implementation, Connector interface
├── config.go             # Configuration structures and parsing
├── password.go           # Password authentication logic
├── passkey.go            # WebAuthn passkey support
├── totp.go               # TOTP 2FA logic
├── magiclink.go          # Magic link authentication
├── storage.go            # Storage interface definition
├── handlers.go           # HTTP request handlers
├── session.go            # Session management
├── validation.go         # Input validation helpers
├── errors.go             # Custom error types
├── local_test.go         # Main connector tests
├── passkey_test.go       # Passkey-specific tests
├── totp_test.go          # TOTP-specific tests
├── testing.go            # Test helpers and utilities
└── templates/            # HTML templates
    ├── login.html
    ├── setup-auth.html
    └── manage-credentials.html
```

### 2. File Structure

**Standard order**:
```go
// 1. Package declaration and doc comment
// Package local provides an enhanced local authentication connector.
package local

// 2. Imports
import (
    // ...
)

// 3. Constants
const (
    defaultTimeout = 5 * time.Minute
    maxAttempts    = 3
)

// 4. Type definitions
type Connector struct {
    // fields
}

// 5. Constructor functions
func NewConnector(config *Config) (*Connector, error) {
    // implementation
}

// 6. Public methods (alphabetical)
func (c *Connector) BeginAuthentication(...) { }

// 7. Private methods (alphabetical)
func (c *Connector) validateCredentials(...) { }

// 8. Helper functions
func generateChallenge() ([]byte, error) { }
```

### 3. File Size

- **Target**: < 500 lines per file
- **Maximum**: 1000 lines (rare exceptions)
- Split large files into multiple focused files

---

## Naming Conventions

### 1. General Rules

- **Exported names**: Start with uppercase letter
- **Unexported names**: Start with lowercase letter
- **Acronyms**: Use consistent capitalization (e.g., `userID`, `HTTPAPI`, `URLPath`)

### 2. Variables

```go
// Good
userID := "123"
maxRetries := 3
httpClient := &http.Client{}

// Bad
userId := "123"           // Use ID not Id
MAX_RETRIES := 3          // No snake_case
HTTPClient := &http.Client{}  // Should be httpClient if local variable
```

### 3. Functions and Methods

**Use verbs for function names**:
```go
// Good
func CreateUser(user *User) error
func GetPasskey(id string) (*Passkey, error)
func ValidateToken(token string) bool
func BeginRegistration() (*Session, error)

// Bad
func User(user *User) error          // Not descriptive
func Passkey(id string) (*Passkey, error)  // Ambiguous
func Token(token string) bool         // Unclear action
```

**Boolean functions**:
```go
// Good - use Is/Has/Can/Should prefix
func (u *User) IsVerified() bool
func (u *User) HasPasskey() bool
func (c *Connector) CanAuthenticate() bool

// Bad
func (u *User) Verified() bool       // Ambiguous
func (u *User) Passkey() bool        // Could be getter
```

### 4. Types

**Use nouns for type names**:
```go
// Good
type User struct { }
type PasskeyCredential struct { }
type AuthenticationRequest struct { }

// Bad
type UserData struct { }             // "Data" suffix unnecessary
type PasskeyCredentialInfo struct { } // "Info" suffix unnecessary
```

**Interface names**:
```go
// Good - use "-er" suffix for single-method interfaces
type Authenticator interface {
    Authenticate(ctx context.Context) error
}

type Storage interface {
    GetUser(ctx context.Context, id string) (*User, error)
    SaveUser(ctx context.Context, user *User) error
}

// Acceptable - descriptive names for multi-method interfaces
type UserManager interface {
    CreateUser(ctx context.Context, user *User) error
    DeleteUser(ctx context.Context, id string) error
    ListUsers(ctx context.Context) ([]*User, error)
}
```

### 5. Constants

**Use camelCase for const**:
```go
// Good
const (
    defaultTimeout      = 5 * time.Minute
    maxLoginAttempts    = 3
    sessionCookieName   = "dex_session"
)

// Bad
const (
    DEFAULT_TIMEOUT      = 5 * time.Minute  // No SCREAMING_SNAKE_CASE
    Max_Login_Attempts   = 3                // No snake_case
)
```

### 6. File Names

- Use **snake_case** for file names
- Use descriptive names: `passkey.go`, `totp.go`, `magic_link.go`
- Test files: `passkey_test.go`, `totp_test.go`

---

## Error Handling

### 1. Always Handle Errors

```go
// Good
data, err := fetchData()
if err != nil {
    return fmt.Errorf("failed to fetch data: %w", err)
}

// Bad
data, _ := fetchData()  // Never ignore errors
```

### 2. Error Wrapping

**Use `%w` to wrap errors**:
```go
// Good - preserves error chain
if err := validateUser(user); err != nil {
    return fmt.Errorf("user validation failed: %w", err)
}

// Bad - loses error context
if err := validateUser(user); err != nil {
    return fmt.Errorf("user validation failed: %v", err)
}
```

### 3. Custom Errors

**Define custom error types for important errors**:
```go
// errors.go
type AuthenticationError struct {
    Reason string
    UserID string
}

func (e *AuthenticationError) Error() string {
    return fmt.Sprintf("authentication failed for user %s: %s", e.UserID, e.Reason)
}

// Usage
if !valid {
    return &AuthenticationError{
        Reason: "invalid credentials",
        UserID: userID,
    }
}
```

### 4. Error Messages

**Error messages should**:
- Be lowercase (no capitalization)
- Not end with punctuation
- Provide context
- Be actionable when possible

```go
// Good
return fmt.Errorf("failed to parse passkey credential: invalid format")
return fmt.Errorf("user %s not found", userID)
return fmt.Errorf("rate limit exceeded: try again in %d seconds", waitTime)

// Bad
return fmt.Errorf("Error")                        // Too vague
return fmt.Errorf("Failed to parse credential.")  // Capitalized, ends with period
return fmt.Errorf("Something went wrong")         // Not helpful
```

### 5. Sentinel Errors

**Use `var` for sentinel errors**:
```go
var (
    ErrUserNotFound      = errors.New("user not found")
    ErrInvalidCredential = errors.New("invalid credential")
    ErrSessionExpired    = errors.New("session expired")
)

// Usage
user, err := storage.GetUser(ctx, userID)
if errors.Is(err, ErrUserNotFound) {
    // Handle specific error
}
```

---

## Logging

### 1. Log Levels

**Use appropriate log levels**:
```go
// Debug - detailed information for debugging
logger.Debug("challenge generated", "length", len(challenge))

// Info - general information about system operation
logger.Info("user registered passkey", "userID", userID, "credentialID", credID)

// Warn - warning about potential issues
logger.Warn("rate limit approaching", "userID", userID, "attempts", attempts)

// Error - error conditions
logger.Error("passkey verification failed", "error", err, "userID", userID)
```

### 2. Structured Logging

**Use key-value pairs**:
```go
// Good - structured logging
logger.Info("authentication successful",
    "userID", user.ID,
    "method", "passkey",
    "duration", time.Since(start),
)

// Bad - string interpolation
logger.Info(fmt.Sprintf("authentication successful for %s using passkey in %v", user.ID, time.Since(start)))
```

### 3. Never Log Sensitive Data

```go
// NEVER log these:
// - Passwords (even hashed)
// - API keys or secrets
// - Session tokens
// - Private keys
// - Full credit card numbers
// - Personal identification numbers

// Bad - logs sensitive data
logger.Debug("user login", "password", password)           // NO!
logger.Info("created token", "token", token)               // NO!

// Good - logs non-sensitive identifiers
logger.Debug("user login attempt", "userID", userID)       // OK
logger.Info("created token", "tokenID", tokenID[:8])       // OK (partial ID)
```

### 4. Context in Logs

**Provide enough context**:
```go
// Good - includes relevant context
logger.Error("failed to save passkey",
    "error", err,
    "userID", userID,
    "credentialID", credentialID,
    "operation", "registration",
)

// Bad - not enough context
logger.Error("save failed", "error", err)
```

---

## Testing Standards

### 1. Test File Organization

```go
// passkey_test.go
package local

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestBeginPasskeyRegistration(t *testing.T) {
    // Test implementation
}

func TestFinishPasskeyRegistration(t *testing.T) {
    // Test implementation
}
```

### 2. Test Naming

**Function names**: `Test<FunctionName>`
**Table-driven test cases**: `Test<FunctionName>_<Scenario>`

```go
// Good
func TestCreateUser(t *testing.T) { }
func TestCreateUser_DuplicateEmail(t *testing.T) { }
func TestCreateUser_InvalidInput(t *testing.T) { }

// Bad
func TestUserCreation(t *testing.T) { }       // Not clear what's being tested
func Test_create_user(t *testing.T) { }       // Wrong naming convention
```

### 3. Test Structure (AAA Pattern)

**Arrange, Act, Assert**:
```go
func TestBeginPasskeyRegistration(t *testing.T) {
    // Arrange - set up test dependencies and data
    config := DefaultTestConfig(t)
    connector := NewConnector(config)
    user := NewTestUser("test@example.com")

    // Act - execute the function being tested
    session, options, err := connector.BeginPasskeyRegistration(context.Background(), user)

    // Assert - verify the results
    require.NoError(t, err)
    assert.NotNil(t, session)
    assert.Equal(t, 32, len(options.Challenge))
    assert.Equal(t, "auth.enopax.io", options.RPID)
}
```

### 4. Table-Driven Tests

**Use table-driven tests for multiple scenarios**:
```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {
            name:    "valid email",
            email:   "user@example.com",
            wantErr: false,
        },
        {
            name:    "missing @",
            email:   "userexample.com",
            wantErr: true,
        },
        {
            name:    "missing domain",
            email:   "user@",
            wantErr: true,
        },
        {
            name:    "empty email",
            email:   "",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateEmail(tt.email)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 5. Test Coverage

**Requirements**:
- **Minimum**: 80% coverage for all packages
- **Critical paths**: 100% coverage (auth, credential storage)
- **Edge cases**: Test error conditions, boundary values

```bash
# Check coverage
go test -cover ./...

# Generate HTML report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 6. Test Helpers

**Create reusable test helpers** in `testing.go`:
```go
// testing.go
package local

// TestConfig creates a test configuration
func TestConfig(t *testing.T) *Config {
    t.Helper()
    return &Config{
        DataDir: t.TempDir(),
        RPID:    "localhost",
        // ...
    }
}

// NewTestUser creates a test user
func NewTestUser(email string) *User {
    return &User{
        ID:    generateTestUserID(email),
        Email: email,
    }
}
```

### 7. Assertions

**Use `require` for must-pass checks, `assert` for comparisons**:
```go
// Good
user, err := storage.GetUser(ctx, userID)
require.NoError(t, err)           // Test stops if this fails
assert.Equal(t, "alice", user.Username)

// Bad
user, err := storage.GetUser(ctx, userID)
assert.NoError(t, err)            // Test continues even if this fails
assert.Equal(t, "alice", user.Username)  // Will panic if user is nil
```

---

## Security Guidelines

### 1. Cryptographic Operations

**Always use crypto/rand for random values**:
```go
// Good
import "crypto/rand"

func generateChallenge() ([]byte, error) {
    challenge := make([]byte, 32)
    if _, err := rand.Read(challenge); err != nil {
        return nil, fmt.Errorf("failed to generate challenge: %w", err)
    }
    return challenge, nil
}

// Bad
import "math/rand"

func generateChallenge() []byte {
    challenge := make([]byte, 32)
    rand.Read(challenge)  // NOT cryptographically secure!
    return challenge
}
```

### 2. Constant-Time Comparisons

**Use subtle.ConstantTimeCompare for secret comparison**:
```go
import "crypto/subtle"

// Good - prevents timing attacks
func validateToken(provided, expected string) bool {
    providedBytes := []byte(provided)
    expectedBytes := []byte(expected)

    return subtle.ConstantTimeCompare(providedBytes, expectedBytes) == 1
}

// Bad - vulnerable to timing attacks
func validateToken(provided, expected string) bool {
    return provided == expected
}
```

### 3. Input Validation

**Always validate user input**:
```go
// Good
func (c *Connector) GetUser(userID string) (*User, error) {
    // Validate input
    if userID == "" {
        return nil, errors.New("user ID cannot be empty")
    }
    if !isValidUUID(userID) {
        return nil, errors.New("invalid user ID format")
    }

    // Process request
    return c.storage.GetUser(context.Background(), userID)
}

// Bad - no validation
func (c *Connector) GetUser(userID string) (*User, error) {
    return c.storage.GetUser(context.Background(), userID)
}
```

### 4. Rate Limiting

**Implement rate limiting for sensitive operations**:
```go
type RateLimiter struct {
    attempts map[string][]time.Time
    mu       sync.RWMutex
}

func (rl *RateLimiter) AllowAttempt(identifier string, maxAttempts int, window time.Duration) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-window)

    // Remove old attempts
    attempts := rl.attempts[identifier]
    var recent []time.Time
    for _, t := range attempts {
        if t.After(cutoff) {
            recent = append(recent, t)
        }
    }

    // Check limit
    if len(recent) >= maxAttempts {
        return false
    }

    // Record attempt
    rl.attempts[identifier] = append(recent, now)
    return true
}
```

### 5. Timeout and Context

**Always use context with timeout for external calls**:
```go
// Good
func (c *Connector) fetchUserData(userID string) (*UserData, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    return c.client.GetUserData(ctx, userID)
}

// Bad - no timeout
func (c *Connector) fetchUserData(userID string) (*UserData, error) {
    return c.client.GetUserData(context.Background(), userID)
}
```

---

## Documentation Requirements

### 1. Package Documentation

**Every package must have package-level documentation**:
```go
// Package local provides an enhanced local authentication connector for Dex.
//
// This connector supports multiple authentication methods including:
//   - Password-based authentication
//   - WebAuthn passkeys (FIDO2)
//   - TOTP-based 2FA
//   - Magic link email authentication
//
// The connector integrates with Dex's OAuth/OIDC flow and provides
// gRPC APIs for user management by the Enopax Platform.
package local
```

### 2. Function Documentation

**Exported functions must have documentation**:
```go
// BeginPasskeyRegistration starts the WebAuthn registration ceremony.
//
// It generates a challenge and registration options for the client,
// creates a session to track the registration flow, and returns the
// PublicKeyCredentialCreationOptions that should be passed to
// navigator.credentials.create() in the browser.
//
// The session must be validated in FinishPasskeyRegistration within 5 minutes.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User who is registering the passkey
//
// Returns:
//   - session: WebAuthn session (must be validated in FinishPasskeyRegistration)
//   - options: Options to pass to Web Authentication API
//   - error: Any error that occurred
func (c *Connector) BeginPasskeyRegistration(
    ctx context.Context,
    user *User,
) (*WebAuthnSession, *protocol.CredentialCreation, error) {
    // Implementation
}
```

### 3. Struct Documentation

**Exported structs must have documentation**:
```go
// User represents an enhanced user account with support for multiple
// authentication methods.
//
// A user can have zero or more authentication methods enabled. The only
// required field is Email. All authentication methods are optional,
// allowing for passwordless accounts, passkey-only accounts, etc.
type User struct {
    // ID is a deterministic UUID derived from the user's email address.
    // It remains constant even if the email changes.
    ID string `json:"id"`

    // Email is the user's primary email address (required, unique).
    Email string `json:"email"`

    // PasswordHash is the bcrypt hash of the user's password.
    // Nil if user has not set a password (passwordless account).
    PasswordHash *string `json:"password_hash,omitempty"`

    // Passkeys is the list of WebAuthn credentials registered by the user.
    Passkeys []Passkey `json:"passkeys,omitempty"`

    // TOTPSecret is the base32-encoded TOTP secret.
    // Nil if TOTP is not enabled.
    TOTPSecret *string `json:"totp_secret,omitempty"`

    // More fields...
}
```

### 4. TODO Comments

**Use TODO comments for future work**:
```go
// TODO(username): Implement rate limiting for magic link requests
// TODO: Add support for backup emails
// FIXME: This doesn't handle concurrent passkey registrations correctly
```

---

## Commit Message Guidelines

### 1. Semantic Commit Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### 2. Types

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation only
- `test:` - Adding tests
- `refactor:` - Code refactoring
- `perf:` - Performance improvement
- `chore:` - Maintenance tasks
- `style:` - Code style changes (formatting)
- `build:` - Build system changes
- `ci:` - CI/CD changes

### 3. Scope

- `passkey` - Passkey-related changes
- `totp` - TOTP-related changes
- `magic-link` - Magic link changes
- `storage` - Storage changes
- `api` - gRPC API changes
- `auth` - General authentication changes

### 4. Examples

```
feat(passkey): add WebAuthn registration endpoint

Implement POST /auth/passkey/register/begin endpoint that:
- Generates challenge using crypto/rand
- Creates WebAuthn session with 5-minute TTL
- Returns PublicKeyCredentialCreationOptions

Refs: TODO.md Phase 2 Week 6
```

```
fix(storage): prevent race condition in file writes

Add file locking using syscall.Flock to prevent concurrent
writes to the same user file.

Fixes #123
```

```
docs(api): document gRPC user management endpoints

Add detailed documentation for CreateUser, UpdateUser,
and DeleteUser endpoints with example usage.
```

```
test(totp): add integration tests for 2FA flow

Add tests covering:
- TOTP enrollment
- TOTP validation
- Backup code usage
- Rate limiting

Achieves 95% coverage for totp.go
```

### 5. Commit Message Rules

- **Subject line**:
  - Use imperative mood ("add" not "added" or "adds")
  - Don't capitalize first letter after type
  - No period at the end
  - Keep under 72 characters

- **Body** (optional but recommended):
  - Wrap at 72 characters
  - Explain what and why, not how
  - Separate from subject with blank line

- **Footer** (optional):
  - Reference issues: `Refs: #123` or `Fixes: #123`
  - Reference TODOs: `Refs: TODO.md Phase 2 Week 6`
  - Breaking changes: `BREAKING CHANGE: <description>`

---

## Code Review Checklist

Before submitting a PR, ensure:

**Functionality**:
- [ ] Code works as intended
- [ ] Edge cases handled
- [ ] Errors handled properly
- [ ] No panic() calls (except in init or tests)

**Testing**:
- [ ] Unit tests added/updated
- [ ] Integration tests for new features
- [ ] Tests pass (`make test`)
- [ ] Coverage meets minimum (80%)

**Code Quality**:
- [ ] Follows coding standards
- [ ] No linter warnings (`make lint`)
- [ ] Code formatted (`make fmt`)
- [ ] No unnecessary comments
- [ ] TODO comments include owner

**Security**:
- [ ] Input validation present
- [ ] No sensitive data in logs
- [ ] Crypto operations use crypto/rand
- [ ] Rate limiting for sensitive operations
- [ ] Timeouts set appropriately

**Documentation**:
- [ ] Package documentation updated
- [ ] Function documentation added
- [ ] README updated if needed
- [ ] CLAUDE.md updated if workflow changed
- [ ] TODO.md updated (tasks marked complete)

**Git**:
- [ ] Semantic commit messages
- [ ] Commits are logical units
- [ ] No merge commits (rebase instead)
- [ ] Branch up to date with main

---

## Tools and Automation

### Pre-commit Checks

Create `.git/hooks/pre-commit`:
```bash
#!/bin/bash
set -e

echo "Running pre-commit checks..."

# Format code
echo "- Formatting code..."
make fmt

# Run linter
echo "- Running linter..."
make lint

# Run tests
echo "- Running tests..."
make test

echo "✓ All checks passed"
```

Make it executable:
```bash
chmod +x .git/hooks/pre-commit
```

### Editor Configuration

**VS Code** (`.vscode/settings.json`):
```json
{
  "go.useLanguageServer": true,
  "go.lintOnSave": "workspace",
  "go.formatTool": "gofmt",
  "go.lintTool": "golangci-lint",
  "editor.formatOnSave": true,
  "editor.rulers": [100, 120],
  "[go]": {
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  }
}
```

---

## References

**Official Go Resources**:
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Proverbs](https://go-proverbs.github.io/)

**Project Documentation**:
- [CLAUDE.md](./CLAUDE.md) - AI assistant guide
- [DEVELOPMENT.md](./DEVELOPMENT.md) - Development setup
- [TODO.md](./TODO.md) - Implementation tasks

---

**Last Updated**: 2025-11-18
**Version**: 1.0
**Maintainer**: Enopax Platform Team
