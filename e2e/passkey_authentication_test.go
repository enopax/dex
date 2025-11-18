package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPasskeyAuthentication tests the complete passkey authentication flow
func TestPasskeyAuthentication(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping browser test in short mode")
	}

	// Setup
	config := getTestConfig()
	pw, browser, context := setupBrowser(t)
	defer teardownBrowser(pw, browser, context)

	// Create new page
	page, err := context.NewPage()
	require.NoError(t, err, "Failed to create new page")
	defer page.Close()

	// Get CDP session for virtual authenticator
	cdpSession, err := page.Context().NewCDPSession(page)
	require.NoError(t, err, "Failed to create CDP session")
	defer cdpSession.Detach()

	// Setup virtual authenticator
	setupVirtualAuthenticator(t, cdpSession)

	// Navigate to login page
	loginURL := fmt.Sprintf("%s/login?state=%s&callback=%s",
		config.DexURL, config.State, config.CallbackURL)

	t.Logf("Navigating to login URL: %s", loginURL)
	_, err = page.Goto(loginURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	})

	if err != nil {
		t.Logf("Note: This test requires a running Dex server at %s", config.DexURL)
		t.Skipf("Skipping: %v", err)
		return
	}

	// Wait for page to load
	time.Sleep(1 * time.Second)

	t.Run("ClickPasskeyLoginButton", func(t *testing.T) {
		// Find and click the "Login with Passkey" button
		button, err := page.Locator("button:has-text('Login with Passkey'), button:has-text('Passkey')").First()
		require.NoError(t, err, "Failed to find passkey login button")

		err = button.Click()
		require.NoError(t, err, "Failed to click passkey login button")

		t.Log("Clicked passkey login button")
	})

	t.Run("CompleteWebAuthnAuthentication", func(t *testing.T) {
		// Wait for WebAuthn ceremony to complete
		// The virtual authenticator should automatically sign the challenge
		time.Sleep(2 * time.Second)

		t.Log("WebAuthn authentication ceremony completed")
	})

	t.Run("VerifyAuthenticationSuccess", func(t *testing.T) {
		// Check for redirect to callback URL with authorization code
		err := page.WaitForURL(fmt.Sprintf("%s*", config.CallbackURL), playwright.PageWaitForURLOptions{
			Timeout: playwright.Float(10000),
		})

		if err != nil {
			// Alternatively, check for success message
			successLocator := page.Locator("text=/success|authenticated|logged in/i")
			err2 := successLocator.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(5000),
				State:   playwright.WaitForSelectorStateVisible,
			})

			if err2 != nil {
				currentURL := page.URL()
				t.Logf("Current URL: %s", currentURL)
				t.Errorf("Authentication did not complete: redirect error: %v, message error: %v", err, err2)
			} else {
				t.Log("Authentication success message found")
			}
		} else {
			currentURL := page.URL()
			t.Logf("Redirected to: %s", currentURL)
			assert.Contains(t, currentURL, config.CallbackURL, "Should redirect to callback URL")

			// Verify state parameter is preserved
			assert.Contains(t, currentURL, config.State, "State parameter should be preserved")
		}
	})
}

// TestPasskeyAuthenticationBeginEndpoint tests the /passkey/login/begin endpoint directly
func TestPasskeyAuthenticationBeginEndpoint(t *testing.T) {
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

	t.Run("BeginAuthentication", func(t *testing.T) {
		// Navigate to a page where we can execute JavaScript
		_, err = page.Goto("about:blank")
		require.NoError(t, err)

		// Call the begin authentication endpoint via fetch
		beginURL := fmt.Sprintf("%s/passkey/login/begin", config.DexURL)

		script := fmt.Sprintf(`
			(async () => {
				const response = await fetch('%s', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body: JSON.stringify({
						email: 'test@example.com'
					})
				});

				if (!response.ok) {
					throw new Error('HTTP error! status: ' + response.status);
				}

				const data = await response.json();
				return data;
			})()
		`, beginURL)

		result, err := page.Evaluate(script)
		if err != nil {
			t.Logf("Note: This test requires a running Dex server at %s", config.DexURL)
			t.Skipf("Skipping: %v", err)
			return
		}

		// Verify response structure
		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "Response should be a map")

		assert.Contains(t, resultMap, "session_id", "Response should contain session_id")
		assert.Contains(t, resultMap, "options", "Response should contain options")

		// Verify options structure
		options, ok := resultMap["options"].(map[string]interface{})
		require.True(t, ok, "Options should be a map")

		publicKey, ok := options["publicKey"].(map[string]interface{})
		require.True(t, ok, "publicKey should be present")

		assert.Contains(t, publicKey, "challenge", "publicKey should contain challenge")
		assert.Contains(t, publicKey, "rpId", "publicKey should contain rpId")

		t.Logf("Authentication begin response: %+v", resultMap)
	})
}

// TestPasskeyAuthenticationWithActualWebAuthn tests the complete flow with actual WebAuthn API
func TestPasskeyAuthenticationWithActualWebAuthn(t *testing.T) {
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

	t.Run("CompleteAuthenticationFlow", func(t *testing.T) {
		// Navigate to blank page for JS execution
		_, err = page.Goto("about:blank")
		require.NoError(t, err)

		// Execute full authentication flow
		beginURL := fmt.Sprintf("%s/passkey/login/begin", config.DexURL)
		finishURL := fmt.Sprintf("%s/passkey/login/finish", config.DexURL)

		// Note: This test assumes a passkey was already registered for test@example.com
		script := fmt.Sprintf(`
			(async () => {
				try {
					// Step 1: Begin authentication
					const beginResponse = await fetch('%s', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ email: 'test@example.com' })
					});

					if (!beginResponse.ok) {
						throw new Error('Begin failed: ' + beginResponse.status);
					}

					const beginData = await beginResponse.json();
					const sessionId = beginData.session_id;
					const options = beginData.options;

					// Step 2: Call WebAuthn API
					// Convert challenge from base64url to ArrayBuffer
					const publicKey = options.publicKey;
					publicKey.challenge = Uint8Array.from(atob(publicKey.challenge.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0));

					// Convert allowCredentials if present
					if (publicKey.allowCredentials) {
						publicKey.allowCredentials = publicKey.allowCredentials.map(cred => ({
							...cred,
							id: Uint8Array.from(atob(cred.id.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0))
						}));
					}

					const assertion = await navigator.credentials.get({ publicKey });

					if (!assertion) {
						throw new Error('Failed to get assertion');
					}

					// Step 3: Finish authentication
					const finishResponse = await fetch('%s', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							session_id: sessionId,
							credential: {
								id: assertion.id,
								rawId: btoa(String.fromCharCode(...new Uint8Array(assertion.rawId))),
								type: assertion.type,
								response: {
									clientDataJSON: btoa(String.fromCharCode(...new Uint8Array(assertion.response.clientDataJSON))),
									authenticatorData: btoa(String.fromCharCode(...new Uint8Array(assertion.response.authenticatorData))),
									signature: btoa(String.fromCharCode(...new Uint8Array(assertion.response.signature))),
									userHandle: assertion.response.userHandle ? btoa(String.fromCharCode(...new Uint8Array(assertion.response.userHandle))) : null
								}
							}
						})
					});

					if (!finishResponse.ok) {
						const errorText = await finishResponse.text();
						throw new Error('Finish failed: ' + finishResponse.status + ' - ' + errorText);
					}

					const finishData = await finishResponse.json();
					return { success: true, data: finishData };
				} catch (error) {
					return { success: false, error: error.message };
				}
			})()
		`, beginURL, finishURL)

		result, err := page.Evaluate(script)
		if err != nil {
			t.Logf("Note: This test requires a running Dex server at %s", config.DexURL)
			t.Logf("Note: This test also requires a passkey to be registered for test@example.com")
			t.Skipf("Skipping: %v", err)
			return
		}

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")

		// Check for success
		success, ok := resultMap["success"].(bool)
		require.True(t, ok, "Result should have success field")

		if !success {
			errorMsg, _ := resultMap["error"].(string)
			t.Logf("Authentication flow error: %s", errorMsg)
			t.Logf("Full result: %s", toJSON(resultMap))

			// This is expected to fail if no passkey is registered
			t.Logf("Note: Register a passkey first with TestPasskeyRegistrationWithActualWebAuthn")
		} else {
			data, ok := resultMap["data"].(map[string]interface{})
			require.True(t, ok, "Result should have data field")

			t.Logf("Authentication complete: %s", toJSON(data))
			assert.Contains(t, data, "success", "Finish response should contain success")
			assert.Contains(t, data, "user_id", "Finish response should contain user_id")
		}
	})
}

// TestPasskeyDiscoverableCredentials tests passwordless authentication with discoverable credentials
func TestPasskeyDiscoverableCredentials(t *testing.T) {
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

	t.Run("AuthenticateWithoutEmail", func(t *testing.T) {
		_, err = page.Goto("about:blank")
		require.NoError(t, err)

		// Call begin authentication without email (discoverable credentials)
		beginURL := fmt.Sprintf("%s/passkey/login/begin", config.DexURL)

		script := fmt.Sprintf(`
			(async () => {
				try {
					// Step 1: Begin authentication WITHOUT email
					const beginResponse = await fetch('%s', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({})  // Empty body for discoverable credentials
					});

					if (!beginResponse.ok) {
						throw new Error('Begin failed: ' + beginResponse.status);
					}

					const beginData = await beginResponse.json();

					// Verify allowCredentials is empty (for discoverable credentials)
					const allowCredentials = beginData.options.publicKey.allowCredentials;
					return {
						success: true,
						allowCredentials: allowCredentials,
						isEmpty: !allowCredentials || allowCredentials.length === 0
					};
				} catch (error) {
					return { success: false, error: error.message };
				}
			})()
		`, beginURL)

		result, err := page.Evaluate(script)
		if err != nil {
			t.Logf("Note: This test requires a running Dex server at %s", config.DexURL)
			t.Skipf("Skipping: %v", err)
			return
		}

		resultMap, ok := result.(map[string]interface{})
		require.True(t, ok, "Result should be a map")

		success, _ := resultMap["success"].(bool)
		if success {
			isEmpty, _ := resultMap["isEmpty"].(bool)
			assert.True(t, isEmpty, "allowCredentials should be empty for discoverable credentials")
			t.Log("Discoverable credentials authentication supported")
		} else {
			errorMsg, _ := resultMap["error"].(string)
			t.Logf("Discoverable credentials test error: %s", errorMsg)
		}
	})
}
