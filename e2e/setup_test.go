package e2e

import (
	"fmt"
	"os"
	"testing"

	"github.com/playwright-community/playwright-go"
)

// TestMain sets up and tears down Playwright for all tests in this package
func TestMain(m *testing.M) {
	// Install Playwright browsers if not already installed
	if err := playwright.Install(); err != nil {
		fmt.Printf("could not install playwright: %v\n", err)
		os.Exit(1)
	}

	// Run tests
	exitCode := m.Run()

	os.Exit(exitCode)
}

// setupBrowser creates a new browser instance for testing
func setupBrowser(t *testing.T) (*playwright.Playwright, playwright.Browser, playwright.BrowserContext) {
	t.Helper()

	// Create Playwright instance
	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("could not start playwright: %v", err)
	}

	// Launch Chromium browser with headless mode
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args: []string{
			"--disable-blink-features=AutomationControlled",
		},
	})
	if err != nil {
		pw.Stop()
		t.Fatalf("could not launch browser: %v", err)
	}

	// Create browser context
	context, err := browser.NewContext(playwright.BrowserNewContextOptions{
		AcceptDownloads:    playwright.Bool(false),
		IgnoreHttpsErrors: playwright.Bool(true), // Allow self-signed certs in dev
	})
	if err != nil {
		browser.Close()
		pw.Stop()
		t.Fatalf("could not create context: %v", err)
	}

	return pw, browser, context
}

// setupVirtualAuthenticator creates a virtual authenticator for WebAuthn testing
func setupVirtualAuthenticator(t *testing.T, cdpSession playwright.CDPSession) {
	t.Helper()

	// Add virtual authenticator via CDP
	_, err := cdpSession.Send("WebAuthn.enable", nil)
	if err != nil {
		t.Fatalf("could not enable WebAuthn: %v", err)
	}

	// Add virtual authenticator with platform authenticator
	params := map[string]interface{}{
		"options": map[string]interface{}{
			"protocol":            "ctap2",
			"transport":           "internal",
			"hasUserVerification": true,
			"isUserVerified":      true,
			"hasResidentKey":      true,
		},
	}

	_, err = cdpSession.Send("WebAuthn.addVirtualAuthenticator", params)
	if err != nil {
		t.Fatalf("could not add virtual authenticator: %v", err)
	}

	t.Log("Virtual authenticator configured successfully")
}

// teardownBrowser closes browser resources
func teardownBrowser(pw *playwright.Playwright, browser playwright.Browser, context playwright.BrowserContext) {
	if context != nil {
		context.Close()
	}
	if browser != nil {
		browser.Close()
	}
	if pw != nil {
		pw.Stop()
	}
}

// testConfig holds test configuration
type testConfig struct {
	DexURL      string
	CallbackURL string
	State       string
}

// getTestConfig returns test configuration from environment or defaults
func getTestConfig() testConfig {
	dexURL := os.Getenv("DEX_URL")
	if dexURL == "" {
		dexURL = "http://localhost:5556" // Default Dex dev server
	}

	return testConfig{
		DexURL:      dexURL,
		CallbackURL: "http://localhost:5555/callback",
		State:       "test-state-123",
	}
}
