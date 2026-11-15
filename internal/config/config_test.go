package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoaderMustLoad(t *testing.T) {
	t.Setenv("SERVER_GRPC_AUTH_ENABLED", "true")
	t.Setenv("SERVER_GRPC_AUTH_TYPE", "oidc")
	t.Setenv("SERVER_GRPC_AUTH_OIDC_ISSUERURL", "https://idp.example.com")
	t.Setenv("SERVER_GRPC_AUTH_OIDC_CLIENTID", "pbuf-registry")
	t.Setenv("DATA_DATABASE_DSN", "postgres://test:test@localhost:5432/test?sslmode=disable")
	t.Setenv("DAEMONS_PROTOPARSING_WEBHOOK_URL", "https://hooks.slack.com/services/T000/B000/XXX")
	t.Setenv("DAEMONS_PROTOPARSING_WEBHOOK_MIN_SEVERITY", "CRITICAL")
	t.Setenv("DAEMONS_PROTOPARSING_WEBHOOK_FORMAT", "slack")
	t.Setenv("DAEMONS_PROTOPARSING_WEBHOOK_TIMEOUT", "15s")

	loader := NewLoader()
	loader.MustLoad()

	assert.True(t, Cfg.Server.GRPC.Auth.Enabled)
	assert.Equal(t, "oidc", Cfg.Server.GRPC.Auth.Type)
	assert.Equal(t, "https://idp.example.com", Cfg.Server.GRPC.Auth.OIDC.IssuerURL)
	assert.Equal(t, "pbuf-registry", Cfg.Server.GRPC.Auth.OIDC.ClientID)
	assert.Equal(t, "postgres://test:test@localhost:5432/test?sslmode=disable", Cfg.Data.Database.DSN)
	assert.Equal(t, "https://hooks.slack.com/services/T000/B000/XXX", Cfg.Daemons.ProtoParsing.Webhook.URL)
	assert.Equal(t, "CRITICAL", Cfg.Daemons.ProtoParsing.Webhook.MinSeverity)
	assert.Equal(t, "slack", Cfg.Daemons.ProtoParsing.Webhook.Format)
	assert.Equal(t, 15*time.Second, Cfg.Daemons.ProtoParsing.Webhook.Timeout)
}
