package middleware

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/pbufio/pbuf-registry/internal/config"
)

const (
	defaultJWKSCacheTTL    = 15 * time.Minute
	minJWKSRefreshInterval = 5 * time.Second
	oidcHTTPTimeout        = 10 * time.Second
)

// OIDCClaims holds the verified identity extracted from an OIDC JWT.
type OIDCClaims struct {
	Subject  string
	Username string
	Groups   []string
	IsAdmin  bool
}

type oidcDiscoveryDoc struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

type jwksDocument struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// OIDCVerifier verifies OIDC JWT tokens against an OIDC issuer's JWKS.
type OIDCVerifier struct {
	cfg        config.OIDCConfig
	httpClient *http.Client

	refreshMu   sync.Mutex
	mu          sync.RWMutex
	keys        map[string]crypto.PublicKey
	defaultKey  crypto.PublicKey
	jwksURL     string
	lastFetched time.Time
}

// NewOIDCVerifier creates a new OIDCVerifier for the given OIDC configuration.
func NewOIDCVerifier(cfg config.OIDCConfig) (*OIDCVerifier, error) {
	if strings.TrimSpace(cfg.IssuerURL) == "" {
		return nil, errors.New("oidc issuerUrl is required")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, errors.New("oidc clientId is required")
	}
	if cfg.UsernameClaim == "" {
		cfg.UsernameClaim = "email"
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}

	return &OIDCVerifier{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: oidcHTTPTimeout,
		},
		keys:    make(map[string]crypto.PublicKey),
		jwksURL: strings.TrimSpace(cfg.JWKSURL),
	}, nil
}

// VerifyToken parses and validates a raw JWT string against the configured OIDC provider.
func (v *OIDCVerifier) VerifyToken(ctx context.Context, rawToken string) (*OIDCClaims, error) {
	claims := jwtv5.MapClaims{}

	token, err := jwtv5.ParseWithClaims(rawToken, claims, func(token *jwtv5.Token) (interface{}, error) {
		switch token.Method.(type) {
		case *jwtv5.SigningMethodRSA, *jwtv5.SigningMethodECDSA:
		default:
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, _ := token.Header["kid"].(string)
		return v.getPublicKey(ctx, kid)
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid oidc token: %w", err)
	}

	// Verify issuer (normalized without trailing slash)
	iss, err := claims.GetIssuer()
	if err != nil || normalizeIssuer(iss) != normalizeIssuer(v.cfg.IssuerURL) {
		return nil, errors.New("oidc token issuer mismatch")
	}

	// Verify audience
	aud, err := claims.GetAudience()
	if err != nil || !containsString(aud, v.cfg.ClientID) {
		return nil, errors.New("oidc token audience mismatch")
	}

	sub, _ := claims.GetSubject()
	username := extractUsernameClaim(claims, v.cfg.UsernameClaim, sub)
	if username == "" {
		return nil, errors.New("oidc token missing username or subject claim")
	}

	groups := extractStringSliceClaim(claims, v.cfg.GroupsClaim)
	isAdmin := hasMatchingGroup(groups, v.cfg.AdminGroups)

	return &OIDCClaims{
		Subject:  sub,
		Username: username,
		Groups:   groups,
		IsAdmin:  isAdmin,
	}, nil
}

func (v *OIDCVerifier) getPublicKey(ctx context.Context, kid string) (crypto.PublicKey, error) {
	v.mu.RLock()
	expired := time.Since(v.lastFetched) > defaultJWKSCacheTTL
	key, ok := v.lookupKeyLocked(kid)
	v.mu.RUnlock()

	if ok && !expired {
		return key, nil
	}

	if err := v.refreshKeys(ctx, kid); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.lookupKeyLocked(kid)
	if !ok {
		return nil, fmt.Errorf("signing key %q not found in JWKS", kid)
	}
	return key, nil
}

func (v *OIDCVerifier) lookupKeyLocked(kid string) (crypto.PublicKey, bool) {
	if kid != "" {
		key, ok := v.keys[kid]
		return key, ok
	}
	if v.defaultKey != nil {
		return v.defaultKey, true
	}
	return nil, false
}

func (v *OIDCVerifier) refreshKeys(ctx context.Context, kid string) error {
	v.refreshMu.Lock()
	defer v.refreshMu.Unlock()

	v.mu.RLock()
	_, ok := v.lookupKeyLocked(kid)
	age := time.Since(v.lastFetched)
	hasFetched := !v.lastFetched.IsZero()
	jwksURL := v.jwksURL
	v.mu.RUnlock()

	if ok && age <= defaultJWKSCacheTTL {
		return nil
	}
	if hasFetched && age < minJWKSRefreshInterval {
		return nil
	}

	if jwksURL == "" {
		discoveredURL, err := v.discoverJWKSURL(ctx)
		if err != nil {
			return err
		}
		jwksURL = discoveredURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create JWKS request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d from JWKS endpoint", resp.StatusCode)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("failed to decode JWKS: %w", err)
	}

	newKeys := make(map[string]crypto.PublicKey, len(doc.Keys))
	var firstKey crypto.PublicKey
	for _, k := range doc.Keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		pub, err := parseJWK(k)
		if err != nil {
			continue
		}
		if firstKey == nil {
			firstKey = pub
		}
		if k.Kid != "" {
			newKeys[k.Kid] = pub
		}
	}

	if firstKey == nil && len(newKeys) == 0 {
		return errors.New("no valid signing keys found in JWKS")
	}

	v.mu.Lock()
	if v.jwksURL == "" {
		v.jwksURL = jwksURL
	}
	v.keys = newKeys
	v.defaultKey = firstKey
	v.lastFetched = time.Now()
	v.mu.Unlock()
	return nil
}

func (v *OIDCVerifier) discoverJWKSURL(ctx context.Context) (string, error) {
	discoveryURL := strings.TrimSuffix(v.cfg.IssuerURL, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create OIDC discovery request: %w", err)
	}

	resp, err := v.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch OIDC discovery document: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d from OIDC discovery endpoint", resp.StatusCode)
	}

	var doc oidcDiscoveryDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("failed to decode OIDC discovery document: %w", err)
	}
	if doc.JWKSURI == "" {
		return "", errors.New("oidc discovery document missing jwks_uri")
	}
	return doc.JWKSURI, nil
}

func parseJWK(k jwkKey) (crypto.PublicKey, error) {
	switch strings.ToUpper(k.Kty) {
	case "RSA":
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			return nil, err
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			return nil, err
		}
		eInt := new(big.Int).SetBytes(eBytes).Int64()
		if eInt <= 0 {
			return nil, errors.New("invalid RSA exponent")
		}
		return &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(eInt),
		}, nil
	case "EC":
		xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, err
		}
		yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
		if err != nil {
			return nil, err
		}
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported EC curve: %s", k.Crv)
		}
		return &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported key type: %s", k.Kty)
	}
}

func normalizeIssuer(iss string) string {
	return strings.TrimSuffix(strings.TrimSpace(iss), "/")
}

func extractUsernameClaim(claims jwtv5.MapClaims, preferredClaim, fallbackSub string) string {
	candidates := []string{preferredClaim, "email", "preferred_username", "upn", "name"}
	for _, key := range candidates {
		if key == "" {
			continue
		}
		if val, ok := claims[key].(string); ok && strings.TrimSpace(val) != "" {
			return strings.TrimSpace(val)
		}
	}
	return strings.TrimSpace(fallbackSub)
}

func extractStringSliceClaim(claims jwtv5.MapClaims, key string) []string {
	raw, ok := claims[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if v == "" {
			return nil
		}
		return strings.Fields(v)
	default:
		return nil
	}
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

func hasMatchingGroup(userGroups, adminGroups []string) bool {
	if len(userGroups) == 0 || len(adminGroups) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(adminGroups))
	for _, g := range adminGroups {
		if trimmed := strings.TrimSpace(g); trimmed != "" {
			set[trimmed] = struct{}{}
		}
	}
	for _, g := range userGroups {
		if _, ok := set[strings.TrimSpace(g)]; ok {
			return true
		}
	}
	return false
}

func isLikelyJWT(token string) bool {
	return strings.Count(token, ".") == 2 && !strings.HasPrefix(token, "pbuf_")
}
