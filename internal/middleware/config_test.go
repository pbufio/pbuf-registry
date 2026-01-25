package middleware

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopUserRepo struct{}

func (n *noopUserRepo) CreateUser(ctx context.Context, user *model.User) error { panic("not used") }
func (n *noopUserRepo) GetUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	panic("not used")
}
func (n *noopUserRepo) GetUserByName(ctx context.Context, name string) (*model.User, error) {
	panic("not used")
}
func (n *noopUserRepo) GetUserByToken(ctx context.Context, token string) (*model.User, error) {
	panic("not used")
}
func (n *noopUserRepo) ListUsers(ctx context.Context, limit int, offset int) ([]*model.User, error) {
	panic("not used")
}
func (n *noopUserRepo) UpdateUser(ctx context.Context, user *model.User) error { panic("not used") }
func (n *noopUserRepo) DeleteUser(ctx context.Context, id uuid.UUID) error     { panic("not used") }
func (n *noopUserRepo) SetUserActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	panic("not used")
}

var _ data.UserRepository = (*noopUserRepo)(nil)

func Test_CreateAuthMiddleware(t *testing.T) {
	t.Run("disabled -> no auth", func(t *testing.T) {
		mw, err := CreateAuthMiddleware(&config.Auth{Enabled: false}, nil, log.DefaultLogger)
		require.NoError(t, err)
		_, ok := mw.(*noAuth)
		assert.True(t, ok)
	})

	t.Run("static-token -> requires SERVER_STATIC_TOKEN", func(t *testing.T) {
		t.Setenv("SERVER_STATIC_TOKEN", "")
		_, err := CreateAuthMiddleware(&config.Auth{Enabled: true, Type: "static-token"}, nil, log.DefaultLogger)
		require.Error(t, err)
	})

	t.Run("static-token -> returns static token auth", func(t *testing.T) {
		t.Setenv("SERVER_STATIC_TOKEN", "secret")
		mw, err := CreateAuthMiddleware(&config.Auth{Enabled: true, Type: "static-token"}, nil, log.DefaultLogger)
		require.NoError(t, err)
		_, ok := mw.(*staticTokenAuth)
		assert.True(t, ok)
	})

	t.Run("acl -> requires user repo", func(t *testing.T) {
		t.Setenv("SERVER_STATIC_TOKEN", "secret")
		_, err := CreateAuthMiddleware(&config.Auth{Enabled: true, Type: "acl"}, nil, log.DefaultLogger)
		require.Error(t, err)
	})

	t.Run("acl -> returns ACL auth", func(t *testing.T) {
		t.Setenv("SERVER_STATIC_TOKEN", "secret")
		mw, err := CreateAuthMiddleware(&config.Auth{Enabled: true, Type: "acl"}, &noopUserRepo{}, log.DefaultLogger)
		require.NoError(t, err)
		_, ok := mw.(*aclAuth)
		assert.True(t, ok)
	})
}
