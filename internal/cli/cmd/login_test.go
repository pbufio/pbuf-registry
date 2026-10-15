package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
