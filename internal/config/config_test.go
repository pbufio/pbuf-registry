package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoaderMustLoad(t *testing.T) {
	t.Setenv("SERVER_GRPC_AUTH_ENABLED", "true")
	t.Setenv("SERVER_GRPC_AUTH_TYPE", "oidc")
	t.Setenv("SERVER_GRPC_AUTH_OIDC_ISSUERURL", "https://idp.example.com")
	t.Setenv("SERVER_GRPC_AUTH_OIDC_CLIENTID", "pbuf-registry")
	t.Setenv("DATA_DATABASE_DSN", "postgres://test:test@localhost:5432/test?sslmode=disable")

	loader := NewLoader()
	loader.MustLoad()

	assert.True(t, Cfg.Server.GRPC.Auth.Enabled)
	assert.Equal(t, "oidc", Cfg.Server.GRPC.Auth.Type)
	assert.Equal(t, "https://idp.example.com", Cfg.Server.GRPC.Auth.OIDC.IssuerURL)
	assert.Equal(t, "pbuf-registry", Cfg.Server.GRPC.Auth.OIDC.ClientID)
	assert.Equal(t, "postgres://test:test@localhost:5432/test?sslmode=disable", Cfg.Data.Database.DSN)
}
