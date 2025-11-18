# Migration Guide: Local Connector to Enhanced Local Connector

**Document Version**: 1.0
**Last Updated**: 2025-11-18
**Target Audience**: Dex Administrators, DevOps Engineers

---

## Table of Contents

1. [Overview](#overview)
2. [Pre-Migration Checklist](#pre-migration-checklist)
3. [Understanding the Differences](#understanding-the-differences)
4. [Migration Strategy](#migration-strategy)
5. [Step-by-Step Migration](#step-by-step-migration)
6. [Data Migration](#data-migration)
7. [Configuration Migration](#configuration-migration)
8. [Testing the Migration](#testing-the-migration)
9. [Rollback Plan](#rollback-plan)
10. [Troubleshooting](#troubleshooting)
11. [Post-Migration Tasks](#post-migration-tasks)

---

## Overview

This guide helps you migrate from Dex's standard **local connector** (password-only authentication) to the **Enhanced Local Connector** with support for multiple authentication methods including passkeys, TOTP 2FA, and magic links.

### Why Migrate?

The Enhanced Local Connector provides:

- **Stronger Security**: Hardware-backed passkeys (WebAuthn) for phishing-resistant authentication
- **Better User Experience**: Passwordless login with Touch ID, Windows Hello, security keys
- **True 2FA**: Multiple authentication methods per user (password + passkey, password + TOTP)
- **Flexible Authentication**: Support for magic links, TOTP, backup codes
- **Platform Integration**: gRPC API for programmatic user management
- **Modern Standards**: Compliance with FIDO2/WebAuthn specifications

### Migration Timeline

| Phase | Duration | Description |
|-------|----------|-------------|
| Planning & Backup | 1-2 hours | Review guide, backup data, test environment setup |
| Configuration | 1 hour | Update Dex configuration |
| Data Migration | 2-4 hours | Convert existing users (automated) |
| Testing | 2-4 hours | Verify authentication flows |
| Deployment | 1 hour | Deploy to production |
| **Total** | **7-12 hours** | Complete migration |

---

## Pre-Migration Checklist

Before starting the migration, ensure you have:

### ✅ Required Items

- [ ] **Full backup** of current Dex data directory
- [ ] **Access** to Dex configuration files
- [ ] **Test environment** that mirrors production
- [ ] **Downtime window** scheduled (recommended: 1-2 hours)
- [ ] **Rollback plan** documented and tested
- [ ] **Admin access** to Dex server
- [ ] **Communication plan** for users about downtime

### ✅ Technical Requirements

- [ ] Dex v2.37.0+ (Enhanced Local Connector requires recent Dex version)
- [ ] Go 1.21+ (for building Dex with Enhanced Local Connector)
- [ ] HTTPS certificate (WebAuthn requires HTTPS in production)
- [ ] SMTP server configured (for magic link emails, optional)
- [ ] gRPC port available (for Platform integration, default 5557)

### ✅ Knowledge Requirements

- [ ] Familiarity with Dex connector configuration
- [ ] Understanding of YAML configuration syntax
- [ ] Basic knowledge of OAuth/OIDC flows
- [ ] Command-line proficiency

---

## Understanding the Differences

### Old Local Connector (Standard)

**Storage Structure**:
```
storage:
  type: sqlite3
  config:
    file: /var/dex/dex.db
```

**Data Format** (SQLite):
```sql
CREATE TABLE password (
  email TEXT PRIMARY KEY,
  hash BLOB,
  username TEXT,
  user_id TEXT
);
```

**Configuration**:
```yaml
connectors:
  - type: local
    id: local
    name: Email
    config:
      # No additional config needed
```

**Features**:
- Password-only authentication
- Email as unique identifier
- Username optional
- No 2FA support
- No additional auth methods

---

### Enhanced Local Connector (New)

**Storage Structure**:
```
storage:
  type: file
  config:
    dataDir: /var/lib/dex/data
```

**Data Format** (JSON files):
```
data/
├── users/{user-id}.json           # User records
├── passkeys/{credential-id}.json  # WebAuthn credentials
├── webauthn-sessions/{session-id}.json
├── magic-link-tokens/{token}.json
├── 2fa-sessions/{session-id}.json
└── auth-setup-tokens/{token}.json
```

**User Record** (JSON):
```json
{
  "id": "deterministic-uuid",
  "email": "user@example.com",
  "username": "alice",
  "display_name": "Alice Smith",
  "email_verified": true,
  "password_hash": "$2a$10$...",
  "passkeys": [
    {
      "id": "passkey-id",
      "name": "MacBook Touch ID",
      "public_key": "...",
      "created_at": "2025-11-18T10:00:00Z"
    }
  ],
  "totp_secret": "BASE32SECRET",
  "totp_enabled": true,
  "backup_codes": [
    {"code": "HASHED", "used": false}
  ],
  "magic_link_enabled": true,
  "require_2fa": false,
  "created_at": "2025-11-18T10:00:00Z",
  "updated_at": "2025-11-18T10:00:00Z",
  "last_login_at": "2025-11-18T12:00:00Z"
}
```

**Configuration**:
```yaml
connectors:
  - type: local-enhanced
    id: local
    name: Enopax Authentication
    config:
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

**Features**:
- Multiple auth methods per user (password, passkey, TOTP, magic link)
- True 2FA (password + passkey/TOTP)
- Passwordless authentication (passkey-only, magic link-only)
- Deterministic user IDs (SHA-256 of email)
- gRPC API for Platform integration
- Flexible authentication policies

---

## Migration Strategy

### Recommended Approach: **Parallel Migration**

Run both connectors side-by-side during migration:

1. **Deploy** Enhanced Local Connector alongside existing connector
2. **Migrate** users in batches
3. **Test** authentication with both connectors
4. **Switch** OAuth clients to new connector
5. **Decommission** old connector after verification

### Alternative Approach: **Direct Migration**

Replace existing connector in one operation:

1. **Backup** all data
2. **Export** users from old connector
3. **Stop** Dex server
4. **Import** users to Enhanced Local Connector
5. **Update** configuration
6. **Start** Dex with new connector
7. **Verify** authentication

**Recommended**: Parallel migration for production environments with active users.

---

## Step-by-Step Migration

### Phase 1: Preparation (30-60 minutes)

#### 1.1. Backup Current Data

```bash
# Stop Dex (if using database storage)
sudo systemctl stop dex

# Backup entire Dex data directory
sudo tar -czf /backup/dex-backup-$(date +%Y%m%d).tar.gz /var/lib/dex/

# Backup configuration
sudo cp /etc/dex/config.yaml /backup/dex-config-$(date +%Y%m%d).yaml

# Restart Dex
sudo systemctl start dex
```

#### 1.2. Set Up Test Environment

```bash
# Clone production Dex configuration
cp /etc/dex/config.yaml /tmp/dex-test-config.yaml

# Create test data directory
mkdir -p /tmp/dex-test-data

# Copy production data to test directory
cp -r /var/lib/dex/* /tmp/dex-test-data/
```

#### 1.3. Build Dex with Enhanced Local Connector

```bash
# Clone Enopax Dex fork
git clone https://github.com/enopax/dex.git
cd dex

# Checkout feature branch
git checkout feature/passkeys

# Build Dex
make build

# Verify binary
./bin/dex version
```

---

### Phase 2: Configuration Migration (30-60 minutes)

#### 2.1. Update Connector Configuration

**Before** (Old Local Connector):
```yaml
connectors:
  - type: local
    id: local
    name: Email
```

**After** (Enhanced Local Connector):
```yaml
connectors:
  - type: local-enhanced
    id: local-enhanced
    name: Enopax Authentication
    config:
      dataDir: /var/lib/dex/data
      baseURL: https://auth.enopax.io

      # Passkey (WebAuthn) settings
      passkey:
        enabled: true
        rpID: auth.enopax.io
        rpName: Enopax
        rpOrigins:
          - https://auth.enopax.io
        userVerification: preferred

      # Two-factor authentication settings
      twoFactor:
        required: false  # Don't force 2FA immediately
        methods:
          - totp
          - passkey
        gracePeriod: 604800  # 7 days for users to set up 2FA

      # Magic link settings
      magicLink:
        enabled: true
        ttl: 600  # 10 minutes
        rateLimit:
          perHour: 3
          perDay: 10

      # Email settings (required for magic links)
      email:
        smtp:
          host: smtp.example.com
          port: 587
          username: noreply@enopax.io
          password: ${SMTP_PASSWORD}
          from: noreply@enopax.io
```

#### 2.2. Update Storage Configuration

**Before** (SQLite):
```yaml
storage:
  type: sqlite3
  config:
    file: /var/dex/dex.db
```

**After** (File-based, if using old local connector storage):
```yaml
storage:
  type: file
  config:
    dataDir: /var/lib/dex/storage
```

**Note**: Enhanced Local Connector uses its own file storage in `config.dataDir`, separate from Dex's main storage.

#### 2.3. Add gRPC Configuration

```yaml
grpc:
  addr: 0.0.0.0:5557
  tlsCert: /etc/dex/grpc-server-cert.pem
  tlsKey: /etc/dex/grpc-server-key.pem
  tlsClientCA: /etc/dex/grpc-client-ca.pem
```

#### 2.4. Update OAuth Client Configuration

**Update redirect URIs** to include new connector:

```yaml
staticClients:
  - id: example-app
    redirectURIs:
      - https://app.example.com/callback
    name: Example App
    secret: ${CLIENT_SECRET}
```

---

### Phase 3: Data Migration (1-3 hours)

#### 3.1. Export Users from Old Connector

**Option A: SQLite Export** (if using sqlite3 storage)

```bash
# Export users from SQLite database
sqlite3 /var/dex/dex.db <<EOF
.mode json
.output /tmp/dex-users-export.json
SELECT email, username, hash, user_id FROM password;
.quit
EOF
```

**Option B: Manual Export** (if using other storage)

Use Dex's gRPC API to list all users:

```bash
# Install grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# List users (requires dex API to be enabled)
grpcurl -plaintext localhost:5557 dex.api.Dex/ListPasswords
```

#### 3.2. Create Migration Script

Save as `migrate-users.sh`:

```bash
#!/bin/bash
set -euo pipefail

# Configuration
EXPORT_FILE="/tmp/dex-users-export.json"
DATA_DIR="/var/lib/dex/data"
USERS_DIR="$DATA_DIR/users"

# Create directories
mkdir -p "$USERS_DIR"

# Read exported users and convert to new format
jq -c '.[]' "$EXPORT_FILE" | while read -r user; do
    email=$(echo "$user" | jq -r '.email')
    username=$(echo "$user" | jq -r '.username // .email')
    hash=$(echo "$user" | jq -r '.hash')

    # Generate deterministic user ID (SHA-256 of email)
    user_id=$(echo -n "$email" | sha256sum | awk '{print $1}' | \
              sed 's/\(........\)\(....\)\(....\)\(....\)\(............\)/\1-\2-\3-\4-\5/')

    # Create user JSON file
    cat > "$USERS_DIR/$user_id.json" <<EOF
{
  "id": "$user_id",
  "email": "$email",
  "username": "$username",
  "display_name": "$username",
  "email_verified": true,
  "password_hash": "$hash",
  "passkeys": [],
  "totp_secret": null,
  "totp_enabled": false,
  "backup_codes": [],
  "magic_link_enabled": true,
  "require_2fa": false,
  "created_at": "$(date -Iseconds)",
  "updated_at": "$(date -Iseconds)",
  "last_login_at": null
}
EOF

    echo "Migrated user: $email → $user_id"
done

# Set correct permissions
chmod 700 "$DATA_DIR"
chmod 700 "$USERS_DIR"
chmod 600 "$USERS_DIR"/*.json

echo "Migration complete! Migrated $(ls -1 "$USERS_DIR" | wc -l) users."
```

#### 3.3. Run Migration Script

```bash
# Make script executable
chmod +x migrate-users.sh

# Run migration (dry run first)
DRY_RUN=1 ./migrate-users.sh

# Run actual migration
./migrate-users.sh
```

#### 3.4. Verify Migrated Data

```bash
# Check number of users migrated
ls -1 /var/lib/dex/data/users/ | wc -l

# Inspect a sample user
cat /var/lib/dex/data/users/*.json | jq '.' | head -30

# Verify file permissions
ls -la /var/lib/dex/data/users/
```

---

### Phase 4: Testing (1-2 hours)

#### 4.1. Start Dex with New Configuration

```bash
# Test configuration syntax
./bin/dex serve /tmp/dex-test-config.yaml --validate

# Start Dex in test mode
./bin/dex serve /tmp/dex-test-config.yaml --log-level=debug
```

#### 4.2. Test Password Authentication

```bash
# Navigate to Dex login
open https://auth.enopax.io/auth/local-enhanced?...

# Try logging in with existing password
# Verify successful authentication
```

#### 4.3. Test User Registration

```bash
# Use gRPC API to create test user
grpcurl -plaintext \
  -d '{
    "email": "test@example.com",
    "username": "testuser",
    "display_name": "Test User"
  }' \
  localhost:5557 \
  api.EnhancedLocalConnector/CreateUser

# Set password for test user
grpcurl -plaintext \
  -d '{
    "user_id": "USER_ID_FROM_ABOVE",
    "password": "SecurePass123"
  }' \
  localhost:5557 \
  api.EnhancedLocalConnector/SetPassword
```

#### 4.4. Test Passkey Registration

1. Navigate to auth setup page: `https://auth.enopax.io/setup-auth?token=...`
2. Click "Set up Passkey"
3. Complete WebAuthn ceremony (Touch ID, Windows Hello, etc.)
4. Verify passkey stored in user record

```bash
# Check user's passkeys
cat /var/lib/dex/data/users/USER_ID.json | jq '.passkeys'
```

#### 4.5. Test OAuth Flow

```bash
# Initiate OAuth flow from test client
open https://auth.enopax.io/auth?client_id=example-app&redirect_uri=https://app.example.com/callback&response_type=code&scope=openid+email+profile

# Authenticate with password
# Verify redirect to callback with authorization code
# Exchange code for tokens
```

---

### Phase 5: Production Deployment (30-60 minutes)

#### 5.1. Schedule Maintenance Window

**Announcement Template**:
```
Subject: Scheduled Maintenance - Authentication System Upgrade

Dear Users,

We will be performing a scheduled upgrade to our authentication system on:

Date: [DATE]
Time: [TIME] - [TIME] (estimated 1 hour)

During this time:
- You will not be able to log in
- Active sessions will remain valid
- No data will be lost

After the upgrade, you will have access to new authentication methods including:
- Touch ID / Windows Hello (passkey support)
- Two-factor authentication (TOTP)
- Email magic links

Thank you for your patience.
```

#### 5.2. Deploy to Production

```bash
# Stop Dex
sudo systemctl stop dex

# Backup production data (final backup)
sudo tar -czf /backup/dex-pre-migration-$(date +%Y%m%d-%H%M).tar.gz /var/lib/dex/

# Copy new Dex binary
sudo cp ./bin/dex /usr/local/bin/dex

# Update configuration
sudo cp /tmp/dex-production-config.yaml /etc/dex/config.yaml

# Run migration script on production data
sudo ./migrate-users.sh

# Start Dex with new configuration
sudo systemctl start dex

# Check status
sudo systemctl status dex

# Monitor logs
sudo journalctl -u dex -f
```

#### 5.3. Verify Production Deployment

```bash
# Test authentication endpoint
curl -k https://auth.enopax.io/healthz

# Test connector info endpoint
curl -k https://auth.enopax.io/auth/local-enhanced

# Test login flow (manual)
```

#### 5.4. Update Client Applications

If using Platform integration:

```typescript
// Update OAuth configuration
const dexConfig = {
  issuer: 'https://auth.enopax.io',
  clientId: 'example-app',
  clientSecret: process.env.DEX_CLIENT_SECRET,
  redirectUri: 'https://app.example.com/callback',
  scope: 'openid email profile',
};

// Update connector ID in login link
const loginUrl = `https://auth.enopax.io/auth/local-enhanced?...`;
```

---

## Rollback Plan

If migration fails, follow these steps to rollback:

### Rollback Procedure

```bash
# 1. Stop Dex
sudo systemctl stop dex

# 2. Restore old binary
sudo cp /backup/dex-old-binary /usr/local/bin/dex

# 3. Restore old configuration
sudo cp /backup/dex-config-YYYYMMDD.yaml /etc/dex/config.yaml

# 4. Restore old data
sudo rm -rf /var/lib/dex/*
sudo tar -xzf /backup/dex-backup-YYYYMMDD.tar.gz -C /

# 5. Start Dex
sudo systemctl start dex

# 6. Verify rollback
sudo systemctl status dex
curl -k https://auth.enopax.io/healthz
```

### Rollback Verification Checklist

- [ ] Dex service running
- [ ] Old connector accessible
- [ ] Users can log in with password
- [ ] OAuth flow working
- [ ] No errors in logs

---

## Troubleshooting

### Issue: Users Can't Log In After Migration

**Symptoms**:
- "Invalid email or password" error
- User exists in new storage but authentication fails

**Diagnosis**:
```bash
# Check if user exists
ls /var/lib/dex/data/users/ | grep USER_ID

# Verify user data
cat /var/lib/dex/data/users/USER_ID.json | jq '.'

# Check password hash format
cat /var/lib/dex/data/users/USER_ID.json | jq '.password_hash'
```

**Resolution**:
- Verify password hash copied correctly from old storage
- Ensure bcrypt hash format is valid
- Check user ID generation (must be deterministic from email)

---

### Issue: Connector Not Found

**Symptoms**:
- "Connector with ID 'local-enhanced' not found"
- 404 error when accessing connector

**Diagnosis**:
```bash
# Check Dex logs
sudo journalctl -u dex -n 100 | grep connector

# Verify configuration syntax
./bin/dex serve /etc/dex/config.yaml --validate
```

**Resolution**:
- Verify `type: local-enhanced` in configuration
- Ensure connector ID matches OAuth client configuration
- Rebuild Dex with Enhanced Local Connector code

---

### Issue: WebAuthn Errors

**Symptoms**:
- "WebAuthn not supported" error
- Passkey registration fails

**Diagnosis**:
```bash
# Check HTTPS configuration
curl -I https://auth.enopax.io

# Verify RP ID matches domain
cat /etc/dex/config.yaml | grep rpID

# Check browser console for WebAuthn errors
```

**Resolution**:
- Ensure HTTPS is enabled (WebAuthn requires HTTPS)
- Verify `rpID` matches domain exactly (no protocol, no port)
- Check `rpOrigins` includes correct URL
- Test in different browser

---

### Issue: File Permission Errors

**Symptoms**:
- "Permission denied" errors in logs
- Can't read/write user files

**Diagnosis**:
```bash
# Check directory permissions
ls -la /var/lib/dex/data/

# Check Dex service user
ps aux | grep dex
```

**Resolution**:
```bash
# Fix permissions
sudo chown -R dex:dex /var/lib/dex/data/
sudo chmod 700 /var/lib/dex/data/
sudo chmod 700 /var/lib/dex/data/users/
sudo chmod 600 /var/lib/dex/data/users/*.json
```

---

### Issue: gRPC API Not Accessible

**Symptoms**:
- "Connection refused" when calling gRPC API
- Platform can't create users

**Diagnosis**:
```bash
# Check if gRPC port is listening
sudo netstat -tlnp | grep 5557

# Test gRPC endpoint
grpcurl -plaintext localhost:5557 list
```

**Resolution**:
- Ensure gRPC configuration in `config.yaml`
- Check firewall rules allow port 5557
- Verify TLS certificates if using mTLS

---

## Post-Migration Tasks

### Immediate Tasks (Within 24 hours)

- [ ] Monitor authentication logs for errors
- [ ] Verify all users can log in
- [ ] Test passkey registration with multiple devices
- [ ] Check email delivery for magic links
- [ ] Update documentation with new login instructions

### Short-term Tasks (Within 1 week)

- [ ] Send user communication about new features
- [ ] Enable optional 2FA for users
- [ ] Configure backup codes
- [ ] Set up monitoring alerts for authentication failures
- [ ] Document troubleshooting procedures for support team

### Long-term Tasks (Within 1 month)

- [ ] Analyze authentication metrics (passkey vs password usage)
- [ ] Gradually enable 2FA requirement for admin users
- [ ] Decommission old connector (after verification period)
- [ ] Archive old data backups
- [ ] Review and optimize authentication performance

---

## User Communication

### Email Template: Migration Complete

```
Subject: Authentication System Upgraded - New Features Available

Dear [User],

We've successfully upgraded our authentication system. You can now log in using:

🔐 **Passkeys** (Touch ID, Windows Hello, Security Keys)
   - Faster and more secure than passwords
   - Set up at: https://auth.enopax.io/settings

🔑 **Your existing password** still works
   - No action required if you prefer passwords

📧 **Magic links** via email
   - Click the link in your email to log in instantly

🛡️ **Two-factor authentication** (optional)
   - Add an extra layer of security
   - Enable in your account settings

Your existing password remains valid. You can continue using it or try the new methods.

Questions? Contact support@enopax.io

Best regards,
Enopax Team
```

---

## Appendix

### A. User ID Generation Algorithm

The Enhanced Local Connector uses **deterministic user IDs** based on email addresses:

```go
import (
    "crypto/sha256"
    "fmt"
)

func generateUserID(email string) string {
    hash := sha256.Sum256([]byte(email))
    return fmt.Sprintf("%x-%x-%x-%x-%x",
        hash[0:4], hash[4:6], hash[6:8], hash[8:10], hash[10:16])
}

// Example:
// email: "alice@example.com"
// user_id: "2bd806c9-7f04-4ca4-9ce9-8f8b9b7c5e3f"
```

This ensures:
- Same email always gets same user ID
- No database lookup required to find user
- User ID collision impossible (SHA-256 hash)

### B. Configuration Examples

See `docs/enhancements/configuration-guide.md` for comprehensive configuration examples:

- Development environment
- Staging environment
- Production environment
- Multiple email provider examples

### C. API Reference

For gRPC API details, see `docs/enhancements/grpc-api.md`.

For authentication flows, see `docs/enhancements/authentication-flows.md`.

### D. Migration Script (Complete)

Full version of `migrate-users.sh` available at:
`scripts/migrate-users.sh` (to be created)

---

## Support

If you encounter issues during migration:

1. **Check this guide** - Most common issues are documented
2. **Review logs** - Dex logs contain detailed error information
3. **Test in non-production** - Always test migration in staging first
4. **Contact support** - enopax@example.com
5. **GitHub Issues** - https://github.com/enopax/dex/issues

---

**Document Version**: 1.0
**Last Updated**: 2025-11-18
**Maintainer**: Enopax Platform Team
**License**: MIT
