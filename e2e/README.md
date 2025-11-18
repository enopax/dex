# End-to-End Browser Tests

This directory contains end-to-end browser tests for the Enhanced Local Connector, specifically testing WebAuthn passkey functionality with virtual authenticators.

## Overview

These tests use [Playwright for Go](https://github.com/playwright-community/playwright-go) to automate browser interactions and test the complete authentication flows in a real browser environment.

### What's Tested

- **Passkey Registration**: Complete WebAuthn registration ceremony with virtual authenticator
- **Passkey Authentication**: Complete WebAuthn authentication ceremony
- **OAuth Integration**: Full OAuth flow with passkey authentication
- **Discoverable Credentials**: Passwordless authentication without email
- **Error Handling**: Invalid client IDs, redirect URIs, etc.

## Prerequisites

### 1. Install Playwright

The tests will automatically install Playwright browsers on first run via `TestMain`, but you can also install them manually:

```bash
# Install Playwright browsers
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install
```

### 2. Running Dex Server

These tests require a running Dex server instance. You can start Dex in development mode:

```bash
# Build Dex
make build

# Run Dex with development config
./bin/dex serve config.dev.yaml
```

Default Dex URL: `http://localhost:5556`

### 3. Configuration

The tests use environment variables for configuration:

- `DEX_URL`: Dex server URL (default: `http://localhost:5556`)

Example:

```bash
export DEX_URL=http://localhost:5556
```

## Running Tests

### Run All E2E Tests

```bash
# Run all end-to-end tests
go test -v ./e2e/

# Run with coverage
go test -v -coverprofile=e2e-coverage.out ./e2e/
```

### Run Specific Tests

```bash
# Run only passkey registration tests
go test -v ./e2e/ -run TestPasskeyRegistration

# Run only OAuth integration tests
go test -v ./e2e/ -run TestOAuth

# Run only authentication tests
go test -v ./e2e/ -run TestPasskeyAuthentication
```

### Skip E2E Tests

E2E tests are skipped in short mode:

```bash
# Skip all E2E tests
go test -short ./e2e/
```

### Running with Visible Browser (Headful Mode)

To see the browser during tests (useful for debugging), modify the `setupBrowser` function in `setup_test.go`:

```go
browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
    Headless: playwright.Bool(false),  // Change to false
    SlowMo:   playwright.Float(1000),  // Slow down actions by 1 second
})
```

## Test Structure

### Test Files

- **`setup_test.go`**: Common test setup, browser initialization, virtual authenticator setup
- **`passkey_registration_test.go`**: Passkey registration tests with WebAuthn
- **`passkey_authentication_test.go`**: Passkey authentication tests
- **`oauth_integration_test.go`**: OAuth flow integration tests

### Helper Functions

#### `setupBrowser(t *testing.T)`

Creates a new Playwright browser instance with:
- Headless Chromium browser
- Isolated browser context
- HTTPS error ignoring (for self-signed certs in dev)

#### `setupVirtualAuthenticator(t *testing.T, cdpSession playwright.CDPSession)`

Configures a virtual WebAuthn authenticator via Chrome DevTools Protocol (CDP):
- Protocol: CTAP2 (modern WebAuthn)
- Transport: Internal (platform authenticator)
- User verification: Enabled
- Resident keys: Enabled (for discoverable credentials)

This simulates a hardware security key or platform authenticator (like Touch ID) without requiring physical hardware.

#### `getTestConfig()`

Returns test configuration with defaults:
- Dex URL: `http://localhost:5556`
- Callback URL: `http://localhost:5555/callback`
- OAuth state: `test-state-123`

## Test Scenarios

### 1. Passkey Registration Flow

**Test**: `TestPasskeyRegistration`

**Flow**:
1. Navigate to auth setup page
2. Click "Set up Passkey" button
3. Enter passkey name
4. Complete WebAuthn ceremony (virtual authenticator signs challenge)
5. Verify registration success

**Expected Result**: Passkey registered successfully, user can proceed to platform.

### 2. Passkey Registration with WebAuthn API

**Test**: `TestPasskeyRegistrationWithActualWebAuthn`

**Flow**:
1. Call `POST /passkey/register/begin` endpoint
2. Parse registration options
3. Call `navigator.credentials.create()` with virtual authenticator
4. Call `POST /passkey/register/finish` with credential
5. Verify passkey created

**Expected Result**: Server returns `passkey_id` and success message.

### 3. Passkey Authentication Flow

**Test**: `TestPasskeyAuthentication`

**Flow**:
1. Navigate to login page with OAuth parameters
2. Click "Login with Passkey" button
3. Complete WebAuthn authentication ceremony
4. Verify redirect to OAuth callback with authorization code

**Expected Result**: User authenticated, redirected with OAuth code and preserved state.

### 4. Passkey Authentication with WebAuthn API

**Test**: `TestPasskeyAuthenticationWithActualWebAuthn`

**Flow**:
1. Call `POST /passkey/login/begin` endpoint
2. Parse authentication options
3. Call `navigator.credentials.get()` with virtual authenticator
4. Call `POST /passkey/login/finish` with assertion
5. Verify authentication success

**Expected Result**: Server returns `user_id` and `email`.

**Note**: Requires a passkey to be registered first for the test user.

### 5. Discoverable Credentials (Passwordless)

**Test**: `TestPasskeyDiscoverableCredentials`

**Flow**:
1. Call `/passkey/login/begin` WITHOUT email
2. Verify `allowCredentials` is empty
3. User can authenticate without providing email (platform authenticator returns user)

**Expected Result**: Authentication works without prior knowledge of email.

### 6. OAuth Integration with Passkey

**Test**: `TestOAuthPasskeyFlow`

**Flow**:
1. Initiate OAuth authorization request
2. Select local-enhanced connector
3. Authenticate with passkey
4. Verify redirect to callback with authorization code
5. Verify state parameter is preserved

**Expected Result**: Complete OAuth flow with passkey authentication.

### 7. OAuth Error Handling

**Tests**:
- `TestOAuthStateValidation`: State parameter preservation
- `TestOAuthErrorHandling`: Invalid client ID, redirect URI
- `TestOAuthFlowWithLoginHint`: Pre-filled email from `login_hint`

## Virtual Authenticator Details

### What is a Virtual Authenticator?

A virtual authenticator is a simulated WebAuthn authenticator provided by Chromium for testing purposes. It behaves like a real hardware security key or platform authenticator (Touch ID, Windows Hello) but is entirely software-based.

### Configuration

Our virtual authenticator is configured as:

```go
{
    "protocol": "ctap2",              // Modern WebAuthn protocol
    "transport": "internal",          // Platform authenticator (not USB/NFC)
    "hasUserVerification": true,      // Can verify user (PIN, biometric)
    "isUserVerified": true,           // User is verified automatically
    "hasResidentKey": true,           // Supports resident keys (discoverable credentials)
}
```

### Benefits

- **No Physical Hardware**: Tests run on CI/CD without security keys
- **Consistent Behavior**: No variation between different authenticators
- **Automatic User Verification**: No manual PIN/biometric input required
- **Resident Key Support**: Test discoverable credentials (passwordless)

### Limitations

- **Chromium Only**: Virtual authenticators are a Chrome/Chromium feature
- **Limited Error Testing**: Can't test all hardware failure scenarios
- **No Cross-Browser Testing**: Firefox, Safari require different approaches

## Debugging Tests

### View Browser Actions

Set `Headless: false` in `setupBrowser`:

```go
browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
    Headless: playwright.Bool(false),
    SlowMo:   playwright.Float(500),  // Slow down by 500ms per action
})
```

### Screenshots on Failure

Add screenshot capture in test:

```go
if t.Failed() {
    page.Screenshot(playwright.PageScreenshotOptions{
        Path: playwright.String(fmt.Sprintf("failure-%s.png", t.Name())),
    })
}
```

### Verbose Logging

Run tests with verbose flag:

```bash
go test -v ./e2e/ -run TestPasskeyRegistration
```

### Browser Console Logs

Capture console logs:

```go
page.On("console", func(msg playwright.ConsoleMessage) {
    t.Logf("Browser console: %s: %s", msg.Type(), msg.Text())
})
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: E2E Tests

on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.25'

      - name: Install Playwright
        run: go run github.com/playwright-community/playwright-go/cmd/playwright@latest install --with-deps chromium

      - name: Build Dex
        run: make build

      - name: Start Dex Server
        run: ./bin/dex serve config.dev.yaml &
        env:
          DEX_LOG_LEVEL: debug

      - name: Wait for Dex
        run: sleep 5

      - name: Run E2E Tests
        run: go test -v ./e2e/
        env:
          DEX_URL: http://localhost:5556
```

## Best Practices

### 1. Test Independence

Each test should be independent and not rely on state from other tests.

**Bad**:
```go
// TestA registers a passkey
// TestB assumes passkey exists from TestA
```

**Good**:
```go
// TestB registers its own passkey first, then authenticates
```

### 2. Cleanup

Always clean up resources:

```go
defer teardownBrowser(pw, browser, context)
defer page.Close()
defer cdpSession.Detach()
```

### 3. Timeouts

Set reasonable timeouts for network operations:

```go
page.Goto(url, playwright.PageGotoOptions{
    Timeout: playwright.Float(30000),  // 30 seconds
})
```

### 4. Error Messages

Provide helpful error messages:

```go
require.NoError(t, err, "Failed to click passkey button - button may not exist or be disabled")
```

### 5. Skip Gracefully

Skip tests if dependencies are missing:

```go
if err != nil {
    t.Logf("Note: This test requires a running Dex server at %s", config.DexURL)
    t.Skipf("Skipping: %v", err)
    return
}
```

## Troubleshooting

### "Failed to install playwright"

**Solution**: Install Playwright manually:

```bash
go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium
```

### "Connection refused to localhost:5556"

**Solution**: Start Dex server:

```bash
./bin/dex serve config.dev.yaml
```

### "WebAuthn.addVirtualAuthenticator failed"

**Solution**: Ensure you're using Chromium browser (not Firefox or WebKit):

```go
browser, err := pw.Chromium.Launch(...)  // Use Chromium
```

### "navigator.credentials is undefined"

**Solution**: Ensure HTTPS or localhost (WebAuthn requires secure context):

- Use `http://localhost` (allowed for testing)
- Or use HTTPS with valid certificate

### Tests Timeout

**Solution**: Increase timeout or check Dex logs:

```bash
# Check Dex logs
tail -f /tmp/dex.log

# Increase timeout
page.Goto(url, playwright.PageGotoOptions{
    Timeout: playwright.Float(60000),  // 60 seconds
})
```

## Future Enhancements

- [ ] Cross-browser testing (Firefox, Safari, Edge)
- [ ] Mobile browser testing (iOS Safari, Android Chrome)
- [ ] Performance benchmarks (authentication latency)
- [ ] Visual regression testing (screenshot comparison)
- [ ] Accessibility testing (WCAG compliance)
- [ ] Network failure simulation (offline mode, slow 3G)
- [ ] Multiple authenticators (test credential selection)

## References

- [Playwright Go Documentation](https://playwright.dev/docs/intro)
- [WebAuthn Spec](https://www.w3.org/TR/webauthn-2/)
- [Chrome DevTools Protocol - WebAuthn](https://chromedevtools.github.io/devtools-protocol/tot/WebAuthn/)
- [WebAuthn Guide](https://webauthn.guide/)
- [go-webauthn Library](https://github.com/go-webauthn/webauthn)

## Contact

For questions or issues with browser tests, please:

1. Check this README first
2. Check existing GitHub issues
3. Create a new issue with:
   - Test name that failed
   - Error message
   - Dex configuration
   - Browser version (from Playwright)
   - Operating system

---

**Last Updated**: 2025-11-18
**Test Coverage**: Passkey registration, authentication, OAuth integration, discoverable credentials
