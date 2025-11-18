# Development Guide - Dex Enhanced Local Connector

**Project**: Dex Fork with Enhanced Local Connector
**Repository**: enopax/dex
**Last Updated**: 2025-11-18

---

## Table of Contents

1. [Quick Start](#quick-start)
2. [Prerequisites](#prerequisites)
3. [Development Environment Setup](#development-environment-setup)
4. [Project Structure](#project-structure)
5. [Building and Running](#building-and-running)
6. [Testing](#testing)
7. [Development Workflow](#development-workflow)
8. [Code Style and Standards](#code-style-and-standards)
9. [Troubleshooting](#troubleshooting)

---

## Quick Start

For experienced developers who want to get started immediately:

```bash
# Clone the repository
git clone https://github.com/enopax/dex.git
cd dex

# Switch to feature branch
git checkout feature/passkeys

# Install dependencies (Option 1: Using Nix - Recommended)
nix develop

# Install dependencies (Option 2: Manual installation)
make deps

# Build Dex
make build

# Run tests
make testall

# Run Dex in development mode
./bin/dex serve config.dev.yaml
```

---

## Prerequisites

### Required Software

#### Option 1: Nix + direnv (Recommended)

**Why Nix?** Nix provides a reproducible development environment with all dependencies automatically managed.

1. **Install Nix** (multi-user installation recommended):
   ```bash
   sh <(curl -L https://nixos.org/nix/install) --daemon
   ```

2. **Install direnv**:
   ```bash
   # macOS
   brew install direnv

   # Linux (Ubuntu/Debian)
   sudo apt-get install direnv

   # Add to your shell (~/.bashrc, ~/.zshrc, etc.)
   eval "$(direnv hook bash)"  # For bash
   eval "$(direnv hook zsh)"   # For zsh
   ```

3. **Allow direnv** in the project directory:
   ```bash
   cd dex
   direnv allow
   ```

**Benefits**:
- ✅ Automatic dependency management
- ✅ Reproducible builds
- ✅ No manual version management
- ✅ Isolated from system dependencies

#### Option 2: Manual Installation

If you prefer not to use Nix, install the following manually:

1. **Go 1.25.0+**
   ```bash
   # macOS
   brew install go

   # Linux
   # Download from https://go.dev/dl/
   wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
   sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
   export PATH=$PATH:/usr/local/go/bin
   ```

2. **Docker** (for development database and testing)
   ```bash
   # macOS
   brew install --cask docker

   # Linux
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh
   ```

3. **Make**
   ```bash
   # macOS (comes with Xcode Command Line Tools)
   xcode-select --install

   # Linux (Ubuntu/Debian)
   sudo apt-get install build-essential
   ```

4. **Development Tools**
   ```bash
   # Install development dependencies (protoc, linters, etc.)
   make deps
   ```

### Verify Installation

```bash
# Check Go version
go version  # Should be 1.25.0 or higher

# Check Docker
docker --version

# Check Make
make --version

# Check git
git --version
```

---

## Development Environment Setup

### 1. Clone the Repository

```bash
# Clone from GitHub
git clone https://github.com/enopax/dex.git
cd dex

# Or if you already have it cloned
cd /path/to/dex
```

### 2. Configure Git Remote

```bash
# Verify remotes
git remote -v

# Add upstream if not present (to sync with dexidp/dex)
git remote add upstream https://github.com/dexidp/dex.git

# Fetch all branches
git fetch --all
```

### 3. Switch to Feature Branch

```bash
# Switch to the passkeys feature branch
git checkout feature/passkeys

# Or create a new feature branch
git checkout main
git pull origin main
git checkout -b feature/your-feature-name
```

### 4. Install Dependencies

#### Using Nix (Recommended)

```bash
# If direnv is set up, dependencies are auto-loaded
cd dex
# direnv will automatically load the environment

# Or manually enter the Nix shell
nix develop
```

#### Using Make

```bash
# Install all development dependencies
make deps
```

This installs:
- `protoc` - Protocol buffer compiler
- `protoc-gen-go` - Go protobuf plugin
- `protoc-gen-go-grpc` - gRPC Go plugin
- `golangci-lint` - Go linter
- `gotestsum` - Test runner with better output
- `kind` - Kubernetes in Docker (for integration tests)

### 5. Set Up Configuration

```bash
# Copy example configuration
cp config.yaml.dist config.yaml

# For development with file storage
cat > config.yaml <<EOF
issuer: http://127.0.0.1:5556/dex

storage:
  type: file
  config:
    dir: ./data

web:
  http: 0.0.0.0:5556

logger:
  level: debug
  format: text

oauth2:
  skipApprovalScreen: true

staticClients:
- id: example-app
  redirectURIs:
  - 'http://127.0.0.1:5555/callback'
  name: 'Example App'
  secret: ZXhhbXBsZS1hcHAtc2VjcmV0

connectors:
- type: local-enhanced
  id: local
  name: Enopax Authentication
  config:
    passkey:
      enabled: true
      rpID: 127.0.0.1
      rpName: Dex Development
EOF
```

### 6. Create Data Directory

```bash
# Create directory for file storage
mkdir -p data/passwords
mkdir -p data/passkeys
mkdir -p data/totp
mkdir -p data/sessions
mkdir -p data/magic-link-tokens
```

---

## Project Structure

### Directory Overview

```
dex/
├── api/                        # gRPC API definitions (protobuf)
│   └── v2/                     # API v2 (current version)
│       └── api.proto           # Protobuf service definitions
│
├── bin/                        # Compiled binaries (gitignored)
│   ├── dex                     # Main Dex binary
│   ├── example-app             # Example OAuth client
│   └── grpc-client             # Example gRPC client
│
├── cmd/                        # Command-line entry points
│   ├── dex/                    # Main Dex server
│   └── docker-entrypoint/      # Docker container entry point
│
├── connector/                  # Authentication connectors
│   ├── github/                 # GitHub OAuth connector
│   ├── ldap/                   # LDAP connector
│   ├── local-enhanced/         # Enhanced local connector (TO BE CREATED)
│   └── ...                     # Other connectors
│
├── docs/                       # Documentation
│   ├── enhancements/           # Enhancement proposals
│   │   └── passkey-webauthn-support.md
│   ├── img/                    # Images and diagrams
│   └── logos/                  # Dex logos
│
├── examples/                   # Example applications
│   ├── example-app/            # Example OAuth client
│   └── grpc-client/            # Example gRPC API usage
│
├── pkg/                        # Shared packages
│   ├── log/                    # Logging utilities
│   └── ...
│
├── server/                     # HTTP server and handlers
│   ├── handlers.go             # OAuth and OIDC endpoints
│   ├── oauth2.go               # OAuth2 flow logic
│   └── server.go               # Server initialization
│
├── storage/                    # Storage backends
│   ├── file/                   # File-based storage (CUSTOM)
│   ├── memory/                 # In-memory storage
│   ├── sql/                    # SQL databases
│   └── etcd/                   # etcd storage
│
├── web/                        # Frontend templates and assets
│   ├── templates/              # HTML templates
│   └── static/                 # CSS, JS, images
│
├── CLAUDE.md                   # AI assistant guide (this project)
├── TODO.md                     # Implementation task list
├── DEVELOPMENT.md              # This file - development guide
├── README.md                   # Main project README
├── Makefile                    # Build and development tasks
├── go.mod                      # Go module dependencies
└── config.yaml                 # Dex configuration
```

### Key Files for Enhanced Local Connector

When implementing the enhanced local connector, you'll primarily work in:

```
connector/local-enhanced/       # TO BE CREATED
├── local.go                    # Connector interface implementation
├── config.go                   # Configuration structures
├── password.go                 # Password authentication
├── passkey.go                  # WebAuthn passkey support
├── totp.go                     # TOTP 2FA
├── magiclink.go                # Magic link authentication
├── storage.go                  # Storage interface
├── handlers.go                 # HTTP request handlers
├── local_test.go               # Unit tests
└── templates/                  # HTML templates
    ├── login.html              # Login page
    ├── setup-auth.html         # Auth setup wizard
    └── manage-credentials.html # Credential management
```

---

## Building and Running

### Building Dex

```bash
# Build main Dex binary
make build

# Build example applications
make examples

# Build release binary (optimized, static linking)
make release-binary
```

**Output**:
- `./bin/dex` - Main Dex server
- `./bin/example-app` - Example OAuth client
- `./bin/grpc-client` - Example gRPC client

### Running Dex

#### Development Mode (Debug Logging)

```bash
# Run with debug logging
./bin/dex serve config.yaml --log-level=debug

# Or use config.dev.yaml
./bin/dex serve config.dev.yaml
```

**Expected Output**:
```
time="2025-11-18T10:00:00Z" level=info msg="config issuer: http://127.0.0.1:5556/dex"
time="2025-11-18T10:00:00Z" level=info msg="config storage: file"
time="2025-11-18T10:00:00Z" level=info msg="listening (http) on 0.0.0.0:5556"
```

#### Running Example App

In a separate terminal:

```bash
# Run example OAuth client
./bin/example-app --issuer http://127.0.0.1:5556/dex

# Visit http://127.0.0.1:5555 in your browser
```

#### Using Docker Compose (Full Environment)

```bash
# Start all services (Dex + example app + databases)
make up

# Stop all services
make down
```

**Services**:
- Dex: http://127.0.0.1:5556
- Example App: http://127.0.0.1:5555
- PostgreSQL: localhost:5432 (if configured)

---

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run tests with race detection
make testrace

# Run all tests (includes race detection)
make testall

# Run specific package tests
go test -v ./storage/file/

# Run specific test function
go test -v -run TestCreateUser ./storage/file/

# Run tests with coverage
go test -cover ./...

# Generate HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Test Coverage Requirements

- **Minimum Coverage**: 80% for all packages
- **Critical Paths**: 100% coverage (authentication, credential storage)
- **Integration Tests**: All major user flows

### Writing Tests

#### Unit Test Example

```go
// storage/file/user_test.go
package file

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCreateUser(t *testing.T) {
    // Arrange
    storage := NewTestStorage(t)
    user := &User{
        ID:    "test-user-id",
        Email: "test@example.com",
    }

    // Act
    err := storage.CreateUser(context.Background(), user)

    // Assert
    require.NoError(t, err)

    // Verify user was created
    retrieved, err := storage.GetUser(context.Background(), user.ID)
    require.NoError(t, err)
    assert.Equal(t, user.Email, retrieved.Email)
}
```

#### Integration Test Example

```go
// connector/local-enhanced/integration_test.go
func TestPasskeyRegistrationFlow(t *testing.T) {
    // Setup test server
    server := setupTestServer(t)
    defer server.Close()

    // Step 1: Begin registration
    beginResp := httptest.NewRecorder()
    req := httptest.NewRequest("POST", "/auth/passkey/register/begin",
        strings.NewReader(`{"user_id":"test-user"}`))

    server.Handler.ServeHTTP(beginResp, req)
    require.Equal(t, http.StatusOK, beginResp.Code)

    // Step 2: Verify response contains challenge
    var options map[string]interface{}
    err := json.Unmarshal(beginResp.Body.Bytes(), &options)
    require.NoError(t, err)
    assert.Contains(t, options, "challenge")
}
```

### Test Data

Create test fixtures in `testdata/` directories:

```bash
# Create test data directory
mkdir -p storage/file/testdata

# Example test user
cat > storage/file/testdata/user.json <<EOF
{
  "id": "test-user-123",
  "email": "test@example.com",
  "username": "testuser",
  "email_verified": true
}
EOF
```

---

## Development Workflow

### Daily Development Cycle

```bash
# 1. Update your branch
git checkout feature/passkeys
git pull origin feature/passkeys

# 2. Sync with main if needed
git fetch origin main
git rebase origin/main

# 3. Make changes
# Edit files...

# 4. Run tests frequently
make test

# 5. Run linter
make lint

# 6. Fix linter issues
make fix

# 7. Build and test locally
make build
./bin/dex serve config.yaml

# 8. Commit changes (see below for commit guidelines)
git add .
git commit -m "feat: implement passkey registration"

# 9. Push to remote
git push origin feature/passkeys
```

### Working with TODO.md

**IMPORTANT**: Every task from TODO.md must be marked complete when finished.

```bash
# 1. Pick a task from TODO.md
# 2. Mark it as [x] when complete
# 3. Commit TODO.md with your changes

# Example commit
git add TODO.md
git add connector/local-enhanced/
git commit -m "feat(passkey): implement registration endpoint

Completed task from TODO.md Phase 2 Week 6:
- Implement POST /auth/passkey/register/begin
- Generate registration options
- Create WebAuthn session

Refs: TODO.md Phase 2 Week 6"
```

### Generating Code

```bash
# Generate all code (protobuf + ent)
make generate

# Generate protobuf only
make generate-proto

# Generate database ORM only
make generate-ent

# Verify generated code is committed
make verify
```

### Working with Dependencies

```bash
# Add a new dependency
go get github.com/go-webauthn/webauthn@v0.11.2

# Update go.mod
make go-mod-tidy

# Verify go.mod changes
make verify-go-mod
```

---

## Code Style and Standards

### Go Code Standards

#### 1. Imports

```go
import (
    // Standard library (alphabetical)
    "context"
    "encoding/json"
    "fmt"
    "time"

    // External dependencies (alphabetical)
    "github.com/go-webauthn/webauthn/webauthn"
    "github.com/pquerna/otp/totp"

    // Internal packages (alphabetical)
    "github.com/dexidp/dex/connector"
    "github.com/dexidp/dex/storage"
)
```

#### 2. Error Handling

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

#### 3. Logging

```go
// Use structured logging with appropriate levels
log.Infof("user %s registered passkey: %s", userID, credentialID)
log.Warnf("rate limit exceeded for user: %s", email)
log.Errorf("WebAuthn verification failed: %v", err)
log.Debugf("session created: %s", sessionID)
```

#### 4. Context Usage

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

#### 5. Documentation

```go
// Package documentation
// Package local provides an enhanced local authentication connector
// supporting multiple authentication methods including passwords,
// passkeys (WebAuthn), TOTP, and magic links.
package local

// Function documentation
// BeginPasskeyRegistration starts the WebAuthn registration ceremony.
//
// It generates a challenge and registration options for the client,
// creates a session to track the registration flow, and returns the
// PublicKeyCredentialCreationOptions.
//
// Parameters:
//   - ctx: Context for timeout and cancellation
//   - user: User who is registering the passkey
//
// Returns:
//   - session: WebAuthn session
//   - options: Options for Web Authentication API
//   - error: Any error that occurred
func (c *Connector) BeginPasskeyRegistration(ctx context.Context, user *User) (*WebAuthnSession, *protocol.CredentialCreation, error) {
    // Implementation...
}
```

### Running Linters

```bash
# Run linter (checks code style, bugs, performance)
make lint

# Auto-fix issues where possible
make fix

# Run Go formatting
go fmt ./...

# Run go vet (built-in Go linter)
go vet ./...
```

### Security Best Practices

**ALWAYS**:
- ✅ Use HTTPS-only in production
- ✅ Validate all user input
- ✅ Use constant-time comparisons for secrets
- ✅ Generate cryptographically secure random values
- ✅ Set appropriate timeouts
- ✅ Implement rate limiting

**NEVER**:
- ❌ Log sensitive data (passwords, tokens, credentials)
- ❌ Store plaintext passwords
- ❌ Trust client-provided IDs without validation
- ❌ Skip origin validation in WebAuthn
- ❌ Allow HTTP for authentication endpoints

---

## Troubleshooting

### Common Issues

#### 1. Build Fails: "cannot find package"

```bash
# Solution: Install dependencies
make deps

# Or manually
go mod download
go mod tidy
```

#### 2. Tests Fail: "no such file or directory"

```bash
# Solution: Create data directories
mkdir -p data/passwords data/passkeys data/sessions

# Or run from project root
cd /path/to/dex
go test ./...
```

#### 3. Dex Won't Start: "address already in use"

```bash
# Solution: Kill existing Dex process
pkill dex

# Or change port in config.yaml
web:
  http: 0.0.0.0:5557  # Different port
```

#### 4. Protobuf Generation Fails

```bash
# Solution: Install protoc tools
make deps

# Or manually
make bin/protoc
make bin/protoc-gen-go
make bin/protoc-gen-go-grpc
```

#### 5. Docker Compose Fails

```bash
# Solution: Reset Docker environment
make down
docker system prune -a
make up
```

#### 6. Go Version Mismatch

```bash
# Check required version in go.mod
grep "^go " go.mod

# Install correct version
# macOS
brew upgrade go

# Or download from https://go.dev/dl/
```

### Debug Mode

```bash
# Run Dex with debug logging
./bin/dex serve config.yaml --log-level=debug

# Enable verbose test output
go test -v ./...

# Run with race detector (slower but catches concurrency bugs)
go test -race ./...
```

### Getting Help

**Internal Resources**:
- Read `CLAUDE.md` for AI assistant guidance
- Check `TODO.md` for implementation status
- Review `docs/enhancements/passkey-webauthn-support.md` for architecture

**External Resources**:
- Dex Documentation: https://dexidp.io/docs/
- Dex GitHub: https://github.com/dexidp/dex
- Go Documentation: https://go.dev/doc/
- WebAuthn Guide: https://webauthn.guide/

**Community**:
- CNCF Slack: #dexidp channel
- Dex Discussions: https://github.com/dexidp/dex/discussions
- Mailing List: dex-dev@googlegroups.com

---

## Next Steps

After setting up your development environment:

1. **Read the documentation**:
   - `CLAUDE.md` - AI assistant guide
   - `TODO.md` - Implementation tasks
   - `docs/enhancements/passkey-webauthn-support.md` - Passkey concept

2. **Explore the codebase**:
   ```bash
   # Look at existing connectors
   ls -la connector/

   # Understand file storage implementation
   cat storage/file/file.go
   ```

3. **Run the example app**:
   ```bash
   make build
   make examples
   ./bin/dex serve config.dev.yaml &
   ./bin/example-app --issuer http://127.0.0.1:5556/dex
   ```

4. **Pick a task from TODO.md**:
   - Start with Phase 0 tasks
   - Mark tasks complete as you finish
   - Commit frequently with semantic messages

5. **Write tests as you go**:
   - Aim for 80%+ coverage
   - Test both success and error cases
   - Run tests frequently: `make test`

---

## Additional Resources

### Recommended Reading

**Dex**:
- [Dex Architecture](https://dexidp.io/docs/architecture/)
- [Writing Connectors](https://dexidp.io/docs/connectors/)
- [Storage Backends](https://dexidp.io/docs/storage/)

**WebAuthn**:
- [W3C WebAuthn Specification](https://www.w3.org/TR/webauthn-2/)
- [go-webauthn Library](https://github.com/go-webauthn/webauthn)
- [WebAuthn Guide](https://webauthn.guide/)

**TOTP**:
- [RFC 6238 - TOTP](https://tools.ietf.org/html/rfc6238)
- [go-otp Library](https://github.com/pquerna/otp)

**Go Development**:
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Testing](https://go.dev/doc/tutorial/add-a-test)

### Useful Commands

```bash
# Quick reference
make help               # Show all make targets
make build              # Build Dex binary
make test               # Run tests
make testall            # Run all tests (with race detector)
make lint               # Run linter
make fix                # Auto-fix linter issues
make deps               # Install dev dependencies
make generate           # Generate protobuf and ent code
make up                 # Start Docker environment
make down               # Stop Docker environment
make clean              # Remove build artifacts

# Git helpers
git status              # Check current status
git log --oneline -10   # View recent commits
git diff                # View unstaged changes
git diff --staged       # View staged changes

# Go helpers
go mod tidy             # Clean up dependencies
go test -cover ./...    # Run tests with coverage
go build ./...          # Build all packages
go fmt ./...            # Format all code
```

---

**Last Updated**: 2025-11-18
**Version**: 1.0
**Maintainer**: Enopax Platform Team
