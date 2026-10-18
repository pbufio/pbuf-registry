package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
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
)

type userContextKeyType struct{}

type isAdminKeyType struct{}

var (
	userContextKey = userContextKeyType{}
	isAdminKey     = isAdminKeyType{}
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
				authHeader := serverContext.RequestHeader().Get(authorizationKey)
				token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
				if token == "" || token != s.token {
					return nil, jwt.ErrTokenInvalid
				}
				return handler(ctx, req)
			}
			return nil, jwt.ErrWrongContext
		}
	}
}

type aclAuth struct {
	adminToken    string
	userRepo      data.UserRepository
	oidcVerifier  *OIDCVerifier
	autoProvision bool
	logger        *log.Helper
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

// NewOIDCAuth returns an OIDC + ACL-aware auth middleware.
// It validates OIDC JWT tokens (resolving/provisioning users for ACL checks) while
// also supporting static admin tokens and user/bot tokens for CI pipelines.
func NewOIDCAuth(adminToken string, userRepo data.UserRepository, verifier *OIDCVerifier, autoProvision bool, logger log.Logger) AuthMiddleware {
	return &aclAuth{
		adminToken:    adminToken,
		userRepo:      userRepo,
		oidcVerifier:  verifier,
		autoProvision: autoProvision,
		logger:        log.NewHelper(log.With(logger, "module", "middleware/OIDCAuth")),
	}
}

func (a *aclAuth) NewAuthMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if IsAdmin(ctx) {
				return handler(ctx, req)
			}
			if _, ok := GetUserFromContext(ctx); ok {
				return handler(ctx, req)
			}

			serverContext, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, jwt.ErrWrongContext
			}

			authHeader := serverContext.RequestHeader().Get(authorizationKey)
			if authHeader == "" {
				return nil, jwt.ErrTokenInvalid
			}

			// Extract token from "Bearer <token>" format
			token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			if token == "" {
				return nil, jwt.ErrTokenInvalid
			}

			// Check if it's the admin token
			if a.adminToken != "" && token == a.adminToken {
				// Admin has full access
				ctx = context.WithValue(ctx, isAdminKey, true)
				return handler(ctx, req)
			}

			// If OIDC verifier is configured and token is a JWT, verify via OIDC
			if a.oidcVerifier != nil && isLikelyJWT(token) {
				return a.authenticateOIDC(ctx, req, handler, token)
			}

			if a.userRepo == nil {
				return nil, jwt.ErrTokenInvalid
			}

			// Check if it's a user/bot token (verified using pgcrypto)
			user, err := a.userRepo.GetUserByToken(ctx, token)
			if err != nil {
				if errors.Is(err, data.ErrUserNotFound) {
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

func (a *aclAuth) authenticateOIDC(ctx context.Context, req interface{}, handler middleware.Handler, token string) (interface{}, error) {
	claims, err := a.oidcVerifier.VerifyToken(ctx, token)
	if err != nil {
		a.logger.Warnf("oidc token verification failed: %v", err)
		return nil, jwt.ErrTokenInvalid
	}

	if claims.IsAdmin {
		ctx = context.WithValue(ctx, isAdminKey, true)
		return handler(ctx, req)
	}

	if a.userRepo == nil {
		return nil, jwt.ErrTokenInvalid
	}

	user, err := a.userRepo.GetUserByName(ctx, claims.Username)
	if err != nil {
		if !errors.Is(err, data.ErrUserNotFound) {
			a.logger.Errorf("failed to get oidc user %s: %v", claims.Username, err)
			return nil, jwt.ErrTokenInvalid
		}
		if !a.autoProvision {
			return nil, jwt.ErrTokenInvalid
		}

		// Auto-provision active user on first OIDC login with an unguessable random token
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			a.logger.Errorf("failed to generate random token for oidc user %s: %v", claims.Username, err)
			return nil, jwt.ErrTokenInvalid
		}
		user = &model.User{
			Name:     claims.Username,
			Token:    "pbuf_user_" + base64.RawURLEncoding.EncodeToString(randomBytes),
			Type:     model.UserTypeUser,
			IsActive: true,
		}
		if createErr := a.userRepo.CreateUser(ctx, user); createErr != nil {
			// Handle concurrent creation race by re-fetching
			user, err = a.userRepo.GetUserByName(ctx, claims.Username)
			if err != nil {
				a.logger.Errorf("failed to auto-provision oidc user %s: %v", claims.Username, createErr)
				return nil, jwt.ErrTokenInvalid
			}
		}
	}

	if !user.IsActive || user.Type != model.UserTypeUser {
		return nil, jwt.ErrTokenInvalid
	}

	ctx = context.WithValue(ctx, userContextKey, user)
	ctx = context.WithValue(ctx, isAdminKey, false)
	return handler(ctx, req)
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
