package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdx/go-netrc"
	"github.com/pbufio/pbuf-registry/internal/cli/model"
	"github.com/spf13/cobra"
)

type oidcProviderMetadata struct {
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

type deviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type deviceTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
}

// NewLoginCmd creates the `pbuf login` command supporting OIDC OAuth2 Device Authorization Flow.
func NewLoginCmd(config *model.Config, usr *user.User, auth *netrc.Netrc) *cobra.Command {
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with the registry via OIDC SSO",
		Long:  "Login initiates the OAuth 2.0 Device Authorization Flow against an OIDC identity provider and stores the token in ~/.netrc",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			issuer, _ := cmd.Flags().GetString("issuer")
			clientID, _ := cmd.Flags().GetString("client-id")
			scope, _ := cmd.Flags().GetString("scope")

			if issuer == "" {
				issuer = os.Getenv("PBUF_OIDC_ISSUER")
			}
			if issuer == "" && config != nil {
				issuer = config.Registry.OIDC.Issuer
			}
			if clientID == "" {
				clientID = os.Getenv("PBUF_OIDC_CLIENT_ID")
			}
			if clientID == "" && config != nil {
				clientID = config.Registry.OIDC.ClientID
			}

			if strings.TrimSpace(issuer) == "" || strings.TrimSpace(clientID) == "" {
				return errors.New("OIDC --issuer and --client-id are required (or set PBUF_OIDC_ISSUER / PBUF_OIDC_CLIENT_ID or registry.oidc in pbuf.yaml)")
			}

			token, err := runDeviceCodeFlow(cmd.Context(), issuer, clientID, scope, cmd.OutOrStdout())
			if err != nil {
				return err
			}

			if err := saveTokenToNetrc(config, usr, auth, token); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Successfully logged in via OIDC and saved token to ~/.netrc")
			return nil
		},
	}

	loginCmd.Flags().String("issuer", "", "OIDC issuer URL")
	loginCmd.Flags().String("client-id", "", "OIDC client ID")
	loginCmd.Flags().String("scope", "openid profile email", "OIDC scopes to request")

	return loginCmd
}

var oidcHTTPClient = &http.Client{Timeout: 15 * time.Second}

func runDeviceCodeFlow(ctx context.Context, issuer, clientID, scope string, out io.Writer) (string, error) {
	httpClient := oidcHTTPClient

	discoveryURL := strings.TrimSuffix(strings.TrimSpace(issuer), "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch OIDC discovery metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OIDC discovery returned status %d", resp.StatusCode)
	}

	var meta oidcProviderMetadata
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return "", fmt.Errorf("failed to decode OIDC discovery metadata: %w", err)
	}
	if meta.DeviceAuthorizationEndpoint == "" || meta.TokenEndpoint == "" {
		return "", errors.New("OIDC provider does not advertise device_authorization_endpoint or token_endpoint")
	}

	form := url.Values{
		"client_id": {clientID},
		"scope":     {scope},
	}
	devResp, err := httpClient.PostForm(meta.DeviceAuthorizationEndpoint, form)
	if err != nil {
		return "", fmt.Errorf("failed to start device authorization: %w", err)
	}
	defer func() { _ = devResp.Body.Close() }()

	if devResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(devResp.Body)
		return "", fmt.Errorf("device authorization failed (status %d): %s", devResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var devAuth deviceAuthResponse
	if err := json.NewDecoder(devResp.Body).Decode(&devAuth); err != nil {
		return "", fmt.Errorf("failed to decode device authorization response: %w", err)
	}

	if devAuth.VerificationURIComplete != "" {
		_, _ = fmt.Fprintf(out, "Open the following URL in your browser to authenticate:\n  %s\n", devAuth.VerificationURIComplete)
	} else {
		_, _ = fmt.Fprintf(out, "Open %s in your browser and enter code: %s\n", devAuth.VerificationURI, devAuth.UserCode)
	}

	interval := time.Duration(devAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 2 * time.Second
	}
	deadline := time.Now().Add(time.Duration(devAuth.ExpiresIn) * time.Second)
	if devAuth.ExpiresIn <= 0 {
		deadline = time.Now().Add(5 * time.Minute)
	}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		tokForm := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {devAuth.DeviceCode},
			"client_id":   {clientID},
		}
		tokHTTPResp, err := httpClient.PostForm(meta.TokenEndpoint, tokForm)
		if err != nil {
			continue
		}

		var tokResp deviceTokenResponse
		_ = json.NewDecoder(tokHTTPResp.Body).Decode(&tokResp)
		_ = tokHTTPResp.Body.Close()

		switch tokResp.Error {
		case "":
			if tokResp.IDToken != "" {
				return tokResp.IDToken, nil
			}
			if tokResp.AccessToken != "" {
				return tokResp.AccessToken, nil
			}
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 2 * time.Second
			continue
		default:
			return "", fmt.Errorf("OIDC token exchange failed: %s", tokResp.Error)
		}
	}

	return "", errors.New("timed out waiting for OIDC device authorization")
}

func saveTokenToNetrc(config *model.Config, usr *user.User, auth *netrc.Netrc, token string) error {
	if auth == nil {
		if usr == nil {
			var err error
			usr, err = user.Current()
			if err != nil {
				return fmt.Errorf("failed to resolve current user: %w", err)
			}
		}
		auth = &netrc.Netrc{
			Path: filepath.Join(usr.HomeDir, ".netrc"),
		}
	}

	machine := "localhost"
	if config != nil && config.Registry.Addr != "" {
		machine = config.Registry.Addr
		if host, _, err := net.SplitHostPort(machine); err == nil {
			machine = host
		}
	}

	m := auth.Machine(machine)
	if m == nil {
		auth.AddMachine(machine, "unused", "unused")
		m = auth.Machine(machine)
	}
	m.Set("token", token)

	if err := auth.Save(); err != nil {
		log.Printf("failed to save .netrc file: %v", err)
		return err
	}
	return nil
}
