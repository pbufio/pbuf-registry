package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
)

var (
	ErrInvalidRequest = errors.BadRequest("INVALID_REQUEST", "invalid request")
	ErrUserNotFound   = errors.NotFound("USER_NOT_FOUND", "user not found")
	ErrUnauthorized   = errors.Unauthorized("UNAUTHORIZED", "unauthorized")
)

type UserService struct {
	userRepo data.UserRepository
	aclRepo  data.ACLRepository
	logger   *log.Helper
}

func NewUserService(userRepo data.UserRepository, aclRepo data.ACLRepository, logger log.Logger) *UserService {
	return &UserService{
		userRepo: userRepo,
		aclRepo:  aclRepo,
		logger:   log.NewHelper(log.With(logger, "module", "server/UserService")),
	}
}

// CreateUser creates a new user or bot and generates a token
func (s *UserService) CreateUser(ctx context.Context, name string, userType model.UserType) (*model.User, string, error) {
	if name == "" {
		return nil, "", ErrInvalidRequest
	}

	// Generate a secure token
	token, err := generateToken(userType)
	if err != nil {
		s.logger.Errorf("failed to generate token: %v", err)
		return nil, "", errors.InternalServer("TOKEN_GENERATION_FAILED", "failed to generate token")
	}

	// Create user (token will be encrypted by database using pgcrypto)
	user := &model.User{
		Name:     name,
		Token:    token,
		Type:     userType,
		IsActive: true,
	}

	err = s.userRepo.CreateUser(ctx, user)
	if err != nil {
		s.logger.Errorf("failed to create user: %v", err)
		return nil, "", errors.InternalServer("USER_CREATION_FAILED", "failed to create user")
	}

	s.logger.Infof("created user %s (type: %s) with id %s", name, userType, user.ID)
	return user, token, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user, err := s.userRepo.GetUser(ctx, id)
	if err != nil {
		if err == data.ErrUserNotFound {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return nil, errors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}
	return user, nil
}

// ListUsers retrieves all users with pagination
func (s *UserService) ListUsers(ctx context.Context, pageSize, page int) ([]*model.User, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	if page < 0 {
		page = 0
	}

	offset := page * pageSize
	users, err := s.userRepo.ListUsers(ctx, pageSize, offset)
	if err != nil {
		s.logger.Errorf("failed to list users: %v", err)
		return nil, errors.InternalServer("USER_LIST_FAILED", "failed to list users")
	}
	return users, nil
}

// UpdateUser updates a user's information
func (s *UserService) UpdateUser(ctx context.Context, id uuid.UUID, name string, isActive bool) (*model.User, error) {
	user, err := s.userRepo.GetUser(ctx, id)
	if err != nil {
		if err == data.ErrUserNotFound {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return nil, errors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}

	// Update fields if provided
	if name != "" {
		user.Name = name
	}
	user.IsActive = isActive

	err = s.userRepo.UpdateUser(ctx, user)
	if err != nil {
		s.logger.Errorf("failed to update user: %v", err)
		return nil, errors.InternalServer("USER_UPDATE_FAILED", "failed to update user")
	}

	s.logger.Infof("updated user %s", user.ID)
	return user, nil
}

// DeleteUser deletes a user and all their permissions
func (s *UserService) DeleteUser(ctx context.Context, id uuid.UUID) error {
	// Delete user permissions first
	err := s.aclRepo.DeleteUserPermissions(ctx, id)
	if err != nil {
		s.logger.Errorf("failed to delete user permissions: %v", err)
		return errors.InternalServer("PERMISSION_DELETE_FAILED", "failed to delete user permissions")
	}

	// Delete user
	err = s.userRepo.DeleteUser(ctx, id)
	if err != nil {
		if err == data.ErrUserNotFound {
			return ErrUserNotFound
		}
		s.logger.Errorf("failed to delete user: %v", err)
		return errors.InternalServer("USER_DELETE_FAILED", "failed to delete user")
	}

	s.logger.Infof("deleted user %s", id)
	return nil
}

// RegenerateToken generates a new token for a user
func (s *UserService) RegenerateToken(ctx context.Context, id uuid.UUID) (string, error) {
	user, err := s.userRepo.GetUser(ctx, id)
	if err != nil {
		if err == data.ErrUserNotFound {
			return "", ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return "", errors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}

	// Generate new token
	token, err := generateToken(user.Type)
	if err != nil {
		s.logger.Errorf("failed to generate token: %v", err)
		return "", errors.InternalServer("TOKEN_GENERATION_FAILED", "failed to generate token")
	}

	// Update token (will be encrypted by database using pgcrypto)
	user.Token = token
	err = s.userRepo.UpdateUser(ctx, user)
	if err != nil {
		s.logger.Errorf("failed to update user token: %v", err)
		return "", errors.InternalServer("USER_UPDATE_FAILED", "failed to update user token")
	}

	s.logger.Infof("regenerated token for user %s", user.ID)
	return token, nil
}

// GrantPermission grants a permission to a user
func (s *UserService) GrantPermission(ctx context.Context, userID uuid.UUID, moduleName string, permission model.Permission) (*model.ACLEntry, error) {
	// Verify user exists
	_, err := s.userRepo.GetUser(ctx, userID)
	if err != nil {
		if err == data.ErrUserNotFound {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return nil, errors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}

	// Create ACL entry
	entry := &model.ACLEntry{
		UserID:     userID,
		ModuleName: moduleName,
		Permission: permission,
	}

	err = s.aclRepo.GrantPermission(ctx, entry)
	if err != nil {
		s.logger.Errorf("failed to grant permission: %v", err)
		return nil, errors.InternalServer("PERMISSION_GRANT_FAILED", "failed to grant permission")
	}

	s.logger.Infof("granted %s permission on module %s to user %s", permission, moduleName, userID)
	return entry, nil
}

// RevokePermission revokes a permission from a user
func (s *UserService) RevokePermission(ctx context.Context, userID uuid.UUID, moduleName string) error {
	err := s.aclRepo.RevokePermission(ctx, userID, moduleName)
	if err != nil {
		if err == data.ErrPermissionNotFound {
			return errors.NotFound("PERMISSION_NOT_FOUND", "permission not found")
		}
		s.logger.Errorf("failed to revoke permission: %v", err)
		return errors.InternalServer("PERMISSION_REVOKE_FAILED", "failed to revoke permission")
	}

	s.logger.Infof("revoked permission on module %s from user %s", moduleName, userID)
	return nil
}

// ListUserPermissions lists all permissions for a user
func (s *UserService) ListUserPermissions(ctx context.Context, userID uuid.UUID) ([]*model.ACLEntry, error) {
	entries, err := s.aclRepo.ListUserPermissions(ctx, userID)
	if err != nil {
		s.logger.Errorf("failed to list user permissions: %v", err)
		return nil, errors.InternalServer("PERMISSION_LIST_FAILED", "failed to list user permissions")
	}
	return entries, nil
}

// generateToken generates a secure random token with type prefix
func generateToken(userType model.UserType) (string, error) {
	// Generate 32 random bytes
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", err
	}

	// Encode to base64
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)

	// Format: pbuf_<type>_<random>
	return fmt.Sprintf("pbuf_%s_%s", userType, encoded), nil
}
