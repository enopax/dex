package yamlfile

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dexidp/dex/storage"
)

// AuthRequest operations

func (s *yamlFileStorage) CreateAuthRequest(ctx context.Context, a storage.AuthRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", a.ID)

	if s.fileExists("auth-requests", filename) {
		return storage.ErrAlreadyExists
	}

	return s.writeFile("auth-requests", filename, a)
}

func (s *yamlFileStorage) GetAuthRequest(ctx context.Context, id string) (storage.AuthRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var a storage.AuthRequest
	filename := fmt.Sprintf("%s.yaml", id)

	if err := s.readFile("auth-requests", filename, &a); err != nil {
		return storage.AuthRequest{}, err
	}

	return a, nil
}

func (s *yamlFileStorage) UpdateAuthRequest(ctx context.Context, id string, updater func(a storage.AuthRequest) (storage.AuthRequest, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, err := s.GetAuthRequest(ctx, id)
	if err != nil {
		return err
	}

	updated, err := updater(a)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.yaml", id)
	return s.writeFile("auth-requests", filename, updated)
}

func (s *yamlFileStorage) DeleteAuthRequest(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", id)
	return s.deleteFile("auth-requests", filename)
}

// AuthCode operations

func (s *yamlFileStorage) CreateAuthCode(ctx context.Context, c storage.AuthCode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", c.ID)

	if s.fileExists("auth-codes", filename) {
		return storage.ErrAlreadyExists
	}

	return s.writeFile("auth-codes", filename, c)
}

func (s *yamlFileStorage) GetAuthCode(ctx context.Context, id string) (storage.AuthCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var c storage.AuthCode
	filename := fmt.Sprintf("%s.yaml", id)

	if err := s.readFile("auth-codes", filename, &c); err != nil {
		return storage.AuthCode{}, err
	}

	return c, nil
}

func (s *yamlFileStorage) DeleteAuthCode(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", id)
	return s.deleteFile("auth-codes", filename)
}

// RefreshToken operations

func (s *yamlFileStorage) CreateRefresh(ctx context.Context, r storage.RefreshToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", r.ID)

	if s.fileExists("refresh-tokens", filename) {
		return storage.ErrAlreadyExists
	}

	return s.writeFile("refresh-tokens", filename, r)
}

func (s *yamlFileStorage) GetRefresh(ctx context.Context, id string) (storage.RefreshToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var r storage.RefreshToken
	filename := fmt.Sprintf("%s.yaml", id)

	if err := s.readFile("refresh-tokens", filename, &r); err != nil {
		return storage.RefreshToken{}, err
	}

	return r, nil
}

func (s *yamlFileStorage) UpdateRefreshToken(ctx context.Context, id string, updater func(r storage.RefreshToken) (storage.RefreshToken, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, err := s.GetRefresh(ctx, id)
	if err != nil {
		return err
	}

	updated, err := updater(r)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.yaml", id)
	return s.writeFile("refresh-tokens", filename, updated)
}

func (s *yamlFileStorage) DeleteRefresh(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", id)
	return s.deleteFile("refresh-tokens", filename)
}

func (s *yamlFileStorage) ListRefreshTokens(ctx context.Context) ([]storage.RefreshToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := os.ReadDir(filepath.Join(s.dataDir, "refresh-tokens"))
	if err != nil {
		return nil, err
	}

	var tokens []storage.RefreshToken
	for _, file := range files {
		var r storage.RefreshToken
		if err := s.readFile("refresh-tokens", file.Name(), &r); err != nil {
			s.logger.Warn("failed to read refresh token file", "file", file.Name(), "error", err)
			continue
		}
		tokens = append(tokens, r)
	}

	return tokens, nil
}

// OfflineSessions operations

func (s *yamlFileStorage) CreateOfflineSessions(ctx context.Context, o storage.OfflineSessions) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s-%s.yaml", o.UserID, o.ConnID)

	if s.fileExists("offline-sessions", filename) {
		return storage.ErrAlreadyExists
	}

	return s.writeFile("offline-sessions", filename, o)
}

func (s *yamlFileStorage) GetOfflineSessions(ctx context.Context, userID string, connID string) (storage.OfflineSessions, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var o storage.OfflineSessions
	filename := fmt.Sprintf("%s-%s.yaml", userID, connID)

	if err := s.readFile("offline-sessions", filename, &o); err != nil {
		return storage.OfflineSessions{}, err
	}

	return o, nil
}

func (s *yamlFileStorage) UpdateOfflineSessions(ctx context.Context, userID string, connID string, updater func(s storage.OfflineSessions) (storage.OfflineSessions, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	o, err := s.GetOfflineSessions(ctx, userID, connID)
	if err != nil {
		return err
	}

	updated, err := updater(o)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%s.yaml", userID, connID)
	return s.writeFile("offline-sessions", filename, updated)
}

func (s *yamlFileStorage) DeleteOfflineSessions(ctx context.Context, userID string, connID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s-%s.yaml", userID, connID)
	return s.deleteFile("offline-sessions", filename)
}

// Connector operations

func (s *yamlFileStorage) CreateConnector(ctx context.Context, c storage.Connector) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", c.ID)

	if s.fileExists("connectors", filename) {
		return storage.ErrAlreadyExists
	}

	return s.writeFile("connectors", filename, c)
}

func (s *yamlFileStorage) GetConnector(ctx context.Context, id string) (storage.Connector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var c storage.Connector
	filename := fmt.Sprintf("%s.yaml", id)

	if err := s.readFile("connectors", filename, &c); err != nil {
		return storage.Connector{}, err
	}

	return c, nil
}

func (s *yamlFileStorage) UpdateConnector(ctx context.Context, id string, updater func(c storage.Connector) (storage.Connector, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, err := s.GetConnector(ctx, id)
	if err != nil {
		return err
	}

	updated, err := updater(c)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.yaml", id)
	return s.writeFile("connectors", filename, updated)
}

func (s *yamlFileStorage) DeleteConnector(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", id)
	return s.deleteFile("connectors", filename)
}

func (s *yamlFileStorage) ListConnectors(ctx context.Context) ([]storage.Connector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := os.ReadDir(filepath.Join(s.dataDir, "connectors"))
	if err != nil {
		return nil, err
	}

	var connectors []storage.Connector
	for _, file := range files {
		var c storage.Connector
		if err := s.readFile("connectors", file.Name(), &c); err != nil {
			s.logger.Warn("failed to read connector file", "file", file.Name(), "error", err)
			continue
		}
		connectors = append(connectors, c)
	}

	return connectors, nil
}

// DeviceRequest operations

func (s *yamlFileStorage) CreateDeviceRequest(ctx context.Context, d storage.DeviceRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", d.UserCode)

	if s.fileExists("device-requests", filename) {
		return storage.ErrAlreadyExists
	}

	return s.writeFile("device-requests", filename, d)
}

func (s *yamlFileStorage) GetDeviceRequest(ctx context.Context, userCode string) (storage.DeviceRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var d storage.DeviceRequest
	filename := fmt.Sprintf("%s.yaml", userCode)

	if err := s.readFile("device-requests", filename, &d); err != nil {
		return storage.DeviceRequest{}, err
	}

	return d, nil
}

func (s *yamlFileStorage) DeleteDeviceRequest(ctx context.Context, userCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", userCode)
	return s.deleteFile("device-requests", filename)
}

// DeviceToken operations

func (s *yamlFileStorage) CreateDeviceToken(ctx context.Context, d storage.DeviceToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", d.DeviceCode)

	if s.fileExists("device-tokens", filename) {
		return storage.ErrAlreadyExists
	}

	return s.writeFile("device-tokens", filename, d)
}

func (s *yamlFileStorage) GetDeviceToken(ctx context.Context, deviceCode string) (storage.DeviceToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var d storage.DeviceToken
	filename := fmt.Sprintf("%s.yaml", deviceCode)

	if err := s.readFile("device-tokens", filename, &d); err != nil {
		return storage.DeviceToken{}, err
	}

	return d, nil
}

func (s *yamlFileStorage) UpdateDeviceToken(ctx context.Context, deviceCode string, updater func(t storage.DeviceToken) (storage.DeviceToken, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, err := s.GetDeviceToken(ctx, deviceCode)
	if err != nil {
		return err
	}

	updated, err := updater(d)
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%s.yaml", deviceCode)
	return s.writeFile("device-tokens", filename, updated)
}

func (s *yamlFileStorage) DeleteDeviceToken(ctx context.Context, deviceCode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filename := fmt.Sprintf("%s.yaml", deviceCode)
	return s.deleteFile("device-tokens", filename)
}

// Keys operations

func (s *yamlFileStorage) GetKeys(ctx context.Context) (storage.Keys, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys storage.Keys
	filename := "signing-keys.yaml"

	if err := s.readFile("keys", filename, &keys); err != nil {
		if err == storage.ErrNotFound {
			// Return empty keys if file doesn't exist yet
			return storage.Keys{}, nil
		}
		return storage.Keys{}, err
	}

	return keys, nil
}

func (s *yamlFileStorage) UpdateKeys(ctx context.Context, updater func(old storage.Keys) (storage.Keys, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Read keys directly without calling GetKeys() to avoid deadlock
	var keys storage.Keys
	filename := "signing-keys.yaml"

	if err := s.readFile("keys", filename, &keys); err != nil {
		if err != storage.ErrNotFound {
			return err
		}
		// If file doesn't exist, keys will remain as empty struct
	}

	updated, err := updater(keys)
	if err != nil {
		return err
	}

	return s.writeFile("keys", filename, updated)
}
