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

const maxOIDCResponseBytes = 1 << 20 // 1 MiB

func validateOIDCEndpoint(rawEndpoint, issuerScheme string) error {
	u, err := url.Parse(strings.TrimSpace(rawEndpoint))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return fmt.Errorf("invalid OIDC endpoint URL %q", rawEndpoint)
	}
	if issuerScheme == "https" && u.Scheme != "https" {
		return fmt.Errorf("OIDC endpoint %q must use https when issuer uses https", rawEndpoint)
	}
	return nil
}

func postFormWithContext(ctx context.Context, client *http.Client, endpoint string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.Do(req)
}

var errOIDCRedirectBlocked = errors.New("OIDC redirect blocked")

func runDeviceCodeFlow(ctx context.Context, issuer, clientID, scope string, out io.Writer) (string, error) {
	trimmedIssuer := strings.TrimSuffix(strings.TrimSpace(issuer), "/")
	issuerURL, err := url.Parse(trimmedIssuer)
	if err != nil || issuerURL.Host == "" || (issuerURL.Scheme != "https" && issuerURL.Scheme != "http") {
		return "", fmt.Errorf("invalid OIDC issuer URL %q", issuer)
	}

	clientCopy := *oidcHTTPClient
	if clientCopy.CheckRedirect == nil {
		clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("%w: stopped after 10 redirects", errOIDCRedirectBlocked)
			}
			if err := validateOIDCEndpoint(req.URL.String(), issuerURL.Scheme); err != nil {
				return fmt.Errorf("%w: %w", errOIDCRedirectBlocked, err)
			}
			return nil
		}
	}
	httpClient := &clientCopy

	discoveryURL := trimmedIssuer + "/.well-known/openid-configuration"
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxOIDCResponseBytes)).Decode(&meta); err != nil {
		return "", fmt.Errorf("failed to decode OIDC discovery metadata: %w", err)
	}
	if meta.DeviceAuthorizationEndpoint == "" || meta.TokenEndpoint == "" {
		return "", errors.New("OIDC provider does not advertise device_authorization_endpoint or token_endpoint")
	}
	if err := validateOIDCEndpoint(meta.DeviceAuthorizationEndpoint, issuerURL.Scheme); err != nil {
		return "", err
	}
	if err := validateOIDCEndpoint(meta.TokenEndpoint, issuerURL.Scheme); err != nil {
		return "", err
	}

	form := url.Values{
		"client_id": {clientID},
		"scope":     {scope},
	}
	devResp, err := postFormWithContext(ctx, httpClient, meta.DeviceAuthorizationEndpoint, form)
	if err != nil {
		return "", fmt.Errorf("failed to start device authorization: %w", err)
	}
	defer func() { _ = devResp.Body.Close() }()

	if devResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(devResp.Body, maxOIDCResponseBytes))
		return "", fmt.Errorf("device authorization failed (status %d): %s", devResp.StatusCode, strings.TrimSpace(string(body)))
	}

	var devAuth deviceAuthResponse
	if err := json.NewDecoder(io.LimitReader(devResp.Body, maxOIDCResponseBytes)).Decode(&devAuth); err != nil {
		return "", fmt.Errorf("failed to decode device authorization response: %w", err)
	}

	if devAuth.VerificationURIComplete != "" {
		_, _ = fmt.Fprintf(out, "Open the following URL in your browser to authenticate:\n  %s\n", devAuth.VerificationURIComplete)
	} else {
		_, _ = fmt.Fprintf(out, "Open %s in your browser and enter code: %s\n", devAuth.VerificationURI, devAuth.UserCode)
	}

	interval := time.Duration(devAuth.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
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
		tokHTTPResp, err := postFormWithContext(ctx, httpClient, meta.TokenEndpoint, tokForm)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if errors.Is(err, errOIDCRedirectBlocked) {
				return "", err
			}
			continue
		}

		tokBody, readErr := io.ReadAll(io.LimitReader(tokHTTPResp.Body, maxOIDCResponseBytes))
		_ = tokHTTPResp.Body.Close()
		if readErr != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			continue
		}

		if tokHTTPResp.StatusCode >= http.StatusInternalServerError || tokHTTPResp.StatusCode == http.StatusTooManyRequests {
			if tokHTTPResp.StatusCode == http.StatusTooManyRequests {
				interval += 5 * time.Second
			}
			continue
		}

		var tokResp deviceTokenResponse
		decodeErr := json.Unmarshal(tokBody, &tokResp)
		if tokHTTPResp.StatusCode != http.StatusOK && tokHTTPResp.StatusCode != http.StatusBadRequest {
			if tokResp.Error != "" {
				return "", fmt.Errorf("OIDC token exchange failed (status %d): %s", tokHTTPResp.StatusCode, tokResp.Error)
			}
			return "", fmt.Errorf("OIDC token endpoint returned status %d: %s", tokHTTPResp.StatusCode, strings.TrimSpace(string(tokBody)))
		}
		if decodeErr != nil {
			return "", fmt.Errorf("failed to decode OIDC token response (status %d): %w", tokHTTPResp.StatusCode, decodeErr)
		}

		switch tokResp.Error {
		case "":
			if tokHTTPResp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("OIDC token endpoint returned status %d: %s", tokHTTPResp.StatusCode, strings.TrimSpace(string(tokBody)))
			}
			if tokResp.IDToken != "" {
				return tokResp.IDToken, nil
			}
			if tokResp.AccessToken != "" {
				return tokResp.AccessToken, nil
			}
			return "", errors.New("OIDC token response did not contain id_token or access_token")
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
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
		netrcPath := filepath.Join(usr.HomeDir, ".netrc")
		if parsed, err := netrc.Parse(netrcPath); err == nil {
			auth = parsed
		} else {
			auth = &netrc.Netrc{
				Path: netrcPath,
			}
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
		auth.AddMachine(machine, "unused", token)
		m = auth.Machine(machine)
	} else {
		m.Set("password", token)
	}
	m.Set("token", token)

	if err := auth.Save(); err != nil {
		log.Printf("failed to save .netrc file: %v", err)
		return err
	}
	_ = os.Chmod(auth.Path, 0o600)
	return nil
}
