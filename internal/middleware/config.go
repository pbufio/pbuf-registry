package middleware

import (
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/pbufio/pbuf-registry/internal/config"
)

func CreateAuthMiddleware(cfg *config.Auth, logger log.Logger) (AuthMiddleware, error) {
	helper := log.NewHelper(logger)

	if !cfg.Enabled {
		helper.Infof("Auth is disabled. NoAuth middleware is used.")
		return NewNoAuth(), nil
	}

	switch cfg.Type {
	case "static-token":
		staticToken := os.Getenv("SERVER_STATIC_TOKEN")
		if staticToken == "" {
			panic("SERVER_STATIC_TOKEN is not set")
		}
		return NewStaticTokenAuth(staticToken), nil
	}

	return nil, fmt.Errorf("unknown auth type: %s", cfg.Type)
}
