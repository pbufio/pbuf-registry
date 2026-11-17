#!/bin/sh
set -e

# Default values for environment variables
API_BASE_URL="${API_BASE_URL:-http://pbuf.cloud}"
API_TOKEN="${API_TOKEN:-}"
PUBLIC_ENABLED="${PUBLIC_ENABLED:-}"
OIDC_ENABLED="${OIDC_ENABLED:-}"
OIDC_ISSUER_URL="${OIDC_ISSUER_URL:-}"
OIDC_CLIENT_ID="${OIDC_CLIENT_ID:-}"
OIDC_REDIRECT_URI="${OIDC_REDIRECT_URI:-}"
OIDC_SCOPES="${OIDC_SCOPES:-openid profile email}"

# Normalize API_TOKEN: we always build the header as `Bearer ${API_TOKEN}` in nginx.
if [ -n "$API_TOKEN" ]; then
    API_TOKEN="$(echo "$API_TOKEN" | sed -E 's/^[Bb]earer[[:space:]]+//')"
fi

# Export variables so envsubst can access them
export API_BASE_URL
export API_TOKEN
export PUBLIC_ENABLED
export OIDC_ENABLED
export OIDC_ISSUER_URL
export OIDC_CLIENT_ID
export OIDC_REDIRECT_URI
export OIDC_SCOPES

echo "Starting nginx with configuration:"
echo "  API_BASE_URL: $API_BASE_URL"
echo "  API_TOKEN: ${API_TOKEN:+***set***}"
echo "  PUBLIC_ENABLED: ${PUBLIC_ENABLED:+***set***}"
echo "  OIDC_ENABLED: ${OIDC_ENABLED:+***set***}"

# Escape a string for safe interpolation inside a single-quoted JS string literal in config.js
js_escape() {
    printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e "s/'/\\\\'/g" | tr -d '\r\n'
}

# Validate API_BASE_URL cannot inject nginx directives
case "$API_BASE_URL" in
    http://*|https://*) ;;
    *)
        echo "ERROR: API_BASE_URL must start with http:// or https://" >&2
        exit 1
        ;;
esac
case "$API_BASE_URL" in
    *\;*|*\{*|*\}*|*[[:space:]]*)
        echo "ERROR: API_BASE_URL contains invalid characters" >&2
        exit 1
        ;;
esac

# Create a temporary nginx config with environment variables substituted
envsubst '$API_BASE_URL $API_TOKEN $PUBLIC_ENABLED' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf

# Generate runtime app config for the browser with escaped JS string values
if [ -f /usr/share/nginx/html/config.template.js ]; then
    PUBLIC_ENABLED="$(js_escape "$PUBLIC_ENABLED")" \
    OIDC_ENABLED="$(js_escape "$OIDC_ENABLED")" \
    OIDC_ISSUER_URL="$(js_escape "$OIDC_ISSUER_URL")" \
    OIDC_CLIENT_ID="$(js_escape "$OIDC_CLIENT_ID")" \
    OIDC_REDIRECT_URI="$(js_escape "$OIDC_REDIRECT_URI")" \
    OIDC_SCOPES="$(js_escape "$OIDC_SCOPES")" \
    envsubst '$PUBLIC_ENABLED $OIDC_ENABLED $OIDC_ISSUER_URL $OIDC_CLIENT_ID $OIDC_REDIRECT_URI $OIDC_SCOPES' < /usr/share/nginx/html/config.template.js > /usr/share/nginx/html/config.js
fi

# Test nginx configuration
nginx -t

# Start nginx
exec "$@"
