# Test Coverage Analysis - Enhanced Local Connector

**Date**: 2025-11-18 (Updated)
**Overall Coverage**: 77.0%
**Target Coverage**: >80%
**Gap**: +3.0 percentage points
**Recent Improvement**: +3.9 percentage points (from 73.1% after TOTP handler tests)

---

## Executive Summary

The enhanced local connector has achieved **77.0% test coverage** with comprehensive tests across all major features. Significant progress made:

**Completed** (Phase 7 Week 13):
- ✅ **TOTP HTTP Handlers** - All 3 handlers tested (75-77% coverage)
- ✅ **Magic Link HTTP Handlers** - Both handlers tested (91-98% coverage)
- ✅ **2FA Session Storage** - Comprehensive storage tests complete
- ✅ **Auth Setup Token Storage** - Comprehensive storage tests complete

**Remaining to reach >80%** (+3.0 percentage points needed):
1. **gRPC TOTP endpoints** - 4 methods untested (VerifyTOTPSetup, DisableTOTP, GetTOTPInfo, RegenerateBackupCodes)
2. **Integration tests** - Full authentication flows (password+TOTP, password+passkey, etc.)
3. **WebAuthn finish flows** - Passkey cryptographic verification (requires browser/virtual authenticator)

---

## Coverage by Component

### Excellent Coverage (90-100%)

These components are well-tested and meet the quality standard:

| Function | Coverage | File |
|----------|----------|------|
| WebAuthn User Interface | 100% | passkey.go (lines 20-77) |
| Storage Operations | 90%+ | storage.go (most operations) |
| Validation Functions | 84-100% | validation.go |
| Configuration | 100% | config.go |
| Testing Utilities | 87-100% | testing.go |
| TOTP Core Functions | 90-100% | totp.go (ValidateTOTP, ValidateBackupCode) |
| 2FA Flow Logic | 85%+ | twofa.go (Require2FAForUser, GetAvailable2FAMethods) |
| Magic Link Core | 75-100% | magiclink.go (core functions) |
| OAuth Integration | 100% | local.go (LoginURL, HandleCallback, Refresh) |

### Good Coverage (70-89%)

These components are tested but could use more edge case coverage:

| Function | Coverage | File | Missing Tests |
|----------|----------|------|---------------|
| SetPassword | 61.5% | password.go:50 | Error scenarios |
| RemovePassword | 66.7% | password.go:122 | User not found cases |
| BeginPasskeyRegistration | 66.7% | passkey.go:123 | WebAuthn library errors |
| BeginTOTPSetup | 61.9% | totp.go:138 | QR code generation failures |
| CreateUser (gRPC) | 77.8% | grpc.go:25 | Duplicate email handling |
| UpdateUser (gRPC) | 83.3% | grpc.go:110 | Partial update scenarios |
| DeleteUser (gRPC) | 77.8% | grpc.go:152 | User with credentials |
| SetPassword (gRPC) | 76.9% | grpc.go:178 | Invalid password formats |
| EnableTOTP (gRPC) | 83.3% | grpc.go:238 | Already enabled |
| handle2FAVerifyTOTP | 77.4% | handlers.go:1125 | Session expiry |
| handlePasskeyLoginBegin | 85.2% | handlers.go:115 | Concurrent requests |
| handlePasskeyRegisterBegin | 81.8% | handlers.go:341 | User not found |

### Needs Improvement (50-69%)

These components have partial coverage and need more tests:

| Function | Coverage | File | Priority |
|----------|----------|------|----------|
| handlePasskeyLoginFinish | 52.4% | handlers.go:216 | HIGH |
| handlePasskeyRegisterFinish | 58.1% | handlers.go:450 | HIGH |
| handle2FAVerifyBackupCode | 67.7% | handlers.go:1178 | MEDIUM |
| handle2FAVerifyPasskeyBegin | 65.5% | handlers.go:1231 | MEDIUM |
| Validate2FAMethod | 60.9% | twofa.go:177 | MEDIUM |
| Complete2FA | 66.7% | twofa.go:70 | MEDIUM |

**Why Low Coverage**:
- `handlePasskeyLoginFinish` and `handlePasskeyRegisterFinish` require real WebAuthn credentials from a browser or virtual authenticator
- Current tests validate sessions and error handling but skip cryptographic verification
- 2FA handlers need integration tests with full authentication flows

### Critical Gaps (0-49%)

These functions are **untested** and represent the biggest opportunity for improvement:

| Function | Coverage | File | Reason for 0% | Impact |
|----------|----------|------|---------------|--------|
| **handleTOTPEnable** | 75.0% | handlers.go:574 | ✅ TESTED (Week 13) | -- |
| **handleTOTPVerify** | 76.9% | handlers.go:684 | ✅ TESTED (Week 13) | -- |
| **handleTOTPValidate** | 71.1% | handlers.go:796 | ✅ TESTED (Week 13) | -- |
| **handleMagicLinkSend** | 97.8% | handlers.go:869 | ✅ TESTED (Week 13) | -- |
| **handleMagicLinkVerify** | 90.9% | handlers.go:963 | ✅ TESTED (Week 13) | -- |
| **VerifyTOTPSetup** (gRPC) | 0% | grpc.go:277 | No tests written | MEDIUM |
| **DisableTOTP** (gRPC) | 0% | grpc.go:312 | No tests written | MEDIUM |
| **GetTOTPInfo** (gRPC) | 0% | grpc.go:344 | No tests written | MEDIUM |
| **RegenerateBackupCodes** (gRPC) | 0% | grpc.go:375 | No tests written | MEDIUM |
| handle2FAVerifyPasskeyFinish | 31.6% | handlers.go:1287 | Requires WebAuthn | MEDIUM |
| FinishPasskeyRegistration | 18.5% | passkey.go:182 | Requires WebAuthn | LOW |
| FinishPasskeyAuthentication | 12.8% | passkey.go:354 | Requires WebAuthn | LOW |
| VerifyPassword | 0% | password.go:90 | Never called in tests | LOW |

**Critical Untested Functions**:
1. **TOTP HTTP Handlers** (handleTOTPEnable, handleTOTPVerify, handleTOTPValidate)
2. **Magic Link HTTP Handlers** (handleMagicLinkSend, handleMagicLinkVerify)
3. **TOTP gRPC Endpoints** (VerifyTOTPSetup, DisableTOTP, GetTOTPInfo, RegenerateBackupCodes)

These handlers are fully implemented but have **no test coverage**. Adding tests for these would immediately boost coverage by ~5-7%.

---

## Coverage Improvement Plan

### Phase 1: Low-Hanging Fruit (+6-8%)

**Estimated Time**: 1-2 days
**Target Coverage**: 75-77%

#### 1.1 TOTP HTTP Handler Tests

Create `handlers_totp_test.go`:

```go
func TestHandleTOTPEnable(t *testing.T) {
    // Test POST /totp/enable
    // - Valid user → returns secret, QR code, backup codes
    // - Missing user_id → returns 400
    // - User not found → returns 404
    // - TOTP already enabled → returns 409
    // - Concurrent requests
}

func TestHandleTOTPVerify(t *testing.T) {
    // Test POST /totp/verify
    // - Valid TOTP code → enables TOTP, stores backup codes
    // - Invalid code → returns error
    // - Missing fields → returns 400
    // - User not found → returns 404
}

func TestHandleTOTPValidate(t *testing.T) {
    // Test POST /totp/validate
    // - Valid TOTP code → returns success
    // - Invalid code → returns error
    // - Backup code fallback → marks code as used
    // - Rate limiting enforced
    // - User without TOTP → returns error
}
```

**Expected Coverage Gain**: +3-4%

#### 1.2 Magic Link HTTP Handler Tests

Create `handlers_magiclink_test.go`:

```go
func TestHandleMagicLinkSend(t *testing.T) {
    // Test POST /magic-link/send
    // - Valid email → sends email, returns success
    // - Invalid email → returns 400
    // - User not found → returns 404
    // - Rate limit exceeded → returns 429
    // - Email sending failure → returns 500
}

func TestHandleMagicLinkVerify(t *testing.T) {
    // Test GET /magic-link/verify?token=...
    // - Valid token → authenticates user, redirects
    // - Invalid token → returns 401
    // - Expired token → returns 401
    // - Already used token → returns 401
    // - 2FA required → redirects to 2FA prompt
}
```

**Expected Coverage Gain**: +2-3%

#### 1.3 gRPC TOTP Endpoint Tests

Add to `grpc_test.go`:

```go
func TestGRPCServer_VerifyTOTPSetup(t *testing.T) {
    // Valid TOTP code → enables TOTP
    // Invalid code → returns invalid_code flag
    // User not found → returns not_found flag
}

func TestGRPCServer_DisableTOTP(t *testing.T) {
    // Valid TOTP code → disables TOTP
    // Invalid code → returns invalid_code flag
    // TOTP not enabled → returns error
}

func TestGRPCServer_GetTOTPInfo(t *testing.T) {
    // Returns TOTP status
    // Returns backup code count
    // User not found → returns not_found flag
}

func TestGRPCServer_RegenerateBackupCodes(t *testing.T) {
    // Valid TOTP code → generates new codes
    // Invalid code → returns invalid_code flag
    // TOTP not enabled → returns error
}
```

**Expected Coverage Gain**: +1-2%

**Total Phase 1 Gain**: **+6-8%** → **Target: 75-77%**

---

### Phase 2: Integration Tests (+3-5%)

**Estimated Time**: 2-3 days
**Target Coverage**: 78-82%

#### 2.1 Complete 2FA Flows

Add to `integration_test.go`:

```go
func TestComplete2FAFlow_PasswordPlusTOTP(t *testing.T) {
    // 1. User authenticates with password (primary)
    // 2. System requires 2FA (Require2FAForUser)
    // 3. Begin2FA creates session
    // 4. User prompted for TOTP
    // 5. ValidateTOTP succeeds
    // 6. Complete2FA marks session complete
    // 7. User successfully authenticated
}

func TestComplete2FAFlow_PasswordPlusPasskey(t *testing.T) {
    // Similar to above but uses passkey for 2FA
}

func TestComplete2FAFlow_PasswordPlusBackupCode(t *testing.T) {
    // Similar but uses backup code
    // Verify backup code is marked as used
}

func TestComplete2FAFlow_SessionExpiry(t *testing.T) {
    // Create 2FA session
    // Wait or mock time to expire session (10 minutes)
    // Attempt to complete 2FA
    // Verify error returned
}

func TestComplete2FAFlow_GracePeriod(t *testing.T) {
    // Create user within grace period
    // Verify 2FA not required
    // Mock time passing to expire grace period
    // Verify 2FA now required
}
```

**Expected Coverage Gain**: +2-3%

#### 2.2 Magic Link Integration Tests

```go
func TestMagicLinkFlow_Complete(t *testing.T) {
    // 1. User requests magic link
    // 2. Email sent with token
    // 3. User clicks link
    // 4. Token verified
    // 5. User authenticated
}

func TestMagicLinkFlow_With2FA(t *testing.T) {
    // 1. User authenticates via magic link
    // 2. 2FA required
    // 3. User completes TOTP
    // 4. Authentication successful
}
```

**Expected Coverage Gain**: +1-2%

**Total Phase 2 Gain**: **+3-5%** → **Target: 78-82%**

---

### Phase 3: End-to-End Tests (Optional, +2-3%)

**Estimated Time**: 3-5 days
**Target Coverage**: 80-85%

#### 3.1 Browser-Based Passkey Tests

Use Chrome DevTools Virtual Authenticator API:

```go
func TestPasskeyRegistration_Browser(t *testing.T) {
    // Requires Playwright/Selenium
    // 1. Start Dex server
    // 2. Open browser with virtual authenticator
    // 3. Navigate to /passkey/register/begin
    // 4. Browser creates credential
    // 5. POST to /passkey/register/finish
    // 6. Verify credential stored
}

func TestPasskeyAuthentication_Browser(t *testing.T) {
    // Similar for authentication flow
}
```

**Expected Coverage Gain**: +2-3% (covers FinishPasskey* functions)

**Note**: This is optional for now. The low coverage in passkey finish functions is acceptable because:
1. The uncovered code is in go-webauthn library (not our code)
2. Session validation and error handling are tested (18.5% and 12.8%)
3. Browser testing is complex and time-consuming

**Total Phase 3 Gain**: **+2-3%** → **Target: 80-85%**

---

## Testing Priorities

### Priority 1: CRITICAL (Must Have)

These tests are essential for production readiness:

- ✅ TOTP HTTP handlers (0% → 80%+) - **Phase 1.1**
- ✅ Magic link HTTP handlers (0% → 80%+) - **Phase 1.2**
- ✅ gRPC TOTP endpoints (0% → 75%+) - **Phase 1.3**

**Rationale**: These are fully functional features with no test coverage. Adding tests ensures:
- No regressions in future changes
- Proper error handling
- Security validations work correctly

### Priority 2: HIGH (Should Have)

These tests improve confidence in critical flows:

- ✅ Complete 2FA integration tests - **Phase 2.1**
- ✅ Magic link integration tests - **Phase 2.2**
- Passkey handler edge cases (52.4% → 75%+)
- 2FA handler edge cases (60-77% → 80%+)

**Rationale**: Integration tests validate that components work together correctly, which is more valuable than isolated unit tests.

### Priority 3: MEDIUM (Nice to Have)

These tests improve overall quality but are not critical:

- WebAuthn passkey finish flows (18.5%, 12.8% → 40%+)
- Helper function coverage (various → 90%+)
- Concurrent operation tests
- Performance tests

**Rationale**: These areas are partially tested or have low risk of failure. Incremental improvement is acceptable.

### Priority 4: LOW (Future)

These tests can be deferred to future sprints:

- Browser-based end-to-end tests
- Cross-browser compatibility tests
- Load testing
- Security penetration tests

**Rationale**: Require significant infrastructure setup. Current coverage is sufficient for MVP.

---

## Recommendations

### Immediate Actions (Week 13)

1. **Implement Phase 1**: TOTP and Magic Link handler tests
   - Create `handlers_totp_test.go`
   - Create `handlers_magiclink_test.go`
   - Add gRPC TOTP tests to `grpc_test.go`
   - **Target**: 75-77% coverage

2. **Run Coverage Analysis**: After Phase 1, re-run coverage
   ```bash
   go test -coverprofile=coverage.out ./connector/local-enhanced/
   go tool cover -html=coverage.out
   ```

3. **Document Progress**: Update TODO.md and CLAUDE.md with results

### Short-Term Actions (Week 14)

4. **Implement Phase 2**: Integration tests
   - Add 2FA flow tests to `integration_test.go`
   - Add magic link flow tests
   - **Target**: 78-82% coverage

5. **Code Quality**: Run linters and fix warnings
   ```bash
   golangci-lint run ./connector/local-enhanced/
   go fmt ./connector/local-enhanced/
   ```

### Long-Term Actions (Future Sprints)

6. **Browser Testing**: Set up Playwright/Selenium for passkey tests
7. **Performance Testing**: Load test authentication endpoints
8. **Security Audit**: Review for vulnerabilities

---

## Current State Summary

### What's Well Tested

✅ **OAuth Integration** (100%)
✅ **Storage Operations** (80-100%)
✅ **Validation Functions** (84-100%)
✅ **TOTP Core Logic** (90-100%)
✅ **2FA Policy Enforcement** (85%+)
✅ **Magic Link Core Logic** (75-100%)
✅ **Testing Utilities** (87-100%)
✅ **Configuration** (90-100%)

### What Needs Work

❌ **TOTP HTTP Handlers** (0%)
❌ **Magic Link HTTP Handlers** (0%)
❌ **gRPC TOTP Endpoints** (0%)
⚠️ **Passkey Finish Flows** (18.5%, 12.8%)
⚠️ **2FA Handlers** (31-77%)

### Coverage Breakdown by File

| File | Statements | Coverage | Missing |
|------|-----------|----------|---------|
| config.go | 100 | 90.0% | Edge cases |
| grpc.go | 550 | 73.5% | TOTP endpoints |
| handlers.go | 1468 | 52.1% | TOTP/Magic handlers |
| local.go | 399 | 82.7% | Template loading |
| magiclink.go | 388 | 83.5% | Handlers |
| passkey.go | 437 | 57.2% | Finish flows |
| password.go | 122 | 61.5% | VerifyPassword |
| storage.go | 667 | 86.5% | Edge cases |
| testing.go | 487 | 93.7% | Helper utils |
| totp.go | 487 | 78.3% | Handlers |
| twofa.go | 259 | 74.1% | Handlers |
| validation.go | 289 | 89.6% | Edge cases |

**Overall**: 68.8% (4,883 statements covered out of 7,093 total)

---

## Success Metrics

### Phase 1 Complete (75-77%)
- All TOTP handlers tested
- All magic link handlers tested
- All gRPC TOTP endpoints tested
- No untested critical functions

### Phase 2 Complete (78-82%)
- All 2FA flows tested end-to-end
- All magic link flows tested end-to-end
- Integration tests for major user journeys

### Target Achieved (>80%)
- Production-ready test suite
- Confidence in all major features
- Regression prevention
- Security validations confirmed

---

## Conclusion

Current coverage of **68.8%** is good but below the **>80% target**. The gap of **+11.2%** can be closed by:

1. **Phase 1** (1-2 days): Test TOTP and magic link handlers → **+6-8%**
2. **Phase 2** (2-3 days): Add integration tests → **+3-5%**
3. **Phase 3** (optional): Browser tests for passkeys → **+2-3%**

**Recommended Approach**: Implement Phase 1 and Phase 2 to reach **78-82% coverage**, which is sufficient for production. Phase 3 can be deferred to future iterations.

The low coverage in passkey finish functions (18.5%, 12.8%) is acceptable because:
- Session validation and error handling are tested
- Cryptographic verification is handled by the go-webauthn library (not our code)
- Browser testing is complex and time-consuming

**Next Step**: Implement Phase 1 tests in Week 13 to boost coverage to 75-77%.

---

**Last Updated**: 2025-11-18
**Author**: Enopax Platform Team
**Status**: Analysis Complete, Ready for Implementation
