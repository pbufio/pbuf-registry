package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/user"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/jdx/go-netrc"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type handlerRoundTripper struct {
	handler http.Handler
}

func (h handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	h.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

func TestRunDeviceCodeFlow(t *testing.T) {
	var pollCount int32
	const srvURL = "https://idp.example.com"

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(oidcProviderMetadata{
			DeviceAuthorizationEndpoint: srvURL + "/device/code",
			TokenEndpoint:               srvURL + "/oauth/token",
		})
	})

	mux.HandleFunc("/device/code", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(deviceAuthResponse{
			DeviceCode:              "dev-code-123",
			UserCode:                "ABCD-EFGH",
			VerificationURI:         srvURL + "/activate",
			VerificationURIComplete: srvURL + "/activate?user_code=ABCD-EFGH",
			ExpiresIn:               30,
			Interval:                1,
		})
	})

	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
		attempt := atomic.AddInt32(&pollCount, 1)
		if attempt == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(deviceTokenResponse{Error: "authorization_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(deviceTokenResponse{
			IDToken:     "eyJhbGciOiJSUzI1NiJ9.test.signature",
			AccessToken: "access-token-123",
			TokenType:   "Bearer",
		})
	})

	origClient := oidcHTTPClient
	oidcHTTPClient = &http.Client{Transport: handlerRoundTripper{handler: mux}}
	defer func() { oidcHTTPClient = origClient }()

	var out bytes.Buffer
	token, err := runDeviceCodeFlow(context.Background(), srvURL, "pbuf-cli", "openid email", &out)
	require.NoError(t, err)
	assert.Equal(t, "eyJhbGciOiJSUzI1NiJ9.test.signature", token)
	assert.Contains(t, out.String(), "ABCD-EFGH")
}

func TestRunDeviceCodeFlow_RejectsSchemeDowngradeAndNon400TokenErrors(t *testing.T) {
	t.Run("rejects http endpoint when issuer uses https", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(oidcProviderMetadata{
				DeviceAuthorizationEndpoint: "http://idp.example.com/device/code",
				TokenEndpoint:               "https://idp.example.com/oauth/token",
			})
		})
		origClient := oidcHTTPClient
		oidcHTTPClient = &http.Client{Transport: handlerRoundTripper{handler: mux}}
		defer func() { oidcHTTPClient = origClient }()

		var out bytes.Buffer
		_, err := runDeviceCodeFlow(context.Background(), "https://idp.example.com", "pbuf-cli", "openid", &out)
		require.ErrorContains(t, err, "must use https when issuer uses https")
	})

	t.Run("fails fast on 401 unauthorized from token endpoint", func(t *testing.T) {
		const srvURL = "https://idp.example.com"
		var tokenCalls int32
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(oidcProviderMetadata{
				DeviceAuthorizationEndpoint: srvURL + "/device/code",
				TokenEndpoint:               srvURL + "/oauth/token",
			})
		})
		mux.HandleFunc("/device/code", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(deviceAuthResponse{
				DeviceCode:      "dev-code-123",
				UserCode:        "ABCD-EFGH",
				VerificationURI: srvURL + "/activate",
				ExpiresIn:       30,
				Interval:        1,
			})
		})
		mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&tokenCalls, 1)
			http.Error(w, "invalid client credentials", http.StatusUnauthorized)
		})

		origClient := oidcHTTPClient
		oidcHTTPClient = &http.Client{Transport: handlerRoundTripper{handler: mux}}
		defer func() { oidcHTTPClient = origClient }()

		var out bytes.Buffer
		_, err := runDeviceCodeFlow(context.Background(), srvURL, "pbuf-cli", "openid", &out)
		require.ErrorContains(t, err, "status 401")
		assert.EqualValues(t, 1, atomic.LoadInt32(&tokenCalls))
	})

	t.Run("rejects https-to-http redirect downgrade on token endpoint", func(t *testing.T) {
		const srvURL = "https://idp.example.com"
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(oidcProviderMetadata{
				DeviceAuthorizationEndpoint: srvURL + "/device/code",
				TokenEndpoint:               srvURL + "/oauth/token",
			})
		})
		mux.HandleFunc("/device/code", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(deviceAuthResponse{
				DeviceCode:      "dev-code-123",
				UserCode:        "ABCD-EFGH",
				VerificationURI: srvURL + "/activate",
				ExpiresIn:       30,
				Interval:        1,
			})
		})
		mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://idp.example.com/oauth/token", http.StatusTemporaryRedirect)
		})

		origClient := oidcHTTPClient
		oidcHTTPClient = &http.Client{Transport: handlerRoundTripper{handler: mux}}
		defer func() { oidcHTTPClient = origClient }()

		var out bytes.Buffer
		_, err := runDeviceCodeFlow(context.Background(), srvURL, "pbuf-cli", "openid", &out)
		require.ErrorContains(t, err, "must use https when issuer uses https")
	})

	t.Run("retries transient 503 and fails on empty 200 token response", func(t *testing.T) {
		const srvURL = "https://idp.example.com"
		var calls int32
		mux := http.NewServeMux()
		mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(oidcProviderMetadata{
				DeviceAuthorizationEndpoint: srvURL + "/device/code",
				TokenEndpoint:               srvURL + "/oauth/token",
			})
		})
		mux.HandleFunc("/device/code", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(deviceAuthResponse{
				DeviceCode:      "dev-code-123",
				UserCode:        "ABCD-EFGH",
				VerificationURI: srvURL + "/activate",
				ExpiresIn:       30,
				Interval:        1,
			})
		})
		mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, _ *http.Request) {
			if atomic.AddInt32(&calls, 1) == 1 {
				http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		})

		origClient := oidcHTTPClient
		oidcHTTPClient = &http.Client{Transport: handlerRoundTripper{handler: mux}}
		defer func() { oidcHTTPClient = origClient }()

		var out bytes.Buffer
		_, err := runDeviceCodeFlow(context.Background(), srvURL, "pbuf-cli", "openid", &out)
		require.ErrorContains(t, err, "did not contain id_token or access_token")
		assert.EqualValues(t, 2, atomic.LoadInt32(&calls))
	})
}

func TestSaveTokenToNetrcPreservesExistingMachinesWhenLazy(t *testing.T) {
	homeDir := t.TempDir()
	netrcPath := filepath.Join(homeDir, ".netrc")

	existing := &netrc.Netrc{Path: netrcPath}
	existing.AddMachine("github.com", "git", "gh-token")
	require.NoError(t, existing.Save())

	cfg := &model.Config{Registry: model.Registry{Addr: "pbuf.cloud:6777"}}
	require.NoError(t, saveTokenToNetrc(cfg, &user.User{HomeDir: homeDir}, nil, "pbuf-token"))

	parsed, err := netrc.Parse(netrcPath)
	require.NoError(t, err)
	require.NotNil(t, parsed.Machine("github.com"))
	assert.Equal(t, "gh-token", parsed.Machine("github.com").Get("password"))
	require.NotNil(t, parsed.Machine("pbuf.cloud"))
	assert.Equal(t, "pbuf-token", parsed.Machine("pbuf.cloud").Get("token"))
}
