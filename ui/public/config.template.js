// Template used in production (Docker/nginx).
// This file is transformed into `/config.js` at container startup.
window.__APP_CONFIG__ = {
  PUBLIC_ENABLED: '$PUBLIC_ENABLED',
  OIDC_ENABLED: '$OIDC_ENABLED',
  OIDC_ISSUER_URL: '$OIDC_ISSUER_URL',
  OIDC_CLIENT_ID: '$OIDC_CLIENT_ID',
  OIDC_REDIRECT_URI: '$OIDC_REDIRECT_URI',
  OIDC_SCOPES: '$OIDC_SCOPES'
}
