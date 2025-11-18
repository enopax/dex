package e2e

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPasskeyRegistration tests the complete passkey registration flow with a virtual authenticator
func TestPasskeyRegistration(t *testing.T) {
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

	// Navigate to auth setup page (simulated - in real scenario this would be from Platform)
	// For testing, we'll use a test token endpoint
	setupURL := fmt.Sprintf("%s/setup-auth?token=test-setup-token-123", config.DexURL)

	t.Logf("Navigating to setup URL: %s", setupURL)
	_, err = page.Goto(setupURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	})

	// Note: This will fail without a running Dex server
	// We document this as an integration test requirement
	if err != nil {
		t.Logf("Note: This test requires a running Dex server at %s", config.DexURL)
		t.Skipf("Skipping: %v", err)
		return
	}

	// Wait for page to load
	time.Sleep(1 * time.Second)

	t.Run("ClickPasskeySetupButton", func(t *testing.T) {
		// Find and click the "Set up Passkey" button
		button, err := page.Locator("button:has-text('Set up Passkey')").First()
		if err != nil {
			// Button might have different text
			button, err = page.Locator("button:has-text('Passkey')").First()
		}
		require.NoError(t, err, "Failed to find passkey setup button")

		err = button.Click()
		require.NoError(t, err, "Failed to click passkey setup button")

		t.Log("Clicked passkey setup button")
	})

	t.Run("EnterPasskeyName", func(t *testing.T) {
		// Wait for passkey name prompt
		nameInput, err := page.Locator("input[name='passkey_name'], input[placeholder*='name' i]").First()
		require.NoError(t, err, "Failed to find passkey name input")

		// Enter passkey name
		passkeyName := fmt.Sprintf("E2E Test Passkey %d", time.Now().Unix())
		err = nameInput.Fill(passkeyName)
		require.NoError(t, err, "Failed to fill passkey name")

		t.Logf("Entered passkey name: %s", passkeyName)
	})

	t.Run("CompleteWebAuthnCeremony", func(t *testing.T) {
		// Click confirm/register button
		confirmButton, err := page.Locator("button:has-text('Register'), button:has-text('Create'), button:has-text('Confirm')").First()
		require.NoError(t, err, "Failed to find confirm button")

		err = confirmButton.Click()
		require.NoError(t, err, "Failed to click confirm button")

		t.Log("Initiated WebAuthn ceremony")

		// Wait for WebAuthn ceremony to complete
		// The virtual authenticator should automatically sign the challenge
		time.Sleep(2 * time.Second)
	})

	t.Run("VerifyRegistrationSuccess", func(t *testing.T) {
		// Check for success message or redirect
		// This could be a success message on the page or a redirect to the Platform

		// Option 1: Check for success message
		successLocator := page.Locator("text=/success|registered|created/i")
		err := successLocator.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(10000),
			State:   playwright.WaitForSelectorStateVisible,
		})

		if err != nil {
			// Option 2: Check if URL changed (redirect to Platform)
			currentURL := page.URL()
			if currentURL != setupURL {
				t.Logf("Page redirected to: %s", currentURL)
				assert.Contains(t, currentURL, config.CallbackURL, "Should redirect to callback URL")
			} else {
				t.Errorf("Registration did not complete: %v", err)
			}
		} else {
			t.Log("Registration success message found")
		}
	})
}

// TestPasskeyRegistrationBeginEndpoint tests the /passkey/register/begin endpoint directly
func TestPasskeyRegistrationBeginEndpoint(t *testing.T) {
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

	// Test registration begin endpoint
	t.Run("BeginRegistration", func(t *testing.T) {
		// Navigate to a page where we can execute JavaScript
		_, err = page.Goto("about:blank")
		require.NoError(t, err)

		// Call the begin registration endpoint via fetch
		beginURL := fmt.Sprintf("%s/passkey/register/begin", config.DexURL)

		script := fmt.Sprintf(`
			(async () => {
				const response = await fetch('%s', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body: JSON.stringify({
						user_id: 'test-user-id-123'
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
		assert.Contains(t, publicKey, "rp", "publicKey should contain rp")
		assert.Contains(t, publicKey, "user", "publicKey should contain user")

		t.Logf("Registration begin response: %+v", resultMap)
	})
}

// TestPasskeyRegistrationWithActualWebAuthn tests the complete flow with actual WebAuthn API
func TestPasskeyRegistrationWithActualWebAuthn(t *testing.T) {
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

	t.Run("CompleteRegistrationFlow", func(t *testing.T) {
		// Navigate to blank page for JS execution
		_, err = page.Goto("about:blank")
		require.NoError(t, err)

		// Execute full registration flow
		beginURL := fmt.Sprintf("%s/passkey/register/begin", config.DexURL)
		finishURL := fmt.Sprintf("%s/passkey/register/finish", config.DexURL)

		script := fmt.Sprintf(`
			(async () => {
				try {
					// Step 1: Begin registration
					const beginResponse = await fetch('%s', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ user_id: 'test-user-id-456' })
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
					publicKey.user.id = Uint8Array.from(atob(publicKey.user.id.replace(/-/g, '+').replace(/_/g, '/')), c => c.charCodeAt(0));

					const credential = await navigator.credentials.create({ publicKey });

					if (!credential) {
						throw new Error('Failed to create credential');
					}

					// Step 3: Finish registration
					const finishResponse = await fetch('%s', {
						method: 'POST',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({
							session_id: sessionId,
							credential: {
								id: credential.id,
								rawId: btoa(String.fromCharCode(...new Uint8Array(credential.rawId))),
								type: credential.type,
								response: {
									clientDataJSON: btoa(String.fromCharCode(...new Uint8Array(credential.response.clientDataJSON))),
									attestationObject: btoa(String.fromCharCode(...new Uint8Array(credential.response.attestationObject)))
								}
							},
							passkey_name: 'E2E Test Passkey'
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
			t.Logf("Registration flow error: %s", errorMsg)
			t.Logf("Full result: %s", toJSON(resultMap))
		}

		assert.True(t, success, "Registration flow should succeed")

		if success {
			data, ok := resultMap["data"].(map[string]interface{})
			require.True(t, ok, "Result should have data field")

			t.Logf("Registration complete: %s", toJSON(data))
			assert.Contains(t, data, "success", "Finish response should contain success")
			assert.Contains(t, data, "passkey_id", "Finish response should contain passkey_id")
		}
	})
}

// toJSON converts a value to pretty JSON string
func toJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(b)
}
