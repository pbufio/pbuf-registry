package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	stdErrors "errors"
	"fmt"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var (
	ErrInvalidRequest = kerrors.BadRequest("INVALID_REQUEST", "invalid request")
	ErrUserNotFound   = kerrors.NotFound("USER_NOT_FOUND", "user not found")
	ErrUnauthorized   = kerrors.Unauthorized("UNAUTHORIZED", "unauthorized")
)

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

type UsersServer struct {
	v1.UnimplementedUserServiceServer

	userRepo data.UserRepository
	aclRepo  data.ACLRepository
	logger   *log.Helper
}

func NewUsersServer(userRepo data.UserRepository, aclRepo data.ACLRepository, logger log.Logger) *UsersServer {
	return &UsersServer{
		userRepo: userRepo,
		aclRepo:  aclRepo,
		logger:   log.NewHelper(log.With(logger, "module", "server/UsersServer")),
	}
}

func (s *UsersServer) CreateUser(ctx context.Context, request *v1.CreateUserRequest) (*v1.CreateUserResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	userType, err := userTypeFromV1(request.Type)
	if err != nil {
		return nil, err
	}
	if request.Name == "" {
		return nil, ErrInvalidRequest
	}

	token, err := generateToken(userType)
	if err != nil {
		s.logger.Errorf("failed to generate token: %v", err)
		return nil, kerrors.InternalServer("TOKEN_GENERATION_FAILED", "failed to generate token")
	}

	user := &model.User{
		Name:     request.Name,
		Token:    token,
		Type:     userType,
		IsActive: true,
	}

	err = s.userRepo.CreateUser(ctx, user)
	if err != nil {
		s.logger.Errorf("failed to create user: %v", err)
		return nil, kerrors.InternalServer("USER_CREATION_FAILED", "failed to create user")
	}

	s.logger.Infof("created user %s (type: %s) with id %s", request.Name, userType, user.ID)

	return &v1.CreateUserResponse{
		User:  toV1User(user),
		Token: token,
	}, nil
}

func (s *UsersServer) ListUsers(ctx context.Context, request *v1.ListUsersRequest) (*v1.ListUsersResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	pageSize := int(request.PageSize)
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	page := int(request.Page)
	if page < 0 {
		page = 0
	}
	offset := page * pageSize

	users, err := s.userRepo.ListUsers(ctx, pageSize, offset)
	if err != nil {
		s.logger.Errorf("failed to list users: %v", err)
		return nil, kerrors.InternalServer("USER_LIST_FAILED", "failed to list users")
	}

	resp := &v1.ListUsersResponse{
		Users: make([]*v1.User, 0, len(users)),
		Total: int32(len(users)),
	}
	for _, u := range users {
		resp.Users = append(resp.Users, toV1User(u))
	}
	return resp, nil
}

func (s *UsersServer) GetUser(ctx context.Context, request *v1.GetUserRequest) (*v1.User, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	id, err := parseUUID(request.Id)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUser(ctx, id)
	if err != nil {
		if stdErrors.Is(err, data.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return nil, kerrors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}

	return toV1User(user), nil
}

func (s *UsersServer) UpdateUser(ctx context.Context, request *v1.UpdateUserRequest) (*v1.User, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	id, err := parseUUID(request.Id)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUser(ctx, id)
	if err != nil {
		if stdErrors.Is(err, data.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return nil, kerrors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}

	if request.Name != "" {
		user.Name = request.Name
	}
	user.IsActive = request.IsActive

	err = s.userRepo.UpdateUser(ctx, user)
	if err != nil {
		s.logger.Errorf("failed to update user: %v", err)
		return nil, kerrors.InternalServer("USER_UPDATE_FAILED", "failed to update user")
	}

	s.logger.Infof("updated user %s", user.ID)

	return toV1User(user), nil
}

func (s *UsersServer) DeleteUser(ctx context.Context, request *v1.DeleteUserRequest) (*v1.DeleteUserResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	id, err := parseUUID(request.Id)
	if err != nil {
		return nil, err
	}

	err = s.aclRepo.DeleteUserPermissions(ctx, id)
	if err != nil {
		s.logger.Errorf("failed to delete user permissions: %v", err)
		return nil, kerrors.InternalServer("PERMISSION_DELETE_FAILED", "failed to delete user permissions")
	}

	err = s.userRepo.DeleteUser(ctx, id)
	if err != nil {
		if stdErrors.Is(err, data.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to delete user: %v", err)
		return nil, kerrors.InternalServer("USER_DELETE_FAILED", "failed to delete user")
	}

	s.logger.Infof("deleted user %s", id)

	return &v1.DeleteUserResponse{Id: request.Id}, nil
}

func (s *UsersServer) RegenerateToken(ctx context.Context, request *v1.RegenerateTokenRequest) (*v1.RegenerateTokenResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	id, err := parseUUID(request.Id)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetUser(ctx, id)
	if err != nil {
		if stdErrors.Is(err, data.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return nil, kerrors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}

	token, err := generateToken(user.Type)
	if err != nil {
		s.logger.Errorf("failed to generate token: %v", err)
		return nil, kerrors.InternalServer("TOKEN_GENERATION_FAILED", "failed to generate token")
	}

	user.Token = token
	err = s.userRepo.UpdateUser(ctx, user)
	if err != nil {
		s.logger.Errorf("failed to update user token: %v", err)
		return nil, kerrors.InternalServer("USER_UPDATE_FAILED", "failed to update user token")
	}

	s.logger.Infof("regenerated token for user %s", user.ID)

	return &v1.RegenerateTokenResponse{Token: token}, nil
}

func (s *UsersServer) GrantPermission(ctx context.Context, request *v1.GrantPermissionRequest) (*v1.GrantPermissionResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	userID, err := parseUUID(request.UserId)
	if err != nil {
		return nil, err
	}

	permission, err := permissionFromV1(request.Permission)
	if err != nil {
		return nil, err
	}

	_, err = s.userRepo.GetUser(ctx, userID)
	if err != nil {
		if stdErrors.Is(err, data.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("failed to get user: %v", err)
		return nil, kerrors.InternalServer("USER_FETCH_FAILED", "failed to get user")
	}

	entry := &model.ACLEntry{
		UserID:     userID,
		ModuleName: request.ModuleName,
		Permission: permission,
	}

	err = s.aclRepo.GrantPermission(ctx, entry)
	if err != nil {
		s.logger.Errorf("failed to grant permission: %v", err)
		return nil, kerrors.InternalServer("PERMISSION_GRANT_FAILED", "failed to grant permission")
	}

	s.logger.Infof("granted %s permission on module %s to user %s", permission, request.ModuleName, userID)

	return &v1.GrantPermissionResponse{Entry: toV1ACLEntry(entry)}, nil
}

func (s *UsersServer) RevokePermission(ctx context.Context, request *v1.RevokePermissionRequest) (*v1.RevokePermissionResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	userID, err := parseUUID(request.UserId)
	if err != nil {
		return nil, err
	}

	err = s.aclRepo.RevokePermission(ctx, userID, request.ModuleName)
	if err != nil {
		if stdErrors.Is(err, data.ErrPermissionNotFound) {
			return nil, kerrors.NotFound("PERMISSION_NOT_FOUND", "permission not found")
		}
		s.logger.Errorf("failed to revoke permission: %v", err)
		return nil, kerrors.InternalServer("PERMISSION_REVOKE_FAILED", "failed to revoke permission")
	}

	s.logger.Infof("revoked permission on module %s from user %s", request.ModuleName, userID)

	return &v1.RevokePermissionResponse{Success: true}, nil
}

func (s *UsersServer) ListUserPermissions(ctx context.Context, request *v1.ListUserPermissionsRequest) (*v1.ListUserPermissionsResponse, error) {
	if request == nil {
		return nil, ErrInvalidRequest
	}

	userID, err := parseUUID(request.UserId)
	if err != nil {
		return nil, err
	}

	entries, err := s.aclRepo.ListUserPermissions(ctx, userID)
	if err != nil {
		s.logger.Errorf("failed to list user permissions: %v", err)
		return nil, kerrors.InternalServer("PERMISSION_LIST_FAILED", "failed to list user permissions")
	}

	resp := &v1.ListUserPermissionsResponse{Permissions: make([]*v1.ACLEntry, 0, len(entries))}
	for _, e := range entries {
		resp.Permissions = append(resp.Permissions, toV1ACLEntry(e))
	}
	return resp, nil
}

func parseUUID(id string) (uuid.UUID, error) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, ErrInvalidRequest
	}
	return parsed, nil
}

func userTypeFromV1(t v1.UserType) (model.UserType, error) {
	switch t {
	case v1.UserType_USER_TYPE_USER:
		return model.UserTypeUser, nil
	case v1.UserType_USER_TYPE_BOT:
		return model.UserTypeBot, nil
	default:
		return "", ErrInvalidRequest
	}
}

func userTypeToV1(t model.UserType) v1.UserType {
	switch t {
	case model.UserTypeUser:
		return v1.UserType_USER_TYPE_USER
	case model.UserTypeBot:
		return v1.UserType_USER_TYPE_BOT
	default:
		return v1.UserType_USER_TYPE_UNSPECIFIED
	}
}

func permissionFromV1(p v1.Permission) (model.Permission, error) {
	switch p {
	case v1.Permission_PERMISSION_READ:
		return model.PermissionRead, nil
	case v1.Permission_PERMISSION_WRITE:
		return model.PermissionWrite, nil
	case v1.Permission_PERMISSION_ADMIN:
		return model.PermissionAdmin, nil
	default:
		return "", ErrInvalidRequest
	}
}

func permissionToV1(p model.Permission) v1.Permission {
	switch p {
	case model.PermissionRead:
		return v1.Permission_PERMISSION_READ
	case model.PermissionWrite:
		return v1.Permission_PERMISSION_WRITE
	case model.PermissionAdmin:
		return v1.Permission_PERMISSION_ADMIN
	default:
		return v1.Permission_PERMISSION_UNSPECIFIED
	}
}

func toTimestamp(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func toV1User(user *model.User) *v1.User {
	if user == nil {
		return nil
	}
	return &v1.User{
		Id:        user.ID.String(),
		Name:      user.Name,
		Type:      userTypeToV1(user.Type),
		IsActive:  user.IsActive,
		CreatedAt: toTimestamp(user.CreatedAt),
		UpdatedAt: toTimestamp(user.UpdatedAt),
	}
}

func toV1ACLEntry(entry *model.ACLEntry) *v1.ACLEntry {
	if entry == nil {
		return nil
	}
	return &v1.ACLEntry{
		Id:         entry.ID.String(),
		UserId:     entry.UserID.String(),
		ModuleName: entry.ModuleName,
		Permission: permissionToV1(entry.Permission),
		CreatedAt:  toTimestamp(entry.CreatedAt),
	}
}
