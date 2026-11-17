package middleware

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
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

func (m *memoryUserRepo) ListUsers(_ context.Context, _, _ int) ([]*model.User, int, error) {
	return nil, 0, nil
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
	verifier.httpClient.Transport = httpClient.Transport

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
		assert.Contains(t, capturedUser.Token, "pbuf_user_")
		assert.GreaterOrEqual(t, len(capturedUser.Token), 50)
		assert.False(t, capturedAdmin)

		// Predictable forged token oidc:<sub>:<email> must be rejected
		forgedCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer oidc:user-sub-123:alice@example.com")},
		)
		_, err = handler(forgedCtx, nil)
		require.Error(t, err)
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

		// OIDC JWT claiming the name of a bot account must be rejected
		botJWT := signTestJWT(t, privKey, kid, jwtv5.MapClaims{
			"iss":   issuerURL,
			"sub":   "bot-impersonator",
			"aud":   "pbuf-registry",
			"email": "ci-bot",
			"exp":   time.Now().Add(10 * time.Minute).Unix(),
		})
		impersonateCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer "+botJWT)},
		)
		_, err = handler(impersonateCtx, nil)
		require.Error(t, err)
	})

	t.Run("rejects fallback display name impersonation and unverified emails", func(t *testing.T) {
		handler := authMW(func(_ context.Context, _ interface{}) (interface{}, error) {
			return "ok", nil
		})

		// Omitting email while setting name="alice@example.com" must NOT impersonate Alice
		nameFallbackJWT := signTestJWT(t, privKey, kid, jwtv5.MapClaims{
			"iss":                issuerURL,
			"sub":                "attacker-sub",
			"aud":                "pbuf-registry",
			"name":               "alice@example.com",
			"preferred_username": "alice@example.com",
			"exp":                time.Now().Add(10 * time.Minute).Unix(),
		})
		sCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer "+nameFallbackJWT)},
		)
		_, err := handler(sCtx, nil)
		require.Error(t, err)

		// Unverified email (email_verified=false) must be rejected
		unverifiedJWT := signTestJWT(t, privKey, kid, jwtv5.MapClaims{
			"iss":            issuerURL,
			"sub":            "attacker-sub",
			"aud":            "pbuf-registry",
			"email":          "alice@example.com",
			"email_verified": false,
			"exp":            time.Now().Add(10 * time.Minute).Unix(),
		})
		unverifiedCtx := transport.NewServerContext(
			context.Background(),
			&Transport{reqHeader: newTokenHeader(authorizationKey, "Bearer "+unverifiedJWT)},
		)
		_, err = handler(unverifiedCtx, nil)
		require.Error(t, err)
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

	t.Run("missing expiration is rejected", func(t *testing.T) {
		rawJWT := signTestJWT(t, privKey, kid, jwtv5.MapClaims{
			"iss":   issuerURL,
			"sub":   "user-sub-123",
			"aud":   "pbuf-registry",
			"email": "alice@example.com",
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

func TestParseJWKECDSAValidation(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	rawPub, err := priv.PublicKey.Bytes()
	require.NoError(t, err)

	validKey := jwkKey{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString(rawPub[1:33]),
		Y:   base64.RawURLEncoding.EncodeToString(rawPub[33:65]),
	}
	pub, err := parseJWK(validKey)
	require.NoError(t, err)
	assert.IsType(t, &ecdsa.PublicKey{}, pub)

	// Off-curve point (1, 1) must be rejected by ParseUncompressedPublicKey.
	offCurve := jwkKey{
		Kty: "EC",
		Crv: "P-256",
		X:   base64.RawURLEncoding.EncodeToString([]byte{1}),
		Y:   base64.RawURLEncoding.EncodeToString([]byte{1}),
	}
	_, err = parseJWK(offCurve)
	require.Error(t, err)
}

func TestOIDCVerifier_BoundsAndSchemeDowngradeProtection(t *testing.T) {
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	_, err = NewOIDCVerifier(config.OIDCConfig{
		IssuerURL: "https://idp.example.com",
		ClientID:  "pbuf",
		JWKSURL:   "http://insecure.example.com/jwks.json",
	})
	require.ErrorContains(t, err, "must use https scheme")

	// Oversized discovery document (> 1 MiB) is truncated by LimitReader and rejected cleanly
	oversizedMux := http.NewServeMux()
	oversizedMux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"issuer":"https://idp.example.com","pad":"` + strings.Repeat("a", maxOIDCResponseBytes+1024) + `","jwks_uri":"https://idp.example.com/jwks.json"}`))
	})
	verifier, err := NewOIDCVerifier(config.OIDCConfig{
		IssuerURL: "  https://idp.example.com\n",
		ClientID:  "  pbuf\n",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://idp.example.com", verifier.cfg.IssuerURL)
	assert.Equal(t, "pbuf", verifier.cfg.ClientID)
	verifier.httpClient.Transport = handlerRoundTripper{handler: oversizedMux}

	rawJWT := signTestJWT(t, privKey, "k1", jwtv5.MapClaims{
		"iss": "https://idp.example.com",
		"sub": "u1",
		"aud": "pbuf",
		"exp": time.Now().Add(5 * time.Minute).Unix(),
	})
	_, err = verifier.VerifyToken(context.Background(), rawJWT)
	require.ErrorContains(t, err, "failed to decode OIDC discovery document")

	// Redirect from https:// issuer to http:// endpoint must be rejected by CheckRedirect
	downgradeMux := http.NewServeMux()
	downgradeMux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://insecure.example.com/openid", http.StatusFound)
	})
	downgradeVerifier, err := NewOIDCVerifier(config.OIDCConfig{
		IssuerURL: "https://idp.example.com",
		ClientID:  "pbuf",
	})
	require.NoError(t, err)
	downgradeVerifier.httpClient.Transport = handlerRoundTripper{handler: downgradeMux}
	_, err = downgradeVerifier.VerifyToken(context.Background(), rawJWT)
	require.ErrorContains(t, err, "refusing OIDC redirect from https to non-https")
}
