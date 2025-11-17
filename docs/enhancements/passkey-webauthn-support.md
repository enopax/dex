# Passkey (WebAuthn) Support for Dex

**Status**: Concept / Proposal
**Created**: 2025-11-17
**Author**: Enopax Platform Team
**Target Branch**: `feature/passkeys`

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Background](#background)
3. [Current State Analysis](#current-state-analysis)
4. [Proposed Solution](#proposed-solution)
5. [Technical Architecture](#technical-architecture)
6. [Implementation Plan](#implementation-plan)
7. [Security Considerations](#security-considerations)
8. [Testing Strategy](#testing-strategy)
9. [Migration Path](#migration-path)
10. [Open Questions](#open-questions)
11. [References](#references)

---

## Executive Summary

This document proposes adding **Passkey (WebAuthn)** authentication support to Dex as a native authentication method. Currently, Dex does not have built-in support for WebAuthn/FIDO2, which limits passwordless authentication options for users.

### Goals

1. **Passwordless Authentication**: Enable users to authenticate using passkeys (biometrics, hardware security keys)
2. **Enhanced Security**: Leverage WebAuthn's phishing-resistant authentication
3. **User Experience**: Simplify login process with modern authentication methods
4. **Platform Support**: Support cross-platform passkeys (iOS, Android, Windows, macOS)

### Non-Goals

- Replace existing connectors (GitHub, LDAP, etc.)
- Implement full FIDO2 device attestation (initial version)
- Support legacy U2F protocol
- Create a custom WebAuthn server from scratch (leverage existing Go libraries)

---

## Background

### What are Passkeys?

Passkeys are a passwordless authentication method based on the **FIDO2** and **WebAuthn** standards:

- **FIDO2**: Industry standard for passwordless authentication
- **WebAuthn**: W3C web standard for public key authentication
- **Passkeys**: Consumer-friendly term for WebAuthn credentials (coined by Apple, Google, Microsoft)

### Why Passkeys?

**Security Benefits**:
- **Phishing-resistant**: Credentials are origin-bound
- **No shared secrets**: Private keys never leave the user's device
- **Strong authentication**: Uses public key cryptography

**User Experience Benefits**:
- **Passwordless**: No password to remember
- **Fast**: Biometric or PIN unlock
- **Cross-platform**: Sync across devices (via platform providers)

### Industry Adoption

- **Apple**: Passkeys in iOS 16+, macOS Ventura+
- **Google**: Passkeys in Android 9+, Chrome
- **Microsoft**: Windows Hello, Edge
- **1Password, Dashlane**: Passkey managers

---

## Current State Analysis

### Dex Architecture

Dex currently supports authentication through:

1. **Connectors** (federation):
   - GitHub, GitLab, Google, Microsoft
   - LDAP, SAML, OAuth2

2. **Local Authentication**:
   - Static passwords (in config)
   - Password database (via storage backend)

### Gaps

- **No native passkey support**: Users cannot register/authenticate with WebAuthn
- **No MFA beyond connectors**: Local auth has no built-in 2FA
- **Limited passwordless options**: Only through external IdPs

### Community Interest

Based on GitHub issues/discussions, there is **significant community interest** in passkey support:
- Multiple feature requests
- Users seeking passwordless authentication
- Interest in FIDO2/WebAuthn integration

---

## Proposed Solution

### Overview

Implement passkey support as a **native Dex connector** (similar to the local password connector), allowing:

1. **Passkey Registration**: Users register WebAuthn credentials
2. **Passkey Authentication**: Users login with registered passkeys
3. **Credential Management**: Users can manage (list, rename, delete) passkeys
4. **Fallback Authentication**: Optional password fallback

### Architecture Choice

**Option A: Standalone Passkey Connector** (Recommended)
- Separate connector for passkey-only authentication
- Clean separation of concerns
- Users choose: password OR passkey

**Option B: Enhanced Local Connector**
- Extend existing local connector with passkey support
- Users can have both password AND passkey
- More complex implementation

**Recommendation**: Start with Option A for simplicity, consider Option B later.

---

## Technical Architecture

### Components

```
┌─────────────────────────────────────────────────────────────┐
│                         DEX SERVER                          │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │           Passkey Connector                          │  │
│  │  - Registration handler                              │  │
│  │  - Authentication handler                            │  │
│  │  - Credential storage interface                      │  │
│  └────────────────┬─────────────────────────────────────┘  │
│                   │                                         │
│                   │                                         │
│  ┌────────────────▼─────────────────────────────────────┐  │
│  │      WebAuthn Library (Go)                           │  │
│  │  - github.com/go-webauthn/webauthn                   │  │
│  │  - Challenge generation                              │  │
│  │  - Credential verification                           │  │
│  │  - CBOR/COSE parsing                                 │  │
│  └────────────────┬─────────────────────────────────────┘  │
│                   │                                         │
│                   │                                         │
│  ┌────────────────▼─────────────────────────────────────┐  │
│  │      Storage Backend                                 │  │
│  │  - Passkey credentials (per user)                    │  │
│  │  - Challenge sessions (temporary)                    │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
└─────────────────────────────────────────────────────────────┘

                           │
                           │ WebAuthn Protocol
                           │
                           ▼

┌─────────────────────────────────────────────────────────────┐
│                    CLIENT (Browser)                         │
│  - navigator.credentials.create()  (Registration)          │
│  - navigator.credentials.get()     (Authentication)        │
└─────────────────────────────────────────────────────────────┘
```

### Storage Schema

#### Passkey Credential

```go
type PasskeyCredential struct {
    ID              []byte    // Credential ID (from WebAuthn)
    UserID          string    // Dex user ID
    PublicKey       []byte    // COSE-encoded public key
    AttestationType string    // "none", "basic", "self", "attCA"
    AAGUID          []byte    // Authenticator AAGUID
    SignCount       uint32    // Signature counter (for cloned device detection)
    Transports      []string  // ["usb", "nfc", "ble", "internal"]

    // User-friendly metadata
    Name            string    // User-assigned name ("iPhone 15", "YubiKey 5")
    CreatedAt       time.Time
    LastUsedAt      time.Time

    // Backup state (for synced passkeys)
    BackupEligible  bool
    BackupState     bool
}
```

#### Challenge Session (temporary)

```go
type WebAuthnSession struct {
    SessionID   string    // Random session ID
    UserID      string    // Dex user ID (for authentication)
    Challenge   []byte    // Random challenge bytes
    Operation   string    // "registration" or "authentication"
    ExpiresAt   time.Time // TTL: 5 minutes
}
```

### API Endpoints

#### 1. Registration Flow

**POST `/auth/passkey/register/begin`**
```json
Request:
{
  "user_id": "user-123",
  "username": "alice@example.com",
  "display_name": "Alice Developer"
}

Response:
{
  "session_id": "sess-abc123",
  "options": {
    "publicKey": {
      "challenge": "base64-encoded-challenge",
      "rp": {
        "name": "Enopax",
        "id": "auth.enopax.io"
      },
      "user": {
        "id": "base64-encoded-user-id",
        "name": "alice@example.com",
        "displayName": "Alice Developer"
      },
      "pubKeyCredParams": [
        {"type": "public-key", "alg": -7},  // ES256
        {"type": "public-key", "alg": -257} // RS256
      ],
      "authenticatorSelection": {
        "residentKey": "preferred",
        "userVerification": "preferred"
      },
      "attestation": "none"
    }
  }
}
```

**POST `/auth/passkey/register/finish`**
```json
Request:
{
  "session_id": "sess-abc123",
  "credential_name": "iPhone 15",
  "credential": {
    "id": "base64-credential-id",
    "rawId": "base64-raw-id",
    "response": {
      "clientDataJSON": "base64-client-data",
      "attestationObject": "base64-attestation"
    },
    "type": "public-key"
  }
}

Response:
{
  "success": true,
  "credential_id": "cred-xyz789"
}
```

#### 2. Authentication Flow

**POST `/auth/passkey/login/begin`**
```json
Request:
{
  "username": "alice@example.com"  // Optional for discoverable credentials
}

Response:
{
  "session_id": "sess-def456",
  "options": {
    "publicKey": {
      "challenge": "base64-encoded-challenge",
      "rpId": "auth.enopax.io",
      "allowCredentials": [  // Omit for discoverable credential flow
        {
          "id": "base64-credential-id",
          "type": "public-key",
          "transports": ["internal", "usb"]
        }
      ],
      "userVerification": "preferred"
    }
  }
}
```

**POST `/auth/passkey/login/finish`**
```json
Request:
{
  "session_id": "sess-def456",
  "credential": {
    "id": "base64-credential-id",
    "rawId": "base64-raw-id",
    "response": {
      "clientDataJSON": "base64-client-data",
      "authenticatorData": "base64-authenticator-data",
      "signature": "base64-signature",
      "userHandle": "base64-user-handle"  // For discoverable credentials
    },
    "type": "public-key"
  }
}

Response:
{
  "success": true,
  "redirect_url": "/approval?req=..."  // Standard Dex OAuth flow
}
```

#### 3. Credential Management

**GET `/api/v2/passkeys`** (gRPC endpoint)
```protobuf
message ListPasskeysRequest {
  string user_id = 1;
}

message ListPasskeysResponse {
  repeated PasskeyInfo passkeys = 1;
}

message PasskeyInfo {
  string id = 1;
  string name = 2;
  string created_at = 3;
  string last_used_at = 4;
  repeated string transports = 5;
  bool backup_eligible = 6;
  bool backup_state = 7;
}
```

**DELETE `/api/v2/passkeys/{id}`** (gRPC endpoint)
```protobuf
message DeletePasskeyRequest {
  string user_id = 1;
  string credential_id = 2;
}

message DeletePasskeyResponse {
  bool success = 1;
}
```

### Go Libraries

**Primary Library**: `github.com/go-webauthn/webauthn`
- Most mature Go WebAuthn library
- Active maintenance (2024-2025)
- Supports WebAuthn Level 2 spec
- Used by production systems

**Installation**:
```bash
go get github.com/go-webauthn/webauthn
```

**Key Types**:
```go
import (
    "github.com/go-webauthn/webauthn/webauthn"
    "github.com/go-webauthn/webauthn/protocol"
)

// WebAuthn instance
webAuthn, err := webauthn.New(&webauthn.Config{
    RPDisplayName: "Enopax",
    RPID:          "auth.enopax.io",
    RPOrigins:     []string{"https://auth.enopax.io"},
})

// User interface (implement for Dex users)
type User interface {
    WebAuthnID() []byte
    WebAuthnName() string
    WebAuthnDisplayName() string
    WebAuthnIcon() string
    WebAuthnCredentials() []webauthn.Credential
}
```

---

## Implementation Plan

### Phase 1: Foundation (Week 1-2)

**Goal**: Set up basic WebAuthn infrastructure

**Tasks**:
- [ ] Add `github.com/go-webauthn/webauthn` dependency
- [ ] Create passkey connector skeleton (`connector/passkey/`)
- [ ] Define storage schema for credentials
- [ ] Implement storage interface methods
- [ ] Add configuration options

**Deliverables**:
- Connector structure
- Storage schema implemented
- Configuration documented

### Phase 2: Registration Flow (Week 3-4)

**Goal**: Enable users to register passkeys

**Tasks**:
- [ ] Implement `/register/begin` endpoint
- [ ] Implement `/register/finish` endpoint
- [ ] Create registration UI (HTML/JS)
- [ ] Handle credential storage
- [ ] Add session management for challenges
- [ ] Write unit tests

**Deliverables**:
- Working registration flow
- Registration UI
- Tests passing

### Phase 3: Authentication Flow (Week 5-6)

**Goal**: Enable passkey-based login

**Tasks**:
- [ ] Implement `/login/begin` endpoint
- [ ] Implement `/login/finish` endpoint
- [ ] Update login UI with passkey option
- [ ] Integrate with Dex OAuth flow
- [ ] Handle signature verification
- [ ] Write unit tests

**Deliverables**:
- Working authentication flow
- Login UI with passkey button
- Tests passing

### Phase 4: Credential Management (Week 7)

**Goal**: Allow users to manage their passkeys

**Tasks**:
- [ ] Implement gRPC list passkeys endpoint
- [ ] Implement gRPC delete passkey endpoint
- [ ] Add credential renaming
- [ ] Create management UI
- [ ] Write integration tests

**Deliverables**:
- Credential management API
- Management UI
- Tests passing

### Phase 5: Testing & Polish (Week 8-9)

**Goal**: Ensure production readiness

**Tasks**:
- [ ] End-to-end testing (browser automation)
- [ ] Cross-platform testing (iOS, Android, Windows, macOS)
- [ ] Security audit
- [ ] Performance testing
- [ ] Documentation
- [ ] Example configuration

**Deliverables**:
- Comprehensive test suite
- Security review complete
- Documentation published

### Phase 6: Advanced Features (Week 10+)

**Goal**: Enhance functionality

**Tasks**:
- [ ] Discoverable credentials (resident keys)
- [ ] Device attestation verification
- [ ] Backup state handling
- [ ] Passkey sync detection
- [ ] Multi-device enrollment

**Deliverables**:
- Advanced features implemented
- Enhanced user experience

---

## Security Considerations

### Threat Model

**Protected Against**:
- ✅ Phishing attacks (origin-bound credentials)
- ✅ Password database breaches (no passwords)
- ✅ Credential stuffing (no shared secrets)
- ✅ Man-in-the-middle (TLS + challenge-response)
- ✅ Session hijacking (short-lived challenges)

**Not Protected Against** (require additional measures):
- ⚠️ Device theft + biometric spoofing (OS-level protection)
- ⚠️ Malware on user device (out of scope)
- ⚠️ Social engineering (user education)

### Security Best Practices

1. **Challenge Generation**:
   - Use cryptographically secure random number generator
   - Challenge length: 32 bytes minimum
   - TTL: 5 minutes (prevent replay attacks)

2. **Origin Validation**:
   - Verify RP ID matches server domain
   - Validate origin in clientDataJSON
   - Use HTTPS only (no HTTP fallback)

3. **Signature Verification**:
   - Verify signature using stored public key
   - Check signature counter (detect cloned authenticators)
   - Validate authenticatorData flags

4. **Storage Security**:
   - Store credentials encrypted at rest (if sensitive)
   - Use secure session storage for challenges
   - Clean up expired challenge sessions

5. **User Verification**:
   - Prefer `userVerification: "preferred"` or `"required"`
   - Ensures biometric/PIN check on device
   - Balance security vs. UX

### HTTPS Requirement

WebAuthn **requires HTTPS** (except localhost for testing):
- Configure TLS for Dex server
- Use Let's Encrypt for certificates
- Test environment can use localhost

---

## Testing Strategy

### Unit Tests

Test coverage for:
- Challenge generation
- Credential registration logic
- Authentication verification
- Storage operations
- Error handling

**Example**:
```go
func TestRegistrationChallenge(t *testing.T) {
    connector := &PasskeyConnector{...}
    challenge, session, err := connector.BeginRegistration(ctx, userID)

    assert.NoError(t, err)
    assert.Len(t, challenge, 32)
    assert.NotNil(t, session)
}
```

### Integration Tests

Test full flows:
- Registration: begin → finish → store credential
- Authentication: begin → finish → verify signature
- Credential management: list → rename → delete

### End-to-End Tests

Use Playwright or Selenium with WebAuthn virtual authenticator:
- Register passkey
- Login with passkey
- Manage credentials

**Example** (Playwright):
```javascript
test('register passkey', async ({ page }) => {
  const client = await page.context().newCDPSession(page);
  await client.send('WebAuthn.enable');
  await client.send('WebAuthn.addVirtualAuthenticator', {
    options: {
      protocol: 'ctap2',
      transport: 'internal',
      hasResidentKey: true,
      hasUserVerification: true,
      isUserVerified: true,
    }
  });

  await page.goto('https://auth.enopax.io/register');
  await page.click('#passkey-register');

  // WebAuthn ceremony happens automatically
  await expect(page.locator('.success')).toBeVisible();
});
```

### Browser Testing Matrix

| Browser | Platform | Support Level |
|---------|----------|---------------|
| Chrome 108+ | Windows, macOS, Linux, Android | ✅ Full support |
| Safari 16+ | macOS, iOS | ✅ Full support |
| Firefox 119+ | Windows, macOS, Linux | ✅ Full support |
| Edge 108+ | Windows, macOS | ✅ Full support |

### Device Testing

- **Platform authenticators**: Touch ID (macOS), Face ID (iOS), Windows Hello
- **Roaming authenticators**: YubiKey, Titan Security Key
- **Virtual authenticators**: Chrome DevTools, Playwright

---

## User Registration & Enrollment

### Critical Design Decision: How Do Users Register?

Since Dex is primarily an **identity federation layer** (OIDC provider), not a traditional "sign-up" service, user registration requires careful consideration.

### Current Dex Behavior

**Dex does NOT provide user registration by default**:
- Users authenticate via **connectors** (GitHub, LDAP, SAML, etc.)
- The **local connector** (password database) requires pre-provisioned users
- No built-in self-service registration UI

### Option B Approach: Enhanced Local Connector with Registration

For passkeys to work effectively, we need **self-service registration**. Here are the registration strategies:

---

### Registration Strategy 1: Platform-Managed Registration (Recommended for Enopax)

**Architecture**: The **Enopax Platform** (Next.js app) handles user registration, then creates the user in Dex via gRPC API.

```
┌─────────────────────────────────────────────────────────────┐
│                  PLATFORM-MANAGED REGISTRATION              │
└─────────────────────────────────────────────────────────────┘

User                Enopax Platform          Dex Server
 |                       |                        |
 |  1. Visit             |                        |
 | platform.enopax.io/   |                        |
 |    signup             |                        |
 ├──────────────────────>|                        |
 |                       |                        |
 |  2. Registration form |                        |
 |  (email, name,        |                        |
 |   choose auth method) |                        |
 |<──────────────────────┤                        |
 |                       |                        |
 |  3. Submit            |                        |
 ├──────────────────────>|                        |
 |                       |                        |
 |                       |  4. Validate &         |
 |                       |     Create user        |
 |                       |     (gRPC API)         |
 |                       ├───────────────────────>|
 |                       |                        |
 |                       |  5. User created       |
 |                       |<───────────────────────┤
 |                       |                        |
 |                       |  6. Register passkey/  |
 |                       |     password via Dex   |
 |                       ├───────────────────────>|
 |                       |                        |
 |  7. Account ready     |                        |
 |<──────────────────────┤                        |
 |                       |                        |
 |  8. Redirect to login |                        |
 |  (OAuth flow)         |                        |
 ├──────────────────────>├───────────────────────>|
```

**Pros**:
- ✅ **Full control** over registration UX
- ✅ **Integration** with Enopax platform logic (org creation, team setup)
- ✅ **Customizable** registration fields
- ✅ **Billing integration** (can require payment during signup)
- ✅ **Email verification** before Dex account creation

**Cons**:
- ❌ Requires gRPC API for user management
- ❌ Platform becomes source of truth for user lifecycle
- ❌ More complexity (two systems)

**Implementation**:
```go
// Enopax Platform (Next.js API Route)
// POST /api/auth/register
async function registerUser(req, res) {
  const { email, name, authMethod } = req.body;

  // 1. Validate input
  if (!isValidEmail(email)) {
    return res.status(400).json({ error: 'Invalid email' });
  }

  // 2. Check if user exists in Platform DB
  const existingUser = await prisma.user.findUnique({ where: { email } });
  if (existingUser) {
    return res.status(409).json({ error: 'User already exists' });
  }

  // 3. Send verification email
  const verificationToken = generateToken();
  await sendVerificationEmail(email, verificationToken);

  // 4. Store pending user
  await prisma.pendingUser.create({
    data: { email, name, authMethod, verificationToken }
  });

  return res.json({ message: 'Check your email to verify' });
}

// After email verification
async function verifyAndCreateUser(token) {
  const pendingUser = await prisma.pendingUser.findUnique({
    where: { verificationToken: token }
  });

  // 5. Create user in Dex via gRPC
  const dexClient = createDexClient();
  const dexUser = await dexClient.createUser({
    email: pendingUser.email,
    username: pendingUser.email,
    displayName: pendingUser.name,
  });

  // 6. Create user in Platform DB
  await prisma.user.create({
    data: {
      id: dexUser.id,
      email: pendingUser.email,
      name: pendingUser.name,
    }
  });

  // 7. Redirect to auth method setup
  if (pendingUser.authMethod === 'passkey') {
    return { redirect: '/setup-passkey' };
  } else if (pendingUser.authMethod === 'password') {
    return { redirect: '/setup-password' };
  }
}
```

---

### Registration Strategy 2: Dex Self-Service Registration (Alternative)

**Architecture**: Dex provides its own registration UI and handles user creation directly.

```
┌─────────────────────────────────────────────────────────────┐
│                  DEX SELF-SERVICE REGISTRATION              │
└─────────────────────────────────────────────────────────────┘

User                     Dex Server
 |                           |
 |  1. Visit                 |
 | auth.enopax.io/register   |
 ├──────────────────────────>|
 |                           |
 |  2. Registration form     |
 |  (built into Dex)         |
 |<──────────────────────────┤
 |                           |
 |  3. Submit                |
 | (email, name, choose      |
 |  password/passkey)        |
 ├──────────────────────────>|
 |                           |
 |                           |  4. Create user
 |                           |     in storage
 |                           |
 |  5. Setup auth method     |
 |  (password or passkey)    |
 |<──────────────────────────┤
 |                           |
 |  6. Complete setup        |
 ├──────────────────────────>|
 |                           |
 |  7. Redirect to OAuth     |
 |     client (Platform)     |
 |<──────────────────────────┤
```

**Pros**:
- ✅ **Self-contained** (Dex handles everything)
- ✅ **Simpler** for Dex-only deployments
- ✅ **No gRPC dependency** from Platform

**Cons**:
- ❌ **Less control** over registration UX
- ❌ **Platform doesn't know** about new users until first login
- ❌ **No email verification** (unless built into Dex)
- ❌ **No integration** with Platform onboarding

**Implementation**:
- Add `/register` endpoint to Dex
- Create registration UI templates
- Handle user creation in storage backend
- Webhook/event system to notify Platform (optional)

---

### Recommended Registration Flow for Enopax (Option B)

**Hybrid Approach**: Platform-initiated, Dex-completed

```
┌─────────────────────────────────────────────────────────────┐
│             RECOMMENDED: HYBRID REGISTRATION                │
└─────────────────────────────────────────────────────────────┘

Step 1: Platform Handles Signup
  - User visits platform.enopax.io/signup
  - User enters email, name, organisation details
  - Platform validates and sends verification email
  - User clicks verification link

Step 2: Platform Creates User Shell
  - Platform creates user in Platform DB
  - Platform creates user in Dex via gRPC (minimal info)
  - User record exists but has NO auth method yet

Step 3: Redirect to Dex for Auth Method Setup
  - Platform generates secure token
  - Redirects to: auth.enopax.io/setup-auth?token=...
  - Dex validates token with Platform (API call)

Step 4: User Chooses Auth Method
  - Dex shows: "Choose how to log in"
    ☐ Set a password
    ☐ Register a passkey
    ☐ Both (password + passkey for 2FA)

Step 5: User Completes Auth Setup
  - If passkey: WebAuthn registration ceremony
  - If password: Set password (with strength requirements)
  - If both: Set password first, then register passkey

Step 6: Redirect Back to Platform
  - Dex redirects to Platform with OAuth code
  - User logs in automatically
  - Platform onboarding flow begins
```

**Code Example**:

```typescript
// Platform: /api/auth/signup
export async function POST(req: Request) {
  const { email, name, orgName } = await req.json();

  // 1. Validate email
  if (await userExists(email)) {
    return Response.json({ error: 'Email already registered' }, { status: 409 });
  }

  // 2. Create pending signup
  const token = crypto.randomBytes(32).toString('hex');
  await prisma.pendingSignup.create({
    data: { email, name, orgName, token, expiresAt: addMinutes(new Date(), 30) }
  });

  // 3. Send verification email
  await sendEmail({
    to: email,
    subject: 'Verify your Enopax account',
    body: `Click here: https://platform.enopax.io/verify?token=${token}`
  });

  return Response.json({ message: 'Check your email' });
}

// Platform: /api/auth/verify
export async function GET(req: Request) {
  const { searchParams } = new URL(req.url);
  const token = searchParams.get('token');

  const pending = await prisma.pendingSignup.findUnique({ where: { token } });
  if (!pending || pending.expiresAt < new Date()) {
    return Response.json({ error: 'Invalid or expired token' }, { status: 400 });
  }

  // 1. Create user in Platform DB
  const user = await prisma.user.create({
    data: {
      email: pending.email,
      name: pending.name,
      emailVerified: true,
    }
  });

  // 2. Create user in Dex via gRPC
  const dexClient = createDexClient();
  await dexClient.createUser({
    email: pending.email,
    username: pending.email,
    displayName: pending.name,
  });

  // 3. Generate setup token for Dex
  const setupToken = signJWT({ userId: user.id, email: user.email });

  // 4. Redirect to Dex auth setup
  return Response.redirect(
    `https://auth.enopax.io/setup-auth?token=${setupToken}&redirect_uri=${encodeURIComponent('https://platform.enopax.io/welcome')}`
  );
}
```

```go
// Dex: /setup-auth endpoint
func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    redirectURI := r.URL.Query().Get("redirect_uri")

    // 1. Validate token with Platform
    user, err := s.validateSetupToken(token)
    if err != nil {
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }

    // 2. Show auth method selection UI
    if r.Method == "GET" {
        s.templates.Render(w, "setup-auth.html", map[string]interface{}{
            "Email": user.Email,
            "Name":  user.Name,
            "Token": token,
        })
        return
    }

    // 3. Handle auth method setup
    authMethod := r.FormValue("method")
    switch authMethod {
    case "password":
        password := r.FormValue("password")
        if err := s.setUserPassword(user.ID, password); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
    case "passkey":
        // Redirect to passkey registration flow
        http.Redirect(w, r, "/passkey/register/begin?user_id="+user.ID, http.StatusSeeOther)
        return
    case "both":
        // Set password first, then passkey
        password := r.FormValue("password")
        if err := s.setUserPassword(user.ID, password); err != nil {
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }
        http.Redirect(w, r, "/passkey/register/begin?user_id="+user.ID, http.StatusSeeOther)
        return
    }

    // 4. Redirect back to Platform
    http.Redirect(w, r, redirectURI, http.StatusSeeOther)
}
```

---

### Registration UI Flow

**Platform Signup Page** (`platform.enopax.io/signup`):
```html
┌─────────────────────────────────────────────────────────┐
│              Create your Enopax account                 │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Email                                                  │
│  ┌──────────────────────────────────────┐              │
│  │ you@company.com                      │              │
│  └──────────────────────────────────────┘              │
│                                                         │
│  Full Name                                              │
│  ┌──────────────────────────────────────┐              │
│  │ Alice Developer                      │              │
│  └──────────────────────────────────────┘              │
│                                                         │
│  Organisation Name                                      │
│  ┌──────────────────────────────────────┐              │
│  │ Acme Corp                            │              │
│  └──────────────────────────────────────┘              │
│                                                         │
│  ☑ I agree to Terms of Service                         │
│                                                         │
│  ┌──────────────────────────────────────┐              │
│  │   Create Account                     │              │
│  └──────────────────────────────────────┘              │
│                                                         │
│  Already have an account? Log in                       │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

**Dex Auth Setup Page** (`auth.enopax.io/setup-auth`):
```html
┌─────────────────────────────────────────────────────────┐
│         Welcome, alice@company.com!                     │
│         Choose how you want to log in                   │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  🔐  Passkey (Recommended)                              │
│      Use your fingerprint, face, or security key       │
│      ┌──────────────────────────────────────┐          │
│      │   Set up Passkey                     │          │
│      └──────────────────────────────────────┘          │
│      ✓ Faster login                                     │
│      ✓ More secure                                      │
│      ✓ No password to remember                          │
│                                                         │
│  ─────────────── OR ────────────────                   │
│                                                         │
│  🔑  Password                                           │
│      Traditional password-based login                   │
│      ┌──────────────────────────────────────┐          │
│      │   Set up Password                    │          │
│      └──────────────────────────────────────┘          │
│                                                         │
│  ─────────────── OR ────────────────                   │
│                                                         │
│  🔒  Both (Most Secure)                                 │
│      Password + Passkey for two-factor authentication  │
│      ┌──────────────────────────────────────┐          │
│      │   Set up Both                        │          │
│      └──────────────────────────────────────┘          │
│      ✓ Strongest security                               │
│      ✓ Required for admin accounts                      │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

---

### gRPC API Requirements

To support Platform-managed registration, Dex needs these gRPC endpoints:

```protobuf
// User management
service Dex {
  // Create a new user (without auth method)
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);

  // Set password for a user
  rpc SetPassword(SetPasswordRequest) returns (SetPasswordResponse);

  // Register passkey for a user
  rpc RegisterPasskey(RegisterPasskeyRequest) returns (RegisterPasskeyResponse);

  // Get user by email
  rpc GetUserByEmail(GetUserByEmailRequest) returns (GetUserByEmailResponse);

  // Update user details
  rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);

  // Delete user
  rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
}

message CreateUserRequest {
  string email = 1;
  string username = 2;
  string display_name = 3;
  bool email_verified = 4;
}

message CreateUserResponse {
  string user_id = 1;
}
```

---

### Email Verification Best Practices

**Why Email Verification Matters for Passkeys**:
1. **Prevent account takeover**: Ensure user owns the email
2. **Passkey binding**: Passkeys should be bound to verified identities
3. **Recovery**: Email is the recovery method if passkey is lost
4. **Trust**: Verified emails can receive magic links

**Implementation**:
```typescript
// Platform: Email verification flow
async function sendVerificationEmail(email: string, token: string) {
  await sendEmail({
    to: email,
    subject: 'Verify your Enopax account',
    html: `
      <h1>Welcome to Enopax!</h1>
      <p>Click the link below to verify your email and complete registration:</p>
      <a href="https://platform.enopax.io/verify?token=${token}">
        Verify Email
      </a>
      <p>This link expires in 30 minutes.</p>
      <p>If you didn't create an account, you can safely ignore this email.</p>
    `
  });
}
```

---

## Migration Path

### For Existing Users

**Scenario 1: Password → Passkey Migration**
1. User logs in with password
2. After login, prompt: "Add a passkey for faster login?"
3. User registers passkey
4. Next login: option to use passkey OR password

**Scenario 2: New Users (Passkey-Only)**
1. User completes Platform signup
2. User chooses "Set up Passkey" during auth setup
3. No password set (fully passwordless)

**Scenario 3: New Users (2FA from Day 1)**
1. User completes Platform signup
2. User chooses "Both" during auth setup
3. Sets password first, then registers passkey
4. All future logins require both factors

### Configuration

```yaml
# Passkey connector config
connectors:
  - type: passkey
    id: passkey
    name: Passkey
    config:
      # Relying Party information
      rpID: auth.enopax.io
      rpName: Enopax
      rpOrigins:
        - https://auth.enopax.io
        - https://platform.enopax.io  # Allow multiple origins

      # User verification preference
      userVerification: preferred  # "required", "preferred", "discouraged"

      # Authenticator attachment
      authenticatorAttachment: cross-platform  # "platform", "cross-platform", or omit for both

      # Resident key (discoverable credentials)
      residentKey: preferred  # "required", "preferred", "discouraged"

      # Attestation (for enterprise deployments)
      attestation: none  # "none", "indirect", "direct", "enterprise"

      # Timeout (milliseconds)
      timeout: 60000  # 60 seconds

      # Allow fallback to password
      passwordFallback: true
```

### Backward Compatibility

- Passkey connector is **opt-in** (doesn't affect existing setups)
- Can run alongside other connectors
- No breaking changes to existing APIs

---

## Open Questions

### 1. Credential Storage Location

**Question**: Should passkey credentials be stored separately from passwords or in the same `passwords` storage?

**Options**:
- **Separate storage** (`passkeys/` directory in file storage)
  - Pros: Clean separation, easier to manage
  - Cons: Another storage location to maintain
- **Combined storage** (extend user object with passkey array)
  - Pros: Single source of truth per user
  - Cons: Mixing different credential types

**Recommendation**: Separate storage for clarity.

### 2. Discoverable Credentials

**Question**: Should we require resident keys (discoverable credentials) or allow non-resident keys?

**Options**:
- **Prefer resident keys** (`residentKey: "preferred"`)
  - Pros: Better UX (no username entry), modern
  - Cons: Not all authenticators support it
- **Allow both**
  - Pros: Wider device compatibility
  - Cons: UX varies by authenticator

**Recommendation**: Prefer resident keys, allow both.

### 3. Passkey + Password Accounts

**Question**: Should users be able to have both password AND passkey?

**Options**:
- **Both allowed**
  - Pros: Flexibility, gradual migration
  - Cons: More complex auth logic
- **Either/or**
  - Pros: Simpler, clearer authentication method
  - Cons: Forces users to choose

**Recommendation**: Allow both initially, option to disable password later.

### 4. Multi-Passkey Support

**Question**: Should users be allowed to register multiple passkeys?

**Answer**: **Yes** (essential for:
- Multiple devices (phone, laptop, hardware key)
- Backup keys
- Device replacement

**Implementation**: Store array of credentials per user.

### 5. Backup State Handling

**Question**: How should we handle backup-eligible passkeys (synced via iCloud, Google Password Manager)?

**Options**:
- **Trust backup state** (allow synced passkeys)
  - Pros: Better UX, user expectation
  - Cons: Less control over credential lifecycle
- **Restrict to device-bound**
  - Pros: Higher security assurance
  - Cons: Poor UX (lose passkey if device lost)

**Recommendation**: Trust backup state (align with industry standards).

---

## References

### Standards & Specifications

- [WebAuthn Level 2 Specification (W3C)](https://www.w3.org/TR/webauthn-2/)
- [WebAuthn Level 3 Draft](https://www.w3.org/TR/webauthn-3/)
- [CTAP2 Specification (FIDO Alliance)](https://fidoalliance.org/specs/fido-v2.1-ps-20210615/fido-client-to-authenticator-protocol-v2.1-ps-20210615.html)

### Libraries & Tools

- [go-webauthn/webauthn (Go)](https://github.com/go-webauthn/webauthn)
- [SimpleWebAuthn (JavaScript)](https://github.com/MasterKale/SimpleWebAuthn)
- [webauthn.guide (Educational)](https://webauthn.guide/)
- [webauthn.io (Demo)](https://webauthn.io/)

### Dex Documentation

- [Dex Documentation](https://dexidp.io/docs/)
- [Dex Connectors](https://dexidp.io/docs/connectors/)
- [Dex Storage](https://dexidp.io/docs/storage/)

### Industry Resources

- [FIDO Alliance](https://fidoalliance.org/)
- [Passkeys.dev](https://passkeys.dev/)
- [Apple Passkeys](https://developer.apple.com/passkeys/)
- [Google Passkeys](https://developers.google.com/identity/passkeys)

---

## Appendix

### Example User Flow (Diagrams)

#### Registration Flow

```
User                Browser             Dex Server           Storage
 |                     |                     |                  |
 |  1. Click          |                     |                  |
 | "Register Passkey" |                     |                  |
 |------------------->|                     |                  |
 |                    |  2. POST /begin     |                  |
 |                    |-------------------->|                  |
 |                    |                     |  3. Generate     |
 |                    |                     |     challenge    |
 |                    |                     |                  |
 |                    |  4. Challenge +     |                  |
 |                    |     PublicKeyOptions|                  |
 |                    |<--------------------|                  |
 |                    |                     |                  |
 |  5. navigator.     |                     |                  |
 |  credentials.      |                     |                  |
 |  create()          |                     |                  |
 |<-------------------|                     |                  |
 |                    |                     |                  |
 |  6. Biometric/PIN  |                     |                  |
 |     verification   |                     |                  |
 |                    |                     |                  |
 |  7. Credential     |                     |                  |
 |------------------->|                     |                  |
 |                    |  8. POST /finish    |                  |
 |                    |     + credential    |                  |
 |                    |-------------------->|                  |
 |                    |                     |  9. Verify &     |
 |                    |                     |     Store        |
 |                    |                     |----------------->|
 |                    |                     |                  |
 |                    |  10. Success        |                  |
 |                    |<--------------------|                  |
 |  11. Confirmation  |                     |                  |
 |<-------------------|                     |                  |
```

#### Authentication Flow

```
User                Browser             Dex Server           Storage
 |                     |                     |                  |
 |  1. Click          |                     |                  |
 | "Login with        |                     |                  |
 |  Passkey"          |                     |                  |
 |------------------->|                     |                  |
 |                    |  2. POST /login/    |                  |
 |                    |     begin           |                  |
 |                    |-------------------->|                  |
 |                    |                     |  3. Get user's   |
 |                    |                     |     credentials  |
 |                    |                     |<-----------------|
 |                    |                     |                  |
 |                    |  4. Challenge +     |                  |
 |                    |     allowCredentials|                  |
 |                    |<--------------------|                  |
 |                    |                     |                  |
 |  5. navigator.     |                     |                  |
 |  credentials.get() |                     |                  |
 |<-------------------|                     |                  |
 |                    |                     |                  |
 |  6. Biometric/PIN  |                     |                  |
 |     verification   |                     |                  |
 |                    |                     |                  |
 |  7. Assertion      |                     |                  |
 |------------------->|                     |                  |
 |                    |  8. POST /login/    |                  |
 |                    |     finish          |                  |
 |                    |-------------------->|                  |
 |                    |                     |  9. Verify       |
 |                    |                     |     signature    |
 |                    |                     |<-----------------|
 |                    |                     |                  |
 |                    |  10. OAuth redirect |                  |
 |                    |<--------------------|                  |
 |  11. Logged in     |                     |                  |
 |<-------------------|                     |                  |
```

---

## Next Steps

1. **Team Review**: Share this document with the team for feedback
2. **Prototype**: Create proof-of-concept implementation
3. **Security Review**: Conduct security assessment
4. **Community Input**: Share with Dex community (upstream)
5. **Implementation**: Begin Phase 1 development

---

**Document Version**: 1.0
**Last Updated**: 2025-11-17
**Feedback**: Submit issues or PRs to `enopax/dex` repository
