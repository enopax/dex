# Authentication Flows Documentation

**Project**: Dex Enhanced Local Connector
**Version**: 1.0
**Last Updated**: 2025-11-18
**Status**: Complete

---

## Table of Contents

1. [Overview](#overview)
2. [User Registration Flow](#user-registration-flow)
3. [Passkey Authentication Flow](#passkey-authentication-flow)
4. [Password Authentication Flow](#password-authentication-flow)
5. [Magic Link Authentication Flow](#magic-link-authentication-flow)
6. [Two-Factor Authentication (2FA) Flow](#two-factor-authentication-2fa-flow)
7. [TOTP Setup Flow](#totp-setup-flow)
8. [OAuth Integration](#oauth-integration)
9. [Error Handling](#error-handling)
10. [Security Considerations](#security-considerations)

---

## Overview

This document describes all authentication flows supported by the Enhanced Local Connector for Dex. The connector supports multiple authentication methods and can enforce two-factor authentication (2FA) based on configurable policies.

### Supported Authentication Methods

| Method | Type | Requires Enrollment | Passwordless |
|--------|------|---------------------|--------------|
| **Passkey** | WebAuthn | Yes | Yes |
| **Password** | Traditional | No (optional) | No |
| **Magic Link** | Email-based | No | Yes |
| **TOTP** | 2FA only | Yes | No |
| **Backup Code** | 2FA only | Yes (via TOTP) | No |

### Authentication Modes

1. **Single-Factor Authentication** - One method (password, passkey, or magic link)
2. **Two-Factor Authentication** - Primary method + secondary verification
3. **Passwordless Authentication** - Passkey or magic link only

---

## User Registration Flow

### Overview

User registration is initiated by the Enopax Platform via gRPC API. After creating a user account, the Platform generates an auth setup token and redirects the user to configure their authentication method(s).

### Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    USER REGISTRATION FLOW                    │
└─────────────────────────────────────────────────────────────┘

Platform (Next.js)                     Dex Enhanced Connector
       │                                         │
       │ 1. POST /api/users (Next.js API)       │
       │ Create user in Platform database       │
       │ ────────────────────────────────►      │
       │                                         │
       │ 2. gRPC: CreateUser()                  │
       │ ────────────────────────────────────────►
       │                                         │
       │ ◄────────────────────────────────────── │
       │ Returns: user_id, email                 │
       │                                         │
       │ 3. Generate AuthSetupToken             │
       │ (Platform's database)                  │
       │ ────┐                                   │
       │     │                                   │
       │ ◄───┘                                   │
       │                                         │
       │ 4. Send Email with Setup Link          │
       │ https://auth.enopax.io/setup-auth?token=...
       │ ────────────────────────────────►      │
       │                              User's Email
       │                                         │
       ▼                                         │
                                                 │
User clicks link in email                       │
       │                                         │
       │ 5. GET /setup-auth?token=...           │
       │ ────────────────────────────────────────►
       │                                         │
       │ ◄────────────────────────────────────── │
       │ Returns: Auth setup page (HTML)        │
       │ Options: Passkey, Password, Both        │
       │                                         │
       │ 6. User chooses authentication method  │
       │ ────┐                                   │
       │     │                                   │
       │ ◄───┘                                   │
       │                                         │
       │ Option A: Set up Passkey               │
       │ 7a. POST /passkey/register/begin        │
       │ ────────────────────────────────────────►
       │                                         │
       │ ◄────────────────────────────────────── │
       │ Returns: WebAuthn challenge + options   │
       │                                         │
       │ 8a. navigator.credentials.create()      │
       │ (Browser WebAuthn API)                  │
       │ ────┐                                   │
       │     │ User verifies with biometric/PIN │
       │ ◄───┘                                   │
       │                                         │
       │ 9a. POST /passkey/register/finish       │
       │ (Credential response)                   │
       │ ────────────────────────────────────────►
       │                                         │
       │ ◄────────────────────────────────────── │
       │ Returns: success, passkey_id            │
       │                                         │
       │ Option B: Set up Password              │
       │ 7b. POST /setup-auth/password           │
       │ {user_id, password}                     │
       │ ────────────────────────────────────────►
       │                                         │
       │ ◄────────────────────────────────────── │
       │ Returns: success                        │
       │                                         │
       │ 10. Redirect to Platform dashboard      │
       │ ────────────────────────────────────────►
       ▼                                Platform
```

### Steps

#### 1. Platform Creates User

Platform creates user record in its own database (Next.js + PostgreSQL).

```typescript
// Platform API (Next.js)
POST /api/users
{
  "email": "alice@example.com",
  "username": "alice",
  "displayName": "Alice Smith"
}
```

#### 2. Platform Calls Dex gRPC API

Platform calls `CreateUser` RPC to create user in Dex storage.

```typescript
// Platform Backend (gRPC Client)
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';

const client = new proto.api.EnhancedLocalConnector(
  'localhost:5557',
  grpc.credentials.createInsecure()
);

const response = await client.CreateUser({
  email: 'alice@example.com',
  username: 'alice',
  displayName: 'Alice Smith'
});

console.log('User created:', response.user.id);
```

#### 3. Platform Generates Auth Setup Token

Platform creates an auth setup token in its own database.

```typescript
const token = generateSecureToken(); // 32-byte random token
const authSetupToken = {
  token: token,
  userId: response.user.id,
  email: 'alice@example.com',
  createdAt: new Date(),
  expiresAt: new Date(Date.now() + 24 * 60 * 60 * 1000), // 24 hours
  used: false,
  returnUrl: 'https://platform.enopax.io/dashboard'
};

await db.authSetupTokens.create(authSetupToken);
```

#### 4. Platform Sends Setup Email

Platform sends email with auth setup link.

```typescript
await emailService.send({
  to: 'alice@example.com',
  subject: 'Complete Your Enopax Account Setup',
  html: `
    <h1>Welcome to Enopax!</h1>
    <p>Click the link below to set up your authentication:</p>
    <a href="https://auth.enopax.io/setup-auth?token=${token}">
      Set Up Authentication
    </a>
    <p>This link expires in 24 hours.</p>
  `
});
```

#### 5. User Accesses Setup Page

User clicks link and lands on Dex auth setup page.

**Request**:
```http
GET /setup-auth?token=abc123...xyz
```

**Response** (HTML page with options):
- Set up Passkey (Recommended)
- Set up Password
- Set up Both (Most Secure)

#### 6. User Chooses Authentication Method

**Option A: Passkey Setup**

User clicks "Set up Passkey" button, which triggers WebAuthn registration flow (see [Passkey Registration](#passkey-registration-flow) for details).

**Option B: Password Setup**

User enters desired password in form.

**Request**:
```http
POST /setup-auth/password
Content-Type: application/json

{
  "user_id": "user-uuid",
  "password": "SecurePass123"
}
```

**Response**:
```json
{
  "success": true,
  "message": "Password set successfully"
}
```

#### 7. Redirect to Platform

After successful auth setup, user is redirected to Platform dashboard.

```http
HTTP/1.1 302 Found
Location: https://platform.enopax.io/dashboard
```

### gRPC API Reference

**CreateUser RPC**:

```protobuf
rpc CreateUser(CreateUserReq) returns (CreateUserResp);

message CreateUserReq {
  string email = 1;
  string username = 2;
  string display_name = 3;
}

message CreateUserResp {
  EnhancedUser user = 1;
  bool already_exists = 2;  // True if user already existed
}
```

**SetPassword RPC** (Alternative to web form):

```protobuf
rpc SetPassword(SetPasswordReq) returns (SetPasswordResp);

message SetPasswordReq {
  string user_id = 1;
  string password = 2;
}

message SetPasswordResp {
  bool success = 1;
  bool not_found = 2;  // True if user not found
}
```

### Security Considerations

1. **Token Security**:
   - Auth setup tokens are cryptographically random (32 bytes)
   - Tokens expire after 24 hours
   - One-time use only
   - Stored with file permissions 0600

2. **Password Validation**:
   - Minimum 8 characters
   - Maximum 128 characters
   - At least one letter
   - At least one number
   - Hashed with bcrypt (cost 10)

3. **Email Delivery**:
   - Use HTTPS for all setup links
   - Include token expiry in email
   - Warn users not to share links

---

## Passkey Authentication Flow

### Overview

Passkey authentication uses the WebAuthn standard to provide passwordless, phishing-resistant authentication. Users authenticate with biometrics (Touch ID, Face ID, Windows Hello) or a hardware security key.

### Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                  PASSKEY AUTHENTICATION FLOW                │
└─────────────────────────────────────────────────────────────┘

Browser                      Dex Connector                 OAuth Client
   │                               │                             │
   │ 1. OAuth Authorization        │                             │
   │ Request with login_hint       │                             │
   │ ──────────────────────────────────────────────────────────► │
   │                               │                             │
   │ ◄───────────────────────────────────────────────────────── │
   │ 2. Redirect to Dex            │                             │
   │ /auth?client_id=...&state=... │                             │
   │                               │                             │
   │ 3. GET /login?state=...&callback=...                       │
   │ ──────────────────────────────►                            │
   │                               │                             │
   │ ◄──────────────────────────── │                             │
   │ 4. Login page (HTML)          │                             │
   │ [Login with Passkey] button   │                             │
   │                               │                             │
   │ 5. Click "Login with Passkey" │                             │
   │ ────┐                         │                             │
   │     │                         │                             │
   │ ◄───┘                         │                             │
   │                               │                             │
   │ 6. POST /passkey/login/begin  │                             │
   │ {email: "alice@example.com"}  │                             │
   │ ──────────────────────────────►                            │
   │                               │                             │
   │ ◄──────────────────────────── │                             │
   │ 7. WebAuthn challenge         │                             │
   │ {session_id, options}         │                             │
   │                               │                             │
   │ 8. navigator.credentials.get() │                            │
   │ ────┐                         │                             │
   │     │ Browser prompts for     │                             │
   │     │ biometric/PIN           │                             │
   │ ◄───┘                         │                             │
   │                               │                             │
   │ 9. POST /passkey/login/finish │                             │
   │ {session_id, credential}      │                             │
   │ ──────────────────────────────►                            │
   │                               │                             │
   │                               │ 10. Verify signature        │
   │                               │     Update sign counter     │
   │                               │     Update last login       │
   │                               │ ────┐                       │
   │                               │     │                       │
   │                               │ ◄───┘                       │
   │                               │                             │
   │                               │ 11. Check if 2FA required   │
   │                               │ ────┐                       │
   │                               │     │                       │
   │                               │ ◄───┘                       │
   │                               │                             │
   │ If 2FA not required:          │                             │
   │ ◄──────────────────────────── │                             │
   │ 12. Redirect to OAuth callback│                             │
   │ /callback?state=...&user_id=..│                             │
   │                               │                             │
   │ 13. Exchange code for tokens  │                             │
   │ ───────────────────────────────────────────────────────────►│
   │                               │                             │
   │ ◄──────────────────────────────────────────────────────────│
   │ 14. ID token + access token   │                             │
   │                               │                             │
   │ If 2FA required (see 2FA Flow)│                             │
   ▼                               ▼                             ▼
```

### Steps

#### 1. User Initiates Login

User navigates to OAuth client app and clicks "Log in with Enopax".

```http
GET /authorize?
  client_id=example-app&
  redirect_uri=https://app.example.com/callback&
  response_type=code&
  scope=openid email profile&
  state=random-state-value
```

#### 2. Dex Redirects to Connector

Dex redirects user to enhanced local connector login page.

```http
HTTP/1.1 302 Found
Location: https://auth.enopax.io/login?state=AUTH_REQ_ID&callback=https://dex.example.com/callback
```

#### 3. User Sees Login Page

Login page displays authentication options:
- **Login with Passkey** (primary button)
- Email/Password form (fallback)
- Send Magic Link (if enabled)

#### 4. User Clicks "Login with Passkey"

JavaScript initiates passkey authentication.

```javascript
async function loginWithPasskey() {
  // Step 1: Begin authentication
  const beginResp = await fetch('/passkey/login/begin', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      email: 'alice@example.com'  // Optional - can be empty for discoverable credentials
    })
  });

  const { session_id, options } = await beginResp.json();

  // Step 2: Call WebAuthn API
  const credential = await navigator.credentials.get({
    publicKey: options.publicKey
  });

  // Step 3: Finish authentication
  const finishResp = await fetch('/passkey/login/finish', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({
      session_id: session_id,
      credential: {
        id: credential.id,
        type: credential.type,
        rawId: arrayBufferToBase64(credential.rawId),
        response: {
          clientDataJSON: arrayBufferToBase64(credential.response.clientDataJSON),
          authenticatorData: arrayBufferToBase64(credential.response.authenticatorData),
          signature: arrayBufferToBase64(credential.response.signature),
          userHandle: arrayBufferToBase64(credential.response.userHandle)
        }
      }
    })
  });

  const { success, user_id } = await finishResp.json();

  if (success) {
    // Redirect to OAuth callback
    window.location.href = `${callbackURL}?state=${state}&user_id=${user_id}`;
  }
}
```

#### 5. Begin Passkey Authentication

**Request**:
```http
POST /passkey/login/begin
Content-Type: application/json

{
  "email": "alice@example.com"
}
```

**Response**:
```json
{
  "session_id": "base64-session-id",
  "options": {
    "publicKey": {
      "challenge": "base64-challenge",
      "timeout": 60000,
      "rpId": "auth.enopax.io",
      "allowCredentials": [
        {
          "type": "public-key",
          "id": "base64-credential-id",
          "transports": ["internal", "hybrid"]
        }
      ],
      "userVerification": "preferred"
    }
  }
}
```

#### 6. Browser Prompts for Passkey

Browser displays platform-specific authentication prompt:
- **macOS/iOS**: Touch ID or Face ID
- **Windows**: Windows Hello (face, fingerprint, or PIN)
- **Android**: Fingerprint or screen lock

#### 7. Finish Passkey Authentication

**Request**:
```http
POST /passkey/login/finish
Content-Type: application/json

{
  "session_id": "base64-session-id",
  "credential": {
    "id": "credential-id",
    "type": "public-key",
    "rawId": "base64-raw-id",
    "response": {
      "clientDataJSON": "base64-client-data",
      "authenticatorData": "base64-auth-data",
      "signature": "base64-signature",
      "userHandle": "base64-user-handle"
    }
  }
}
```

**Response** (Success):
```json
{
  "success": true,
  "user_id": "user-uuid",
  "email": "alice@example.com",
  "message": "Authentication successful"
}
```

**Response** (2FA Required):
```json
{
  "success": false,
  "require_2fa": true,
  "session_id": "2fa-session-id",
  "message": "Two-factor authentication required"
}
```

#### 8. OAuth Redirect

If 2FA is not required, connector redirects to Dex OAuth callback.

```http
HTTP/1.1 302 Found
Location: https://dex.example.com/callback?state=AUTH_REQ_ID&user_id=user-uuid
```

#### 9. Dex Issues OAuth Code

Dex validates user and issues authorization code.

```http
HTTP/1.1 302 Found
Location: https://app.example.com/callback?code=AUTH_CODE&state=random-state-value
```

#### 10. Client Exchanges Code for Tokens

OAuth client exchanges authorization code for tokens.

**Request**:
```http
POST /token
Content-Type: application/x-www-form-urlencoded

grant_type=authorization_code&
code=AUTH_CODE&
redirect_uri=https://app.example.com/callback&
client_id=example-app&
client_secret=example-secret
```

**Response**:
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIs...",
  "id_token": "eyJhbGciOiJSUzI1NiIs...",
  "token_type": "Bearer",
  "expires_in": 3600
}
```

**ID Token Claims**:
```json
{
  "iss": "https://dex.example.com",
  "sub": "user-uuid",
  "aud": "example-app",
  "exp": 1700000000,
  "iat": 1699996400,
  "email": "alice@example.com",
  "email_verified": true,
  "name": "Alice Smith"
}
```

### Discoverable Credentials (Passwordless)

For passwordless login, user can authenticate without entering email.

**Request** (No email):
```http
POST /passkey/login/begin
Content-Type: application/json

{}
```

**Response**:
```json
{
  "session_id": "base64-session-id",
  "options": {
    "publicKey": {
      "challenge": "base64-challenge",
      "timeout": 60000,
      "rpId": "auth.enopax.io",
      "allowCredentials": [],  // Empty - browser will show all available credentials
      "userVerification": "preferred"
    }
  }
}
```

Platform authenticator will display all passkeys for `auth.enopax.io`, and user selects their account.

### Security Features

1. **Signature Verification**: Each authentication requires a valid cryptographic signature
2. **Clone Detection**: Sign counter is validated to detect credential cloning
3. **User Verification**: Biometric or PIN verification enforced
4. **Phishing Resistance**: Passkeys are bound to the domain (HTTPS only)
5. **Replay Protection**: Each challenge is unique and expires after 5 minutes

### Error Handling

**Invalid Session**:
```json
{
  "success": false,
  "error": "invalid_session",
  "message": "Session not found or expired"
}
```

**Signature Verification Failed**:
```json
{
  "success": false,
  "error": "verification_failed",
  "message": "Passkey verification failed"
}
```

**Cloned Authenticator Detected**:
```json
{
  "success": false,
  "error": "clone_detected",
  "message": "Authenticator appears to be cloned"
}
```

---

## Password Authentication Flow

### Overview

Traditional password-based authentication for users who prefer or require password login.

### Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                 PASSWORD AUTHENTICATION FLOW                │
└─────────────────────────────────────────────────────────────┘

Browser                      Dex Connector
   │                               │
   │ 1. GET /login?state=...       │
   │ ──────────────────────────────►
   │                               │
   │ ◄──────────────────────────── │
   │ 2. Login page (HTML form)     │
   │                               │
   │ 3. User enters email/password │
   │ ────┐                         │
   │     │                         │
   │ ◄───┘                         │
   │                               │
   │ 4. POST /login/password       │
   │ {email, password, state}      │
   │ ──────────────────────────────►
   │                               │
   │                               │ 5. Lookup user by email
   │                               │    Verify password hash
   │                               │    Update last login
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │                               │ 6. Check if 2FA required
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ If 2FA not required:          │
   │ ◄──────────────────────────── │
   │ 7. Redirect to OAuth callback │
   │ /callback?state=...&user_id=..│
   │                               │
   │ If 2FA required:              │
   │ ◄──────────────────────────── │
   │ 8. Redirect to 2FA prompt     │
   │ /2fa/prompt?session_id=...    │
   │                               │
   ▼                               ▼
```

### Steps

#### 1. User Enters Credentials

User fills in email/password form on login page.

```html
<form method="POST" action="/login/password">
  <input type="email" name="email" required>
  <input type="password" name="password" required>
  <input type="hidden" name="state" value="AUTH_REQ_ID">
  <input type="hidden" name="callback" value="https://dex.example.com/callback">
  <button type="submit">Log In</button>
</form>
```

#### 2. Submit Login Form

**Request**:
```http
POST /login/password
Content-Type: application/x-www-form-urlencoded

email=alice@example.com&
password=SecurePass123&
state=AUTH_REQ_ID&
callback=https://dex.example.com/callback
```

**Response** (Success, No 2FA):
```http
HTTP/1.1 303 See Other
Location: https://dex.example.com/callback?state=AUTH_REQ_ID&user_id=user-uuid
```

**Response** (Success, 2FA Required):
```http
HTTP/1.1 303 See Other
Location: /2fa/prompt?session_id=2fa-session-id
```

**Response** (Invalid Credentials):
```http
HTTP/1.1 303 See Other
Location: /login?error=invalid_credentials&state=AUTH_REQ_ID
```

### Security Features

1. **Password Hashing**: bcrypt with cost factor 10
2. **Constant-Time Comparison**: Prevents timing attacks
3. **Rate Limiting**: Prevents brute force attacks (TODO: implement)
4. **Failed Attempt Logging**: Audit trail for failed logins (TODO: implement)

### Error Handling

**Invalid Email**:
```
Error: Invalid email or password
```

**Invalid Password**:
```
Error: Invalid email or password
```

Note: Same error message for both to prevent user enumeration.

**Account Locked** (Future):
```
Error: Account temporarily locked due to too many failed attempts. Please try again in 15 minutes.
```

---

## Magic Link Authentication Flow

### Overview

Email-based passwordless authentication. User receives a unique, time-limited link via email.

### Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                  MAGIC LINK AUTHENTICATION FLOW             │
└─────────────────────────────────────────────────────────────┘

Browser              Dex Connector              Email Service
   │                       │                          │
   │ 1. GET /login         │                          │
   │ ──────────────────────►                         │
   │                       │                          │
   │ ◄────────────────────│                          │
   │ 2. Login page         │                          │
   │ [Send Magic Link]     │                          │
   │                       │                          │
   │ 3. User enters email  │                          │
   │ ────┐                 │                          │
   │     │                 │                          │
   │ ◄───┘                 │                          │
   │                       │                          │
   │ 4. POST /magic-link/send                        │
   │ {email, state, callback}                        │
   │ ──────────────────────►                         │
   │                       │                          │
   │                       │ 5. Generate token        │
   │                       │    Store in database     │
   │                       │ ────┐                    │
   │                       │     │                    │
   │                       │ ◄───┘                    │
   │                       │                          │
   │                       │ 6. Send email            │
   │                       │ ─────────────────────────►
   │                       │                          │
   │                       │                    Email sent
   │                       │                      to user
   │ ◄──────────────────── │                          │
   │ 7. "Check your email" │                          │
   │                       │                          │
   │ User checks email     │                          │
   │ ────┐                 │                          │
   │     │                 │                          │
   │ ◄───┘                 │                          │
   │                       │                          │
   │ 8. Click magic link   │                          │
   │ GET /magic-link/verify?token=...                │
   │ ──────────────────────►                         │
   │                       │                          │
   │                       │ 9. Validate token        │
   │                       │    Check expiry          │
   │                       │    Mark as used          │
   │                       │    Update last login     │
   │                       │ ────┐                    │
   │                       │     │                    │
   │                       │ ◄───┘                    │
   │                       │                          │
   │                       │ 10. Check if 2FA required│
   │                       │ ────┐                    │
   │                       │     │                    │
   │                       │ ◄───┘                    │
   │                       │                          │
   │ If 2FA not required:  │                          │
   │ ◄──────────────────── │                          │
   │ 11. Redirect to OAuth │                          │
   │ /callback?state=...&user_id=...                 │
   │                       │                          │
   │ If 2FA required:      │                          │
   │ ◄──────────────────── │                          │
   │ 12. Redirect to 2FA   │                          │
   │ /2fa/prompt?session_id=...                      │
   │                       │                          │
   ▼                       ▼                          ▼
```

### Steps

#### 1. Request Magic Link

User enters email and clicks "Send Magic Link".

**Request**:
```http
POST /magic-link/send
Content-Type: application/json

{
  "email": "alice@example.com",
  "callback": "https://dex.example.com/callback",
  "state": "AUTH_REQ_ID"
}
```

**Response** (Success):
```json
{
  "success": true,
  "message": "Magic link sent to alice@example.com"
}
```

**Response** (Rate Limited):
```json
{
  "success": false,
  "error": "rate_limit_exceeded",
  "message": "Too many magic link requests. Please try again later.",
  "retry_after": 1800
}
```

#### 2. Email Sent

User receives email with magic link.

**Email Content**:
```html
<!DOCTYPE html>
<html>
<head>
  <style>
    .magic-link-button {
      background-color: #4CAF50;
      color: white;
      padding: 12px 24px;
      text-decoration: none;
      border-radius: 4px;
      display: inline-block;
    }
  </style>
</head>
<body>
  <h1>Your Magic Link</h1>
  <p>Click the button below to log in to Enopax:</p>

  <a href="https://auth.enopax.io/magic-link/verify?token=abc123...xyz"
     class="magic-link-button">
    Log In to Enopax
  </a>

  <p>This link expires in 10 minutes.</p>

  <p><strong>Security Notice:</strong> Do not forward this email or share this link with anyone.</p>

  <p>If you didn't request this link, you can safely ignore this email.</p>
</body>
</html>
```

#### 3. User Clicks Magic Link

**Request**:
```http
GET /magic-link/verify?token=abc123...xyz
```

**Response** (Success, No 2FA):
```http
HTTP/1.1 303 See Other
Location: https://dex.example.com/callback?state=AUTH_REQ_ID&user_id=user-uuid
```

**Response** (Success, 2FA Required):
```http
HTTP/1.1 303 See Other
Location: /2fa/prompt?session_id=2fa-session-id
```

**Response** (Token Expired):
```http
HTTP/1.1 401 Unauthorized
Content-Type: text/html

<h1>Link Expired</h1>
<p>This magic link has expired. Please request a new one.</p>
```

**Response** (Token Already Used):
```http
HTTP/1.1 401 Unauthorized
Content-Type: text/html

<h1>Link Already Used</h1>
<p>This magic link has already been used and cannot be reused.</p>
```

### Configuration

```yaml
magicLink:
  enabled: true
  ttl: 600  # 10 minutes
  rateLimit:
    perHour: 3   # Max 3 magic links per hour
    perDay: 10   # Max 10 magic links per day
```

### Security Features

1. **Time Limitation**: Links expire after 10 minutes
2. **One-Time Use**: Each link can only be used once
3. **Rate Limiting**: 3 links per hour, 10 per day
4. **IP Binding**: IP address captured (optional enforcement)
5. **HTTPS Only**: Links only work over HTTPS
6. **Cryptographic Tokens**: 32-byte random tokens

### Error Handling

**User Not Found**:
```json
{
  "success": false,
  "error": "user_not_found",
  "message": "No account found with that email address"
}
```

**Magic Links Disabled**:
```json
{
  "success": false,
  "error": "magic_link_disabled",
  "message": "Magic link authentication is not enabled"
}
```

**Email Sending Failed**:
```json
{
  "success": false,
  "error": "email_failed",
  "message": "Failed to send magic link email. Please try again."
}
```

---

## Two-Factor Authentication (2FA) Flow

### Overview

After successful primary authentication (password, passkey, or magic link), users may be required to complete a second verification step using TOTP, passkey (if not used in step 1), or backup code.

### Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│            TWO-FACTOR AUTHENTICATION (2FA) FLOW             │
└─────────────────────────────────────────────────────────────┘

Browser                      Dex Connector
   │                               │
   │ After primary authentication: │
   │                               │
   │                               │ 1. Check Require2FAForUser()
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ If 2FA not required:          │
   │ → Redirect to OAuth callback  │
   │                               │
   │ If 2FA required:              │
   │                               │ 2. Begin2FA()
   │                               │    Create TwoFactorSession
   │                               │    TTL: 10 minutes
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ ◄──────────────────────────── │
   │ 3. Redirect to 2FA prompt     │
   │ /2fa/prompt?session_id=...    │
   │                               │
   │ 4. GET /2fa/prompt?session_id=...                          │
   │ ──────────────────────────────►
   │                               │
   │                               │ 5. Get2FASession()
   │                               │    GetAvailable2FAMethods()
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ ◄──────────────────────────── │
   │ 6. 2FA prompt page (HTML)     │
   │ Options:                      │
   │ - TOTP code input             │
   │ - Passkey button              │
   │ - Backup code link            │
   │                               │
   │ Option A: TOTP                │
   │ 7a. User enters TOTP code     │
   │ POST /2fa/verify/totp         │
   │ {session_id, code}            │
   │ ──────────────────────────────►
   │                               │
   │                               │ 8a. ValidateTOTP()
   │                               │     Complete2FA()
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ Option B: Passkey             │
   │ 7b. Click "Use Passkey"       │
   │ POST /2fa/verify/passkey/begin│
   │ {session_id}                  │
   │ ──────────────────────────────►
   │                               │
   │ ◄──────────────────────────── │
   │ 8b. WebAuthn challenge        │
   │                               │
   │ 9b. navigator.credentials.get()│
   │ ────┐                         │
   │     │                         │
   │ ◄───┘                         │
   │                               │
   │ 10b. POST /2fa/verify/passkey/finish                       │
   │ {session_id, credential}      │
   │ ──────────────────────────────►
   │                               │
   │                               │ 11b. FinishPasskeyAuthentication()
   │                               │      Complete2FA()
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ Option C: Backup Code         │
   │ 7c. User enters backup code   │
   │ POST /2fa/verify/backup-code  │
   │ {session_id, code}            │
   │ ──────────────────────────────►
   │                               │
   │                               │ 8c. ValidateBackupCode()
   │                               │     Mark code as used
   │                               │     Complete2FA()
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ After successful 2FA:         │
   │ ◄──────────────────────────── │
   │ 9. Redirect to OAuth callback │
   │ /callback?state=...&user_id=..│
   │                               │
   ▼                               ▼
```

### 2FA Policy Enforcement

The `Require2FAForUser()` function determines if 2FA is required:

```go
func (c *Connector) Require2FAForUser(ctx context.Context, user *User) bool {
    // User-level enforcement
    if user.Require2FA {
        return true
    }

    // Global policy
    if c.config.TwoFactor.Required {
        return true
    }

    // User has TOTP enabled (opt-in 2FA)
    if user.TOTPEnabled {
        return true
    }

    // User has both password and passkey (2FA-capable)
    hasPassword := user.PasswordHash != nil
    hasPasskey := len(user.Passkeys) > 0
    if hasPassword && hasPasskey && c.config.TwoFactor.Required {
        return true
    }

    return false
}
```

### Available 2FA Methods

The `GetAvailable2FAMethods()` function returns available options:

```go
func (c *Connector) GetAvailable2FAMethods(ctx context.Context, user *User, primaryMethod string) []string {
    var methods []string

    // TOTP (if enabled)
    if user.TOTPEnabled && contains(c.config.TwoFactor.Methods, "totp") {
        methods = append(methods, "totp")
    }

    // Passkey (if not used in primary auth)
    if len(user.Passkeys) > 0 && primaryMethod != "passkey" && contains(c.config.TwoFactor.Methods, "passkey") {
        methods = append(methods, "passkey")
    }

    // Backup code (if user has unused codes)
    if hasUnusedBackupCodes(user.BackupCodes) {
        methods = append(methods, "backup_code")
    }

    return methods
}
```

### Steps

#### 1. Primary Authentication Succeeds

After user authenticates with password, passkey, or magic link, connector checks if 2FA is required.

#### 2. Begin 2FA

If 2FA required, connector creates a `TwoFactorSession`:

```go
type TwoFactorSession struct {
    SessionID     string    // Unique session ID
    UserID        string    // User who completed primary auth
    PrimaryMethod string    // "password", "passkey", or "magic_link"
    CreatedAt     time.Time
    ExpiresAt     time.Time // 10-minute TTL
    Completed     bool      // Marked true after 2FA validation
    CallbackURL   string    // OAuth callback URL
    State         string    // OAuth state parameter
}
```

#### 3. Show 2FA Prompt

User is redirected to `/2fa/prompt?session_id=...` with available options:

**TOTP Option**:
```html
<form method="POST" action="/2fa/verify/totp">
  <label>Enter 6-digit code from authenticator app:</label>
  <input type="text" name="code" maxlength="6" required autofocus>
  <input type="hidden" name="session_id" value="2fa-session-id">
  <button type="submit">Verify</button>
</form>
```

**Passkey Option**:
```html
<button onclick="verify2FAWithPasskey()">
  🔐 Verify with Passkey
</button>
```

**Backup Code Option**:
```html
<details>
  <summary>Use backup code</summary>
  <form method="POST" action="/2fa/verify/backup-code">
    <label>Enter 8-character backup code:</label>
    <input type="text" name="code" maxlength="8" required>
    <input type="hidden" name="session_id" value="2fa-session-id">
    <button type="submit">Verify</button>
  </form>
</details>
```

#### 4. Verify TOTP Code

**Request**:
```http
POST /2fa/verify/totp
Content-Type: application/x-www-form-urlencoded

session_id=2fa-session-id&
code=123456
```

**Response** (Success):
```http
HTTP/1.1 303 See Other
Location: https://dex.example.com/callback?state=AUTH_REQ_ID&user_id=user-uuid
```

**Response** (Invalid Code):
```http
HTTP/1.1 303 See Other
Location: /2fa/prompt?session_id=2fa-session-id&error=invalid_code
```

#### 5. Verify with Passkey (2FA)

**Begin Request**:
```http
POST /2fa/verify/passkey/begin
Content-Type: application/json

{
  "session_id": "2fa-session-id"
}
```

**Begin Response**:
```json
{
  "webauthn_session_id": "webauthn-session-id",
  "options": {
    "publicKey": {
      "challenge": "base64-challenge",
      "rpId": "auth.enopax.io",
      "allowCredentials": [...],
      "userVerification": "preferred"
    }
  }
}
```

**Finish Request**:
```http
POST /2fa/verify/passkey/finish
Content-Type: application/json

{
  "session_id": "2fa-session-id",
  "webauthn_session_id": "webauthn-session-id",
  "credential": {
    "id": "credential-id",
    "type": "public-key",
    "rawId": "base64-raw-id",
    "response": {
      "clientDataJSON": "base64-client-data",
      "authenticatorData": "base64-auth-data",
      "signature": "base64-signature"
    }
  }
}
```

**Finish Response**:
```json
{
  "success": true,
  "user_id": "user-uuid"
}
```

JavaScript then redirects to OAuth callback.

#### 6. Verify Backup Code

**Request**:
```http
POST /2fa/verify/backup-code
Content-Type: application/x-www-form-urlencoded

session_id=2fa-session-id&
code=AB7C9KLM
```

**Response** (Success):
```http
HTTP/1.1 303 See Other
Location: https://dex.example.com/callback?state=AUTH_REQ_ID&user_id=user-uuid
```

**Response** (Invalid/Used Code):
```http
HTTP/1.1 303 See Other
Location: /2fa/prompt?session_id=2fa-session-id&error=invalid_backup_code
```

### Grace Period

New users have a grace period to set up 2FA before it's enforced:

```go
func (c *Connector) InGracePeriod(ctx context.Context, user *User) bool {
    // Grace period only applies if no 2FA methods set up
    if user.TOTPEnabled || len(user.Passkeys) > 0 {
        return false
    }

    gracePeriod := time.Duration(c.config.TwoFactor.GracePeriod) * time.Second
    gracePeriodEnd := user.CreatedAt.Add(gracePeriod)

    return time.Now().Before(gracePeriodEnd)
}
```

**Default Grace Period**: 7 days (604800 seconds)

During grace period, user can log in without 2FA but sees reminder:

```html
<div class="warning">
  ⚠️ You have 5 days remaining to set up two-factor authentication.
  <a href="/totp/enable">Set up now</a>
</div>
```

### Configuration

```yaml
twoFactor:
  required: false           # Global 2FA requirement
  methods: [totp, passkey]  # Allowed 2FA methods
  gracePeriod: 604800       # Grace period in seconds (7 days)
```

### Security Features

1. **Session Expiry**: 2FA sessions expire after 10 minutes
2. **One-Time Use**: Sessions cannot be reused
3. **Rate Limiting**: TOTP validation rate-limited (5 attempts per 5 minutes)
4. **Backup Code Tracking**: Codes marked as used with timestamp
5. **Method Validation**: Only configured methods are allowed

---

## TOTP Setup Flow

### Overview

Users can enable TOTP (Time-based One-Time Password) 2FA using an authenticator app like Google Authenticator, Authy, or 1Password.

### Flow Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                      TOTP SETUP FLOW                        │
└─────────────────────────────────────────────────────────────┘

Browser                      Dex Connector
   │                               │
   │ 1. Navigate to settings       │
   │ GET /manage-credentials       │
   │ ──────────────────────────────►
   │                               │
   │ ◄──────────────────────────── │
   │ 2. Credential management page │
   │ [Enable 2FA] button           │
   │                               │
   │ 3. Click "Enable 2FA"         │
   │ POST /totp/enable             │
   │ {user_id}                     │
   │ ──────────────────────────────►
   │                               │
   │                               │ 4. Generate TOTP secret
   │                               │    Generate QR code
   │                               │    Generate 10 backup codes
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ ◄──────────────────────────── │
   │ 5. Returns:                   │
   │ - TOTP secret                 │
   │ - QR code (base64 PNG)        │
   │ - Backup codes (10)           │
   │ - otpauth:// URL              │
   │                               │
   │ 6. Display QR code            │
   │ User scans with auth app      │
   │ ────┐                         │
   │     │                         │
   │ ◄───┘                         │
   │                               │
   │ 7. User saves backup codes    │
   │ ────┐                         │
   │     │                         │
   │ ◄───┘                         │
   │                               │
   │ 8. User enters TOTP code      │
   │ POST /totp/verify             │
   │ {user_id, secret, code, backup_codes}                     │
   │ ──────────────────────────────►
   │                               │
   │                               │ 9. Verify TOTP code
   │                               │    Hash backup codes
   │                               │    Enable TOTP
   │                               │ ────┐
   │                               │     │
   │                               │ ◄───┘
   │                               │
   │ ◄──────────────────────────── │
   │ 10. Success                   │
   │ TOTP enabled ✅               │
   │                               │
   ▼                               ▼
```

### Steps

#### 1. Navigate to Credential Management

User accesses credential management page (after login).

```http
GET /manage-credentials
```

Page displays current authentication methods and option to enable TOTP.

#### 2. Request TOTP Setup

**Request**:
```http
POST /totp/enable
Content-Type: application/json

{
  "user_id": "user-uuid"
}
```

**Response**:
```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_code": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAA...",
  "backup_codes": [
    "A7K9MXLP",
    "B2N4QRST",
    "C8V5WZYX",
    "D3F6HJKL",
    "E9G7MNPQ",
    "F4H8RSTU",
    "G5J9VWXY",
    "H6K2BCDF",
    "J7L3GHKM",
    "K8M4NPQR"
  ],
  "otpauth_url": "otpauth://totp/Enopax:alice@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Enopax"
}
```

#### 3. Scan QR Code

User scans QR code with authenticator app (Google Authenticator, Authy, 1Password, etc.).

Authenticator app displays 6-digit code that refreshes every 30 seconds.

#### 4. Save Backup Codes

User saves backup codes in a secure location (password manager, printed copy, etc.).

**Warning displayed**:
```
⚠️ Save these backup codes in a safe place. Each code can only be used once.
If you lose access to your authenticator app, you'll need these codes to log in.
```

#### 5. Verify TOTP Code

User enters current 6-digit code from authenticator app.

**Request**:
```http
POST /totp/verify
Content-Type: application/json

{
  "user_id": "user-uuid",
  "secret": "JBSWY3DPEHPK3PXP",
  "code": "123456",
  "backup_codes": [
    "A7K9MXLP",
    "B2N4QRST",
    ...
  ]
}
```

**Response** (Success):
```json
{
  "success": true,
  "message": "TOTP enabled successfully"
}
```

**Response** (Invalid Code):
```json
{
  "success": false,
  "error": "invalid_code",
  "message": "Invalid TOTP code. Please try again."
}
```

#### 6. TOTP Enabled

User's account now has TOTP enabled. Next login will require TOTP code after primary authentication.

### Backup Code Format

- **Length**: 8 characters
- **Characters**: Uppercase alphanumeric (A-Z, 2-9)
- **Excluded**: 0, O, 1, I, L (to avoid confusion)
- **Example**: `A7K9MXLP`
- **Storage**: Hashed with bcrypt
- **Count**: 10 codes per user

### Backup Code Usage

When user loses access to authenticator app:

1. Log in with password or passkey (primary auth)
2. On 2FA prompt, click "Use backup code"
3. Enter one of the saved backup codes
4. Code is verified and marked as used
5. User is logged in

**After using backup code**:
```html
<div class="warning">
  ⚠️ You just used a backup code. You have 9 codes remaining.
  <a href="/totp/regenerate-backup-codes">Generate new backup codes</a>
</div>
```

### Regenerating Backup Codes

User can regenerate backup codes (invalidates old ones).

**Request**:
```http
POST /totp/regenerate-backup-codes
Content-Type: application/json

{
  "user_id": "user-uuid",
  "code": "123456"  // Current TOTP code for verification
}
```

**Response**:
```json
{
  "success": true,
  "backup_codes": [
    "X2Y7MNPQ",
    "Z3W8RSTU",
    ...
  ]
}
```

### Disabling TOTP

User can disable TOTP (requires verification).

**Request**:
```http
POST /totp/disable
Content-Type: application/json

{
  "user_id": "user-uuid",
  "code": "123456"  // Current TOTP code for verification
}
```

**Response**:
```json
{
  "success": true,
  "message": "TOTP disabled successfully"
}
```

### gRPC API Reference

Platform can manage TOTP via gRPC:

**EnableTOTP RPC**:
```protobuf
rpc EnableTOTP(EnableTOTPReq) returns (EnableTOTPResp);

message EnableTOTPReq {
  string user_id = 1;
}

message EnableTOTPResp {
  bool success = 1;
  string secret = 2;
  string qr_code = 3;         // Base64-encoded PNG
  repeated string backup_codes = 4;
  string otpauth_url = 5;
  bool already_enabled = 6;
}
```

**VerifyTOTPSetup RPC**:
```protobuf
rpc VerifyTOTPSetup(VerifyTOTPSetupReq) returns (VerifyTOTPSetupResp);

message VerifyTOTPSetupReq {
  string user_id = 1;
  string secret = 2;
  string code = 3;
  repeated string backup_codes = 4;
}

message VerifyTOTPSetupResp {
  bool success = 1;
  bool invalid_code = 2;
  bool not_found = 3;
}
```

**DisableTOTP RPC**:
```protobuf
rpc DisableTOTP(DisableTOTPReq) returns (DisableTOTPResp);

message DisableTOTPReq {
  string user_id = 1;
  string code = 2;  // Current TOTP code
}

message DisableTOTPResp {
  bool success = 1;
  bool invalid_code = 2;
  bool not_found = 3;
}
```

**GetTOTPInfo RPC**:
```protobuf
rpc GetTOTPInfo(GetTOTPInfoReq) returns (GetTOTPInfoResp);

message GetTOTPInfoReq {
  string user_id = 1;
}

message GetTOTPInfoResp {
  bool enabled = 1;
  int32 backup_codes_remaining = 2;
  bool not_found = 3;
}
```

**RegenerateBackupCodes RPC**:
```protobuf
rpc RegenerateBackupCodes(RegenerateBackupCodesReq) returns (RegenerateBackupCodesResp);

message RegenerateBackupCodesReq {
  string user_id = 1;
  string code = 2;  // Current TOTP code
}

message RegenerateBackupCodesResp {
  bool success = 1;
  repeated string backup_codes = 2;
  bool invalid_code = 3;
  bool not_found = 4;
}
```

### Security Features

1. **Secret Generation**: 160-bit secret (32 characters base32)
2. **Time Window**: 30-second time step
3. **Code Validation**: 6-digit codes
4. **Rate Limiting**: 5 attempts per 5 minutes
5. **Backup Code Hashing**: bcrypt with cost 10
6. **One-Time Backup Codes**: Marked as used after validation

---

## OAuth Integration

### Overview

All authentication flows integrate with Dex's OAuth 2.0 / OpenID Connect implementation. After successful authentication (and 2FA if required), users are redirected back to Dex with a user identifier, which Dex uses to issue OAuth tokens.

### OAuth Flow with Enhanced Connector

```
┌─────────────────────────────────────────────────────────────┐
│             OAUTH INTEGRATION WITH CONNECTOR                │
└─────────────────────────────────────────────────────────────┘

OAuth Client       Dex Server       Enhanced Connector    User
      │                  │                    │             │
      │ 1. Authorization │                    │             │
      │ Request          │                    │             │
      │ ─────────────────►                   │             │
      │                  │                    │             │
      │                  │ 2. Call LoginURL() │             │
      │                  │ ───────────────────►            │
      │                  │                    │             │
      │                  │ ◄───────────────── │             │
      │                  │ 3. Login page URL  │             │
      │                  │                    │             │
      │ ◄─────────────── │                    │             │
      │ 4. Redirect to   │                    │             │
      │ connector login  │                    │             │
      │ ─────────────────────────────────────►             │
      │                  │                    │             │
      │                  │                    │ 5. Authenticate
      │                  │                    │ (passkey/password/
      │                  │                    │  magic link + 2FA)
      │                  │                    │ ────────────►
      │                  │                    │             │
      │                  │                    │ ◄────────── │
      │                  │                    │             │
      │ ◄────────────────────────────────────│             │
      │ 6. Redirect to   │                    │             │
      │ Dex callback     │                    │             │
      │ /callback?state=...&user_id=...      │             │
      │ ─────────────────►                   │             │
      │                  │                    │             │
      │                  │ 7. Call HandleCallback()        │
      │                  │ ───────────────────►            │
      │                  │                    │             │
      │                  │ ◄───────────────── │             │
      │                  │ 8. Identity        │             │
      │                  │ (user_id, email, etc.)          │
      │                  │                    │             │
      │                  │ 9. Issue OAuth code│             │
      │                  │ ────┐              │             │
      │                  │     │              │             │
      │                  │ ◄───┘              │             │
      │                  │                    │             │
      │ ◄─────────────── │                    │             │
      │ 10. Redirect to  │                    │             │
      │ client callback  │                    │             │
      │ with auth code   │                    │             │
      │                  │                    │             │
      │ 11. Exchange code│                    │             │
      │ for tokens       │                    │             │
      │ ─────────────────►                   │             │
      │                  │                    │             │
      │ ◄─────────────── │                    │             │
      │ 12. ID token +   │                    │             │
      │ access token     │                    │             │
      │                  │                    │             │
      ▼                  ▼                    ▼             ▼
```

### CallbackConnector Interface

The enhanced local connector implements `connector.CallbackConnector`:

```go
type CallbackConnector interface {
    LoginURL(scopes Scopes, callbackURL, state string) (string, error)
    HandleCallback(scopes Scopes, r *http.Request) (identity Identity, err error)
}
```

### LoginURL Implementation

Called by Dex to get the URL for the connector's login page.

```go
func (c *Connector) LoginURL(scopes connector.Scopes, callbackURL, state string) (string, error) {
    // Build URL to login page with state and callback parameters
    loginURL := c.config.BaseURL + "/login?state=" + state + "&callback=" + url.QueryEscape(callbackURL)

    c.logger.Infof("LoginURL: redirecting to %s", loginURL)
    return loginURL, nil
}
```

**Example**:
```
Input:
  callbackURL = "https://dex.example.com/callback"
  state = "AUTH_REQ_ID_12345"

Output:
  "https://auth.enopax.io/login?state=AUTH_REQ_ID_12345&callback=https%3A%2F%2Fdex.example.com%2Fcallback"
```

### HandleCallback Implementation

Called by Dex after user completes authentication to retrieve user identity.

```go
func (c *Connector) HandleCallback(scopes connector.Scopes, r *http.Request) (connector.Identity, error) {
    // Get user_id from query parameter (set by connector after auth)
    userID := r.URL.Query().Get("user_id")
    if userID == "" {
        return connector.Identity{}, fmt.Errorf("missing user_id parameter")
    }

    // Retrieve user from storage
    ctx := r.Context()
    user, err := c.storage.GetUser(ctx, userID)
    if err != nil {
        return connector.Identity{}, fmt.Errorf("user not found: %w", err)
    }

    // Build connector identity
    identity := connector.Identity{
        UserID:            user.ID,
        Username:          user.Username,
        Email:             user.Email,
        EmailVerified:     user.EmailVerified,
        PreferredUsername: getPreferredUsername(user),
    }

    return identity, nil
}

func getPreferredUsername(user *User) string {
    if user.DisplayName != "" {
        return user.DisplayName
    }
    if user.Username != "" {
        return user.Username
    }
    return user.Email
}
```

### OAuth State Preservation

The `state` parameter is preserved throughout the authentication flow:

1. **OAuth Client** → Dex: `state=client-state`
2. **Dex** → Connector: `state=AUTH_REQ_ID` (Dex's internal auth request ID)
3. **Connector** → Dex: `state=AUTH_REQ_ID&user_id=user-uuid` (after auth)
4. **Dex** → OAuth Client: `state=client-state&code=AUTH_CODE`

This ensures the OAuth client can validate the callback and prevent CSRF attacks.

### Identity Mapping

The `connector.Identity` struct maps to OIDC ID token claims:

```go
type Identity struct {
    UserID            string   // Maps to "sub" claim
    Username          string   // Maps to "preferred_username" claim
    Email             string   // Maps to "email" claim
    EmailVerified     bool     // Maps to "email_verified" claim
    PreferredUsername string   // Maps to "preferred_username" claim (fallback)
    Groups            []string // Maps to "groups" claim (optional)
}
```

**Example ID Token**:
```json
{
  "iss": "https://dex.example.com",
  "sub": "ff8d9819-fc0e-12bf-0d24-892e45987e24",
  "aud": "example-app",
  "exp": 1700000000,
  "iat": 1699996400,
  "email": "alice@example.com",
  "email_verified": true,
  "preferred_username": "Alice Smith",
  "name": "Alice Smith"
}
```

### Configuration

```yaml
connectors:
  - type: local-enhanced
    id: local
    name: Enopax Authentication
    config:
      baseURL: https://auth.enopax.io  # Base URL for login page
      dataDir: /var/lib/dex/data
      passkey:
        enabled: true
        rpID: auth.enopax.io
        rpName: Enopax
        rpOrigins:
          - https://auth.enopax.io
      twoFactor:
        required: false
        methods: [totp, passkey]
      magicLink:
        enabled: true
        ttl: 600
      email:
        smtp:
          host: smtp.example.com
          port: 587
          username: noreply@enopax.io
          password: ${SMTP_PASSWORD}
```

---

## Error Handling

### HTTP Status Codes

| Code | Meaning | Usage |
|------|---------|-------|
| 200 OK | Success | Successful API calls (JSON responses) |
| 303 See Other | Redirect after POST | Form submissions, OAuth redirects |
| 400 Bad Request | Invalid input | Missing fields, validation errors |
| 401 Unauthorized | Authentication failed | Invalid credentials, expired tokens |
| 403 Forbidden | Feature disabled | Passkeys/magic links disabled |
| 404 Not Found | Resource not found | User/session not found |
| 405 Method Not Allowed | Wrong HTTP method | GET on POST endpoint |
| 409 Conflict | Resource conflict | TOTP already enabled |
| 429 Too Many Requests | Rate limited | Too many magic links/TOTP attempts |
| 500 Internal Server Error | Server error | Database errors, unexpected failures |

### Error Response Format

**JSON Errors** (API endpoints):
```json
{
  "success": false,
  "error": "error_code",
  "message": "Human-readable error message"
}
```

**HTML Errors** (Web pages):
```html
<!DOCTYPE html>
<html>
<head><title>Error</title></head>
<body>
  <div class="error-box">
    <h1>❌ Authentication Failed</h1>
    <p>Invalid email or password.</p>
    <a href="/login">Try again</a>
  </div>
</body>
</html>
```

### Common Errors

**Invalid Credentials**:
```
Error Code: invalid_credentials
Message: Invalid email or password
Status: 401 Unauthorized
```

**Session Expired**:
```
Error Code: session_expired
Message: Your session has expired. Please try again.
Status: 401 Unauthorized
```

**Rate Limited**:
```
Error Code: rate_limit_exceeded
Message: Too many requests. Please try again in 15 minutes.
Status: 429 Too Many Requests
Retry-After: 900
```

**2FA Required**:
```
Error Code: 2fa_required
Message: Two-factor authentication is required
Status: 403 Forbidden
Redirect: /2fa/prompt?session_id=...
```

**Feature Disabled**:
```
Error Code: feature_disabled
Message: Passkey authentication is not enabled
Status: 403 Forbidden
```

### User-Friendly Error Messages

Errors are displayed in context on the UI:

```html
<!-- Login page with error -->
<div class="dex-error-box">
  ⚠️ Invalid email or password. Please try again.
</div>

<!-- 2FA page with error -->
<div class="dex-error-box">
  ⚠️ Invalid verification code. Please check your authenticator app and try again.
</div>

<!-- Magic link expired -->
<div class="dex-error-box">
  ⚠️ This magic link has expired. <a href="/login">Request a new one</a>
</div>
```

---

## Security Considerations

### Authentication Security

1. **Password Security**:
   - bcrypt hashing with cost factor 10
   - Minimum password requirements enforced
   - Constant-time comparison to prevent timing attacks
   - Rate limiting on login attempts (TODO)

2. **Passkey Security**:
   - Cryptographic signature verification
   - Challenge-response protocol
   - Clone detection via sign counter
   - User verification required (biometric/PIN)
   - HTTPS-only (WebAuthn requirement)
   - Domain binding (phishing resistance)

3. **Magic Link Security**:
   - Cryptographically random tokens (32 bytes)
   - Short expiry (10 minutes)
   - One-time use only
   - Rate limiting (3/hour, 10/day)
   - IP address capture (optional enforcement)
   - HTTPS-only transmission

4. **TOTP Security**:
   - Time-based validation (30-second window)
   - Rate limiting (5 attempts per 5 minutes)
   - Backup codes hashed with bcrypt
   - One-time use for backup codes
   - Secret storage encrypted at rest (TODO)

5. **2FA Security**:
   - Session expiry (10 minutes)
   - One-time use sessions
   - Method validation
   - Automatic cleanup of expired sessions

### Session Management

1. **WebAuthn Sessions**:
   - 5-minute expiry
   - Cryptographically secure challenge (32 bytes)
   - Session ID is random (32 bytes)
   - Stored with file permissions 0600

2. **2FA Sessions**:
   - 10-minute expiry
   - One-time use (marked completed after validation)
   - Cleanup 1 minute after completion
   - OAuth state and callback preserved

3. **Auth Setup Tokens**:
   - 24-hour expiry
   - One-time use
   - Marked as used after access
   - Cryptographically random (32 bytes)

### OAuth Security

1. **State Parameter**:
   - Validated by Dex (CSRF protection)
   - Preserved throughout flow
   - Random value generated by OAuth client

2. **Authorization Code**:
   - Short-lived (typically 10 minutes)
   - One-time use
   - Bound to client_id and redirect_uri

3. **ID Token**:
   - Signed with Dex's private key
   - Contains user claims
   - Expiry timestamp (typically 1 hour)
   - Audience restricted to client_id

### Data Protection

1. **File Storage**:
   - User files: 0600 permissions (owner read/write only)
   - Directory: 0700 permissions (owner access only)
   - Atomic writes (temp file + rename)
   - File locking for concurrent access

2. **Sensitive Data**:
   - Passwords hashed with bcrypt
   - TOTP secrets stored base32-encoded (TODO: encrypt at rest)
   - Backup codes hashed with bcrypt
   - WebAuthn private keys never leave authenticator

3. **Logging**:
   - No passwords or secrets logged
   - Authentication attempts logged
   - Failed login attempts logged
   - Rate limit violations logged

### HTTPS Requirements

**ALL authentication endpoints MUST use HTTPS in production**:

- WebAuthn requires HTTPS (except localhost for testing)
- Magic links should only be sent over HTTPS
- OAuth requires HTTPS for redirect URIs
- Cookies should use Secure flag
- HSTS header recommended

### Rate Limiting

| Endpoint | Limit | Window |
|----------|-------|--------|
| Password login | TODO | TODO |
| TOTP validation | 5 attempts | 5 minutes |
| Magic link send | 3 requests | 1 hour |
| Magic link send | 10 requests | 1 day |
| Passkey auth | None (clone detection) | N/A |

### Threat Model

**Protected Against**:
- ✅ Credential stuffing (rate limiting, strong passwords)
- ✅ Phishing (passkey domain binding)
- ✅ Man-in-the-middle (HTTPS, signature verification)
- ✅ Replay attacks (unique challenges, one-time tokens)
- ✅ Session hijacking (HTTPS, secure cookies, short expiry)
- ✅ Brute force (rate limiting, bcrypt cost)
- ✅ Timing attacks (constant-time comparisons)
- ✅ CSRF (state parameter, SameSite cookies)

**Partial Protection**:
- ⚠️ Account enumeration (same error for invalid email/password)
- ⚠️ Credential leaks (strong hashing, but data breach possible)

**Not Protected Against** (out of scope):
- ❌ Malware on user device
- ❌ Social engineering
- ❌ Insider threats
- ❌ Physical access to server

---

## Appendix: Complete Example Flows

### Example 1: New User Registration with Passkey

```
1. User registers on Platform (https://platform.enopax.io)
   POST /api/users {email, username}

2. Platform calls Dex gRPC: CreateUser()
   Returns: user_id

3. Platform generates auth setup token
   Stores in Platform database

4. Platform sends email:
   "Click here to set up: https://auth.enopax.io/setup-auth?token=abc123"

5. User clicks link
   GET /setup-auth?token=abc123

6. User sees options:
   [Set up Passkey] [Set up Password] [Set up Both]

7. User clicks "Set up Passkey"
   JavaScript calls POST /passkey/register/begin

8. Dex returns WebAuthn challenge

9. Browser calls navigator.credentials.create()
   User verifies with Touch ID

10. JavaScript calls POST /passkey/register/finish
    Dex stores passkey

11. Redirect to Platform dashboard
    User is now registered with passkey ✅
```

### Example 2: Login with Passkey (No 2FA)

```
1. User visits OAuth client app
   Clicks "Log in with Enopax"

2. Redirected to Dex:
   /authorize?client_id=app&redirect_uri=...&state=xyz

3. Dex redirects to connector:
   /login?state=AUTH_REQ&callback=https://dex.../callback

4. User sees login page
   Clicks "Login with Passkey"

5. JavaScript calls POST /passkey/login/begin

6. Dex returns WebAuthn challenge

7. Browser calls navigator.credentials.get()
   User verifies with Touch ID

8. JavaScript calls POST /passkey/login/finish
   Dex verifies signature ✅

9. Dex checks 2FA requirement: Not required

10. Redirect to Dex callback:
    /callback?state=AUTH_REQ&user_id=user-uuid

11. Dex issues OAuth code:
    Redirect to client: /callback?code=AUTH_CODE&state=xyz

12. Client exchanges code for tokens:
    POST /token → {id_token, access_token}

13. User logged in ✅
```

### Example 3: Login with Password + TOTP (2FA)

```
1. User visits login page

2. Enters email and password
   POST /login/password

3. Dex verifies password ✅

4. Dex checks 2FA requirement: Required (user has TOTP enabled)

5. Dex creates TwoFactorSession
   Redirect to /2fa/prompt?session_id=2fa-123

6. User sees 2FA page
   Options: TOTP, Backup Code

7. User enters 6-digit code from authenticator app
   POST /2fa/verify/totp {session_id, code: "123456"}

8. Dex validates TOTP code ✅

9. Dex marks 2FA session complete

10. Redirect to OAuth callback:
    /callback?state=AUTH_REQ&user_id=user-uuid

11. OAuth flow continues...

12. User logged in with 2FA ✅
```

### Example 4: Passwordless Magic Link with 2FA

```
1. User visits login page
   Clicks "Send Magic Link"

2. Enters email
   POST /magic-link/send {email}

3. Dex generates token
   Sends email to user

4. User checks email
   Clicks magic link:
   /magic-link/verify?token=xyz789

5. Dex validates token ✅
   Token not expired, not used

6. Dex checks 2FA requirement: Required

7. Dex creates TwoFactorSession
   Redirect to /2fa/prompt?session_id=2fa-456

8. User sees 2FA page
   Clicks "Use Passkey"

9. JavaScript calls POST /2fa/verify/passkey/begin
   Dex returns WebAuthn challenge

10. Browser calls navigator.credentials.get()
    User verifies with Face ID

11. JavaScript calls POST /2fa/verify/passkey/finish
    Dex verifies signature ✅

12. Redirect to OAuth callback

13. User logged in with magic link + passkey 2FA ✅
```

---

**End of Authentication Flows Documentation**

For more information, see:
- [Passkey WebAuthn Support](./passkey-webauthn-support.md)
- [gRPC API Reference](./grpc-api.md)
- [Storage Schema](./storage-schema.md)
- [Development Guide](../../DEVELOPMENT.md)
