package middleware

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/auth/jwt"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/pbufio/pbuf-registry/migrations"
	"github.com/pbufio/pbuf-registry/test_utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var integrationSuite IntegrationTestSuite

type IntegrationTestSuite struct {
	psqlContainer  *test_utils.PostgreSQLContainer
	userRepository data.UserRepository
	aclRepository  data.ACLRepository
	pool           *pgxpool.Pool
}

func (s *IntegrationTestSuite) SetupSuite() {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer ctxCancel()

	psqlContainer, err := test_utils.NewPostgreSQLContainer(ctx)
	if err != nil {
		panic(err)
	}

	s.psqlContainer = psqlContainer

	// Wait for container to be ready
	time.Sleep(5 * time.Second)

	dsn := s.psqlContainer.GetDSN()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		panic(err)
	}
	s.pool = pool

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		panic(err)
	}
	migrations.Migrate(db)

	s.userRepository = data.NewUserRepository(pool, log.DefaultLogger)
	s.aclRepository = data.NewACLRepository(pool, log.DefaultLogger)
}

func (s *IntegrationTestSuite) TearDownSuite() {
	ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer ctxCancel()

	if s.pool != nil {
		s.pool.Close()
	}

	err := s.psqlContainer.Terminate(ctx)
	if err != nil {
		panic(err)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	integrationSuite.SetupSuite()
	code := m.Run()
	integrationSuite.TearDownSuite()
	os.Exit(code)
}

func createTestUser(t *testing.T, name, token string) *model.User {
	user := &model.User{
		Name:     name,
		Token:    token,
		Type:     model.UserTypeUser,
		IsActive: true,
	}
	err := integrationSuite.userRepository.CreateUser(context.Background(), user)
	require.NoError(t, err)
	return user
}

func Test_aclAuth_AdminToken(t *testing.T) {
	adminToken := "admin-secret-token"
	auth := NewACLAuth(adminToken, integrationSuite.userRepository, log.DefaultLogger)
	middleware := auth.NewAuthMiddleware()

	tests := []struct {
		name      string
		token     string
		wantErr   bool
		wantAdmin bool
	}{
		{
			name:      "Valid admin token",
			token:     adminToken,
			wantErr:   false,
			wantAdmin: true,
		},
		{
			name:      "Valid admin token with Bearer prefix",
			token:     "Bearer " + adminToken,
			wantErr:   false,
			wantAdmin: true,
		},
		{
			name:    "Invalid admin token",
			token:   "wrong-admin-token",
			wantErr: true,
		},
		{
			name:    "Empty token",
			token:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
				if tt.wantAdmin {
					assert.True(t, IsAdmin(ctx))
				}
				return "success", nil
			})

			serverContext := transport.NewServerContext(
				context.Background(),
				&Transport{reqHeader: newTokenHeader(authorizationKey, tt.token)},
			)

			response, err := handler(serverContext, nil)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, jwt.ErrTokenInvalid, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "success", response)
		})
	}
}

func Test_aclAuth_UserToken(t *testing.T) {
	adminToken := "admin-secret-token-2"
	plainToken := "pbuf_user_integration-test-token"
	
	// Create a test user
	testUser := createTestUser(t, "auth-integration-user", plainToken)

	auth := NewACLAuth(adminToken, integrationSuite.userRepository, log.DefaultLogger)
	middleware := auth.NewAuthMiddleware()

	tests := []struct {
		name      string
		token     string
		wantErr   bool
		wantAdmin bool
		wantUser  bool
	}{
		{
			name:      "Valid user token (pgcrypto verification)",
			token:     plainToken,
			wantErr:   false,
			wantAdmin: false,
			wantUser:  true,
		},
		{
			name:      "Valid user token with Bearer prefix",
			token:     "Bearer " + plainToken,
			wantErr:   false,
			wantAdmin: false,
			wantUser:  true,
		},
		{
			name:    "Invalid user token",
			token:   "pbuf_user_invalid-token",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
				assert.Equal(t, tt.wantAdmin, IsAdmin(ctx))
				
				if tt.wantUser {
					user, ok := GetUserFromContext(ctx)
					assert.True(t, ok)
					assert.Equal(t, testUser.ID, user.ID)
					assert.Equal(t, testUser.Name, user.Name)
				}
				return "success", nil
			})

			serverContext := transport.NewServerContext(
				context.Background(),
				&Transport{reqHeader: newTokenHeader(authorizationKey, tt.token)},
			)

			response, err := handler(serverContext, nil)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "success", response)
		})
	}
}

func Test_aclAuth_InactiveUser(t *testing.T) {
	adminToken := "admin-secret-token-3"
	plainToken := "pbuf_user_inactive-test-token"
	
	// Create an inactive test user
	user := &model.User{
		Name:     "auth-inactive-user",
		Token:    plainToken,
		Type:     model.UserTypeUser,
		IsActive: false,
	}
	err := integrationSuite.userRepository.CreateUser(context.Background(), user)
	require.NoError(t, err)

	auth := NewACLAuth(adminToken, integrationSuite.userRepository, log.DefaultLogger)
	middleware := auth.NewAuthMiddleware()

	t.Run("Inactive user token should be rejected", func(t *testing.T) {
		handler := middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			t.Error("Handler should not be called for inactive user")
			return "success", nil
		})

		serverContext := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, plainToken)},
		)

		_, err := handler(serverContext, nil)
		assert.Error(t, err)
		assert.Equal(t, jwt.ErrTokenInvalid, err)
	})
}

func Test_aclAuth_BotToken(t *testing.T) {
	adminToken := "admin-secret-token-4"
	plainToken := "pbuf_bot_integration-test-token"
	
	// Create a test bot
	bot := &model.User{
		Name:     "auth-integration-bot",
		Token:    plainToken,
		Type:     model.UserTypeBot,
		IsActive: true,
	}
	err := integrationSuite.userRepository.CreateUser(context.Background(), bot)
	require.NoError(t, err)

	auth := NewACLAuth(adminToken, integrationSuite.userRepository, log.DefaultLogger)
	middleware := auth.NewAuthMiddleware()

	t.Run("Valid bot token", func(t *testing.T) {
		handler := middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			assert.False(t, IsAdmin(ctx))
			
			user, ok := GetUserFromContext(ctx)
			assert.True(t, ok)
			assert.Equal(t, bot.ID, user.ID)
			assert.Equal(t, model.UserTypeBot, user.Type)
			return "success", nil
		})

		serverContext := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, plainToken)},
		)

		response, err := handler(serverContext, nil)
		require.NoError(t, err)
		assert.Equal(t, "success", response)
	})
}

func Test_aclAuth_ContextHelpers(t *testing.T) {
	t.Run("GetUserFromContext with no user", func(t *testing.T) {
		ctx := context.Background()
		user, ok := GetUserFromContext(ctx)
		assert.False(t, ok)
		assert.Nil(t, user)
	})

	t.Run("IsAdmin with no admin flag", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, IsAdmin(ctx))
	})

	t.Run("IsAdmin with false value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), isAdminKey, false)
		assert.False(t, IsAdmin(ctx))
	})

	t.Run("IsAdmin with true value", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), isAdminKey, true)
		assert.True(t, IsAdmin(ctx))
	})
}

func Test_aclAuth_NoServerContext(t *testing.T) {
	adminToken := "admin-secret-token-5"
	
	auth := NewACLAuth(adminToken, integrationSuite.userRepository, log.DefaultLogger)
	middleware := auth.NewAuthMiddleware()

	t.Run("No server context should return error", func(t *testing.T) {
		handler := middleware(func(ctx context.Context, req interface{}) (interface{}, error) {
			t.Error("Handler should not be called without server context")
			return "success", nil
		})

		// Call without server context
		_, err := handler(context.Background(), nil)
		assert.Error(t, err)
		assert.Equal(t, jwt.ErrWrongContext, err)
	})
}
