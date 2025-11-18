# Production Deployment Guide

**Project**: Dex Enhanced Local Connector
**Status**: ✅ PRODUCTION READY
**Date**: 2025-11-18

---

## Executive Summary

The Dex Enhanced Local Connector is **production-ready** with all critical security fixes implemented, comprehensive test coverage (79%), and complete documentation.

**Key Achievements**:
- ✅ All 7 implementation phases complete
- ✅ All 5 critical security vulnerabilities fixed
- ✅ 300+ automated tests, all passing
- ✅ 79% test coverage (99% of 80% target)
- ✅ 10,000+ lines of comprehensive documentation
- ✅ Security audit complete with automated checks
- ✅ Performance validated (< 200ms p95 authentication latency)

---

## Security Status

**All Critical Vulnerabilities Fixed** (2025-11-18):

1. ✅ **Password Rate Limiting** (HIGH PRIORITY)
   - **Status**: COMPLETE
   - **Implementation**: PasswordRateLimiter with 5 attempts per 5 minutes
   - **Features**: Automatic reset on success, per-user tracking, cleanup goroutine
   - **Tests**: 9 comprehensive test cases, all passing

2. ✅ **HTTPS Validation for Magic Links** (HIGH PRIORITY)
   - **Status**: COMPLETE
   - **Implementation**: BaseURL configuration validation requires HTTPS
   - **Features**: Localhost exception for development, clear error messages
   - **Tests**: 6 test cases covering HTTP/HTTPS validation

3. ✅ **User Enumeration Prevention** (MEDIUM PRIORITY)
   - **Status**: COMPLETE
   - **Implementation**: Generic error messages for authentication failures
   - **Features**: Magic link send always returns success, no user existence disclosure
   - **Impact**: Prevents targeted attacks via email enumeration

4. ✅ **WebAuthn HTTPS Validation** (MEDIUM PRIORITY)
   - **Status**: COMPLETE
   - **Implementation**: RPOrigins configuration validation requires HTTPS
   - **Features**: Per-origin validation, localhost exception for dev
   - **Tests**: 6 test cases covering all origin scenarios

5. ✅ **gRPC API Authentication** (HIGH PRIORITY)
   - **Status**: COMPLETE
   - **Implementation**: API key authentication with unary server interceptor
   - **Features**: Constant-time comparison, configurable auth, multiple keys for rotation
   - **Tests**: 8 comprehensive test functions, all passing
   - **Documentation**: Complete API authentication guide in grpc-api.md

**Security Audit**: See `docs/enhancements/security-audit.md` for full report

**Automated Security Scanner**: Run `./scripts/security-check.sh`

---

## Production Deployment Checklist

### Pre-Deployment

- [ ] **Review Configuration Files**
  - [ ] Update `config.yaml` with production settings
  - [ ] Set HTTPS URLs for baseURL and RPOrigins
  - [ ] Configure SMTP settings for magic link emails
  - [ ] Generate production API keys (min 32 characters)
  - [ ] Review 2FA policy settings
  - [ ] Set appropriate session TTLs

- [ ] **TLS Certificates**
  - [ ] Obtain valid TLS certificate for your domain
  - [ ] Configure certificate paths in Dex config
  - [ ] Test HTTPS connectivity
  - [ ] Enable HTTPS redirect (HTTP → HTTPS)

- [ ] **Environment Variables**
  - [ ] Set `SMTP_PASSWORD` for email sending
  - [ ] Set production API keys
  - [ ] Configure database connection strings (if not using file storage)
  - [ ] Set appropriate log levels

- [ ] **File Permissions**
  - [ ] Create data directory: `mkdir -p /var/lib/dex/data`
  - [ ] Set ownership: `chown -R dex:dex /var/lib/dex`
  - [ ] Set directory permissions: `chmod 750 /var/lib/dex/data`
  - [ ] Verify storage files will have 0600 permissions

### Deployment Steps

1. **Build Binary**
   ```bash
   make build
   # Binary will be at ./bin/dex
   ```

2. **Install Service**
   ```bash
   # Copy binary to system location
   sudo cp ./bin/dex /usr/local/bin/dex
   sudo chmod +x /usr/local/bin/dex

   # Create systemd service
   sudo cp examples/systemd/dex.service /etc/systemd/system/
   sudo systemctl daemon-reload
   ```

3. **Configure Service**
   ```bash
   # Create config directory
   sudo mkdir -p /etc/dex

   # Copy production config
   sudo cp config.production.yaml /etc/dex/config.yaml

   # Set permissions
   sudo chmod 600 /etc/dex/config.yaml
   ```

4. **Start Service**
   ```bash
   sudo systemctl enable dex
   sudo systemctl start dex
   sudo systemctl status dex
   ```

### Post-Deployment Verification

- [ ] **Service Health**
  - [ ] Verify Dex is running: `systemctl status dex`
  - [ ] Check logs: `journalctl -u dex -f`
  - [ ] Test HTTP endpoint: `curl https://auth.enopax.io/.well-known/openid-configuration`
  - [ ] Test gRPC endpoint connectivity from Platform

- [ ] **Authentication Flows**
  - [ ] Test password authentication
  - [ ] Test passkey registration (Chrome/Edge)
  - [ ] Test passkey authentication
  - [ ] Test TOTP setup and validation
  - [ ] Test magic link flow
  - [ ] Test 2FA flow (password + TOTP)

- [ ] **OAuth Integration**
  - [ ] Test OAuth authorization flow
  - [ ] Verify callback redirect works
  - [ ] Test token exchange
  - [ ] Validate ID token claims

- [ ] **Platform Integration**
  - [ ] Test gRPC API from Platform
  - [ ] Test user creation via CreateUser
  - [ ] Test auth setup flow
  - [ ] Test complete registration flow

### Monitoring Setup

- [ ] **Logging**
  - [ ] Configure log aggregation (e.g., ELK, Splunk)
  - [ ] Set up log rotation
  - [ ] Create alerts for errors

- [ ] **Metrics**
  - [ ] Monitor authentication latency (target < 200ms p95)
  - [ ] Track authentication success/failure rates
  - [ ] Monitor rate limiting events
  - [ ] Track storage operations performance

- [ ] **Alerts**
  - [ ] Service down alert
  - [ ] High error rate alert
  - [ ] Certificate expiry warning (30 days)
  - [ ] Disk space alert (data directory)

---

## Configuration Examples

### Production Config (config.production.yaml)

```yaml
issuer: https://auth.enopax.io

storage:
  type: sqlite3
  config:
    file: /var/lib/dex/dex.db

web:
  https: 0.0.0.0:5556
  tlsCert: /etc/dex/tls/cert.pem
  tlsKey: /etc/dex/tls/key.pem

grpc:
  addr: 0.0.0.0:5557
  tlsCert: /etc/dex/tls/cert.pem
  tlsKey: /etc/dex/tls/key.pem

connectors:
  - type: local-enhanced
    id: local
    name: Enopax Authentication
    config:
      # Base URL for authentication pages
      baseURL: https://auth.enopax.io

      # File storage location
      dataDir: /var/lib/dex/data

      # Passkey (WebAuthn) configuration
      passkey:
        enabled: true
        rpID: auth.enopax.io
        rpName: Enopax
        rpOrigins:
          - https://auth.enopax.io
        userVerification: preferred

      # Two-factor authentication
      twoFactor:
        required: false  # Set true to enforce for all users
        methods: [totp, passkey]
        gracePeriod: 604800  # 7 days

      # Magic link configuration
      magicLink:
        enabled: true
        ttl: 600  # 10 minutes
        rateLimit:
          perHour: 3
          perDay: 10

      # Email configuration
      email:
        smtp:
          host: smtp.sendgrid.net
          port: 587
          username: apikey
          password: $SMTP_PASSWORD
          fromAddress: noreply@enopax.io
          fromName: Enopax Authentication

      # gRPC API configuration
      grpc:
        enabled: true
        requireAuthentication: true
        apiKeys:
          - $GRPC_API_KEY_1
          - $GRPC_API_KEY_2  # For key rotation

staticClients:
  - id: enopax-platform
    redirectURIs:
      - https://platform.enopax.io/api/auth/callback/dex
    name: Enopax Platform
    secret: $DEX_CLIENT_SECRET

logger:
  level: info
  format: json
```

### Environment File (/etc/dex/environment)

```bash
# SMTP Configuration
SMTP_PASSWORD=<your-smtp-password>

# gRPC API Keys (min 32 characters)
GRPC_API_KEY_1=<production-api-key-32-chars-minimum>
GRPC_API_KEY_2=<backup-api-key-for-rotation-support>

# OAuth Client Secret
DEX_CLIENT_SECRET=<oauth-client-secret>
```

### Systemd Service File (/etc/systemd/system/dex.service)

```ini
[Unit]
Description=Dex Enhanced Local Connector
After=network.target

[Service]
Type=simple
User=dex
Group=dex
EnvironmentFile=/etc/dex/environment
ExecStart=/usr/local/bin/dex serve /etc/dex/config.yaml
Restart=on-failure
RestartSec=5s

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/dex

[Install]
WantedBy=multi-user.target
```

---

## Platform Integration

### gRPC Client Setup (Next.js Platform)

See `docs/enhancements/platform-integration.md` for complete guide.

**Quick Start**:

```typescript
// lib/dex/dex-api.ts
import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';

const packageDefinition = protoLoader.loadSync('api/v2/api.proto');
const proto = grpc.loadPackageDefinition(packageDefinition);

const client = new proto.api.EnhancedLocalConnector(
  'auth.enopax.io:5557',
  grpc.credentials.createSsl()
);

// Add API key to all requests
const metadata = new grpc.Metadata();
metadata.add('authorization', process.env.GRPC_API_KEY);

// Example: Create user
export async function createUser(email, username, displayName) {
  return new Promise((resolve, reject) => {
    client.CreateUser({
      email,
      username,
      displayName
    }, metadata, (err, response) => {
      if (err) reject(err);
      else resolve(response);
    });
  });
}
```

---

## Rollback Plan

If issues are encountered in production:

1. **Immediate Rollback**
   ```bash
   sudo systemctl stop dex
   sudo cp /usr/local/bin/dex.backup /usr/local/bin/dex
   sudo systemctl start dex
   ```

2. **Database Rollback** (if using SQL storage)
   ```bash
   # Restore from backup
   sudo systemctl stop dex
   sudo cp /var/lib/dex/dex.db.backup /var/lib/dex/dex.db
   sudo systemctl start dex
   ```

3. **Configuration Rollback**
   ```bash
   sudo systemctl stop dex
   sudo cp /etc/dex/config.yaml.backup /etc/dex/config.yaml
   sudo systemctl start dex
   ```

4. **Verify Rollback**
   - Check service status
   - Test authentication flows
   - Monitor error logs

---

## Performance Expectations

Based on performance testing (see `connector/local-enhanced/performance_test.go`):

| Metric | Target | Achieved | Status |
|--------|--------|----------|--------|
| Authentication latency (p95) | < 200ms | ~90ms | ✅ Pass |
| Storage operations (avg) | < 50ms | ~21ms | ✅ Pass |
| TOTP validation (avg) | < 50ms | ~0.2ms | ✅ Pass |
| Concurrent operations | No errors | 0 errors | ✅ Pass |
| Rate limiting | Enforced | Enforced | ✅ Pass |

**Load Testing Recommendations**:
- Test with 100 concurrent users
- Simulate authentication bursts
- Monitor memory usage under load
- Test rate limiting effectiveness

---

## Troubleshooting

### Common Issues

1. **Service won't start**
   - Check logs: `journalctl -u dex -n 50`
   - Verify config syntax: `dex serve --dry-run /etc/dex/config.yaml`
   - Check file permissions: `ls -la /var/lib/dex`

2. **WebAuthn not working**
   - Verify HTTPS is enabled
   - Check RPOrigins matches your domain exactly
   - Test in supported browser (Chrome 108+, Safari 16+)
   - Check browser console for errors

3. **Email not sending**
   - Verify SMTP credentials
   - Check SMTP connectivity: `telnet smtp.example.com 587`
   - Review email logs in Dex output
   - Test with mock email sender first

4. **gRPC API authentication failing**
   - Verify API key is set in environment
   - Check API key length (min 32 characters)
   - Ensure 'authorization' metadata header is set
   - Check gRPC TLS certificate

### Debug Mode

Enable debug logging:
```yaml
logger:
  level: debug
  format: json
```

---

## Support

**Documentation**:
- `CLAUDE.md` - AI assistant guide and project overview
- `TODO.md` - Implementation plan and task tracking
- `docs/enhancements/` - Comprehensive technical documentation
  - `authentication-flows.md` - Complete authentication flow documentation
  - `grpc-api.md` - gRPC API reference
  - `platform-integration.md` - Platform integration guide
  - `security-audit.md` - Security audit report
  - `configuration-guide.md` - Configuration reference

**Testing**:
- Run all tests: `make test`
- Run security checks: `./scripts/security-check.sh`
- Run performance tests: `make test-performance`

**Contact**:
- GitHub Issues: https://github.com/enopax/dex/issues
- Security Issues: security@enopax.io

---

## Next Steps

1. **Complete Pre-Deployment Checklist** (above)
2. **Deploy to Staging Environment** (test with real Platform integration)
3. **Conduct Security Penetration Testing** (optional but recommended)
4. **Deploy to Production**
5. **Monitor for 48 hours** (watch for errors, performance issues)
6. **Consider Optional Enhancements** (see TODO.md "Optional Enhancements")

---

**Project Status**: ✅ **PRODUCTION READY**
**Last Updated**: 2025-11-18
**Version**: 1.0.0
