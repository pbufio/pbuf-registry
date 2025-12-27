package middleware

import (
	"context"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
)

const (
	authorizationKey = "Authorization"
	userContextKey   = "user"
	isAdminKey       = "is_admin"
)

type AuthMiddleware interface {
	NewAuthMiddleware() middleware.Middleware
}

type noAuth struct {
}

// NewNoAuth returns a no auth middleware
// It doesn't require any token
func NewNoAuth() AuthMiddleware {
	return &noAuth{}
}

func (n noAuth) NewAuthMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			return handler(ctx, req)
		}
	}
}

type staticTokenAuth struct {
	token string
}

// NewStaticTokenAuth returns a simple token auth middleware
// We pass the static token and use it to auth
func NewStaticTokenAuth(token string) AuthMiddleware {
	return &staticTokenAuth{
		token: token,
	}
}

func (s staticTokenAuth) NewAuthMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if serverContext, ok := transport.FromServerContext(ctx); ok {
				if serverContext.RequestHeader().Get(authorizationKey) != s.token {
					return nil, jwt.ErrTokenInvalid
				}
				return handler(ctx, req)
			}
			return nil, jwt.ErrWrongContext
		}
	}
}

type aclAuth struct {
	adminToken string
	userRepo   data.UserRepository
	logger     *log.Helper
}

// NewACLAuth returns an ACL-aware auth middleware
// Supports both admin static token and user/bot tokens from database
func NewACLAuth(adminToken string, userRepo data.UserRepository, logger log.Logger) AuthMiddleware {
	return &aclAuth{
		adminToken: adminToken,
		userRepo:   userRepo,
		logger:     log.NewHelper(log.With(logger, "module", "middleware/ACLAuth")),
	}
}

func (a *aclAuth) NewAuthMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			serverContext, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, jwt.ErrWrongContext
			}

			authHeader := serverContext.RequestHeader().Get(authorizationKey)
			if authHeader == "" {
				return nil, jwt.ErrTokenInvalid
			}

			// Extract token from "Bearer <token>" format
			token := authHeader
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}

 		// Check if it's the admin token
 		if token == a.adminToken {
 			// Admin has full access
 			ctx = context.WithValue(ctx, isAdminKey, true)
 			return handler(ctx, req)
 		}

 		// Check if it's a user/bot token (will be verified using pgcrypto)
 		user, err := a.userRepo.GetUserByToken(ctx, token)
 		if err != nil {
 			if err == data.ErrUserNotFound {
 				return nil, jwt.ErrTokenInvalid
 			}
 			a.logger.Errorf("failed to get user by token: %v", err)
 			return nil, jwt.ErrTokenInvalid
 		}

			// User found and active, add to context
			ctx = context.WithValue(ctx, userContextKey, user)
			ctx = context.WithValue(ctx, isAdminKey, false)
			return handler(ctx, req)
		}
	}
}

// GetUserFromContext extracts the user from context
func GetUserFromContext(ctx context.Context) (*model.User, bool) {
	user, ok := ctx.Value(userContextKey).(*model.User)
	return user, ok
}

// IsAdmin checks if the current request is from admin
func IsAdmin(ctx context.Context) bool {
	isAdmin, ok := ctx.Value(isAdminKey).(bool)
	return ok && isAdmin
}
