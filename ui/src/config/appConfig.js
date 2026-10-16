import { parseBool, parseConfigString } from '@/utils/authUtils'

const getWindowConfig = () => {
  if (typeof window === 'undefined') return {}
  return window.__APP_CONFIG__ || {}
}

const env = typeof import.meta !== 'undefined' && import.meta.env ? import.meta.env : {}

export const appConfig = {
  publicEnabled: parseBool(getWindowConfig().PUBLIC_ENABLED ?? env.VITE_PUBLIC_ENABLED),
  oidcEnabled: parseBool(getWindowConfig().OIDC_ENABLED ?? env.VITE_OIDC_ENABLED),
  oidcIssuerUrl: parseConfigString(getWindowConfig().OIDC_ISSUER_URL ?? env.VITE_OIDC_ISSUER_URL),
  oidcClientId: parseConfigString(getWindowConfig().OIDC_CLIENT_ID ?? env.VITE_OIDC_CLIENT_ID),
  oidcRedirectUri: parseConfigString(getWindowConfig().OIDC_REDIRECT_URI ?? env.VITE_OIDC_REDIRECT_URI),
  oidcScopes: parseConfigString(getWindowConfig().OIDC_SCOPES ?? env.VITE_OIDC_SCOPES, 'openid profile email')
}
