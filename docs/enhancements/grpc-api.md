# Enhanced Local Connector gRPC API

**Purpose**: gRPC API for managing users in the Enhanced Local Connector from the Enopax Platform.

**Version**: 1.0
**Last Updated**: 2025-11-18

---

## Table of Contents

1. [Overview](#overview)
2. [Service Definition](#service-definition)
3. [Authentication](#authentication)
4. [User Management](#user-management)
5. [Password Management](#password-management)
6. [TOTP Management](#totp-management)
7. [Passkey Management](#passkey-management)
8. [Authentication Methods](#authentication-methods)
9. [Error Handling](#error-handling)
10. [Usage Examples](#usage-examples)

---

## Overview

The Enhanced Local Connector gRPC API provides programmatic access to user management and authentication method configuration. This API is designed to be used by the Enopax Platform for:

- Creating user accounts during registration
- Managing user credentials (passwords, passkeys, TOTP)
- Querying authentication method status
- Managing user settings (2FA requirements, email verification)

**Service Name**: `EnhancedLocalConnector`
**Protocol**: gRPC (HTTP/2)
**Protobuf Definition**: `api/v2/api.proto`

---

## Service Definition

```protobuf
service EnhancedLocalConnector {
  // User Management
  rpc CreateUser(CreateUserReq) returns (CreateUserResp);
  rpc GetUser(GetUserReq) returns (GetUserResp);
  rpc UpdateUser(UpdateUserReq) returns (UpdateUserResp);
  rpc DeleteUser(DeleteUserReq) returns (DeleteUserResp);

  // Password Management
  rpc SetPassword(SetPasswordReq) returns (SetPasswordResp);
  rpc RemovePassword(RemovePasswordReq) returns (RemovePasswordResp);

  // TOTP Management
  rpc EnableTOTP(EnableTOTPReq) returns (EnableTOTPResp);
  rpc VerifyTOTPSetup(VerifyTOTPSetupReq) returns (VerifyTOTPSetupResp);
  rpc DisableTOTP(DisableTOTPReq) returns (DisableTOTPResp);
  rpc GetTOTPInfo(GetTOTPInfoReq) returns (GetTOTPInfoResp);
  rpc RegenerateBackupCodes(RegenerateBackupCodesReq) returns (RegenerateBackupCodesResp);

  // Passkey Management
  rpc ListPasskeys(ListPasskeysReq) returns (ListPasskeysResp);
  rpc RenamePasskey(RenamePasskeyReq) returns (RenamePasskeyResp);
  rpc DeletePasskey(DeletePasskeyReq) returns (DeletePasskeyResp);

  // Authentication Method Info
  rpc GetAuthMethods(GetAuthMethodsReq) returns (GetAuthMethodsResp);
}
```

---

## Authentication

### API Authentication (TODO)

Currently, the gRPC API does not implement authentication. This will be added in a future version using one of:

- **API Keys**: Static tokens configured in Dex
- **mTLS**: Mutual TLS certificate authentication
- **JWT**: Platform-issued JWT tokens

**Security Note**: Until authentication is implemented, the gRPC API should only be exposed on a trusted internal network.

---

## User Management

### CreateUser

Creates a new user account.

**Request**:
```protobuf
message CreateUserReq {
  string email = 1;        // Required: Valid email address
  string username = 2;     // Optional: Username (3-64 chars)
  string display_name = 3; // Optional: Display name
}
```

**Response**:
```protobuf
message CreateUserResp {
  EnhancedUser user = 1;
  bool already_exists = 2; // True if user already exists
}
```

**Behavior**:
- If user with email already exists, returns existing user with `already_exists = true`
- User ID is deterministic (derived from email via SHA-256)
- Email is not verified by default (`email_verified = false`)
- 2FA is not required by default (`require_2fa = false`)
- User has no authentication methods by default

**Validation**:
- Email must be valid RFC 5322 format
- Username must be 3-64 alphanumeric characters (if provided)

**Example** (Go):
```go
resp, err := client.CreateUser(ctx, &api.CreateUserReq{
    Email:       "alice@example.com",
    Username:    "alice",
    DisplayName: "Alice Smith",
})
if err != nil {
    log.Fatalf("Failed to create user: %v", err)
}
if resp.AlreadyExists {
    log.Printf("User already exists: %s", resp.User.Id)
} else {
    log.Printf("Created user: %s", resp.User.Id)
}
```

---

### GetUser

Retrieves user details by ID or email.

**Request**:
```protobuf
message GetUserReq {
  string user_id = 1; // Optional: User ID
  string email = 2;   // Optional: Email address
}
```

**Note**: Either `user_id` or `email` must be provided (at least one).

**Response**:
```protobuf
message GetUserResp {
  EnhancedUser user = 1;
  bool not_found = 2;
}
```

**Example** (Go):
```go
// Get by user ID
resp, err := client.GetUser(ctx, &api.GetUserReq{
    UserId: "ff8d9819-fc0e-12bf-0d24-892e45987e24",
})

// Get by email
resp, err := client.GetUser(ctx, &api.GetUserReq{
    Email: "alice@example.com",
})
```

---

### UpdateUser

Updates an existing user's details.

**Request**:
```protobuf
message UpdateUserReq {
  string user_id = 1;        // Required: User ID
  string username = 2;       // Optional: New username
  string display_name = 3;   // Optional: New display name
  bool email_verified = 4;   // Set email verification status
  bool require_2fa = 5;      // Set 2FA requirement
}
```

**Response**:
```protobuf
message UpdateUserResp {
  bool not_found = 1;
}
```

**Behavior**:
- Only provided fields are updated
- Empty strings for username/display_name are ignored
- `updated_at` timestamp is automatically updated

**Example** (Go):
```go
resp, err := client.UpdateUser(ctx, &api.UpdateUserReq{
    UserId:        userID,
    EmailVerified: true,
    Require2Fa:    true,
})
```

---

### DeleteUser

Deletes a user account and all associated credentials.

**Request**:
```protobuf
message DeleteUserReq {
  string user_id = 1; // Required: User ID
}
```

**Response**:
```protobuf
message DeleteUserResp {
  bool not_found = 1;
}
```

**Behavior**:
- Deletes user file and all associated data
- Deletes all passkeys, TOTP secret, backup codes
- Deletes all WebAuthn sessions and magic link tokens
- Operation is permanent (no soft delete)

---

## Password Management

### SetPassword

Sets or updates a user's password.

**Request**:
```protobuf
message SetPasswordReq {
  string user_id = 1;  // Required: User ID
  string password = 2; // Required: Password (8-128 chars)
}
```

**Response**:
```protobuf
message SetPasswordResp {
  bool not_found = 1;
}
```

**Password Requirements**:
- Length: 8-128 characters
- Must contain at least one letter
- Must contain at least one number

**Security**:
- Password is hashed using bcrypt (cost factor 10)
- Plaintext password is never stored

**Example** (Go):
```go
resp, err := client.SetPassword(ctx, &api.SetPasswordReq{
    UserId:   userID,
    Password: "SecurePass123",
})
```

---

### RemovePassword

Removes a user's password (for passwordless accounts).

**Request**:
```protobuf
message RemovePasswordReq {
  string user_id = 1; // Required: User ID
}
```

**Response**:
```protobuf
message RemovePasswordResp {
  bool not_found = 1;
}
```

**Use Case**: Converting a password-based account to passwordless (passkey-only or magic link-only).

**Warning**: Ensure user has at least one other authentication method before removing password.

---

## TOTP Management

### EnableTOTP

Begins TOTP setup and returns QR code and backup codes.

**Request**:
```protobuf
message EnableTOTPReq {
  string user_id = 1; // Required: User ID
}
```

**Response**:
```protobuf
message EnableTOTPResp {
  bool not_found = 1;
  bool already_enabled = 2;
  string secret = 3;              // Base32-encoded TOTP secret
  string qr_code = 4;             // Base64-encoded PNG QR code
  string otpauth_url = 5;         // otpauth:// URL for manual entry
  repeated string backup_codes = 6; // 10 backup codes (8 chars each)
}
```

**Behavior**:
- Generates a new TOTP secret (32 bytes)
- Creates QR code (256x256 PNG)
- Generates 10 backup codes
- TOTP is **not** enabled until verified with `VerifyTOTPSetup`

**Example** (Go):
```go
resp, err := client.EnableTOTP(ctx, &api.EnableTOTPReq{
    UserId: userID,
})
if resp.AlreadyEnabled {
    log.Println("TOTP already enabled")
} else {
    // Display QR code to user
    fmt.Printf("Secret: %s\n", resp.Secret)
    fmt.Printf("QR Code (base64): %s\n", resp.QrCode)
    fmt.Printf("Backup codes: %v\n", resp.BackupCodes)
}
```

---

### VerifyTOTPSetup

Verifies TOTP code and completes setup.

**Request**:
```protobuf
message VerifyTOTPSetupReq {
  string user_id = 1;
  string secret = 2;              // TOTP secret from EnableTOTP
  string code = 3;                // 6-digit TOTP code
  repeated string backup_codes = 4; // Backup codes from EnableTOTP
}
```

**Response**:
```protobuf
message VerifyTOTPSetupResp {
  bool not_found = 1;
  bool invalid_code = 2;
}
```

**Behavior**:
- Validates TOTP code using provided secret
- If valid, enables TOTP and saves secret
- Hashes and saves backup codes
- Sets `totp_enabled = true`

**Example** (Go):
```go
resp, err := client.VerifyTOTPSetup(ctx, &api.VerifyTOTPSetupReq{
    UserId:      userID,
    Secret:      secret,
    Code:        "123456", // Code from authenticator app
    BackupCodes: backupCodes,
})
if resp.InvalidCode {
    log.Println("Invalid TOTP code")
}
```

---

### DisableTOTP

Disables TOTP 2FA for a user.

**Request**:
```protobuf
message DisableTOTPReq {
  string user_id = 1;
  string code = 2; // TOTP code or backup code for verification
}
```

**Response**:
```protobuf
message DisableTOTPResp {
  bool not_found = 1;
  bool invalid_code = 2;
}
```

**Security**: Requires valid TOTP code or backup code to disable TOTP.

---

### GetTOTPInfo

Gets TOTP status and remaining backup codes.

**Request**:
```protobuf
message GetTOTPInfoReq {
  string user_id = 1;
}
```

**Response**:
```protobuf
message GetTOTPInfoResp {
  bool not_found = 1;
  TOTPInfo totp_info = 2;
}

message TOTPInfo {
  bool enabled = 1;
  int32 backup_codes_remaining = 2;
}
```

---

### RegenerateBackupCodes

Generates new backup codes (invalidates old codes).

**Request**:
```protobuf
message RegenerateBackupCodesReq {
  string user_id = 1;
  string code = 2; // TOTP code for verification
}
```

**Response**:
```protobuf
message RegenerateBackupCodesResp {
  bool not_found = 1;
  bool invalid_code = 2;
  repeated string backup_codes = 3; // 10 new backup codes
}
```

---

## Passkey Management

### ListPasskeys

Lists all passkeys for a user.

**Request**:
```protobuf
message ListPasskeysReq {
  string user_id = 1;
}
```

**Response**:
```protobuf
message ListPasskeysResp {
  bool not_found = 1;
  repeated Passkey passkeys = 2;
}

message Passkey {
  string id = 1;
  string user_id = 2;
  string name = 3;
  int64 created_at = 4;
  int64 last_used_at = 5;
  repeated string transports = 6; // "usb", "nfc", "ble", "internal"
  bool backup_eligible = 7;
  bool backup_state = 8;
}
```

---

### RenamePasskey

Renames a passkey.

**Request**:
```protobuf
message RenamePasskeyReq {
  string user_id = 1;
  string passkey_id = 2;
  string new_name = 3;
}
```

**Response**:
```protobuf
message RenamePasskeyResp {
  bool not_found = 1;
}
```

---

### DeletePasskey

Deletes a passkey.

**Request**:
```protobuf
message DeletePasskeyReq {
  string user_id = 1;
  string passkey_id = 2;
}
```

**Response**:
```protobuf
message DeletePasskeyResp {
  bool not_found = 1;
}
```

**Warning**: Ensure user has at least one other authentication method before deleting passkey.

---

## Authentication Methods

### GetAuthMethods

Gets a user's configured authentication methods.

**Request**:
```protobuf
message GetAuthMethodsReq {
  string user_id = 1;
}
```

**Response**:
```protobuf
message GetAuthMethodsResp {
  bool not_found = 1;
  bool has_password = 2;
  int32 passkey_count = 3;
  bool totp_enabled = 4;
  bool magic_link_enabled = 5; // Global setting
}
```

**Use Case**: Check which authentication methods a user has configured before allowing removal.

**Example** (Go):
```go
resp, err := client.GetAuthMethods(ctx, &api.GetAuthMethodsReq{
    UserId: userID,
})
if resp.PasskeyCount == 0 && !resp.HasPassword {
    log.Println("User has no authentication methods!")
}
```

---

## Error Handling

### gRPC Status Codes

| Code | Description | Common Causes |
|------|-------------|---------------|
| `OK` | Success | Operation completed successfully |
| `INVALID_ARGUMENT` | Invalid input | Missing required fields, validation errors |
| `NOT_FOUND` | Resource not found | User does not exist (returned as `not_found = true` in response) |
| `ALREADY_EXISTS` | Resource already exists | User already exists (returned as `already_exists = true` in response) |
| `INTERNAL` | Internal error | Storage failure, unexpected error |

### Error Response Pattern

Most operations return boolean flags instead of gRPC status codes:

- `not_found`: Resource (user, passkey) does not exist
- `already_exists`: Resource already exists
- `invalid_code`: TOTP/backup code validation failed

**Example Error Handling** (Go):
```go
resp, err := client.GetUser(ctx, &api.GetUserReq{UserId: userID})
if err != nil {
    log.Fatalf("gRPC error: %v", err)
}
if resp.NotFound {
    log.Printf("User not found: %s", userID)
    return
}
// Success - use resp.User
```

---

## Usage Examples

### Complete User Registration Flow (Go)

```go
package main

import (
    "context"
    "log"

    "github.com/dexidp/dex/api/v2"
    "google.golang.org/grpc"
)

func main() {
    // Connect to Dex gRPC server
    conn, err := grpc.Dial("localhost:5557", grpc.WithInsecure())
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    client := api.NewEnhancedLocalConnectorClient(conn)
    ctx := context.Background()

    // 1. Create user
    createResp, err := client.CreateUser(ctx, &api.CreateUserReq{
        Email:       "alice@example.com",
        Username:    "alice",
        DisplayName: "Alice Smith",
    })
    if err != nil {
        log.Fatalf("Failed to create user: %v", err)
    }
    userID := createResp.User.Id
    log.Printf("Created user: %s", userID)

    // 2. Set password
    _, err = client.SetPassword(ctx, &api.SetPasswordReq{
        UserId:   userID,
        Password: "SecurePass123",
    })
    if err != nil {
        log.Fatalf("Failed to set password: %v", err)
    }
    log.Println("Password set")

    // 3. Enable TOTP
    totpResp, err := client.EnableTOTP(ctx, &api.EnableTOTPReq{
        UserId: userID,
    })
    if err != nil {
        log.Fatalf("Failed to enable TOTP: %v", err)
    }
    log.Printf("TOTP secret: %s", totpResp.Secret)
    log.Printf("QR code: %s", totpResp.QrCode)
    log.Printf("Backup codes: %v", totpResp.BackupCodes)

    // 4. Verify TOTP (user scans QR code and enters code from authenticator)
    _, err = client.VerifyTOTPSetup(ctx, &api.VerifyTOTPSetupReq{
        UserId:      userID,
        Secret:      totpResp.Secret,
        Code:        "123456", // From authenticator app
        BackupCodes: totpResp.BackupCodes,
    })
    if err != nil {
        log.Fatalf("Failed to verify TOTP: %v", err)
    }
    log.Println("TOTP enabled")

    // 5. Mark email as verified
    _, err = client.UpdateUser(ctx, &api.UpdateUserReq{
        UserId:        userID,
        EmailVerified: true,
    })
    if err != nil {
        log.Fatalf("Failed to update user: %v", err)
    }
    log.Println("Email verified")

    // 6. Check configured auth methods
    authResp, err := client.GetAuthMethods(ctx, &api.GetAuthMethodsReq{
        UserId: userID,
    })
    if err != nil {
        log.Fatalf("Failed to get auth methods: %v", err)
    }
    log.Printf("Auth methods - Password: %v, Passkeys: %d, TOTP: %v",
        authResp.HasPassword, authResp.PasskeyCount, authResp.TotpEnabled)
}
```

### Node.js/TypeScript Example

```typescript
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';

// Load protobuf
const packageDefinition = protoLoader.loadSync('api/v2/api.proto');
const proto = grpc.loadPackageDefinition(packageDefinition);

// Connect to Dex
const client = new proto.api.EnhancedLocalConnector(
  'localhost:5557',
  grpc.credentials.createInsecure()
);

// Create user
client.CreateUser({
  email: 'alice@example.com',
  username: 'alice',
  display_name: 'Alice Smith'
}, (err, response) => {
  if (err) {
    console.error('Error:', err);
    return;
  }
  console.log('Created user:', response.user.id);
});
```

---

## Next Steps

1. **Implement API Authentication**: Add API key or mTLS authentication
2. **Add Audit Logging**: Log all API operations for security auditing
3. **Add Metrics**: Track API usage, latency, error rates
4. **Create Client Libraries**: Official clients for Node.js, Python, Ruby
5. **Add Webhooks**: Notify Platform of user events (login, credential changes)

---

**Last Updated**: 2025-11-18
**Version**: 1.0
**Maintainer**: Enopax Platform Team
