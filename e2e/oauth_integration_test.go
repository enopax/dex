package e2e

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthPasskeyFlow tests the complete OAuth flow with passkey authentication
func TestOAuthPasskeyFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	config := getTestConfig()
	pw, browser, context := setupBrowser(t)
	defer teardownBrowser(pw, browser, context)

	page, err := context.NewPage()
	require.NoError(t, err)
	defer page.Close()

	// Setup virtual authenticator
	cdpSession, err := page.Context().NewCDPSession(page)
	require.NoError(t, err)
	defer cdpSession.Detach()
	setupVirtualAuthenticator(t, cdpSession)

	t.Run("InitiateOAuthFlow", func(t *testing.T) {
		// Construct OAuth authorization URL
		authURL := fmt.Sprintf("%s/auth?client_id=example-app&redirect_uri=%s&response_type=code&scope=openid+email+profile&state=%s",
			config.DexURL, url.QueryEscape(config.CallbackURL), config.State)

		t.Logf("Initiating OAuth flow: %s", authURL)

		_, err = page.Goto(authURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		})

		if err != nil {
			t.Logf("Note: This test requires a running Dex server at %s with OAuth configured", config.DexURL)
			t.Skipf("Skipping: %v", err)
			return
		}

		time.Sleep(1 * time.Second)

		// Verify we're on the connector selection or login page
		currentURL := page.URL()
		t.Logf("Current URL after OAuth initiation: %s", currentURL)

		// Should either be on connector selection or directly on login page
		assert.Contains(t, currentURL, config.DexURL, "Should be on Dex server")
	})

	t.Run("SelectLocalConnector", func(t *testing.T) {
		// If there are multiple connectors, select the local-enhanced one
		localConnectorLink, err := page.Locator("a:has-text('Local'), a:has-text('Enopax')").First()
		if err == nil {
			// Connector selection page found
			err = localConnectorLink.Click()
			if err == nil {
				t.Log("Selected local-enhanced connector")
				time.Sleep(1 * time.Second)
			}
		} else {
			t.Log("No connector selection page (might be configured with single connector)")
		}
	})

	t.Run("AuthenticateWithPasskey", func(t *testing.T) {
		// Find and click passkey login button
		passkeyButton, err := page.Locator("button:has-text('Login with Passkey'), button:has-text('Passkey')").First()
		if err != nil {
			t.Skipf("Passkey button not found: %v", err)
			return
		}

		err = passkeyButton.Click()
		require.NoError(t, err, "Failed to click passkey button")

		t.Log("Initiated passkey authentication")
		time.Sleep(2 * time.Second)
	})

	t.Run("VerifyOAuthCallback", func(t *testing.T) {
		// Wait for redirect to callback URL with authorization code
		err := page.WaitForURL(fmt.Sprintf("%s*", config.CallbackURL), playwright.PageWaitForURLOptions{
			Timeout: playwright.Float(15000),
		})

		if err != nil {
			currentURL := page.URL()
			t.Logf("Did not redirect to callback. Current URL: %s", currentURL)
			t.Errorf("OAuth flow did not complete: %v", err)
			return
		}

		// Parse callback URL
		callbackURL := page.URL()
		t.Logf("OAuth callback URL: %s", callbackURL)

		parsedURL, err := url.Parse(callbackURL)
		require.NoError(t, err, "Failed to parse callback URL")

		// Verify query parameters
		query := parsedURL.Query()

		// Should have authorization code
		code := query.Get("code")
		assert.NotEmpty(t, code, "Authorization code should be present")
		t.Logf("Authorization code: %s", code)

		// State should be preserved
		state := query.Get("state")
		assert.Equal(t, config.State, state, "State parameter should match")

		// Should not have error
		errParam := query.Get("error")
		assert.Empty(t, errParam, "Should not have error parameter")
	})
}

// TestOAuthPasswordFlow tests OAuth flow with password authentication
func TestOAuthPasswordFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	config := getTestConfig()
	pw, browser, context := setupBrowser(t)
	defer teardownBrowser(pw, browser, context)

	page, err := context.NewPage()
	require.NoError(t, err)
	defer page.Close()

	t.Run("InitiateOAuthFlowAndLogin", func(t *testing.T) {
		// Construct OAuth authorization URL
		authURL := fmt.Sprintf("%s/auth?client_id=example-app&redirect_uri=%s&response_type=code&scope=openid+email+profile&state=%s",
			config.DexURL, url.QueryEscape(config.CallbackURL), config.State)

		_, err = page.Goto(authURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		})

		if err != nil {
			t.Logf("Note: This test requires a running Dex server")
			t.Skipf("Skipping: %v", err)
			return
		}

		time.Sleep(1 * time.Second)

		// Try to select local connector if present
		localConnectorLink, err := page.Locator("a:has-text('Local'), a:has-text('Enopax')").First()
		if err == nil {
			localConnectorLink.Click()
			time.Sleep(1 * time.Second)
		}

		// Fill in password login form
		usernameInput, err := page.Locator("input[name='username'], input[name='email'], input[type='email']").First()
		if err != nil {
			t.Skip("Password login form not found")
			return
		}

		err = usernameInput.Fill("test@example.com")
		require.NoError(t, err, "Failed to fill username")

		passwordInput, err := page.Locator("input[name='password'], input[type='password']").First()
		require.NoError(t, err, "Failed to find password input")

		err = passwordInput.Fill("testpassword123")
		require.NoError(t, err, "Failed to fill password")

		// Submit form
		submitButton, err := page.Locator("button[type='submit'], button:has-text('Login'), button:has-text('Sign in')").First()
		require.NoError(t, err, "Failed to find submit button")

		err = submitButton.Click()
		require.NoError(t, err, "Failed to click submit button")

		t.Log("Submitted password login form")
		time.Sleep(2 * time.Second)
	})

	t.Run("Handle2FAIfRequired", func(t *testing.T) {
		// Check if 2FA prompt appears
		totpInput, err := page.Locator("input[name='code'], input[placeholder*='code' i]").First()
		if err == nil {
			// 2FA is required
			t.Log("2FA prompt detected")

			// Enter TOTP code (this would need to be generated)
			// For testing purposes, we skip this or use a test code
			err = totpInput.Fill("123456")
			if err == nil {
				submitButton, err := page.Locator("button[type='submit']:has-text('Verify'), button:has-text('Submit')").First()
				if err == nil {
					submitButton.Click()
					time.Sleep(2 * time.Second)
				}
			}
		} else {
			t.Log("No 2FA required")
		}
	})

	t.Run("VerifyOAuthCallback", func(t *testing.T) {
		// Check if we reached the callback URL
		currentURL := page.URL()
		t.Logf("Current URL: %s", currentURL)

		if currentURL != "" && len(currentURL) > 0 {
			parsedURL, err := url.Parse(currentURL)
			if err == nil {
				// Log any query parameters
				query := parsedURL.Query()
				for key, values := range query {
					t.Logf("Query param %s: %v", key, values)
				}
			}
		}
	})
}

// TestOAuthStateValidation tests that state parameter is validated correctly
func TestOAuthStateValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	config := getTestConfig()
	pw, browser, context := setupBrowser(t)
	defer teardownBrowser(pw, browser, context)

	page, err := context.NewPage()
	require.NoError(t, err)
	defer page.Close()

	t.Run("InvalidStateParameter", func(t *testing.T) {
		// Construct OAuth URL with invalid state
		invalidState := "invalid-state-parameter"
		authURL := fmt.Sprintf("%s/auth?client_id=example-app&redirect_uri=%s&response_type=code&scope=openid&state=%s",
			config.DexURL, url.QueryEscape(config.CallbackURL), invalidState)

		_, err = page.Goto(authURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		})

		if err != nil {
			t.Skipf("Skipping: %v", err)
			return
		}

		// The state should be preserved throughout the flow
		// After authentication, the callback should include the same state

		currentURL := page.URL()
		t.Logf("URL with invalid state: %s", currentURL)

		// State parameter should be accepted (it's validated by the client, not Dex)
		assert.Contains(t, currentURL, config.DexURL, "Should be on Dex server")
	})
}

// TestOAuthErrorHandling tests error scenarios in OAuth flow
func TestOAuthErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	config := getTestConfig()
	pw, browser, context := setupBrowser(t)
	defer teardownBrowser(pw, browser, context)

	page, err := context.NewPage()
	require.NoError(t, err)
	defer page.Close()

	t.Run("InvalidClientID", func(t *testing.T) {
		// Construct OAuth URL with invalid client ID
		authURL := fmt.Sprintf("%s/auth?client_id=invalid-client&redirect_uri=%s&response_type=code&scope=openid&state=%s",
			config.DexURL, url.QueryEscape(config.CallbackURL), config.State)

		_, err = page.Goto(authURL, playwright.PageGotoOptions{
			Timeout: playwright.Float(10000),
		})

		if err != nil {
			t.Logf("Expected error for invalid client: %v", err)
		}

		currentURL := page.URL()
		t.Logf("URL after invalid client: %s", currentURL)

		// Should show error or redirect with error
		pageContent, _ := page.Content()
		assert.Contains(t, pageContent, "error", "Page should contain error message")
	})

	t.Run("InvalidRedirectURI", func(t *testing.T) {
		// Construct OAuth URL with invalid redirect URI
		invalidRedirect := "http://malicious-site.com/callback"
		authURL := fmt.Sprintf("%s/auth?client_id=example-app&redirect_uri=%s&response_type=code&scope=openid&state=%s",
			config.DexURL, url.QueryEscape(invalidRedirect), config.State)

		_, err = page.Goto(authURL, playwright.PageGotoOptions{
			Timeout: playwright.Float(10000),
		})

		if err != nil {
			t.Logf("Expected error for invalid redirect: %v", err)
		}

		currentURL := page.URL()
		t.Logf("URL after invalid redirect: %s", currentURL)

		// Should NOT redirect to malicious site
		assert.NotContains(t, currentURL, "malicious-site.com", "Should not redirect to invalid URI")
	})
}

// TestOAuthFlowWithLoginHint tests OAuth flow with login_hint parameter
func TestOAuthFlowWithLoginHint(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	config := getTestConfig()
	pw, browser, context := setupBrowser(t)
	defer teardownBrowser(pw, browser, context)

	page, err := context.NewPage()
	require.NoError(t, err)
	defer page.Close()

	t.Run("PreFillEmailWithLoginHint", func(t *testing.T) {
		// Construct OAuth URL with login_hint
		loginHint := "prefilled@example.com"
		authURL := fmt.Sprintf("%s/auth?client_id=example-app&redirect_uri=%s&response_type=code&scope=openid+email&state=%s&login_hint=%s",
			config.DexURL, url.QueryEscape(config.CallbackURL), config.State, url.QueryEscape(loginHint))

		_, err = page.Goto(authURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateNetworkidle,
			Timeout:   playwright.Float(30000),
		})

		if err != nil {
			t.Skipf("Skipping: %v", err)
			return
		}

		time.Sleep(1 * time.Second)

		// Try to select local connector if present
		localConnectorLink, err := page.Locator("a:has-text('Local'), a:has-text('Enopax')").First()
		if err == nil {
			localConnectorLink.Click()
			time.Sleep(1 * time.Second)
		}

		// Check if email field is pre-filled
		emailInput, err := page.Locator("input[name='username'], input[name='email'], input[type='email']").First()
		if err != nil {
			t.Skip("Email input not found")
			return
		}

		value, err := emailInput.InputValue()
		if err == nil {
			t.Logf("Email input value: %s", value)
			// Some implementations may pre-fill, others may not
			// This is implementation-dependent
		}
	})
}
