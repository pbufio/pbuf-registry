# Plan: Adding Basic ACL and User/Bot Management to pbuf-registry

Let me first explore the existing project structure to understand the current authentication and data layer implementation.Based on the project structure I explored, here's a comprehensive plan for adding basic ACL (Access Control List) and user/bot management to the pbuf-registry project, using the existing static token as the admin token.

---

# Plan: Basic ACL and User/Bot Management

## Overview

Add a role-based access control system where:
- **Admin** (static token holder): Full access including user/bot management
- **Users/Bots**: Token-based access with configurable permissions per module

---

## Phase 1: Database Schema

### 1.1 Create Migration for Users/Bots Table
**File:** `migrations/YYYYMMDDHHMMSS_users_bots.sql`

```sql
-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL UNIQUE,
    token VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('user', 'bot')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_users_token ON users(token);
CREATE INDEX idx_users_type ON users(type);

-- +goose Down
DROP TABLE IF EXISTS users;
```


### 1.2 Create Migration for ACL Table
**File:** `migrations/YYYYMMDDHHMMSS_acl.sql`

```sql
-- +goose Up
CREATE TABLE acl (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    module_name VARCHAR(255) NOT NULL, -- '*' for all modules
    permission VARCHAR(50) NOT NULL CHECK (permission IN ('read', 'write', 'admin')),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(user_id, module_name)
);

CREATE INDEX idx_acl_user_id ON acl(user_id);
CREATE INDEX idx_acl_module_name ON acl(module_name);

-- +goose Down
DROP TABLE IF EXISTS acl;
```


---

## Phase 2: Domain Models

### 2.1 Add User/Bot Model
**File:** `internal/model/user.go`

```textmate
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
```


---

## Phase 3: Data Layer

### 3.1 Add User Repository
**File:** `internal/data/users.go`

Implement repository interface with methods:
- `CreateUser(ctx, name, token, userType) (*User, error)`
- `GetUserByID(ctx, id) (*User, error)`
- `GetUserByToken(ctx, token) (*User, error)`
- `ListUsers(ctx, userType, pagination) ([]*User, error)`
- `UpdateUser(ctx, user) error`
- `DeleteUser(ctx, id) error`
- `DeactivateUser(ctx, id) error`

### 3.2 Add ACL Repository
**File:** `internal/data/acl.go`

Implement repository interface with methods:
- `GrantPermission(ctx, userID, moduleName, permission) error`
- `RevokePermission(ctx, userID, moduleName) error`
- `GetUserPermissions(ctx, userID) ([]*ACLEntry, error)`
- `CheckPermission(ctx, userID, moduleName, requiredPermission) (bool, error)`
- `GetModulePermissions(ctx, moduleName) ([]*ACLEntry, error)`

---

## Phase 4: API Definition (Protobuf)

### 4.1 Add User Management Proto
**File:** `api/pbuf-registry/v1/users.proto`

```protobuf
syntax = "proto3";

package pbuf.registry.v1;

service UserService {
    // Admin-only endpoints
    rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
    rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
    rpc GetUser(GetUserRequest) returns (User);
    rpc UpdateUser(UpdateUserRequest) returns (User);
    rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
    rpc RegenerateToken(RegenerateTokenRequest) returns (RegenerateTokenResponse);
    
    // ACL management
    rpc GrantPermission(GrantPermissionRequest) returns (GrantPermissionResponse);
    rpc RevokePermission(RevokePermissionRequest) returns (RevokePermissionResponse);
    rpc ListUserPermissions(ListUserPermissionsRequest) returns (ListUserPermissionsResponse);
}

message User {
    string id = 1;
    string name = 2;
    string type = 3; // "user" or "bot"
    bool is_active = 4;
    string created_at = 5;
    string updated_at = 6;
}

message CreateUserRequest {
    string name = 1;
    string type = 2;
}

message CreateUserResponse {
    User user = 1;
    string token = 2; // Only returned on creation
}

// ... other request/response messages
```


---

## Phase 5: Enhanced Authentication Middleware

### 5.1 Update Auth Middleware
**File:** `internal/middleware/auth.go`

Update the existing `staticTokenAuth` to support multiple authentication strategies:

```textmate
type AuthResult struct {
    IsAdmin     bool
    UserID      *uuid.UUID
    Permissions []Permission
}

type Authenticator interface {
    Authenticate(ctx context.Context, token string) (*AuthResult, error)
}

type combinedAuth struct {
    adminToken   string
    userRepo     data.UserRepository
    aclRepo      data.ACLRepository
}

func (a *combinedAuth) Authenticate(ctx context.Context, token string) (*AuthResult, error) {
    // 1. Check if it's admin token
    if subtle.ConstantTimeCompare([]byte(token), []byte(a.adminToken)) == 1 {
        return &AuthResult{IsAdmin: true}, nil
    }
    
    // 2. Look up user by token (pgcrypto handles encryption verification in DB)
    user, err := a.userRepo.GetUserByToken(ctx, token)
    if err != nil {
        return nil, ErrUnauthorized
    }
    
    if !user.IsActive {
        return nil, ErrUserDeactivated
    }
    
    // 3. Load user permissions
    permissions, err := a.aclRepo.GetUserPermissions(ctx, user.ID)
    // ...
    
    return &AuthResult{
        IsAdmin:     false,
        UserID:      &user.ID,
        Permissions: permissions,
    }, nil
}
```


### 5.2 Add Authorization Interceptor
**File:** `internal/middleware/authorization.go`

```textmate
// Per-method permission requirements
var methodPermissions = map[string]Permission{
    "/pbuf.registry.v1.Registry/GetModule":      PermissionRead,
    "/pbuf.registry.v1.Registry/RegisterModule": PermissionWrite,
    "/pbuf.registry.v1.Registry/DeleteModule":   PermissionAdmin,
    "/pbuf.registry.v1.UserService/*":           PermissionAdmin, // Admin only
}

func AuthorizationInterceptor(authResult *AuthResult) grpc.UnaryServerInterceptor {
    // Check if user has required permission for the method
    // Admin token bypasses all checks
}
```


---

## Phase 6: Server Integration

### 6.1 Add User Service Implementation
**File:** `internal/server/users.go`

Implement the `UserService` gRPC service with admin-only access check.

### 6.2 Update Server Configuration
**File:** `internal/config/config.go`

No changes needed - existing `SERVER_STATIC_TOKEN` becomes the admin token.

---

## Phase 7: Security Considerations

### 7.1 Token Generation & Storage
- Generate secure random tokens (32+ bytes, base64 encoded)
- Store tokens encrypted using PostgreSQL pgcrypto extension (bcrypt)
- Token verification is done in the database using crypt() function

### 7.2 Token Format
```
pbuf_<type>_<random_32_bytes_base64>
Example: pbuf_bot_a1b2c3d4e5f6g7h8i9j0...
```


---

## Implementation Order

| Step | Task | Estimated Effort |
|------|------|------------------|
| 1 | Create database migrations | 1 hour |
| 2 | Implement domain models | 1 hour |
| 3 | Implement data repositories | 3 hours |
| 4 | Define protobuf API | 1 hour |
| 5 | Generate Go code from proto | 30 min |
| 6 | Update authentication middleware | 2 hours |
| 7 | Add authorization interceptor | 2 hours |
| 8 | Implement UserService | 3 hours |
| 9 | Write tests | 4 hours |
| 10 | Update documentation | 1 hour |

**Total: ~18-20 hours**

---

## Configuration Summary

| Variable | Purpose | Current Usage |
|----------|---------|---------------|
| `SERVER_STATIC_TOKEN` | Admin token (full access) | Already exists |
| New user tokens | Generated per user/bot | Stored encrypted in DB using pgcrypto |

---

## Usage Example

```shell script
# Admin creates a bot with read access to specific module
grpcurl -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"name": "ci-bot", "type": "bot"}' \
  localhost:6777 pbuf.registry.v1.UserService/CreateUser

# Returns: {"user": {...}, "token": "pbuf_bot_xxx..."}

# Grant permission
grpcurl -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"user_id": "...", "module_name": "my-org/my-module", "permission": "read"}' \
  localhost:6777 pbuf.registry.v1.UserService/GrantPermission

# Bot can now access that specific module
grpcurl -H "Authorization: Bearer pbuf_bot_xxx..." \
  localhost:6777 pbuf.registry.v1.Registry/GetModule
```


---

This plan provides a foundation that can be extended later with features like:
- Token expiration
- Audit logging
- Team/organization-based permissions
- OAuth2/OIDC integration