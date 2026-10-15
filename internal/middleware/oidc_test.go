package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pbufio/pbuf-registry/internal/config"
	"github.com/pbufio/pbuf-registry/internal/data"
	"github.com/pbufio/pbuf-registry/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryUserRepo struct {
	usersByName map[string]*model.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		usersByName: make(map[string]*model.User),
	}
}

func (m *memoryUserRepo) CreateUser(_ context.Context, user *model.User) error {
	user.ID = uuid.New()
	m.usersByName[user.Name] = user
	return nil
}

func (m *memoryUserRepo) GetUser(_ context.Context, _ uuid.UUID) (*model.User, error) {
	return nil, data.ErrUserNotFound
}

func (m *memoryUserRepo) GetUserByName(_ context.Context, name string) (*model.User, error) {
	if u, ok := m.usersByName[name]; ok {
		return u, nil
	}
	return nil, data.ErrUserNotFound
}

func (m *memoryUserRepo) GetUserByToken(_ context.Context, token string) (*model.User, error) {
	for _, u := range m.usersByName {
		if u.Token == token && u.IsActive {
			return u, nil
		}
	}
	return nil, data.ErrUserNotFound
}

func (m *memoryUserRepo) ListUsers(_ context.Context, _, _ int) ([]*model.User, error) {
	return nil, nil
}

func (m *memoryUserRepo) UpdateUser(_ context.Context, _ *model.User) error { return nil }
func (m *memoryUserRepo) DeleteUser(_ context.Context, _ uuid.UUID) error   { return nil }
func (m *memoryUserRepo) SetUserActive(_ context.Context, _ uuid.UUID, _ bool) error {
	return nil
}

type handlerRoundTripper struct {
	handler http.Handler
}

func (h handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func newFakeOIDCHTTPClient(t *testing.T, issuerURL string, privKey *rsa.PrivateKey, kid string) *http.Client {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(oidcDiscoveryDoc{
			Issuer:  issuerURL,
			JWKSURI: issuerURL + "/jwks.json",
		})
	})

	mux.HandleFunc("/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		pub := privKey.PublicKey
		n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
		_ = json.NewEncoder(w).Encode(jwksDocument{
			Keys: []jwkKey{
				{
					Kty: "RSA",
					Kid: kid,
					Use: "sig",
					Alg: "RS256",
					N:   n,
					E:   e,
				},
			},
		})
	})

	return &http.Client{Transport: handlerRoundTripper{handler: mux}}
}

func signTestJWT(t *testing.T, privKey *rsa.PrivateKey, kid string, claims jwtv5.MapClaims) string {
	t.Helper()
	tok := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(privKey)
	require.NoError(t, err)
	return signed
}

func TestOIDCVerifierAndMiddleware(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	const kid = "test-key-1"
	const issuerURL = "https://idp.example.com"
	httpClient := newFakeOIDCHTTPClient(t, issuerURL, privKey, kid)

	oidcCfg := config.OIDCConfig{
		IssuerURL:     issuerURL,
		ClientID:      "pbuf-registry",
		UsernameClaim: "email",
		GroupsClaim:   "groups",
		AdminGroups:   []string{"platform-admins"},
		AutoProvision: true,
	}

	verifier, err := NewOIDCVerifier(oidcCfg)
	require.NoError(t, err)
	verifier.httpClient = httpClient

	repo := newMemoryUserRepo()
	authMW := NewOIDCAuth("static-admin-secret", repo, verifier, true, log.DefaultLogger).NewAuthMiddleware()

	t.Run("valid OIDC JWT auto-provisions user and attaches to context", func(t *testing.T) {
		rawJWT := signTestJWT(t, privKey, kid, jwtv5.MapClaims{
			"iss":    issuerURL,
			"sub":    "user-sub-123",
			"aud":    "pbuf-registry",
			"email":  "alice@example.com",
			"groups": []string{"developers"},
			"exp":    time.Now().Add(10 * time.Minute).Unix(),
		})

		var capturedUser *model.User
		var capturedAdmin bool
		handler := authMW(func(ctx context.Context, _ interface{}) (interface{}, error) {
			capturedUser, _ = GetUserFromContext(ctx)
			capturedAdmin = IsAdmin(ctx)
			return "ok", nil
		})

		sCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer "+rawJWT)},
		)

		resp, err := handler(sCtx, nil)
		require.NoError(t, err)
		assert.Equal(t, "ok", resp)
		require.NotNil(t, capturedUser)
		assert.Equal(t, "alice@example.com", capturedUser.Name)
		assert.False(t, capturedAdmin)
	})

	t.Run("OIDC user in admin group gets admin privileges", func(t *testing.T) {
		rawJWT := signTestJWT(t, privKey, kid, jwtv5.MapClaims{
			"iss":    issuerURL,
			"sub":    "admin-sub-1",
			"aud":    "pbuf-registry",
			"email":  "admin@example.com",
			"groups": []string{"platform-admins"},
			"exp":    time.Now().Add(10 * time.Minute).Unix(),
		})

		var capturedAdmin bool
		handler := authMW(func(ctx context.Context, _ interface{}) (interface{}, error) {
			capturedAdmin = IsAdmin(ctx)
			return "ok", nil
		})

		sCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer "+rawJWT)},
		)

		_, err := handler(sCtx, nil)
		require.NoError(t, err)
		assert.True(t, capturedAdmin)
	})

	t.Run("OIDC middleware still accepts bot tokens alongside JWTs", func(t *testing.T) {
		bot := &model.User{
			Name:     "ci-bot",
			Token:    "pbuf_bot_secret123",
			Type:     model.UserTypeBot,
			IsActive: true,
		}
		_ = repo.CreateUser(context.Background(), bot)

		var capturedUser *model.User
		handler := authMW(func(ctx context.Context, _ interface{}) (interface{}, error) {
			capturedUser, _ = GetUserFromContext(ctx)
			return "ok", nil
		})

		sCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer pbuf_bot_secret123")},
		)

		_, err := handler(sCtx, nil)
		require.NoError(t, err)
		require.NotNil(t, capturedUser)
		assert.Equal(t, "ci-bot", capturedUser.Name)
	})

	t.Run("wrong audience is rejected", func(t *testing.T) {
		rawJWT := signTestJWT(t, privKey, kid, jwtv5.MapClaims{
			"iss":   issuerURL,
			"sub":   "user-sub-123",
			"aud":   "other-client",
			"email": "alice@example.com",
			"exp":   time.Now().Add(10 * time.Minute).Unix(),
		})

		handler := authMW(func(_ context.Context, _ interface{}) (interface{}, error) {
			return "ok", nil
		})

		sCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer "+rawJWT)},
		)

		_, err := handler(sCtx, nil)
		require.Error(t, err)
	})
}
