# Enhanced Local Connector with Passkeys - Implementation Plan

**Project**: Dex Enhanced Local Connector (Option B)
**Branch**: `feature/passkeys`
**Status**: Planning
**Start Date**: TBD
**Target Completion**: 12-14 weeks

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture Summary](#architecture-summary)
3. [Phase Breakdown](#phase-breakdown)
4. [Detailed Task List](#detailed-task-list)
5. [Dependencies](#dependencies)
6. [Testing Strategy](#testing-strategy)
7. [Risk Management](#risk-management)

---

## Overview

This implementation plan covers building an **Enhanced Local Connector** for Dex that supports:

- ✅ Multiple authentication methods per user (password, passkey, TOTP, magic link)
- ✅ True 2FA (password + passkey/TOTP)
- ✅ Passwordless authentication (passkey-only, magic link-only)
- ✅ Platform-managed user registration with email verification
- ✅ Flexible authentication policies (enforce 2FA, allowed methods, etc.)

### Key Deliverables

1. Enhanced storage schema with multi-auth support
2. WebAuthn passkey registration and authentication
3. TOTP 2FA support
4. Magic link authentication
5. gRPC API for Platform integration
6. Registration and auth setup flows
7. Admin UI for credential management
8. Comprehensive test suite

---

## Architecture Summary

```
┌─────────────────────────────────────────────────────────────┐
│                    ENHANCED LOCAL CONNECTOR                 │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Authentication Methods                              │  │
│  │  ├── Password (optional)                             │  │
│  │  ├── Passkey (WebAuthn)                              │  │
│  │  ├── TOTP (2FA)                                       │  │
│  │  ├── Magic Link (email)                              │  │
│  │  └── Backup Codes                                    │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Storage Backend (File-based)                        │  │
│  │  ├── users/{user-id}.json                            │  │
│  │  ├── passkeys/{credential-id}.json                   │  │
│  │  ├── totp/{user-id}.json                             │  │
│  │  ├── magic-link-tokens/{token}.json                  │  │
│  │  └── webauthn-sessions/{session-id}.json            │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  gRPC API (Platform Integration)                     │  │
│  │  ├── CreateUser                                       │  │
│  │  ├── SetPassword                                      │  │
│  │  ├── RegisterPasskey                                  │  │
│  │  ├── EnableTOTP                                       │  │
│  │  └── ListCredentials                                  │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Phase Breakdown

### Phase 0: Foundation & Setup (Week 1-2)
**Goal**: Set up project structure and dependencies

### Phase 1: Enhanced Storage Schema (Week 3-4)
**Goal**: Implement multi-auth user storage

### Phase 2: Passkey Support (Week 5-7)
**Goal**: WebAuthn registration and authentication

### Phase 3: TOTP 2FA (Week 8-9)
**Goal**: Time-based one-time password support

### Phase 4: Magic Link (Week 10)
**Goal**: Email-based passwordless authentication

### Phase 5: gRPC API (Week 11)
**Goal**: Platform integration endpoints

### Phase 6: Registration Flow (Week 12)
**Goal**: User signup and auth method setup

### Phase 7: Testing & Polish (Week 13-14)
**Goal**: Comprehensive testing and documentation

---

## Detailed Task List

## Phase 0: Foundation & Setup

### Week 1: Project Setup

#### Infrastructure
- [x] Create feature branch `feature/passkeys` (DONE)
- [x] Set up development environment documentation
- [x] Configure Go module dependencies (DONE - go.mod updated with webauthn, otp, jwt, qrcode)
- [x] Set up testing infrastructure (DONE - testing helpers, mock utilities, and test targets in Makefile)

#### Dependencies
- [x] Add `github.com/go-webauthn/webauthn` (WebAuthn support) - v0.11.2
- [x] Add `github.com/pquerna/otp` (TOTP support) - v1.4.0
- [x] Add `github.com/golang-jwt/jwt/v5` (JWT for magic links) - v5.2.1
- [x] Add `github.com/skip2/go-qrcode` (QR code generation for TOTP) - v0.0.0-20200617195104
- [x] Add testing dependencies (testify already available in project)

#### Documentation
- [x] Create DEVELOPMENT.md with setup instructions
- [x] Document coding standards and conventions
- [x] Set up changelog tracking

**Deliverable**: Development environment ready, dependencies installed

---

### Week 2: Architecture & Design

#### Code Structure
- [x] Design package structure:
  ```
  connector/local-enhanced/
  ├── local.go              # Main connector implementation
  ├── config.go             # Configuration structures
  ├── password.go           # Password authentication
  ├── passkey.go            # WebAuthn passkey support
  ├── totp.go               # TOTP 2FA
  ├── magiclink.go          # Magic link authentication
  ├── storage.go            # Storage interface
  ├── handlers.go           # HTTP handlers
  └── templates/            # HTML templates
      ├── login.html
      ├── setup-auth.html
      └── manage-credentials.html
  ```

- [x] Define storage interfaces
- [x] Design configuration schema
- [x] Plan API endpoints

#### Storage Design
- [x] Define enhanced user schema (JSON)
- [x] Design file storage layout
- [x] Plan migration from simple password storage
- [x] Document storage format

**Deliverable**: Architecture documented, code structure defined ✅

---

## Phase 1: Enhanced Storage Schema

### Week 3: User Storage Implementation

#### User Schema
- [x] Implement enhanced User struct:
  ```go
  type User struct {
      ID              string
      Email           string
      Username        string
      DisplayName     string
      EmailVerified   bool

      // Authentication methods
      PasswordHash    *string
      Passkeys        []Passkey
      TOTPSecret      *string
      TOTPEnabled     bool
      BackupCodes     []BackupCode

      // Settings
      MagicLinkEnabled bool
      Require2FA       bool

      // Metadata
      CreatedAt       time.Time
      UpdatedAt       time.Time
      LastLoginAt     time.Time
  }
  ```

- [x] Implement JSON marshaling/unmarshaling (handled automatically by Go's json package)
- [x] Add validation functions (connector/local-enhanced/validation.go)
- [x] Write unit tests for user struct (connector/local-enhanced/validation_test.go)

#### Passkey Schema
- [x] Implement Passkey credential struct:
  ```go
  type Passkey struct {
      ID              string
      UserID          string
      PublicKey       []byte
      AttestationType string
      AAGUID          []byte
      SignCount       uint32
      Transports      []string

      // User-friendly metadata
      Name            string
      CreatedAt       time.Time
      LastUsedAt      time.Time

      // Backup state
      BackupEligible  bool
      BackupState     bool
  }
  ```

- [x] Implement credential storage format (defined in connector/local-enhanced/local.go)
- [x] Add CRUD operations (implemented in connector/local-enhanced/storage.go)
- [x] Write unit tests (connector/local-enhanced/storage_test.go and validation_test.go)

**Deliverable**: Storage schemas implemented and tested ✅

---

### Week 4: Storage Backend

#### File Storage Implementation
- [x] Implement file-based storage backend (connector/local-enhanced/storage.go)
- [x] User file operations (create, read, update, delete, list)
- [x] Passkey file operations (save, get, list, delete)
- [x] Session storage for WebAuthn challenges (save, get, delete)
- [x] Magic link token storage (save, get, delete)

#### Storage Interface
- [x] Define Storage interface with all operations
- [x] Implement file-based storage (FileStorage struct)
- [x] Add atomic file operations (using temp files + rename)
- [x] Implement file locking for concurrent access (using syscall.Flock)
- [x] Write comprehensive tests (storage_test.go - 84.1% coverage)

#### Migration Support (DEFERRED)
- [ ] Implement migration from old password-only storage
- [ ] Create migration tool/script
- [ ] Document migration process
- [ ] Test migration with sample data

> **Note**: Migration support deferred until production deployment. Not needed for initial implementation since there are no existing users to migrate.

**Deliverable**: Storage backend complete ✅ (Migration deferred)

---

## Phase 2: Passkey Support

### Week 5: WebAuthn Library Integration

#### WebAuthn Setup
- [x] Initialize WebAuthn library:
  ```go
  import "github.com/go-webauthn/webauthn/webauthn"

  webAuthn, err := webauthn.New(&webauthn.Config{
      RPDisplayName: "Enopax",
      RPID:          "auth.enopax.io",
      RPOrigins:     []string{"https://auth.enopax.io"},
  })
  ```

- [x] Implement User interface for WebAuthn:
  ```go
  func (u *User) WebAuthnID() []byte
  func (u *User) WebAuthnName() string
  func (u *User) WebAuthnDisplayName() string
  func (u *User) WebAuthnIcon() string
  func (u *User) WebAuthnCredentials() []webauthn.Credential
  ```

- [x] Configure RP ID and origins
- [x] Set up challenge generation
- [x] Write unit tests

#### Session Management
- [x] Implement WebAuthn session storage:
  ```go
  type WebAuthnSession struct {
      SessionID   string
      UserID      string
      Challenge   []byte
      Operation   string // "registration" or "authentication"
      ExpiresAt   time.Time
  }
  ```

- [x] Add session creation and validation
- [x] Implement session cleanup (TTL: 5 minutes)
- [x] Write tests

**Deliverable**: WebAuthn library integrated, sessions working ✅

---

### Week 6: Passkey Registration

#### Registration Endpoints
- [x] Implement `POST /auth/passkey/register/begin`:
  ```go
  func (c *Connector) BeginPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
      // 1. Get user from session/token
      // 2. Generate registration options
      // 3. Create and store session
      // 4. Return PublicKeyCredentialCreationOptions
  }
  ```

- [x] Implement `POST /auth/passkey/register/finish`:
  ```go
  func (c *Connector) FinishPasskeyRegistration(w http.ResponseWriter, r *http.Request) {
      // 1. Validate session
      // 2. Parse credential from request
      // 3. Verify credential
      // 4. Store passkey
      // 5. Return success
  }
  ```

- [x] Add request/response validation
- [x] Implement error handling
- [x] Write integration tests

#### Registration UI
- [ ] Create passkey registration template (HTML + JavaScript):
  ```html
  <button id="register-passkey">Register Passkey</button>
  <script>
    async function registerPasskey() {
      const optionsRes = await fetch('/auth/passkey/register/begin');
      const options = await optionsRes.json();

      const credential = await navigator.credentials.create({
        publicKey: options.publicKey
      });

      await fetch('/auth/passkey/register/finish', {
        method: 'POST',
        body: JSON.stringify(credential)
      });
    }
  </script>
  ```

- [ ] Add credential naming functionality
- [ ] Implement error messages
- [ ] Test in multiple browsers

**Deliverable**: Passkey registration working end-to-end

---

### Week 7: Passkey Authentication

#### Authentication Endpoints
- [ ] Implement `POST /auth/passkey/login/begin`:
  ```go
  func (c *Connector) BeginPasskeyAuthentication(w http.ResponseWriter, r *http.Request) {
      // 1. Get user by email (or allow discoverable credentials)
      // 2. Generate authentication options
      // 3. Create and store session
      // 4. Return PublicKeyCredentialRequestOptions
  }
  ```

- [ ] Implement `POST /auth/passkey/login/finish`:
  ```go
  func (c *Connector) FinishPasskeyAuthentication(w http.ResponseWriter, r *http.Request) {
      // 1. Validate session
      // 2. Parse assertion from request
      // 3. Verify signature
      // 4. Update sign count
      // 5. Create OAuth session
      // 6. Return redirect to OAuth flow
  }
  ```

- [ ] Implement signature verification
- [ ] Add sign counter validation (clone detection)
- [ ] Handle discoverable credentials (resident keys)
- [ ] Write integration tests

#### Authentication UI
- [ ] Update login template to include passkey option
- [ ] Implement "Login with Passkey" button
- [ ] Add fallback to password login
- [ ] Test user flows

#### OAuth Integration
- [ ] Integrate passkey auth with Dex OAuth flow
- [ ] Create user identity from passkey authentication
- [ ] Generate OAuth authorization code
- [ ] Test OAuth redirect

**Deliverable**: Passkey authentication complete, integrated with OAuth

---

## Phase 3: TOTP 2FA

### Week 8: TOTP Implementation

#### TOTP Setup
- [ ] Integrate TOTP library:
  ```go
  import "github.com/pquerna/otp/totp"

  key, err := totp.Generate(totp.GenerateOpts{
      Issuer:      "Enopax",
      AccountName: user.Email,
  })
  ```

- [ ] Implement TOTP secret generation
- [ ] Create QR code generation for enrollment
- [ ] Add secret storage in user record

#### TOTP Endpoints
- [ ] Implement `POST /auth/totp/enable`:
  ```go
  func (c *Connector) EnableTOTP(w http.ResponseWriter, r *http.Request) {
      // 1. Generate TOTP secret
      // 2. Return secret and QR code
      // 3. Wait for verification
  }
  ```

- [ ] Implement `POST /auth/totp/verify`:
  ```go
  func (c *Connector) VerifyTOTP(w http.ResponseWriter, r *http.Request) {
      // 1. Get user's TOTP secret
      // 2. Validate TOTP code
      // 3. Mark TOTP as enabled
  }
  ```

- [ ] Implement `POST /auth/totp/validate` (during login):
  ```go
  func (c *Connector) ValidateTOTP(w http.ResponseWriter, r *http.Request) {
      // 1. Get TOTP code from request
      // 2. Validate against user's secret
      // 3. Continue OAuth flow
  }
  ```

- [ ] Add rate limiting (prevent brute force)
- [ ] Write tests

**Deliverable**: TOTP enrollment and validation working

---

### Week 9: 2FA Flow Integration

#### 2FA Login Flow
- [ ] Implement multi-step login:
  ```
  Step 1: Primary auth (password or passkey)
  Step 2: If 2FA required → Prompt for TOTP/Passkey
  Step 3: Validate second factor
  Step 4: Complete OAuth flow
  ```

- [ ] Add 2FA requirement configuration
- [ ] Implement backup codes:
  ```go
  type BackupCode struct {
      Code      string
      Used      bool
      UsedAt    *time.Time
  }
  ```

- [ ] Generate backup codes (10 codes)
- [ ] Add backup code validation
- [ ] Test 2FA flows

#### 2FA UI
- [ ] Create 2FA prompt template
- [ ] Add TOTP code input form
- [ ] Add passkey 2FA option
- [ ] Add "Use backup code" option
- [ ] Display backup codes after TOTP setup

#### Policy Enforcement
- [ ] Implement `Require2FA` user flag
- [ ] Add global 2FA policy configuration
- [ ] Enforce 2FA for admin users
- [ ] Add grace period for 2FA enrollment

**Deliverable**: Complete 2FA support (TOTP + passkey)

---

## Phase 4: Magic Link

### Week 10: Magic Link Authentication

#### Magic Link Implementation
- [ ] Implement token generation:
  ```go
  type MagicLinkToken struct {
      Token      string
      UserID     string
      Email      string
      CreatedAt  time.Time
      ExpiresAt  time.Time
      Used       bool
      IPAddress  string
  }
  ```

- [ ] Create `POST /auth/magic-link/send`:
  ```go
  func (c *Connector) SendMagicLink(w http.ResponseWriter, r *http.Request) {
      // 1. Get email from request
      // 2. Check rate limit
      // 3. Generate secure token
      // 4. Send email
      // 5. Store token
  }
  ```

- [ ] Implement `GET /auth/magic-link/verify?token=...`:
  ```go
  func (c *Connector) VerifyMagicLink(w http.ResponseWriter, r *http.Request) {
      // 1. Validate token
      // 2. Check expiry (10 minutes)
      // 3. Mark as used
      // 4. Create OAuth session
      // 5. Redirect to OAuth flow
  }
  ```

- [ ] Add email rate limiting (3/hour, 10/day)
- [ ] Implement IP binding (optional security)
- [ ] Write tests

#### Email Integration
- [ ] Configure SMTP settings
- [ ] Create magic link email template:
  ```html
  <h1>Your Enopax login link</h1>
  <p>Click the link below to log in:</p>
  <a href="https://auth.enopax.io/auth/magic-link/verify?token=...">
    Log in to Enopax
  </a>
  <p>This link expires in 10 minutes.</p>
  ```

- [ ] Implement email sending
- [ ] Add email delivery error handling
- [ ] Test email delivery

#### Magic Link UI
- [ ] Add "Send magic link" option to login page
- [ ] Create "Check your email" confirmation page
- [ ] Add link expiry message
- [ ] Test user flow

**Deliverable**: Magic link authentication working

---

## Phase 5: gRPC API

### Week 11: Platform Integration API

#### gRPC Service Definition
- [ ] Define protobuf service:
  ```protobuf
  service EnhancedLocal {
    // User management
    rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
    rpc GetUser(GetUserRequest) returns (GetUserResponse);
    rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
    rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);

    // Authentication methods
    rpc SetPassword(SetPasswordRequest) returns (SetPasswordResponse);
    rpc RegisterPasskey(RegisterPasskeyRequest) returns (RegisterPasskeyResponse);
    rpc EnableTOTP(EnableTOTPRequest) returns (EnableTOTPResponse);

    // Credential management
    rpc ListPasskeys(ListPasskeysRequest) returns (ListPasskeysResponse);
    rpc RenamePasskey(RenamePasskeyRequest) returns (RenamePasskeyResponse);
    rpc DeletePasskey(DeletePasskeyRequest) returns (DeletePasskeyResponse);

    // Settings
    rpc UpdateAuthSettings(UpdateAuthSettingsRequest) returns (UpdateAuthSettingsResponse);
  }
  ```

- [ ] Generate Go code from protobuf
- [ ] Implement gRPC server
- [ ] Add authentication/authorization for gRPC calls

#### API Implementation
- [ ] Implement CreateUser endpoint
- [ ] Implement SetPassword endpoint
- [ ] Implement RegisterPasskey endpoint (server-side)
- [ ] Implement credential management endpoints
- [ ] Add input validation
- [ ] Write API tests

#### API Documentation
- [ ] Document all gRPC endpoints
- [ ] Create example usage (Node.js client for Platform)
- [ ] Add error code documentation
- [ ] Write integration guide

**Deliverable**: gRPC API complete and documented

---

## Phase 6: Registration Flow

### Week 12: User Registration & Auth Setup

#### Auth Setup Endpoint
- [ ] Implement `GET /setup-auth?token=...`:
  ```go
  func (c *Connector) ShowAuthSetup(w http.ResponseWriter, r *http.Request) {
      // 1. Validate setup token from Platform
      // 2. Get user info
      // 3. Show auth method selection UI
  }
  ```

- [ ] Implement `POST /setup-auth`:
  ```go
  func (c *Connector) CompleteAuthSetup(w http.ResponseWriter, r *http.Request) {
      // 1. Get selected method (password/passkey/both)
      // 2. Set up chosen method
      // 3. Redirect back to Platform
  }
  ```

- [ ] Add token validation with Platform
- [ ] Implement method selection logic
- [ ] Write tests

#### Auth Setup UI
- [ ] Create auth setup template:
  ```html
  <h1>Choose how to log in</h1>

  <button onclick="setupPasskey()">
    🔐 Set up Passkey (Recommended)
  </button>

  <button onclick="setupPassword()">
    🔑 Set up Password
  </button>

  <button onclick="setupBoth()">
    🔒 Set up Both (Most Secure)
  </button>
  ```

- [ ] Implement setup flows for each option
- [ ] Add progress indicators
- [ ] Test user experience

#### Platform Integration
- [ ] Document Platform registration API
- [ ] Create example Platform code (TypeScript)
- [ ] Add webhook for user creation notification (optional)
- [ ] Test end-to-end registration flow

**Deliverable**: Complete registration flow from Platform to Dex

---

## Phase 7: Testing & Polish

### Week 13: Comprehensive Testing

#### Unit Tests
- [ ] Test storage operations (100% coverage)
- [ ] Test WebAuthn functions
- [ ] Test TOTP functions
- [ ] Test magic link functions
- [ ] Test gRPC API endpoints
- [ ] Run coverage analysis (target: >80%)

#### Integration Tests
- [ ] Test complete passkey registration flow
- [ ] Test complete passkey authentication flow
- [ ] Test 2FA flow (password + TOTP)
- [ ] Test 2FA flow (password + passkey)
- [ ] Test magic link flow
- [ ] Test OAuth integration
- [ ] Test error scenarios

#### End-to-End Tests
- [ ] Set up Playwright/Selenium tests
- [ ] Test passkey registration in real browser
- [ ] Test passkey authentication in real browser
- [ ] Test with virtual authenticator (Chrome DevTools)
- [ ] Test on multiple browsers (Chrome, Safari, Firefox, Edge)
- [ ] Test on multiple platforms (Windows, macOS, Linux, iOS, Android)

#### Performance Tests
- [ ] Load test authentication endpoints
- [ ] Test concurrent passkey registrations
- [ ] Measure authentication latency
- [ ] Test storage backend performance
- [ ] Optimize slow operations

**Deliverable**: Comprehensive test suite, performance validated

---

### Week 14: Documentation & Polish

#### User Documentation
- [ ] Write user guide:
  - How to set up passkey
  - How to set up 2FA
  - How to manage credentials
  - How to recover account
- [ ] Create FAQ
- [ ] Add troubleshooting guide
- [ ] Record demo videos (optional)

#### Developer Documentation
- [ ] Write integration guide for Platform developers
- [ ] Document gRPC API with examples
- [ ] Create migration guide from old local connector
- [ ] Document configuration options
- [ ] Add architecture diagrams

#### Configuration Documentation
- [ ] Document all configuration options:
  ```yaml
  connectors:
    - type: local-enhanced
      id: local
      name: Enopax Auth
      config:
        # Passkey settings
        passkey:
          enabled: true
          rpID: auth.enopax.io
          rpName: Enopax
          userVerification: preferred

        # 2FA settings
        twoFactor:
          required: false
          methods: [totp, passkey]

        # Magic link settings
        magicLink:
          enabled: true
          ttl: 600
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

- [ ] Add configuration examples
- [ ] Document security best practices
- [ ] Create deployment checklist

#### Code Quality
- [ ] Run linters (golangci-lint)
- [ ] Fix all linter warnings
- [ ] Add code comments
- [ ] Refactor complex functions
- [ ] Optimize imports
- [ ] Run `go fmt` on all files

#### Security Audit
- [ ] Review authentication flows for vulnerabilities
- [ ] Check for timing attacks
- [ ] Validate all user inputs
- [ ] Review error messages (no information leakage)
- [ ] Test rate limiting effectiveness
- [ ] Verify HTTPS requirements
- [ ] Check secret storage security

**Deliverable**: Production-ready code, complete documentation

---

## Dependencies

### External Dependencies

#### Go Libraries
```go
// go.mod additions
require (
    github.com/go-webauthn/webauthn v0.11.2
    github.com/pquerna/otp v1.4.0
    github.com/golang-jwt/jwt/v5 v5.2.0
    github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
    // ... existing dependencies
)
```

#### Browser Support
- Chrome 108+ (Windows, macOS, Linux, Android)
- Safari 16+ (macOS, iOS)
- Firefox 119+ (Windows, macOS, Linux)
- Edge 108+ (Windows, macOS)

#### Infrastructure
- SMTP server for magic link emails
- HTTPS with valid TLS certificate (required for WebAuthn)
- File storage with proper permissions

### Internal Dependencies

#### Platform (Next.js App)
- Platform must implement:
  - User registration API
  - Email verification flow
  - gRPC client for Dex API
  - Redirect handling for auth setup

#### Dex Core
- gRPC server infrastructure
- OAuth flow integration
- Template rendering system
- HTTP handler framework

---

## Testing Strategy

### Test Levels

#### L1: Unit Tests (70% coverage minimum)
- All storage operations
- Authentication logic
- Validation functions
- Helper utilities

#### L2: Integration Tests (Key flows)
- Passkey registration + authentication
- 2FA flows
- Magic link flow
- gRPC API calls

#### L3: End-to-End Tests (Critical paths)
- Complete registration from Platform
- Passkey login with real browser
- 2FA login with real browser
- Cross-platform compatibility

### Test Data
- [ ] Create test users with various auth methods
- [ ] Generate test passkeys
- [ ] Create test TOTP secrets
- [ ] Prepare test email templates

### CI/CD Integration
- [ ] Add tests to CI pipeline
- [ ] Run linters automatically
- [ ] Generate coverage reports
- [ ] Block merge if tests fail

---

## Risk Management

### Technical Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| WebAuthn browser compatibility | High | Medium | Extensive cross-browser testing, fallback to password |
| File storage concurrency issues | High | Low | Implement file locking, thorough testing |
| Email delivery failures | Medium | Medium | Retry logic, queue system, error logging |
| OAuth integration complexity | High | Low | Early integration testing, clear documentation |
| Performance issues at scale | Medium | Low | Load testing, optimization, caching |

### Security Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Passkey credential theft | Critical | Very Low | Hardware-bound credentials, user verification |
| TOTP secret exposure | High | Low | Secure storage, encrypted at rest |
| Magic link interception | High | Low | Short TTL, HTTPS only, IP binding |
| Session hijacking | High | Low | Secure cookies, CSRF protection, short sessions |
| Timing attacks | Medium | Low | Constant-time comparisons |

### Project Risks

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Scope creep | Medium | Medium | Strict phase gates, MVP focus |
| Timeline overrun | Medium | Medium | Buffer weeks, regular progress reviews |
| Resource availability | High | Low | Cross-training, documentation |
| Platform integration delays | Medium | Medium | Early API definition, mock implementations |

---

## Success Criteria

### Functional Requirements
- ✅ Users can register passkeys
- ✅ Users can authenticate with passkeys
- ✅ Users can enable TOTP 2FA
- ✅ Users can use magic links
- ✅ Platform can create users via gRPC
- ✅ Multiple auth methods per user work
- ✅ 2FA enforcement works

### Non-Functional Requirements
- ✅ Authentication response time < 200ms (p95)
- ✅ Test coverage > 80%
- ✅ Works on all major browsers
- ✅ HTTPS-only enforcement
- ✅ Production-ready documentation

### User Experience
- ✅ Passkey registration < 30 seconds
- ✅ Clear error messages
- ✅ Mobile-friendly UI
- ✅ Accessible (WCAG 2.1 AA)

---

## Milestones

| Milestone | Target Week | Deliverable |
|-----------|-------------|-------------|
| M0: Setup Complete | Week 2 | Dev environment, dependencies ready |
| M1: Storage Ready | Week 4 | Enhanced storage with migration |
| M2: Passkeys Working | Week 7 | Passkey registration + auth complete |
| M3: 2FA Complete | Week 9 | TOTP + backup codes working |
| M4: Magic Link Ready | Week 10 | Email-based auth working |
| M5: API Published | Week 11 | gRPC API documented |
| M6: Registration Live | Week 12 | End-to-end registration flow |
| M7: Production Ready | Week 14 | Tests pass, docs complete |

---

## Next Steps

1. **Review this plan** with the team
2. **Approve timeline and milestones**
3. **Allocate resources** (developers, testers)
4. **Set up project tracking** (GitHub Projects, Jira, etc.)
5. **Begin Phase 0** (Foundation & Setup)

---

## Notes

- This plan assumes 1-2 developers working full-time
- Timeline can be compressed with more resources
- Some phases can run in parallel (e.g., magic link while testing passkeys)
- Regular reviews after each phase to adjust course

---

**Last Updated**: 2025-11-17
**Version**: 1.0
**Status**: Ready for Review
