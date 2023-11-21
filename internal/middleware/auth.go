package middleware

import (
	"context"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/transport"
)

const (
	authorizationKey = "Authorization"
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
