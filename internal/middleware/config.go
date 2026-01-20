package middleware

import (
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/data"
)

func CreateAuthMiddleware(cfg *config.Auth, userRepo data.UserRepository, logger log.Logger) (AuthMiddleware, error) {
	helper := log.NewHelper(logger)

	if !cfg.Enabled {
		helper.Infof("Auth is disabled. NoAuth middleware is used.")
		return NewNoAuth(), nil
	}

	switch cfg.Type {
	case "static-token":
		staticToken := os.Getenv("SERVER_STATIC_TOKEN")
		if staticToken == "" {
			return nil, fmt.Errorf("SERVER_STATIC_TOKEN is not set")
		}
		return NewStaticTokenAuth(staticToken), nil
	case "acl":
		if userRepo == nil {
			return nil, fmt.Errorf("user repository is required for auth type: %s", cfg.Type)
		}
		adminToken := os.Getenv("SERVER_STATIC_TOKEN")
		if adminToken == "" {
			return nil, fmt.Errorf("SERVER_STATIC_TOKEN is not set")
		}
		return NewACLAuth(adminToken, userRepo, logger), nil
	}

	return nil, fmt.Errorf("unknown auth type: %s", cfg.Type)
}
