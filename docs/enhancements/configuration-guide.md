# Enhanced Local Connector - Configuration Guide

**Document**: Configuration Guide for Enhanced Local Connector
**Audience**: System Administrators, DevOps Engineers
**Last Updated**: 2025-11-18
**Version**: 1.0

---

## Table of Contents

1. [Overview](#overview)
2. [Basic Configuration](#basic-configuration)
3. [Connector Configuration](#connector-configuration)
4. [Passkey (WebAuthn) Configuration](#passkey-webauthn-configuration)
5. [Two-Factor Authentication (2FA) Configuration](#two-factor-authentication-2fa-configuration)
6. [Magic Link Configuration](#magic-link-configuration)
7. [Email Configuration](#email-configuration)
8. [Storage Configuration](#storage-configuration)
9. [gRPC Configuration](#grpc-configuration)
10. [OAuth Client Configuration](#oauth-client-configuration)
11. [Security Configuration](#security-configuration)
12. [Environment Variables](#environment-variables)
13. [Deployment Examples](#deployment-examples)
14. [Troubleshooting](#troubleshooting)

---

## Overview

The Enhanced Local Connector extends Dex with support for:

- **Multiple Authentication Methods**: Password, Passkey (WebAuthn), TOTP, Magic Link
- **True 2FA**: Require multiple authentication factors
- **Passwordless Authentication**: Passkey-only or magic link-only accounts
- **User Management API**: gRPC API for programmatic user management
- **Flexible Policies**: Configure authentication requirements per user or globally

This guide covers all configuration options for deploying and managing the Enhanced Local Connector.

---

## Basic Configuration

### Minimum Configuration

```yaml
# config.yaml

issuer: https://auth.enopax.io

storage:
  type: sqlite3
  config:
    file: /var/lib/dex/dex.db

web:
  http: 0.0.0.0:5556
  tlsCert: /etc/dex/tls/cert.pem
  tlsKey: /etc/dex/tls/key.pem

connectors:
  - type: local-enhanced
    id: local
    name: Enopax Authentication
    config:
      baseURL: https://auth.enopax.io
      dataDir: /var/lib/dex/data

staticClients:
  - id: example-app
    redirectURIs:
      - https://app.example.com/callback
    name: Example App
    secret: example-client-secret
```

**Key Fields**:
- `issuer`: Your Dex server URL (must be HTTPS in production)
- `storage`: Database configuration (SQLite, PostgreSQL, MySQL)
- `web`: HTTP/HTTPS server settings
- `connectors`: Authentication connectors (Enhanced Local Connector)
- `staticClients`: OAuth clients that can use Dex for authentication

---

## Connector Configuration

### Full Configuration Example

```yaml
connectors:
  - type: local-enhanced
    id: local
    name: Enopax Authentication
    config:
      # Base configuration
      baseURL: https://auth.enopax.io
      dataDir: /var/lib/dex/data

      # Passkey (WebAuthn) settings
      passkey:
        enabled: true
        rpID: auth.enopax.io
        rpName: Enopax
        rpOrigins:
          - https://auth.enopax.io
          - https://platform.enopax.io
        userVerification: preferred
        attestation: none
        timeout: 60000

      # Two-Factor Authentication settings
      twoFactor:
        required: false
        methods:
          - totp
          - passkey
        gracePeriod: 604800  # 7 days in seconds

      # Magic Link settings
      magicLink:
        enabled: true
        ttl: 600  # 10 minutes in seconds
        rateLimit:
          perHour: 3
          perDay: 10

      # Email settings
      email:
        smtp:
          host: smtp.sendgrid.net
          port: 587
          username: apikey
          password: ${SMTP_PASSWORD}
          from: noreply@enopax.io
          fromName: Enopax
```

### Configuration Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `baseURL` | string | Yes | - | Base URL of the connector (must match issuer) |
| `dataDir` | string | Yes | - | Directory for file storage |

---

## Passkey (WebAuthn) Configuration

### Passkey Settings

```yaml
passkey:
  enabled: true
  rpID: auth.enopax.io
  rpName: Enopax
  rpOrigins:
    - https://auth.enopax.io
  userVerification: preferred
  attestation: none
  timeout: 60000
  authenticatorSelection:
    authenticatorAttachment: platform
    requireResidentKey: true
    residentKey: preferred
    userVerification: preferred
```

### Field Reference

#### `enabled` (boolean)

- **Required**: No
- **Default**: `true`
- **Description**: Enable or disable passkey authentication

**Example**:
```yaml
passkey:
  enabled: false  # Disable passkeys entirely
```

#### `rpID` (string)

- **Required**: Yes (if passkey enabled)
- **Default**: None
- **Description**: Relying Party ID (domain of your Dex server)
- **Constraints**: Must be a valid domain, no port, no protocol

**Example**:
```yaml
rpID: auth.enopax.io  # ✅ Correct
rpID: https://auth.enopax.io  # ❌ Wrong (no protocol)
rpID: auth.enopax.io:5556  # ❌ Wrong (no port)
```

#### `rpName` (string)

- **Required**: Yes (if passkey enabled)
- **Default**: None
- **Description**: Human-readable name shown to users during authentication

**Example**:
```yaml
rpName: "Enopax Platform"
```

#### `rpOrigins` (array of strings)

- **Required**: Yes (if passkey enabled)
- **Default**: None
- **Description**: List of allowed origins for WebAuthn (must be HTTPS in production)

**Example**:
```yaml
rpOrigins:
  - https://auth.enopax.io
  - https://platform.enopax.io
  - https://app.enopax.io
```

**Development**:
```yaml
rpOrigins:
  - http://localhost:5556  # OK for development only
```

#### `userVerification` (string)

- **Required**: No
- **Default**: `"preferred"`
- **Options**: `"required"`, `"preferred"`, `"discouraged"`
- **Description**: User verification requirement (biometrics, PIN)

**Options**:
- `required`: Always require user verification (biometric/PIN)
- `preferred`: Request user verification but allow without it
- `discouraged`: Don't request user verification

**Example**:
```yaml
userVerification: required  # Always require biometric/PIN
```

#### `attestation` (string)

- **Required**: No
- **Default**: `"none"`
- **Options**: `"none"`, `"indirect"`, `"direct"`, `"enterprise"`
- **Description**: Attestation conveyance preference

**Options**:
- `none`: No attestation (fastest, most compatible)
- `indirect`: Anonymized attestation
- `direct`: Full attestation
- `enterprise`: Enterprise attestation

**Example**:
```yaml
attestation: none  # Recommended for most use cases
```

#### `timeout` (integer)

- **Required**: No
- **Default**: `60000` (60 seconds)
- **Description**: WebAuthn ceremony timeout in milliseconds

**Example**:
```yaml
timeout: 120000  # 2 minutes
```

#### `authenticatorSelection` (object)

- **Required**: No
- **Default**: See below
- **Description**: Constraints on authenticator selection

**Fields**:
```yaml
authenticatorSelection:
  authenticatorAttachment: platform  # "platform" or "cross-platform"
  requireResidentKey: true           # Require discoverable credentials
  residentKey: preferred             # "required", "preferred", "discouraged"
  userVerification: preferred        # "required", "preferred", "discouraged"
```

**Authenticator Types**:
- `platform`: Built-in authenticators (Touch ID, Windows Hello, Face ID)
- `cross-platform`: External authenticators (USB security keys, NFC)

**Example (Platform Authenticators Only)**:
```yaml
authenticatorSelection:
  authenticatorAttachment: platform
  requireResidentKey: true
  residentKey: required
  userVerification: preferred
```

**Example (Security Keys Only)**:
```yaml
authenticatorSelection:
  authenticatorAttachment: cross-platform
  requireResidentKey: false
  residentKey: discouraged
  userVerification: discouraged
```

### Passkey Best Practices

1. **Production**: Always use HTTPS
2. **RP ID**: Must match your domain exactly
3. **User Verification**: Use `"preferred"` for best compatibility
4. **Resident Keys**: Enable for passwordless authentication
5. **Timeout**: 60 seconds is sufficient for most users

---

## Two-Factor Authentication (2FA) Configuration

### 2FA Settings

```yaml
twoFactor:
  required: false
  methods:
    - totp
    - passkey
  gracePeriod: 604800  # 7 days
  totpIssuer: Enopax
  backupCodeCount: 10
```

### Field Reference

#### `required` (boolean)

- **Required**: No
- **Default**: `false`
- **Description**: Globally require 2FA for all users

**Example**:
```yaml
twoFactor:
  required: true  # All users must set up 2FA
```

**Note**: Individual users can still be required to use 2FA via the `require2FA` user flag, even if this is `false`.

#### `methods` (array of strings)

- **Required**: No
- **Default**: `["totp", "passkey"]`
- **Options**: `"totp"`, `"passkey"`, `"backup_code"`
- **Description**: Allowed 2FA methods

**Example**:
```yaml
methods:
  - totp        # TOTP authenticator apps
  - passkey     # WebAuthn passkeys as second factor
```

**Note**: `backup_code` is always available if user has backup codes, regardless of this setting.

#### `gracePeriod` (integer)

- **Required**: No
- **Default**: `604800` (7 days)
- **Description**: Grace period in seconds for users to set up 2FA after account creation

**Example**:
```yaml
gracePeriod: 1209600  # 14 days
```

During the grace period, users can log in without 2FA even if `required: true`.

#### `totpIssuer` (string)

- **Required**: No
- **Default**: `"Dex"`
- **Description**: Issuer name shown in TOTP authenticator apps

**Example**:
```yaml
totpIssuer: "Enopax Platform"
```

This appears as `Enopax Platform (user@example.com)` in Google Authenticator.

#### `backupCodeCount` (integer)

- **Required**: No
- **Default**: `10`
- **Description**: Number of backup codes to generate

**Example**:
```yaml
backupCodeCount: 20  # Generate 20 backup codes
```

### 2FA Policy Examples

**Example 1: Optional 2FA**
```yaml
twoFactor:
  required: false
  methods:
    - totp
    - passkey
  gracePeriod: 604800
```

Users can enable 2FA if they want, but it's not required.

**Example 2: Mandatory 2FA with Grace Period**
```yaml
twoFactor:
  required: true
  methods:
    - totp
    - passkey
  gracePeriod: 604800  # 7 days to set up 2FA
```

All users must set up 2FA within 7 days of account creation.

**Example 3: TOTP Only**
```yaml
twoFactor:
  required: true
  methods:
    - totp  # Only TOTP, no passkey as 2FA
  gracePeriod: 0  # No grace period
```

Users must set up TOTP immediately upon registration.

---

## Magic Link Configuration

### Magic Link Settings

```yaml
magicLink:
  enabled: true
  ttl: 600  # 10 minutes
  rateLimit:
    perHour: 3
    perDay: 10
  ipBinding: false
```

### Field Reference

#### `enabled` (boolean)

- **Required**: No
- **Default**: `true`
- **Description**: Enable or disable magic link authentication

**Example**:
```yaml
magicLink:
  enabled: false  # Disable magic links
```

#### `ttl` (integer)

- **Required**: No
- **Default**: `600` (10 minutes)
- **Description**: Magic link expiry time in seconds

**Example**:
```yaml
ttl: 300  # 5 minutes
ttl: 1800  # 30 minutes
```

**Recommendation**: 5-15 minutes for security vs. user convenience balance.

#### `rateLimit` (object)

- **Required**: No
- **Default**: `{ perHour: 3, perDay: 10 }`
- **Description**: Rate limiting for magic link requests per email address

**Fields**:
```yaml
rateLimit:
  perHour: 3   # Max 3 magic links per hour
  perDay: 10   # Max 10 magic links per day
```

**Example (Stricter Limits)**:
```yaml
rateLimit:
  perHour: 2
  perDay: 5
```

#### `ipBinding` (boolean)

- **Required**: No
- **Default**: `false`
- **Description**: Bind magic link to IP address (experimental)

**Example**:
```yaml
ipBinding: true  # Require same IP for link click
```

**Warning**: May cause issues with mobile users switching between WiFi and cellular.

---

## Email Configuration

### SMTP Settings

```yaml
email:
  smtp:
    host: smtp.sendgrid.net
    port: 587
    username: apikey
    password: ${SMTP_PASSWORD}
    from: noreply@enopax.io
    fromName: Enopax
    tls: true
    skipVerify: false
```

### Field Reference

#### `smtp.host` (string)

- **Required**: Yes
- **Description**: SMTP server hostname

**Examples**:
```yaml
# SendGrid
host: smtp.sendgrid.net

# Gmail
host: smtp.gmail.com

# AWS SES (us-east-1)
host: email-smtp.us-east-1.amazonaws.com

# Mailgun
host: smtp.mailgun.org
```

#### `smtp.port` (integer)

- **Required**: Yes
- **Default**: `587`
- **Description**: SMTP server port

**Common Ports**:
- `25`: Unencrypted (not recommended)
- `587`: STARTTLS (recommended)
- `465`: SSL/TLS

**Example**:
```yaml
port: 587  # STARTTLS
```

#### `smtp.username` (string)

- **Required**: Yes (for authenticated SMTP)
- **Description**: SMTP authentication username

**Example**:
```yaml
username: apikey  # SendGrid uses "apikey" as username
```

#### `smtp.password` (string)

- **Required**: Yes (for authenticated SMTP)
- **Description**: SMTP authentication password
- **Environment Variable**: Use `${SMTP_PASSWORD}` to reference env var

**Example**:
```yaml
password: ${SMTP_PASSWORD}  # Read from environment
```

**Security**: Never commit passwords to version control. Use environment variables.

#### `smtp.from` (string)

- **Required**: Yes
- **Description**: "From" email address

**Example**:
```yaml
from: noreply@enopax.io
```

**Note**: Must be a verified sender in your email provider.

#### `smtp.fromName` (string)

- **Required**: No
- **Default**: `""`
- **Description**: Display name for "From" field

**Example**:
```yaml
fromName: Enopax Platform
```

Shows as: `Enopax Platform <noreply@enopax.io>`

#### `smtp.tls` (boolean)

- **Required**: No
- **Default**: `true`
- **Description**: Use TLS/STARTTLS

**Example**:
```yaml
tls: true  # Always use TLS (recommended)
```

#### `smtp.skipVerify` (boolean)

- **Required**: No
- **Default**: `false`
- **Description**: Skip TLS certificate verification

**Example**:
```yaml
skipVerify: false  # Always verify certificates (recommended)
```

**Warning**: Only use `skipVerify: true` in development with self-signed certificates.

### Email Provider Examples

**SendGrid**:
```yaml
email:
  smtp:
    host: smtp.sendgrid.net
    port: 587
    username: apikey
    password: ${SENDGRID_API_KEY}
    from: noreply@example.com
    fromName: Example App
```

**AWS SES**:
```yaml
email:
  smtp:
    host: email-smtp.us-east-1.amazonaws.com
    port: 587
    username: ${AWS_SMTP_USERNAME}
    password: ${AWS_SMTP_PASSWORD}
    from: noreply@example.com
    fromName: Example App
```

**Mailgun**:
```yaml
email:
  smtp:
    host: smtp.mailgun.org
    port: 587
    username: postmaster@mg.example.com
    password: ${MAILGUN_SMTP_PASSWORD}
    from: noreply@example.com
    fromName: Example App
```

**Gmail** (for development only):
```yaml
email:
  smtp:
    host: smtp.gmail.com
    port: 587
    username: your-email@gmail.com
    password: ${GMAIL_APP_PASSWORD}  # Use App Password, not regular password
    from: your-email@gmail.com
    fromName: Development
```

---

## Storage Configuration

### File Storage Settings

```yaml
connectors:
  - type: local-enhanced
    config:
      dataDir: /var/lib/dex/data
      filePermissions: 0600
      dirPermissions: 0700
```

### Field Reference

#### `dataDir` (string)

- **Required**: Yes
- **Description**: Directory for file storage

**Example**:
```yaml
dataDir: /var/lib/dex/data
```

**Storage Structure**:
```
/var/lib/dex/data/
├── users/                      # User records
│   └── {user-id}.json
├── webauthn-sessions/          # WebAuthn challenge sessions
│   └── {session-id}.json
├── magic-link-tokens/          # Magic link tokens
│   └── {token}.json
├── auth-setup-tokens/          # Auth setup tokens
│   └── {token}.json
└── 2fa-sessions/               # 2FA sessions
    └── {session-id}.json
```

#### `filePermissions` (integer)

- **Required**: No
- **Default**: `0600` (owner read/write only)
- **Description**: File permissions for stored data

**Example**:
```yaml
filePermissions: 0600  # -rw-------
```

**Options**:
- `0600`: Owner read/write (recommended)
- `0640`: Owner read/write, group read
- `0644`: Owner read/write, group/others read (not recommended for sensitive data)

#### `dirPermissions` (integer)

- **Required**: No
- **Default**: `0700` (owner read/write/execute only)
- **Description**: Directory permissions

**Example**:
```yaml
dirPermissions: 0700  # drwx------
```

### Storage Best Practices

1. **Permissions**: Use `0600` for files, `0700` for directories
2. **Backup**: Regularly backup `/var/lib/dex/data`
3. **Encryption**: Use filesystem encryption (LUKS, dm-crypt)
4. **Cleanup**: Old sessions are automatically cleaned up
5. **Monitoring**: Monitor disk usage

---

## gRPC Configuration

### gRPC Server Settings

```yaml
grpc:
  addr: 0.0.0.0:5557
  tlsCert: /etc/dex/tls/grpc-cert.pem
  tlsKey: /etc/dex/tls/grpc-key.pem
  tlsClientCA: /etc/dex/tls/ca.pem
```

### Field Reference

#### `addr` (string)

- **Required**: Yes (if gRPC enabled)
- **Description**: gRPC server listen address

**Examples**:
```yaml
addr: 0.0.0.0:5557        # All interfaces
addr: 127.0.0.1:5557      # Localhost only
addr: 10.0.1.5:5557       # Specific interface
```

**Production**: Bind to internal network interface or localhost if using reverse proxy.

#### `tlsCert` (string)

- **Required**: Yes (for TLS)
- **Description**: Path to TLS certificate

**Example**:
```yaml
tlsCert: /etc/dex/tls/grpc-cert.pem
```

#### `tlsKey` (string)

- **Required**: Yes (for TLS)
- **Description**: Path to TLS private key

**Example**:
```yaml
tlsKey: /etc/dex/tls/grpc-key.pem
```

#### `tlsClientCA` (string)

- **Required**: No
- **Description**: Path to client CA certificate (for mTLS)

**Example**:
```yaml
tlsClientCA: /etc/dex/tls/ca.pem
```

**mTLS**: Require client certificates for authentication.

### gRPC Security Examples

**Development (Insecure)**:
```yaml
grpc:
  addr: 127.0.0.1:5557
  # No TLS - localhost only
```

**Production (TLS)**:
```yaml
grpc:
  addr: 0.0.0.0:5557
  tlsCert: /etc/dex/tls/grpc-cert.pem
  tlsKey: /etc/dex/tls/grpc-key.pem
```

**Production (mTLS)**:
```yaml
grpc:
  addr: 0.0.0.0:5557
  tlsCert: /etc/dex/tls/grpc-cert.pem
  tlsKey: /etc/dex/tls/grpc-key.pem
  tlsClientCA: /etc/dex/tls/ca.pem  # Require client certs
```

---

## OAuth Client Configuration

### Static Client Configuration

```yaml
staticClients:
  - id: platform-client
    redirectURIs:
      - https://platform.enopax.io/api/auth/callback/dex
    name: Enopax Platform
    secret: ${PLATFORM_CLIENT_SECRET}
    trustedPeers:
      - other-client-id
```

### Field Reference

#### `id` (string)

- **Required**: Yes
- **Description**: Unique client identifier

**Example**:
```yaml
id: platform-client
```

#### `redirectURIs` (array of strings)

- **Required**: Yes
- **Description**: Allowed OAuth callback URLs

**Example**:
```yaml
redirectURIs:
  - https://platform.enopax.io/api/auth/callback/dex
  - https://staging.enopax.io/api/auth/callback/dex
```

**Note**: Must be exact matches (including protocol, domain, port, path).

#### `name` (string)

- **Required**: Yes
- **Description**: Human-readable client name

**Example**:
```yaml
name: Enopax Platform
```

#### `secret` (string)

- **Required**: Yes
- **Description**: Client secret for authentication

**Example**:
```yaml
secret: ${PLATFORM_CLIENT_SECRET}  # Use environment variable
```

**Security**: Use long, random secrets (32+ characters).

#### `trustedPeers` (array of strings)

- **Required**: No
- **Description**: Other client IDs that can request tokens on behalf of this client

**Example**:
```yaml
trustedPeers:
  - mobile-app-client
  - cli-client
```

### Multiple Clients Example

```yaml
staticClients:
  # Web Platform
  - id: web-platform
    redirectURIs:
      - https://platform.enopax.io/callback
    name: Enopax Web Platform
    secret: ${WEB_PLATFORM_SECRET}

  # Mobile App
  - id: mobile-app
    redirectURIs:
      - enopax://callback
    name: Enopax Mobile App
    secret: ${MOBILE_APP_SECRET}
    public: true  # PKCE-only, no client secret validation

  # CLI Tool
  - id: cli-tool
    redirectURIs:
      - http://localhost:8080/callback
    name: Enopax CLI
    secret: ${CLI_TOOL_SECRET}
```

---

## Security Configuration

### Security Best Practices

```yaml
# Use HTTPS in production
issuer: https://auth.enopax.io

web:
  tlsCert: /etc/dex/tls/cert.pem
  tlsKey: /etc/dex/tls/key.pem
  tlsMinVersion: "1.2"
  tlsCipherSuites:
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384

# Secure session configuration
expiry:
  idTokens: "10m"
  authRequests: "24h"
  deviceRequests: "5m"

# Enable audit logging
logger:
  level: info
  format: json
```

### Field Reference

#### `tlsMinVersion` (string)

- **Required**: No
- **Default**: `"1.2"`
- **Options**: `"1.0"`, `"1.1"`, `"1.2"`, `"1.3"`
- **Description**: Minimum TLS version

**Example**:
```yaml
tlsMinVersion: "1.3"  # TLS 1.3 only (most secure)
```

#### `tlsCipherSuites` (array of strings)

- **Required**: No
- **Description**: Allowed TLS cipher suites

**Recommended**:
```yaml
tlsCipherSuites:
  - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
  - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
  - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
  - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
```

#### `expiry` (object)

- **Required**: No
- **Description**: Token and request expiry times

**Fields**:
```yaml
expiry:
  idTokens: "10m"         # ID token lifetime
  authRequests: "24h"     # Auth request TTL
  deviceRequests: "5m"    # Device flow request TTL
```

**Recommendations**:
- `idTokens`: 10-60 minutes
- `authRequests`: 10-60 minutes (long enough for users to complete auth)
- `deviceRequests`: 5-10 minutes

---

## Environment Variables

### Required Environment Variables

```bash
# SMTP Password
export SMTP_PASSWORD="your-smtp-password"

# OAuth Client Secrets
export PLATFORM_CLIENT_SECRET="your-client-secret"

# Optional: Database password
export DATABASE_PASSWORD="your-db-password"
```

### Variable Substitution

Dex supports environment variable substitution in config files using `${VAR_NAME}` syntax:

```yaml
email:
  smtp:
    password: ${SMTP_PASSWORD}

staticClients:
  - id: platform
    secret: ${PLATFORM_CLIENT_SECRET}

storage:
  config:
    password: ${DATABASE_PASSWORD}
```

### Systemd Environment File

Create `/etc/dex/environment`:

```bash
# Email Configuration
SMTP_PASSWORD=your-smtp-password

# OAuth Secrets
PLATFORM_CLIENT_SECRET=your-client-secret
MOBILE_APP_SECRET=your-mobile-secret

# Database
DATABASE_PASSWORD=your-db-password
```

Systemd service file:

```ini
[Unit]
Description=Dex OpenID Connect Provider
After=network.target

[Service]
Type=simple
User=dex
Group=dex
EnvironmentFile=/etc/dex/environment
ExecStart=/usr/local/bin/dex serve /etc/dex/config.yaml
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

---

## Deployment Examples

### Development Environment

```yaml
# config.dev.yaml

issuer: http://localhost:5556

storage:
  type: sqlite3
  config:
    file: dev.db

web:
  http: 127.0.0.1:5556

grpc:
  addr: 127.0.0.1:5557

connectors:
  - type: local-enhanced
    id: local
    name: Dev Auth
    config:
      baseURL: http://localhost:5556
      dataDir: ./data
      passkey:
        enabled: true
        rpID: localhost
        rpName: Dev
        rpOrigins:
          - http://localhost:5556
      magicLink:
        enabled: true
        ttl: 600
      email:
        smtp:
          host: smtp.mailtrap.io  # Development email testing
          port: 587
          username: ${MAILTRAP_USERNAME}
          password: ${MAILTRAP_PASSWORD}
          from: dev@example.com

staticClients:
  - id: dev-client
    redirectURIs:
      - http://localhost:3000/callback
    name: Dev Client
    secret: dev-secret
```

### Staging Environment

```yaml
# config.staging.yaml

issuer: https://auth-staging.enopax.io

storage:
  type: postgres
  config:
    host: postgres.staging.internal
    port: 5432
    database: dex
    user: dex
    password: ${DATABASE_PASSWORD}
    ssl:
      mode: require

web:
  https: 0.0.0.0:5556
  tlsCert: /etc/dex/tls/cert.pem
  tlsKey: /etc/dex/tls/key.pem

grpc:
  addr: 0.0.0.0:5557
  tlsCert: /etc/dex/tls/grpc-cert.pem
  tlsKey: /etc/dex/tls/grpc-key.pem

connectors:
  - type: local-enhanced
    id: local
    name: Staging Auth
    config:
      baseURL: https://auth-staging.enopax.io
      dataDir: /var/lib/dex/data
      passkey:
        enabled: true
        rpID: auth-staging.enopax.io
        rpName: Enopax Staging
        rpOrigins:
          - https://auth-staging.enopax.io
          - https://platform-staging.enopax.io
      twoFactor:
        required: false
        gracePeriod: 604800
      magicLink:
        enabled: true
        ttl: 600
      email:
        smtp:
          host: smtp.sendgrid.net
          port: 587
          username: apikey
          password: ${SENDGRID_API_KEY}
          from: noreply-staging@enopax.io
          fromName: Enopax Staging

staticClients:
  - id: platform-staging
    redirectURIs:
      - https://platform-staging.enopax.io/callback
    name: Platform Staging
    secret: ${PLATFORM_STAGING_SECRET}
```

### Production Environment

```yaml
# config.prod.yaml

issuer: https://auth.enopax.io

storage:
  type: postgres
  config:
    host: postgres.prod.internal
    port: 5432
    database: dex_prod
    user: dex
    password: ${DATABASE_PASSWORD}
    ssl:
      mode: require
      caFile: /etc/ssl/certs/postgres-ca.pem

web:
  https: 0.0.0.0:5556
  tlsCert: /etc/dex/tls/cert.pem
  tlsKey: /etc/dex/tls/key.pem
  tlsMinVersion: "1.2"
  tlsCipherSuites:
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384

grpc:
  addr: 0.0.0.0:5557
  tlsCert: /etc/dex/tls/grpc-cert.pem
  tlsKey: /etc/dex/tls/grpc-key.pem
  tlsClientCA: /etc/dex/tls/ca.pem  # mTLS

connectors:
  - type: local-enhanced
    id: local
    name: Enopax Authentication
    config:
      baseURL: https://auth.enopax.io
      dataDir: /var/lib/dex/data
      passkey:
        enabled: true
        rpID: auth.enopax.io
        rpName: Enopax
        rpOrigins:
          - https://auth.enopax.io
          - https://platform.enopax.io
        userVerification: preferred
      twoFactor:
        required: true
        methods:
          - totp
          - passkey
        gracePeriod: 604800  # 7 days
      magicLink:
        enabled: true
        ttl: 600
        rateLimit:
          perHour: 3
          perDay: 10
      email:
        smtp:
          host: email-smtp.us-east-1.amazonaws.com
          port: 587
          username: ${AWS_SMTP_USERNAME}
          password: ${AWS_SMTP_PASSWORD}
          from: noreply@enopax.io
          fromName: Enopax
          tls: true

staticClients:
  - id: platform-prod
    redirectURIs:
      - https://platform.enopax.io/api/auth/callback/dex
    name: Enopax Platform
    secret: ${PLATFORM_PROD_SECRET}

expiry:
  idTokens: "10m"
  authRequests: "10m"

logger:
  level: info
  format: json
```

---

## Troubleshooting

### Configuration Validation

Check configuration syntax:

```bash
dex serve --dry-run config.yaml
```

### Common Configuration Errors

#### 1. Invalid RP ID

**Error**: `WebAuthn registration failed: invalid RP ID`

**Solution**:
```yaml
# ❌ Wrong
passkey:
  rpID: https://auth.enopax.io  # No protocol

# ✅ Correct
passkey:
  rpID: auth.enopax.io
```

#### 2. Origin Mismatch

**Error**: `Origin not allowed`

**Solution**: Add origin to `rpOrigins`:
```yaml
rpOrigins:
  - https://auth.enopax.io
  - https://platform.enopax.io  # Add your platform URL
```

#### 3. SMTP Authentication Failed

**Error**: `SMTP auth failed: 535 Authentication failed`

**Solutions**:
- Verify SMTP credentials
- Check firewall allows outbound port 587
- Use app-specific password (Gmail)
- Verify sender email is authorized

#### 4. Redirect URI Mismatch

**Error**: `redirect_uri_mismatch`

**Solution**: Ensure exact match in client config:
```yaml
staticClients:
  - id: client
    redirectURIs:
      - https://app.example.com/callback  # Must match exactly
```

#### 5. File Permission Errors

**Error**: `permission denied` when writing to data directory

**Solution**:
```bash
# Fix permissions
sudo chown -R dex:dex /var/lib/dex/data
sudo chmod 700 /var/lib/dex/data
sudo chmod 600 /var/lib/dex/data/*/*.json
```

### Debug Logging

Enable debug logging:

```yaml
logger:
  level: debug
  format: json
```

View logs:

```bash
# Systemd
journalctl -u dex -f

# Docker
docker logs -f dex

# Binary
tail -f /var/log/dex/dex.log
```

---

## Additional Resources

- **Dex Documentation**: https://dexidp.io/docs/
- **WebAuthn Guide**: https://webauthn.guide/
- **gRPC API Reference**: `/docs/enhancements/grpc-api.md`
- **Authentication Flows**: `/docs/enhancements/authentication-flows.md`
- **Platform Integration**: `/docs/enhancements/platform-integration.md`

---

**Last Updated**: 2025-11-18
**Version**: 1.0
**Author**: Enopax Platform Team
