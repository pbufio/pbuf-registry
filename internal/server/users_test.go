package server

import (
	"context"
	"testing"

	v1 "github.com/pbufio/pbuf-registry/gen/pbuf-registry/v1"
)

func TestUsersServer_GetUser_InvalidUUID(t *testing.T) {
	s := &UsersServer{}
	_, err := s.GetUser(context.Background(), &v1.GetUserRequest{Id: "not-a-uuid"})
	if err != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestUsersServer_CreateUser_InvalidType(t *testing.T) {
	s := &UsersServer{}
	_, err := s.CreateUser(context.Background(), &v1.CreateUserRequest{Name: "n", Type: v1.UserType_USER_TYPE_UNSPECIFIED})
	if err != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestUsersServer_GrantPermission_InvalidPermission(t *testing.T) {
	s := &UsersServer{}
	_, err := s.GrantPermission(context.Background(), &v1.GrantPermissionRequest{UserId: "not-a-uuid", Permission: v1.Permission_PERMISSION_UNSPECIFIED})
	if err != ErrInvalidRequest {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}
