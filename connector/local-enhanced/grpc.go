package local

import (
	"context"
	"fmt"
	"time"

	"github.com/dexidp/dex/api/v2"
)

// GRPCServer implements the EnhancedLocalConnector gRPC service.
type GRPCServer struct {
	api.UnimplementedEnhancedLocalConnectorServer
	connector *Connector
}

// NewGRPCServer creates a new gRPC server for the enhanced local connector.
func NewGRPCServer(connector *Connector) *GRPCServer {
	return &GRPCServer{
		connector: connector,
	}
}

// CreateUser creates a new user account.
func (s *GRPCServer) CreateUser(ctx context.Context, req *api.CreateUserReq) (*api.CreateUserResp, error) {
	// Validate input
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}

	if err := ValidateEmail(req.Email); err != nil {
		return nil, fmt.Errorf("invalid email: %w", err)
	}

	// Check if user already exists
	existingUser, err := s.connector.storage.GetUserByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		// User already exists
		return &api.CreateUserResp{
			User:          convertUserToProto(existingUser),
			AlreadyExists: true,
		}, nil
	}

	// Create new user
	user := &User{
		ID:            generateUserID(req.Email),
		Email:         req.Email,
		Username:      req.Username,
		DisplayName:   req.DisplayName,
		EmailVerified: false, // Email not verified by default
		Require2FA:    false, // 2FA not required by default
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Validate user
	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("invalid user: %w", err)
	}

	// Create user
	if err := s.connector.storage.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	s.connector.logger.Infof("gRPC: Created user %s (%s)", user.ID, user.Email)

	return &api.CreateUserResp{
		User:          convertUserToProto(user),
		AlreadyExists: false,
	}, nil
}

// GetUser retrieves user details by ID or email.
func (s *GRPCServer) GetUser(ctx context.Context, req *api.GetUserReq) (*api.GetUserResp, error) {
	var user *User
	var err error

	// Get user by ID or email
	if req.UserId != "" {
		user, err = s.connector.storage.GetUser(ctx, req.UserId)
	} else if req.Email != "" {
		user, err = s.connector.storage.GetUserByEmail(ctx, req.Email)
	} else {
		return nil, fmt.Errorf("either user_id or email must be provided")
	}

	if err != nil {
		return &api.GetUserResp{
			NotFound: true,
		}, nil
	}

	return &api.GetUserResp{
		User:     convertUserToProto(user),
		NotFound: false,
	}, nil
}

// UpdateUser updates an existing user's details.
func (s *GRPCServer) UpdateUser(ctx context.Context, req *api.UpdateUserReq) (*api.UpdateUserResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Get existing user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.UpdateUserResp{
			NotFound: true,
		}, nil
	}

	// Update fields
	if req.Username != "" {
		user.Username = req.Username
	}
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	user.EmailVerified = req.EmailVerified
	user.Require2FA = req.Require_2Fa
	user.UpdatedAt = time.Now()

	// Validate updated user
	if err := user.Validate(); err != nil {
		return nil, fmt.Errorf("invalid user data: %w", err)
	}

	// Save updated user
	if err := s.connector.storage.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	s.connector.logger.Infof("gRPC: Updated user %s", user.ID)

	return &api.UpdateUserResp{
		NotFound: false,
	}, nil
}

// DeleteUser deletes a user account.
func (s *GRPCServer) DeleteUser(ctx context.Context, req *api.DeleteUserReq) (*api.DeleteUserResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Check if user exists
	_, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.DeleteUserResp{
			NotFound: true,
		}, nil
	}

	// Delete user
	if err := s.connector.storage.DeleteUser(ctx, req.UserId); err != nil {
		return nil, fmt.Errorf("failed to delete user: %w", err)
	}

	s.connector.logger.Infof("gRPC: Deleted user %s", req.UserId)

	return &api.DeleteUserResp{
		NotFound: false,
	}, nil
}

// SetPassword sets or updates a user's password.
func (s *GRPCServer) SetPassword(ctx context.Context, req *api.SetPasswordReq) (*api.SetPasswordResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("password is required")
	}

	// Validate password
	if err := ValidatePassword(req.Password); err != nil {
		return nil, fmt.Errorf("invalid password: %w", err)
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.SetPasswordResp{
			NotFound: true,
		}, nil
	}

	// Set password
	if err := s.connector.SetPassword(ctx, user, req.Password); err != nil {
		return nil, fmt.Errorf("failed to set password: %w", err)
	}

	s.connector.logger.Infof("gRPC: Set password for user %s", user.ID)

	return &api.SetPasswordResp{
		NotFound: false,
	}, nil
}

// RemovePassword removes a user's password (for passwordless accounts).
func (s *GRPCServer) RemovePassword(ctx context.Context, req *api.RemovePasswordReq) (*api.RemovePasswordResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.RemovePasswordResp{
			NotFound: true,
		}, nil
	}

	// Remove password
	if err := s.connector.RemovePassword(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to remove password: %w", err)
	}

	s.connector.logger.Infof("gRPC: Removed password for user %s", user.ID)

	return &api.RemovePasswordResp{
		NotFound: false,
	}, nil
}

// EnableTOTP enables TOTP 2FA for a user and returns setup information.
func (s *GRPCServer) EnableTOTP(ctx context.Context, req *api.EnableTOTPReq) (*api.EnableTOTPResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.EnableTOTPResp{
			NotFound: true,
		}, nil
	}

	// Check if TOTP already enabled
	if user.TOTPEnabled {
		return &api.EnableTOTPResp{
			AlreadyEnabled: true,
		}, nil
	}

	// Begin TOTP setup
	result, err := s.connector.BeginTOTPSetup(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to begin TOTP setup: %w", err)
	}

	s.connector.logger.Infof("gRPC: Began TOTP setup for user %s", user.ID)

	return &api.EnableTOTPResp{
		NotFound:       false,
		AlreadyEnabled: false,
		Secret:         result.Secret,
		QrCode:         result.QRCodeDataURL,
		OtpauthUrl:     result.URL,
		BackupCodes:    result.BackupCodes,
	}, nil
}

// VerifyTOTPSetup verifies and completes TOTP setup.
func (s *GRPCServer) VerifyTOTPSetup(ctx context.Context, req *api.VerifyTOTPSetupReq) (*api.VerifyTOTPSetupResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.Secret == "" {
		return nil, fmt.Errorf("secret is required")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.VerifyTOTPSetupResp{
			NotFound: true,
		}, nil
	}

	// Finish TOTP setup
	if err := s.connector.FinishTOTPSetup(ctx, user, req.Secret, req.Code, req.BackupCodes); err != nil {
		return &api.VerifyTOTPSetupResp{
			InvalidCode: true,
		}, nil
	}

	s.connector.logger.Infof("gRPC: Completed TOTP setup for user %s", user.ID)

	return &api.VerifyTOTPSetupResp{
		NotFound:    false,
		InvalidCode: false,
	}, nil
}

// DisableTOTP disables TOTP 2FA for a user.
func (s *GRPCServer) DisableTOTP(ctx context.Context, req *api.DisableTOTPReq) (*api.DisableTOTPResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.DisableTOTPResp{
			NotFound: true,
		}, nil
	}

	// Disable TOTP
	if err := s.connector.DisableTOTP(ctx, user, req.Code); err != nil {
		return &api.DisableTOTPResp{
			InvalidCode: true,
		}, nil
	}

	s.connector.logger.Infof("gRPC: Disabled TOTP for user %s", user.ID)

	return &api.DisableTOTPResp{
		NotFound:    false,
		InvalidCode: false,
	}, nil
}

// GetTOTPInfo retrieves TOTP information for a user.
func (s *GRPCServer) GetTOTPInfo(ctx context.Context, req *api.GetTOTPInfoReq) (*api.GetTOTPInfoResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.GetTOTPInfoResp{
			NotFound: true,
		}, nil
	}

	// Count unused backup codes
	unusedCodes := 0
	for _, code := range user.BackupCodes {
		if !code.Used {
			unusedCodes++
		}
	}

	return &api.GetTOTPInfoResp{
		NotFound: false,
		TotpInfo: &api.TOTPInfo{
			Enabled:               user.TOTPEnabled,
			BackupCodesRemaining: int32(unusedCodes),
		},
	}, nil
}

// RegenerateBackupCodes generates new backup codes for a user.
func (s *GRPCServer) RegenerateBackupCodes(ctx context.Context, req *api.RegenerateBackupCodesReq) (*api.RegenerateBackupCodesResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.RegenerateBackupCodesResp{
			NotFound: true,
		}, nil
	}

	// Regenerate backup codes
	backupCodes, err := s.connector.RegenerateBackupCodes(ctx, user, req.Code)
	if err != nil {
		return &api.RegenerateBackupCodesResp{
			InvalidCode: true,
		}, nil
	}

	s.connector.logger.Infof("gRPC: Regenerated backup codes for user %s", user.ID)

	return &api.RegenerateBackupCodesResp{
		NotFound:    false,
		InvalidCode: false,
		BackupCodes: backupCodes,
	}, nil
}

// ListPasskeys lists all passkeys for a user.
func (s *GRPCServer) ListPasskeys(ctx context.Context, req *api.ListPasskeysReq) (*api.ListPasskeysResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.ListPasskeysResp{
			NotFound: true,
		}, nil
	}

	// Convert passkeys to protobuf
	protoPasskeys := make([]*api.Passkey, len(user.Passkeys))
	for i, passkey := range user.Passkeys {
		protoPasskeys[i] = convertPasskeyToProto(&passkey)
	}

	return &api.ListPasskeysResp{
		NotFound: false,
		Passkeys: protoPasskeys,
	}, nil
}

// RenamePasskey renames a passkey.
func (s *GRPCServer) RenamePasskey(ctx context.Context, req *api.RenamePasskeyReq) (*api.RenamePasskeyResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.PasskeyId == "" {
		return nil, fmt.Errorf("passkey_id is required")
	}
	if req.NewName == "" {
		return nil, fmt.Errorf("new_name is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.RenamePasskeyResp{
			NotFound: true,
		}, nil
	}

	// Find and rename passkey
	found := false
	for i := range user.Passkeys {
		if user.Passkeys[i].ID == req.PasskeyId {
			user.Passkeys[i].Name = req.NewName
			found = true
			break
		}
	}

	if !found {
		return &api.RenamePasskeyResp{
			NotFound: true,
		}, nil
	}

	// Save user
	user.UpdatedAt = time.Now()
	if err := s.connector.storage.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	s.connector.logger.Infof("gRPC: Renamed passkey %s for user %s", req.PasskeyId, user.ID)

	return &api.RenamePasskeyResp{
		NotFound: false,
	}, nil
}

// DeletePasskey deletes a passkey.
func (s *GRPCServer) DeletePasskey(ctx context.Context, req *api.DeletePasskeyReq) (*api.DeletePasskeyResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if req.PasskeyId == "" {
		return nil, fmt.Errorf("passkey_id is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.DeletePasskeyResp{
			NotFound: true,
		}, nil
	}

	// Find and delete passkey
	found := false
	newPasskeys := make([]Passkey, 0, len(user.Passkeys))
	for _, passkey := range user.Passkeys {
		if passkey.ID == req.PasskeyId {
			found = true
			continue
		}
		newPasskeys = append(newPasskeys, passkey)
	}

	if !found {
		return &api.DeletePasskeyResp{
			NotFound: true,
		}, nil
	}

	// Save user with updated passkeys
	user.Passkeys = newPasskeys
	user.UpdatedAt = time.Now()
	if err := s.connector.storage.UpdateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	s.connector.logger.Infof("gRPC: Deleted passkey %s for user %s", req.PasskeyId, user.ID)

	return &api.DeletePasskeyResp{
		NotFound: false,
	}, nil
}

// GetAuthMethods retrieves a user's configured authentication methods.
func (s *GRPCServer) GetAuthMethods(ctx context.Context, req *api.GetAuthMethodsReq) (*api.GetAuthMethodsResp, error) {
	if req.UserId == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	// Get user
	user, err := s.connector.storage.GetUser(ctx, req.UserId)
	if err != nil {
		return &api.GetAuthMethodsResp{
			NotFound: true,
		}, nil
	}

	return &api.GetAuthMethodsResp{
		NotFound:         false,
		HasPassword:      user.PasswordHash != nil,
		PasskeyCount:     int32(len(user.Passkeys)),
		TotpEnabled:      user.TOTPEnabled,
		MagicLinkEnabled: s.connector.config.MagicLink.Enabled,
	}, nil
}

// convertUserToProto converts a User to protobuf EnhancedUser.
func convertUserToProto(user *User) *api.EnhancedUser {
	var lastLoginAt int64
	if user.LastLoginAt != nil {
		lastLoginAt = user.LastLoginAt.Unix()
	}

	return &api.EnhancedUser{
		Id:            user.ID,
		Email:         user.Email,
		Username:      user.Username,
		DisplayName:   user.DisplayName,
		EmailVerified: user.EmailVerified,
		Require_2Fa:   user.Require2FA,
		CreatedAt:     user.CreatedAt.Unix(),
		UpdatedAt:     user.UpdatedAt.Unix(),
		LastLoginAt:   lastLoginAt,
	}
}

// convertPasskeyToProto converts a Passkey to protobuf Passkey.
func convertPasskeyToProto(passkey *Passkey) *api.Passkey {
	return &api.Passkey{
		Id:             passkey.ID,
		UserId:         passkey.UserID,
		Name:           passkey.Name,
		CreatedAt:      passkey.CreatedAt.Unix(),
		LastUsedAt:     passkey.LastUsedAt.Unix(),
		Transports:     passkey.Transports,
		BackupEligible: passkey.BackupEligible,
		BackupState:    passkey.BackupState,
	}
}
