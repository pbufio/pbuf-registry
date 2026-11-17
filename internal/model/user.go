package model

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type UserType string

const (
	UserTypeUser UserType = "user"
	UserTypeBot  UserType = "bot"
)

type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
	PermissionAdmin Permission = "admin"
)

type User struct {
	ID        uuid.UUID
	Name      string
	Token     string
	Type      UserType
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ACLEntry struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	ModuleName string // "*" for all modules
	Permission Permission
	CreatedAt  time.Time
}

type userContextKeyType struct{}
type isAdminKeyType struct{}

var (
	userContextKey = userContextKeyType{}
	isAdminKey     = isAdminKeyType{}
)

// ContextWithUser stores an authenticated user in context.
func ContextWithUser(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// ContextWithAdmin marks whether the request in context has global admin privileges.
func ContextWithAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, isAdminKey, isAdmin)
}

// GetUserFromContext extracts the user from context.
func GetUserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok && user != nil
}

// IsAdmin checks if the current request is from admin.
func IsAdmin(ctx context.Context) bool {
	isAdmin, ok := ctx.Value(isAdminKey).(bool)
	return ok && isAdmin
}
