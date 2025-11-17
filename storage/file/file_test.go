package file

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dexidp/dex/storage"
	"github.com/dexidp/dex/storage/conformance"
)

// TestStorage runs the standard conformance test suite against file storage
func TestStorage(t *testing.T) {
	newStorage := func(t *testing.T) storage.Storage {
		// Create temporary directory for test data
		tempDir, err := os.MkdirTemp("", "dex-file-storage-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		// Clean up after test
		t.Cleanup(func() {
			os.RemoveAll(tempDir)
		})

		// Create logger for test output
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		// Create file storage configuration
		config := &Config{
			DataDir: tempDir,
		}

		// Open storage
		s, err := config.Open(logger)
		if err != nil {
			t.Fatalf("failed to open storage: %v", err)
		}

		return s
	}

	// Run the standard conformance test suite
	conformance.RunTests(t, newStorage)
}

// TestStorageWithTimeout runs tests with timeout to detect potential deadlocks
func TestStorageWithTimeout(t *testing.T) {
	newStorage := func(t *testing.T) storage.Storage {
		tempDir, err := os.MkdirTemp("", "dex-file-storage-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}

		t.Cleanup(func() {
			os.RemoveAll(tempDir)
		})

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		config := &Config{
			DataDir: tempDir,
		}

		s, err := config.Open(logger)
		if err != nil {
			t.Fatalf("failed to open storage: %v", err)
		}

		return s
	}

	// Run tests with timeout to detect deadlocks
	withTimeout(time.Minute*1, func() {
		conformance.RunTests(t, newStorage)
	})
}

// TestFileStorageSpecific tests file-storage-specific behavior
func TestFileStorageSpecific(t *testing.T) {
	t.Run("DirectoryCreation", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dex-file-storage-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		config := &Config{
			DataDir: tempDir,
		}

		_, err = config.Open(logger)
		if err != nil {
			t.Fatalf("failed to open storage: %v", err)
		}

		// Verify all subdirectories were created
		expectedDirs := []string{
			"passwords",
			"clients",
			"auth-requests",
			"auth-codes",
			"refresh-tokens",
			"offline-sessions",
			"connectors",
			"device-requests",
			"device-tokens",
			"keys",
		}

		for _, dir := range expectedDirs {
			dirPath := filepath.Join(tempDir, dir)
			info, err := os.Stat(dirPath)
			if err != nil {
				t.Errorf("directory %s not created: %v", dir, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("%s is not a directory", dir)
			}
			// Verify permissions (should be 0700)
			if info.Mode().Perm() != 0700 {
				t.Errorf("directory %s has wrong permissions: got %o, want 0700", dir, info.Mode().Perm())
			}
		}
	})

	t.Run("JSONFormatting", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dex-file-storage-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		config := &Config{
			DataDir: tempDir,
		}

		s, err := config.Open(logger)
		if err != nil {
			t.Fatalf("failed to open storage: %v", err)
		}

		// Create a test client
		ctx := t.Context()
		client := storage.Client{
			ID:           "test-client",
			Secret:       "test-secret",
			RedirectURIs: []string{"http://localhost/callback"},
			Name:         "Test Client",
		}

		if err := s.CreateClient(ctx, client); err != nil {
			t.Fatalf("failed to create client: %v", err)
		}

		// Read the JSON file and verify it's properly formatted
		filePath := filepath.Join(tempDir, "clients", "test-client.json")
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read client file: %v", err)
		}

		// Check that JSON is indented (contains newlines and spaces)
		dataStr := string(data)
		if len(dataStr) == 0 {
			t.Error("JSON file is empty")
		}
		// Should have newlines (indented JSON)
		hasNewlines := false
		for _, c := range dataStr {
			if c == '\n' {
				hasNewlines = true
				break
			}
		}
		if !hasNewlines {
			t.Error("JSON file is not indented (no newlines found)")
		}
	})

	t.Run("FilePermissions", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dex-file-storage-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		config := &Config{
			DataDir: tempDir,
		}

		s, err := config.Open(logger)
		if err != nil {
			t.Fatalf("failed to open storage: %v", err)
		}

		// Create a password (sensitive data)
		ctx := t.Context()
		password := storage.Password{
			Email:    "test@example.com",
			Hash:     []byte("test-hash"),
			Username: "testuser",
			UserID:   "test-user-id",
		}

		if err := s.CreatePassword(ctx, password); err != nil {
			t.Fatalf("failed to create password: %v", err)
		}

		// Verify file permissions (should be 0600 for sensitive data)
		filePath := filepath.Join(tempDir, "passwords", "test-user-id.json")
		info, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat password file: %v", err)
		}

		if info.Mode().Perm() != 0600 {
			t.Errorf("password file has wrong permissions: got %o, want 0600", info.Mode().Perm())
		}
	})

	t.Run("EmptyDataDir", func(t *testing.T) {
		config := &Config{
			DataDir: "",
		}

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		_, err := config.Open(logger)
		if err == nil {
			t.Error("expected error when DataDir is empty, got nil")
		}
	})

	t.Run("CloseStorage", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "dex-file-storage-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		config := &Config{
			DataDir: tempDir,
		}

		s, err := config.Open(logger)
		if err != nil {
			t.Fatalf("failed to open storage: %v", err)
		}

		// Close should not return error (file storage has no connections to close)
		if err := s.Close(); err != nil {
			t.Errorf("close returned error: %v", err)
		}
	})
}

// withTimeout runs a function with a timeout to detect deadlocks
func withTimeout(timeout time.Duration, f func()) {
	done := make(chan struct{})
	go func() {
		f()
		close(done)
	}()

	select {
	case <-done:
		// Test completed successfully
	case <-time.After(timeout):
		panic("test timed out - possible deadlock")
	}
}
