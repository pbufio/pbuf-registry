package model

import (
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
