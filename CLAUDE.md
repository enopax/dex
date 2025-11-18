# Dex Enhanced Local Connector - AI Assistant Guide

**Project**: Dex Fork with Enhanced Local Connector
**Repository**: enopax/dex
**Branch**: `feature/passkeys` (implementation), `main` (upstream-compatible)
**Last Updated**: 2025-11-18 (Phase 1 Week 3 completed)

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Getting Started](#getting-started)
3. [Repository Structure](#repository-structure)
4. [Branch Strategy](#branch-strategy)
5. [Development Workflow](#development-workflow)
6. [Implementation Guidelines](#implementation-guidelines)
7. [Testing Requirements](#testing-requirements)
8. [Documentation Standards](#documentation-standards)
9. [Best Practices](#best-practices)

---

## Project Overview

This is a **fork of dexidp/dex** (OpenID Connect identity provider) with enhancements for Enopax's authentication needs.

### Core Enhancements

1. **File-based Storage** (`feature/file-storage` branch)
   - JSON file storage backend for users and credentials
   - Individual files per user (one file per user ID)
   - Deterministic user IDs (SHA-256 of email)
   - No database required

2. **Enhanced Local Connector** (`feature/passkeys` branch - IN PROGRESS)
   - Multi-authentication support (password, passkey, TOTP, magic link)
   - True 2FA (password + passkey/TOTP)
   - Passwordless authentication options
   - Platform integration via gRPC API
   - User registration and auth method setup

### What is Dex?

Dex is an identity service that:
- Provides **OpenID Connect (OIDC)** authentication
- Acts as a **federation layer** between apps and identity providers
- Supports multiple **connectors** (GitHub, LDAP, SAML, local passwords, etc.)
- Issues **ID tokens** and **access tokens** for authenticated users

### Enopax Use Case

```
┌─────────────────────────────────────────────────────────────┐
│                     ENOPAX ARCHITECTURE                     │
└─────────────────────────────────────────────────────────────┘

Enopax Platform (Next.js) ←─────┐
                                 │
Kubernetes Clusters ←────────────┤  OAuth/OIDC
                                 │  Authentication
Infrastructure Tools ←───────────┤
(Grafana, Prometheus)            │
                                 │
                           ┌─────▼─────┐
                           │    Dex    │
                           │  (This    │
                           │   Fork)   │
                           └───────────┘
```

---

## Getting Started

### For New Developers

If you're new to this project, **start here**:

1. **Read this file (CLAUDE.md)** - Understand the project architecture and AI assistant workflows
2. **Read DEVELOPMENT.md** - Set up your development environment (prerequisites, installation, building)
3. **Read TODO.md** - Review the implementation plan and current progress
4. **Read docs/enhancements/passkey-webauthn-support.md** - Understand the passkey concept and architecture

### Quick Setup

```bash
# 1. Clone and navigate to the repository
git clone https://github.com/enopax/dex.git
cd dex

# 2. Switch to feature branch
git checkout feature/passkeys

# 3. Install dependencies (choose one method)
# Option A: Using Nix (recommended)
nix develop

# Option B: Manual installation
make deps

# 4. Build and run
make build
./bin/dex serve config.dev.yaml
```

**Full setup instructions**: See [DEVELOPMENT.md](./DEVELOPMENT.md)

### Key Documentation Files

| File | Purpose | When to Read |
|------|---------|--------------|
| **CLAUDE.md** (this file) | AI assistant guide, workflows, coding standards | Start here for overview |
| **DEVELOPMENT.md** | Development environment setup, building, testing | Before writing code |
| **CODING_STANDARDS.md** | Coding conventions, best practices, security guidelines | Before writing code |
| **TODO.md** | Implementation task list and timeline | Before starting a task |
| **CHANGELOG.md** | Project change history and release notes | When updating code |
| **README.md** | Upstream Dex documentation | Understanding Dex basics |
| **docs/enhancements/passkey-webauthn-support.md** | Passkey concept and architecture | Before implementing passkeys |
| **docs/enhancements/storage-schema.md** | Storage schema and file formats | Before implementing storage |

---

## Repository Structure

```
dex/
├── CLAUDE.md                    # This file - AI guidance
├── DEVELOPMENT.md               # Development environment setup guide
├── TODO.md                      # Implementation task list
├── README.md                    # Original Dex README
├── docs/
│   └── enhancements/
│       └── passkey-webauthn-support.md  # Passkey concept doc
│
├── connector/                   # Authentication connectors
│   ├── local-enhanced/          # Enhanced local connector (TO BE CREATED)
│   ├── github/
│   ├── ldap/
│   └── ...
│
├── storage/                     # Storage backends
│   ├── file/                    # File-based storage (CUSTOM)
│   ├── memory/
│   ├── sql/
│   └── ...
│
├── server/                      # Dex server
│   ├── handlers.go
│   └── ...
│
├── api/                         # gRPC API definitions
│   └── v2/
│       └── api.proto            # Protocol buffer definitions
│
└── cmd/
    └── dex/
        └── main.go              # Entry point
```

---

## Branch Strategy

### Main Branches

| Branch | Purpose | Status | Merge Policy |
|--------|---------|--------|--------------|
| `main` | Upstream-compatible, minimal changes | Stable | Sync with upstream dexidp/dex |
| `feature/file-storage` | File-based storage backend | Complete | Ready for merge to main |
| `feature/passkeys` | Enhanced local connector | In Progress | Active development |

### Upstream Relationship

```
dexidp/dex (upstream)
    │
    │ fork
    ▼
enopax/dex (main)
    │
    ├─── feature/file-storage (file storage implementation)
    │
    └─── feature/passkeys (enhanced auth - based on main)
```

### Working with Branches

**Staying up-to-date with upstream**:
```bash
# Add upstream remote (if not already added)
git remote add upstream https://github.com/dexidp/dex.git

# Fetch upstream changes
git fetch upstream

# Merge upstream into main
git checkout main
git merge upstream/master

# Rebase feature branches
git checkout feature/passkeys
git rebase main
```

**Creating new feature branches**:
```bash
# Always branch from main
git checkout main
git pull origin main
git checkout -b feature/new-feature
```

---

## Development Workflow

### Task-Driven Development (IMPORTANT)

**Every task must follow this workflow**:

1. **Pick a task** from `TODO.md`
2. **Create a branch** (if major feature) or work on existing feature branch
3. **Implement the task** following guidelines
4. **Test the implementation**
5. **Update TODO.md** - Mark task as complete with `[x]`
6. **Commit changes** with semantic commit message
7. **Push to remote**

### Commit Workflow

**After completing each task**:

```bash
# 1. Mark task complete in TODO.md
# Edit TODO.md and change [ ] to [x] for completed task

# 2. Stage changes
git add TODO.md
git add <files-you-changed>

# 3. Commit with semantic message
git commit -m "feat: implement passkey registration begin endpoint

Completed task from TODO.md Phase 2 Week 6:
- Implement POST /auth/passkey/register/begin
- Generate registration options using go-webauthn
- Create and store WebAuthn session
- Return PublicKeyCredentialCreationOptions

Refs: TODO.md Phase 2 Week 6"

# 4. Push
git push
```

### Semantic Commit Messages

Use conventional commits format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Types**:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation only
- `test:` - Adding tests
- `refactor:` - Code refactoring
- `chore:` - Maintenance tasks

**Examples**:
```
feat(passkey): add WebAuthn registration endpoint

fix(storage): prevent race condition in file writes

docs(api): document gRPC user management endpoints

test(passkey): add integration tests for authentication flow

refactor(connector): extract session management to separate file

chore(deps): update go-webauthn to v0.11.2
```

---

## Implementation Guidelines

### File Storage (Already Implemented)

**Location**: `storage/file/`

**Key Files**:
- `file.go` - Main storage implementation
- `storage_test.go` - Tests

**User Storage Format** (`data/passwords/{user-id}.json`):
```json
{
  "email": "alice@example.com",
  "hash": "$2a$10$...",
  "hashFromEnv": "",
  "username": "alice",
  "userID": "ff8d9819-fc0e-12bf-0d24-892e45987e24"
}
```

**Deterministic User IDs**:
```go
// SHA-256 hash of email → UUID format
hash := sha256.Sum256([]byte(email))
userID := fmt.Sprintf("%x-%x-%x-%x-%x",
    hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
```

---

### Enhanced Local Connector (In Development)

**Location**: `connector/local-enhanced/`

**Package Structure**:
```
connector/local-enhanced/
├── local.go              # Connector implementation ✅
├── config.go             # Configuration ✅
├── validation.go         # Validation functions ✅
├── password.go           # Password auth (TODO)
├── passkey.go            # WebAuthn passkey (TODO)
├── totp.go               # TOTP 2FA (TODO)
├── magiclink.go          # Magic link auth (TODO)
├── storage.go            # Storage interface ✅
├── handlers.go           # HTTP handlers (partial)
├── testing.go            # Test utilities ✅
├── config_test.go        # Config tests ✅
├── storage_test.go       # Storage tests ✅
├── validation_test.go    # Validation tests ✅
└── templates/            # HTML templates (TODO)
    ├── login.html
    ├── setup-auth.html
    └── manage-credentials.html
```

**Connector Interface** (must implement):
```go
type Connector interface {
    LoginURL(callbackURL, state string) (string, error)
    HandleCallback(s connector.Scopes, r *http.Request) (identity connector.Identity, err error)
}
```

**Data Validation**:

All data structures have validation methods defined in `validation.go`:
```go
// User validation
if err := user.Validate(); err != nil {
    return fmt.Errorf("invalid user: %w", err)
}

// Passkey validation
if err := passkey.Validate(); err != nil {
    return fmt.Errorf("invalid passkey: %w", err)
}

// Email validation
if err := ValidateEmail(email); err != nil {
    return err
}

// Password validation
if err := ValidatePassword(password); err != nil {
    return err
}

// Username validation
if err := ValidateUsername(username); err != nil {
    return err
}
```

**Validation Rules**:
- **User**: Must have ID, email, at least one auth method, valid timestamps
- **Passkey**: Must have ID, user ID, public key, name, valid AAGUID (16 bytes)
- **Email**: Valid RFC 5322 format with domain containing at least one dot
- **Password**: 8-128 characters, at least one letter and one number
- **Username**: 3-64 characters, alphanumeric/hyphens/underscores, starts with letter

---

### Go Code Standards

**Imports**:
```go
import (
    // Standard library
    "context"
    "encoding/json"
    "fmt"
    "time"

    // External dependencies
    "github.com/go-webauthn/webauthn/webauthn"
    "github.com/pquerna/otp/totp"

    // Internal packages
    "github.com/dexidp/dex/connector"
    "github.com/dexidp/dex/storage"
)
```

**Error Handling**:
```go
// Always wrap errors with context
if err != nil {
    return fmt.Errorf("failed to create user: %w", err)
}

// Log errors before returning
if err != nil {
    log.Errorf("passkey registration failed: %v", err)
    return nil, fmt.Errorf("registration failed: %w", err)
}
```

**Logging**:
```go
// Use structured logging
log.Infof("user %s registered passkey: %s", userID, credentialID)
log.Warnf("rate limit exceeded for user: %s", email)
log.Errorf("WebAuthn verification failed: %v", err)
```

**Context Usage**:
```go
// Always accept context as first parameter
func (c *Connector) CreateUser(ctx context.Context, user *User) error {
    // Use context for timeout/cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Continue...
    }
}
```

---

## Testing Requirements

### Test Coverage

**Minimum Requirements**:
- **Unit tests**: 80% coverage
- **Integration tests**: All major flows
- **End-to-end tests**: Critical user paths

### Test Structure

**Unit Test Example**:
```go
func TestBeginPasskeyRegistration(t *testing.T) {
    // Arrange
    connector := NewTestConnector(t)
    user := &User{
        ID:    "test-user-id",
        Email: "test@example.com",
    }

    // Act
    session, options, err := connector.BeginPasskeyRegistration(context.Background(), user)

    // Assert
    require.NoError(t, err)
    assert.NotNil(t, session)
    assert.Len(t, options.Challenge, 32)
    assert.Equal(t, "auth.enopax.io", options.RPID)
}
```

**Integration Test Example**:
```go
func TestPasskeyRegistrationFlow(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    defer server.Close()

    // Step 1: Begin registration
    beginResp, err := http.Post(
        server.URL+"/auth/passkey/register/begin",
        "application/json",
        strings.NewReader(`{"user_id":"test-user"}`),
    )
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, beginResp.StatusCode)

    // Step 2: Complete registration (with mock credential)
    // ... finish flow
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run specific package
go test ./connector/local-enhanced/

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -run TestBeginPasskeyRegistration ./connector/local-enhanced/
```

---

## Documentation Standards

### Code Documentation

**Package Documentation**:
```go
// Package local provides an enhanced local authentication connector
// supporting multiple authentication methods including passwords,
// passkeys (WebAuthn), TOTP, and magic links.
//
// This connector allows Enopax Platform to manage users with flexible
// authentication policies and true 2FA support.
package local
```

**Function Documentation**:
```go
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
    // Implementation...
}
```

**Struct Documentation**:
```go
// User represents an enhanced user account with support for multiple
// authentication methods.
type User struct {
    // ID is a deterministic UUID derived from the user's email address
    ID string `json:"id"`

    // Email is the user's primary email address (required, unique)
    Email string `json:"email"`

    // PasswordHash is the bcrypt hash of the user's password.
    // Nil if user has not set a password (passwordless account).
    PasswordHash *string `json:"password_hash,omitempty"`

    // Passkeys is the list of WebAuthn credentials registered by the user
    Passkeys []Passkey `json:"passkeys,omitempty"`

    // More fields...
}
```

### Updating Documentation

**When to update docs**:
- ✅ After completing a task
- ✅ When changing public APIs
- ✅ When adding new configuration options
- ✅ When fixing bugs (add to changelog)

**Documentation Files**:
- `TODO.md` - Mark tasks complete `[x]`
- `CLAUDE.md` - Update if workflow changes (this file)
- `docs/enhancements/passkey-webauthn-support.md` - Update concept if architecture changes
- `README.md` - Update if user-facing changes

---

## Testing Infrastructure

### Testing Utilities

The enhanced local connector has comprehensive testing utilities in `connector/local-enhanced/testing.go`:

**Test Helpers**:
- `DefaultTestConfig(t)` - Creates a default test configuration with temporary storage
- `TestLogger(t)` - Creates a logger suitable for testing
- `SetupTestStorage(t)` - Sets up temporary storage directories
- `CleanupTestStorage(t, dataDir)` - Cleans up test storage
- `TestContext(t)` - Creates a context with 30-second timeout
- `WithTestTimeout(t, timeout, f)` - Runs a function with timeout to detect deadlocks

**Test Data Generators**:
- `NewTestUser(email)` - Creates a test user with default settings
- `NewTestPasskey(userID, name)` - Creates a test passkey credential
- `NewTestWebAuthnSession(userID, operation, ttl)` - Creates a test WebAuthn session
- `NewTestMagicLinkToken(userID, email, ttl)` - Creates a test magic link token
- `GenerateTestBackupCodes(count)` - Generates backup codes

**Assertion Helpers**:
- `AssertFileExists(t, path)` - Checks if file exists
- `AssertFileNotExists(t, path)` - Checks if file does not exist
- `AssertFilePermissions(t, path, perm)` - Verifies file permissions
- `AssertDirPermissions(t, path, perm)` - Verifies directory permissions

**Mock Objects**:
- `MockEmailSender` - Mock email sender for testing magic links

### Running Tests

```bash
# Run all tests
make test

# Run only enhanced local connector tests
make test-local-enhanced

# Run with coverage report
make test-local-enhanced-coverage

# Run with race detection
make test-local-enhanced-race

# Run specific test
go test -run TestBeginPasskeyRegistration ./connector/local-enhanced/
```

### Writing Tests

**Basic Test Structure**:
```go
func TestFeatureName(t *testing.T) {
    // Setup
    config := DefaultTestConfig(t)
    defer CleanupTestStorage(t, config.DataDir)

    ctx := TestContext(t)
    user := NewTestUser("test@example.com")

    // Test logic here

    // Assertions
    assert.NotNil(t, result)
    require.NoError(t, err)
}
```

**Using Test Helpers**:
```go
func TestStorageOperations(t *testing.T) {
    // Create storage
    dataDir := SetupTestStorage(t)
    defer CleanupTestStorage(t, dataDir)

    // Create test data
    user := NewTestUser("alice@example.com")
    passkey := NewTestPasskey(user.ID, "My Security Key")

    // Test with timeout
    WithTestTimeout(t, 5*time.Second, func() {
        // ... test code that might deadlock
    })
}
```

**Testing Email Sending**:
```go
func TestMagicLinkEmail(t *testing.T) {
    emailSender := NewMockEmailSender()

    // Trigger email send
    err := SendMagicLink(emailSender, "user@example.com")
    require.NoError(t, err)

    // Verify email was sent
    lastEmail := emailSender.GetLastEmail()
    assert.Equal(t, "user@example.com", lastEmail.To)
    assert.Contains(t, lastEmail.Body, "magic link")
}
```

### Test Coverage Goals

- **Unit Tests**: Minimum 80% coverage
- **Integration Tests**: All major authentication flows
- **End-to-End Tests**: Critical user journeys

---

## Best Practices

### Security

**ALWAYS**:
- ✅ Use HTTPS-only in production (WebAuthn requirement)
- ✅ Validate all user input
- ✅ Use constant-time comparisons for secrets
- ✅ Generate cryptographically secure random values
- ✅ Set appropriate timeouts (5 min for WebAuthn challenges)
- ✅ Implement rate limiting (prevent brute force)

**NEVER**:
- ❌ Log sensitive data (passwords, tokens, credentials)
- ❌ Store plaintext passwords
- ❌ Trust client-provided IDs without validation
- ❌ Skip origin validation in WebAuthn
- ❌ Allow HTTP for authentication endpoints

**Example: Secure Random Generation**:
```go
import "crypto/rand"

func generateChallenge() ([]byte, error) {
    challenge := make([]byte, 32)
    if _, err := rand.Read(challenge); err != nil {
        return nil, fmt.Errorf("failed to generate challenge: %w", err)
    }
    return challenge, nil
}
```

**Example: Constant-Time Comparison**:
```go
import "crypto/subtle"

func validateToken(provided, expected string) bool {
    return subtle.ConstantTimeCompare(
        []byte(provided),
        []byte(expected),
    ) == 1
}
```

---

### Configuration

**Configuration Schema**:
```yaml
connectors:
  - type: local-enhanced
    id: local
    name: Enopax Authentication
    config:
      # Passkey settings
      passkey:
        enabled: true
        rpID: auth.enopax.io
        rpName: Enopax
        rpOrigins:
          - https://auth.enopax.io
        userVerification: preferred

      # 2FA settings
      twoFactor:
        required: false  # Global 2FA requirement
        methods:
          - totp
          - passkey

      # Magic link settings
      magicLink:
        enabled: true
        ttl: 600  # 10 minutes
        rateLimit:
          perHour: 3
          perDay: 10

      # Email settings
      email:
        smtp:
          host: smtp.example.com
          port: 587
          username: noreply@enopax.io
          password: ${SMTP_PASSWORD}
```

---

### Performance

**File Operations**:
```go
// Use file locking for concurrent access
import "syscall"

func (s *Storage) writeUserFile(user *User) error {
    f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
    if err != nil {
        return err
    }
    defer f.Close()

    // Lock file for exclusive access
    if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
        return err
    }
    defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

    // Write atomically
    return json.NewEncoder(f).Encode(user)
}
```

**Caching** (if needed):
```go
// Cache frequently accessed data
type Storage struct {
    userCache sync.Map  // thread-safe map
}

func (s *Storage) GetUser(ctx context.Context, userID string) (*User, error) {
    // Check cache first
    if cached, ok := s.userCache.Load(userID); ok {
        return cached.(*User), nil
    }

    // Load from file
    user, err := s.loadUserFromFile(userID)
    if err != nil {
        return nil, err
    }

    // Cache for next time
    s.userCache.Store(userID, user)
    return user, nil
}
```

---

## Common Tasks

### Adding a New Authentication Method

1. **Create handler file** (`connector/local-enhanced/newmethod.go`)
2. **Define data structures** (credentials, sessions)
3. **Implement HTTP endpoints** (begin, finish)
4. **Add to storage interface**
5. **Update user schema**
6. **Create UI templates**
7. **Write tests**
8. **Update configuration schema**
9. **Document in README**
10. **Mark task complete in TODO.md**

### Adding a New gRPC Endpoint

1. **Define in protobuf** (`api/v2/api.proto`)
2. **Generate Go code**: `make proto`
3. **Implement handler** in gRPC service
4. **Add storage operations** if needed
5. **Write tests**
6. **Document API** with examples
7. **Mark task complete in TODO.md**

### Debugging

```bash
# Run Dex in debug mode
./bin/dex serve config.yaml --log-level=debug

# Check logs
tail -f /var/log/dex.log

# Test WebAuthn flow
# Use Chrome DevTools > Application > WebAuthn to add virtual authenticator
```

---

## Task Completion Checklist

**Before marking a task as complete `[x]`**:

- [ ] Implementation complete and working
- [ ] Unit tests written and passing
- [ ] Integration tests written (if applicable)
- [ ] Code documented (comments, doc strings)
- [ ] Configuration documented (if new config added)
- [ ] Error handling implemented
- [ ] Logging added
- [ ] Security reviewed (no secrets logged, input validated)
- [ ] TODO.md updated (task marked `[x]`)
- [ ] CHANGELOG.md updated (change documented)
- [ ] Follows CODING_STANDARDS.md guidelines
- [ ] Changes committed with semantic message
- [ ] Changes pushed to remote

---

## Getting Help

### Resources

**Dex Documentation**:
- Official Docs: https://dexidp.io/docs/
- GitHub: https://github.com/dexidp/dex

**WebAuthn**:
- W3C Spec: https://www.w3.org/TR/webauthn-2/
- go-webauthn: https://github.com/go-webauthn/webauthn
- Guide: https://webauthn.guide/

**TOTP**:
- RFC 6238: https://tools.ietf.org/html/rfc6238
- go-otp: https://github.com/pquerna/otp

**Enopax Docs**:
- Development Guide: `DEVELOPMENT.md`
- Coding Standards: `CODING_STANDARDS.md`
- Changelog: `CHANGELOG.md`
- Passkey Concept: `docs/enhancements/passkey-webauthn-support.md`
- Implementation Plan: `TODO.md`

---

## Quick Reference

### File Locations

| What | Where |
|------|-------|
| Dev guide | `DEVELOPMENT.md` |
| Coding standards | `CODING_STANDARDS.md` |
| Changelog | `CHANGELOG.md` |
| Task list | `TODO.md` |
| Concept doc | `docs/enhancements/passkey-webauthn-support.md` |
| Connector code | `connector/local-enhanced/` (to be created) |
| Storage code | `storage/file/` |
| gRPC definitions | `api/v2/api.proto` |
| Templates | `connector/local-enhanced/templates/` |
| Tests | `*_test.go` files |

### Commands

```bash
# Build
make build

# Test
make test

# Run
./bin/dex serve config.yaml

# Generate protobuf
make proto

# Lint
golangci-lint run
```

---

**Last Updated**: 2025-11-18
**Version**: 1.1
**Maintainer**: Enopax Platform Team
