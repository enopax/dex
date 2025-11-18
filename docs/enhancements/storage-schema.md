# Enhanced Local Connector Storage Schema

**Last Updated**: 2025-11-18
**Version**: 1.0

---

## Overview

The Enhanced Local Connector uses a **file-based storage system** for user data, credentials, and sessions. This design prioritizes simplicity, portability, and ease of debugging while maintaining security and performance.

---

## Storage Directory Structure

```
data/
├── users/                    # User accounts
│   ├── {user-id}.json
│   └── ...
├── passkeys/                 # WebAuthn passkey credentials
│   ├── {credential-id}.json
│   └── ...
├── sessions/                 # WebAuthn challenge sessions
│   ├── {session-id}.json
│   └── ...
└── tokens/                   # Magic link tokens
    ├── {token}.json
    └── ...
```

### Directory Permissions

- **Root data directory**: `0700` (owner read/write/execute only)
- **Subdirectories**: `0700` (owner read/write/execute only)
- **JSON files**: `0600` (owner read/write only)

---

## User Schema

**File**: `data/users/{user-id}.json`

**User ID Generation**: Deterministic SHA-256 hash of email address, formatted as UUID:
```go
hash := sha256.Sum256([]byte(email))
userID := fmt.Sprintf("%x-%x-%x-%x-%x",
    hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
```

**Example**: Email `alice@example.com` → User ID `2bd806c9-7f04-5ef4-3fbb-c05c47ba1f15`

### User JSON Structure

```json
{
  "id": "2bd806c9-7f04-5ef4-3fbb-c05c47ba1f15",
  "email": "alice@example.com",
  "username": "alice",
  "display_name": "Alice Smith",
  "email_verified": true,

  "password_hash": "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy",

  "passkeys": [
    {
      "id": "credential-id-base64",
      "user_id": "2bd806c9-7f04-5ef4-3fbb-c05c47ba1f15",
      "public_key": "base64-encoded-public-key",
      "attestation_type": "none",
      "aaguid": "base64-encoded-aaguid",
      "sign_count": 42,
      "transports": ["usb", "nfc"],
      "name": "My YubiKey",
      "created_at": "2025-11-18T10:00:00Z",
      "last_used_at": "2025-11-18T15:30:00Z",
      "backup_eligible": true,
      "backup_state": false
    }
  ],

  "totp_secret": "JBSWY3DPEHPK3PXP",
  "totp_enabled": true,

  "backup_codes": [
    {
      "code": "$2a$10$hashed-backup-code-1",
      "used": false,
      "used_at": null
    },
    {
      "code": "$2a$10$hashed-backup-code-2",
      "used": true,
      "used_at": "2025-11-18T12:00:00Z"
    }
  ],

  "magic_link_enabled": true,
  "require_2fa": false,

  "created_at": "2025-11-01T09:00:00Z",
  "updated_at": "2025-11-18T15:30:00Z",
  "last_login_at": "2025-11-18T15:30:00Z"
}
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Deterministic user ID (SHA-256 of email) |
| `email` | string | Yes | User's email address (unique) |
| `username` | string | No | Username (optional) |
| `display_name` | string | No | Display name |
| `email_verified` | bool | Yes | Whether email is verified |
| `password_hash` | string | No | Bcrypt password hash (null if passwordless) |
| `passkeys` | array | No | List of registered passkeys |
| `totp_secret` | string | No | Base32-encoded TOTP secret |
| `totp_enabled` | bool | Yes | Whether TOTP 2FA is enabled |
| `backup_codes` | array | No | Hashed backup codes for 2FA recovery |
| `magic_link_enabled` | bool | Yes | Whether magic link auth is allowed |
| `require_2fa` | bool | Yes | Whether user must use 2FA |
| `created_at` | timestamp | Yes | User creation timestamp |
| `updated_at` | timestamp | Yes | Last update timestamp |
| `last_login_at` | timestamp | No | Last successful login |

---

## Passkey Schema

**File**: `data/passkeys/{credential-id}.json`

Passkeys are **also embedded in the user record**, but stored separately for efficient lookup during authentication (when we only have the credential ID).

```json
{
  "id": "credential-id-base64",
  "user_id": "2bd806c9-7f04-5ef4-3fbb-c05c47ba1f15",
  "public_key": "base64-encoded-public-key",
  "attestation_type": "none",
  "aaguid": "base64-encoded-aaguid",
  "sign_count": 42,
  "transports": ["usb", "nfc"],
  "name": "My YubiKey",
  "created_at": "2025-11-18T10:00:00Z",
  "last_used_at": "2025-11-18T15:30:00Z",
  "backup_eligible": true,
  "backup_state": false
}
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Credential ID (base64 encoded) |
| `user_id` | string | Yes | ID of the user who owns this passkey |
| `public_key` | bytes | Yes | Credential's public key |
| `attestation_type` | string | Yes | Attestation type ("none", "indirect", "direct") |
| `aaguid` | bytes | Yes | Authenticator's AAGUID |
| `sign_count` | uint32 | Yes | Signature counter (for clone detection) |
| `transports` | array | No | Supported transports (usb, nfc, ble, internal) |
| `name` | string | Yes | User-friendly credential name |
| `created_at` | timestamp | Yes | Registration timestamp |
| `last_used_at` | timestamp | No | Last authentication timestamp |
| `backup_eligible` | bool | Yes | Whether credential can be backed up |
| `backup_state` | bool | Yes | Whether credential is backed up |

---

## WebAuthn Session Schema

**File**: `data/sessions/{session-id}.json`

WebAuthn sessions track challenge/response flows for both registration and authentication.

```json
{
  "session_id": "session-id-hex",
  "user_id": "2bd806c9-7f04-5ef4-3fbb-c05c47ba1f15",
  "challenge": "base64-encoded-challenge",
  "operation": "registration",
  "expires_at": "2025-11-18T10:05:00Z",
  "created_at": "2025-11-18T10:00:00Z"
}
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | Yes | Unique session identifier (hex) |
| `user_id` | string | Yes | ID of the user |
| `challenge` | bytes | Yes | WebAuthn challenge (32 bytes) |
| `operation` | string | Yes | Either "registration" or "authentication" |
| `expires_at` | timestamp | Yes | Expiration time (typically 5 minutes) |
| `created_at` | timestamp | Yes | Creation timestamp |

**Session Lifetime**: 5 minutes (300 seconds)

**Cleanup**: Expired sessions are automatically cleaned up by periodic cleanup task.

---

## Magic Link Token Schema

**File**: `data/tokens/{token}.json`

Magic link tokens are single-use tokens sent via email for passwordless authentication.

```json
{
  "token": "token-hex-64-chars",
  "user_id": "2bd806c9-7f04-5ef4-3fbb-c05c47ba1f15",
  "email": "alice@example.com",
  "created_at": "2025-11-18T10:00:00Z",
  "expires_at": "2025-11-18T10:10:00Z",
  "used": false,
  "ip_address": "192.168.1.100"
}
```

### Field Descriptions

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `token` | string | Yes | Secure random token (64-char hex) |
| `user_id` | string | Yes | ID of the user |
| `email` | string | Yes | Email address the link was sent to |
| `created_at` | timestamp | Yes | Creation timestamp |
| `expires_at` | timestamp | Yes | Expiration time (typically 10 minutes) |
| `used` | bool | Yes | Whether token has been used |
| `ip_address` | string | No | IP address that requested the link |

**Token Lifetime**: 10 minutes (600 seconds)

**Single-Use**: Token is marked as used after first verification.

**Cleanup**: Expired tokens are automatically cleaned up by periodic cleanup task.

---

## File Operations

### Atomic Writes

All file writes use **atomic rename** to prevent partial writes:

1. Write data to temporary file: `{path}.tmp`
2. Sync file to disk
3. Atomic rename: `mv {path}.tmp {path}`

### File Locking

File operations use **flock** for concurrent access:

- **Exclusive lock** (`LOCK_EX`): For writes
- **Shared lock** (`LOCK_SH`): For reads

### Concurrency Safety

The `FileStorage` implementation uses a `sync.RWMutex` for additional safety:

- **Read operations**: `RLock()` - Multiple concurrent reads allowed
- **Write operations**: `Lock()` - Exclusive access

---

## Migration from Old Storage

### Old Password Storage Format

```json
{
  "email": "alice@example.com",
  "hash": "$2a$10$...",
  "username": "alice",
  "userID": "ff8d9819-fc0e-12bf-0d24-892e45987e24"
}
```

### Migration Strategy

**Option 1: One-time migration script**

```bash
# Migrate all users from old format to new format
./bin/dex migrate-storage --from=./data/passwords --to=./data/users
```

**Option 2: Lazy migration**

- Keep old storage alongside new storage
- On first login, migrate user from old → new format
- Delete old user file after successful migration

**Recommended**: Option 2 (lazy migration) for zero-downtime migration.

---

## Storage Interface

The storage interface abstracts file operations and allows for future backend implementations (SQL, Redis, etc.):

```go
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

    // Cleanup operations
    CleanupExpiredSessions(ctx context.Context) error
    CleanupExpiredTokens(ctx context.Context) error
}
```

---

## Security Considerations

### Password Storage

- **Algorithm**: bcrypt with cost factor 10
- **Never stored in plaintext**
- **Null value allowed** for passwordless accounts

### TOTP Secrets

- **Base32 encoded** for QR code compatibility
- **Stored encrypted at rest** (TODO: implement encryption)
- **Access logged** for security audit

### Backup Codes

- **Hashed with bcrypt** (never plaintext)
- **Single-use** (marked as used after validation)
- **10 codes generated** per user

### Passkey Public Keys

- **Public keys are safe to store** (not secret)
- **Sign count tracked** for clone detection
- **Increment on each use**

### Magic Link Tokens

- **Cryptographically secure random** (32 bytes → 64 hex chars)
- **Short-lived** (10 minutes)
- **Single-use** (marked as used)
- **IP binding** (optional, stored for audit)

---

## Performance Considerations

### File Count

With **10,000 users** and average **2 passkeys per user**:

- Users: 10,000 files
- Passkeys: 20,000 files
- Sessions: ~100 files (active challenges)
- Tokens: ~50 files (active magic links)

**Total**: ~30,150 files

### Read Performance

- **User lookup by email**: Single file read (deterministic ID)
- **Passkey lookup**: Single file read
- **List user passkeys**: Directory scan + filter (optimizable with index)

### Write Performance

- **Atomic writes**: Safe but slower than in-place writes
- **File locking**: Prevents corruption, adds overhead
- **Mutex**: Additional thread safety

**Optimization**: Use in-memory cache for frequently accessed users.

---

## Future Enhancements

### Encryption at Rest

Add encryption for sensitive fields:

- TOTP secrets
- Magic link tokens
- Session challenges

**Implementation**: Use `age` or `gpg` for file encryption.

### Index Files

Add index files for faster queries:

- `data/indexes/email-to-user.json` - Email → User ID mapping
- `data/indexes/passkey-to-user.json` - Credential ID → User ID mapping

### SQL Backend

Implement SQL storage backend for production scale:

- PostgreSQL or SQLite
- Schema migration scripts
- Connection pooling

---

## Testing

### Unit Tests

Test all storage operations:

- User CRUD
- Passkey CRUD
- Session CRUD
- Token CRUD
- Concurrent access
- Error handling

### Integration Tests

Test complete flows:

- User registration → passkey setup → login
- Session expiry and cleanup
- Token expiry and cleanup

---

## Example Usage

```go
// Create storage
storage, err := NewFileStorage("./data")
if err != nil {
    log.Fatal(err)
}

// Create user
user := &User{
    ID:            generateUserID("alice@example.com"),
    Email:         "alice@example.com",
    EmailVerified: true,
}
err = storage.CreateUser(context.Background(), user)

// Get user by email
user, err = storage.GetUserByEmail(context.Background(), "alice@example.com")

// Save passkey
passkey := &Passkey{
    ID:        "credential-id",
    UserID:    user.ID,
    PublicKey: publicKeyBytes,
    Name:      "My YubiKey",
}
err = storage.SavePasskey(context.Background(), passkey)

// List user's passkeys
passkeys, err := storage.ListPasskeys(context.Background(), user.ID)
```

---

**Version**: 1.0
**Last Updated**: 2025-11-18
**Status**: Complete
