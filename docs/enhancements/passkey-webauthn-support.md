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

## Migration Path

### For Existing Users

**Scenario 1: Password → Passkey Migration**
1. User logs in with password
2. After login, prompt: "Add a passkey for faster login?"
3. User registers passkey
4. Next login: option to use passkey OR password

**Scenario 2: New Users (Passkey-Only)**
1. User chooses "Sign up with passkey"
2. Register passkey during account creation
3. No password set (fully passwordless)

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
