const OIDC_STATE_KEY = 'pbuf_oidc_state'
const OIDC_VERIFIER_KEY = 'pbuf_oidc_code_verifier'
const OIDC_NEXT_URL_KEY = 'pbuf_oidc_next_url'

const toBase64Url = (bytes) => {
  let binary = ''
  const arr = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes)
  for (let i = 0; i < arr.byteLength; i++) {
    binary += String.fromCharCode(arr[i])
  }
  const b64 = typeof btoa === 'function'
    ? btoa(binary)
    : Buffer.from(arr).toString('base64')
  return b64.replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

export const generateRandomString = (byteLength = 32) => {
  const bytes = new Uint8Array(byteLength)
  globalThis.crypto.getRandomValues(bytes)
  return toBase64Url(bytes)
}

export const deriveCodeChallenge = async (codeVerifier) => {
  const data = new TextEncoder().encode(codeVerifier)
  const digest = await globalThis.crypto.subtle.digest('SHA-256', data)
  return toBase64Url(new Uint8Array(digest))
}

export const discoverOidcEndpoints = async (issuerUrl, fetchImpl = globalThis.fetch) => {
  const cleanIssuer = String(issuerUrl || '').trim().replace(/\/+$/, '')
  if (!cleanIssuer) {
    throw new Error('OIDC issuer URL is not configured')
  }
  const res = await fetchImpl(`${cleanIssuer}/.well-known/openid-configuration`, {
    method: 'GET',
    headers: { Accept: 'application/json' }
  })
  if (!res.ok) {
    throw new Error(`OIDC discovery failed with status ${res.status}`)
  }
  const doc = await res.json()
  if (!doc.authorization_endpoint || !doc.token_endpoint) {
    throw new Error('OIDC discovery response missing authorization_endpoint or token_endpoint')
  }
  return {
    authorizationEndpoint: doc.authorization_endpoint,
    tokenEndpoint: doc.token_endpoint
  }
}

export const buildAuthorizationUrl = ({
  authorizationEndpoint,
  clientId,
  redirectUri,
  scope = 'openid profile email',
  state,
  codeChallenge
}) => {
  const url = new URL(authorizationEndpoint)
  url.searchParams.set('response_type', 'code')
  url.searchParams.set('client_id', clientId)
  url.searchParams.set('redirect_uri', redirectUri)
  url.searchParams.set('scope', scope)
  url.searchParams.set('state', state)
  url.searchParams.set('code_challenge', codeChallenge)
  url.searchParams.set('code_challenge_method', 'S256')
  return url.toString()
}

export const exchangeCodeForToken = async (
  { tokenEndpoint, clientId, code, redirectUri, codeVerifier },
  fetchImpl = globalThis.fetch
) => {
  const body = new URLSearchParams({
    grant_type: 'authorization_code',
    client_id: clientId,
    code,
    redirect_uri: redirectUri,
    code_verifier: codeVerifier
  })

  const res = await fetchImpl(tokenEndpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
      Accept: 'application/json'
    },
    body: body.toString()
  })

  if (!res.ok) {
    const errText = await res.text().catch(() => '')
    throw new Error(`OIDC token exchange failed (status ${res.status}): ${errText}`)
  }

  const data = await res.json()
  const token = data.id_token || data.access_token
  if (!token) {
    throw new Error('OIDC token response did not contain id_token or access_token')
  }
  return token
}

export const resolveRedirectUri = (config) => {
  if (config?.oidcRedirectUri) {
    return config.oidcRedirectUri
  }
  if (typeof window !== 'undefined' && window.location) {
    return `${window.location.origin}/login`
  }
  return '/login'
}

export const startOidcLogin = async (config, nextUrl = '/modules', fetchImpl = globalThis.fetch) => {
  const endpoints = await discoverOidcEndpoints(config.oidcIssuerUrl, fetchImpl)
  const state = generateRandomString(24)
  const codeVerifier = generateRandomString(32)
  const codeChallenge = await deriveCodeChallenge(codeVerifier)
  const redirectUri = resolveRedirectUri(config)

  if (typeof window !== 'undefined' && window.sessionStorage) {
    window.sessionStorage.setItem(OIDC_STATE_KEY, state)
    window.sessionStorage.setItem(OIDC_VERIFIER_KEY, codeVerifier)
    window.sessionStorage.setItem(OIDC_NEXT_URL_KEY, nextUrl || '/modules')
  }

  const authUrl = buildAuthorizationUrl({
    authorizationEndpoint: endpoints.authorizationEndpoint,
    clientId: config.oidcClientId,
    redirectUri,
    scope: config.oidcScopes || 'openid profile email',
    state,
    codeChallenge
  })

  if (typeof window !== 'undefined' && window.location) {
    window.location.assign(authUrl)
  }
  return authUrl
}

export const handleOidcCallback = async (
  query,
  config,
  storage = typeof window !== 'undefined' ? window.sessionStorage : null,
  fetchImpl = globalThis.fetch
) => {
  if (query.error) {
    const desc = query.error_description ? `: ${query.error_description}` : ''
    throw new Error(`OIDC login failed (${query.error})${desc}`)
  }

  const code = Array.isArray(query.code) ? query.code[0] : query.code
  const state = Array.isArray(query.state) ? query.state[0] : query.state
  if (!code) {
    return null
  }

  const expectedState = storage?.getItem(OIDC_STATE_KEY) || ''
  const codeVerifier = storage?.getItem(OIDC_VERIFIER_KEY) || ''
  const nextUrl = storage?.getItem(OIDC_NEXT_URL_KEY) || '/modules'

  storage?.removeItem(OIDC_STATE_KEY)
  storage?.removeItem(OIDC_VERIFIER_KEY)
  storage?.removeItem(OIDC_NEXT_URL_KEY)

  if (!expectedState || state !== expectedState) {
    throw new Error('OIDC state mismatch; please try signing in again')
  }
  if (!codeVerifier) {
    throw new Error('Missing PKCE code verifier in session')
  }

  const endpoints = await discoverOidcEndpoints(config.oidcIssuerUrl, fetchImpl)
  const redirectUri = resolveRedirectUri(config)

  const token = await exchangeCodeForToken(
    {
      tokenEndpoint: endpoints.tokenEndpoint,
      clientId: config.oidcClientId,
      code,
      redirectUri,
      codeVerifier
    },
    fetchImpl
  )

  return { token, nextUrl }
}
