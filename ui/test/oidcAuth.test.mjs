import assert from 'node:assert/strict'
import {
  buildAuthorizationUrl,
  deriveCodeChallenge,
  discoverOidcEndpoints,
  exchangeCodeForToken,
  handleOidcCallback
} from '../src/auth/oidcAuth.js'

// RFC 7636 Appendix B test vector for S256 PKCE
const verifier = 'dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk'
const challenge = await deriveCodeChallenge(verifier)
assert.equal(challenge, 'E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM')

// buildAuthorizationUrl
const authUrl = buildAuthorizationUrl({
  authorizationEndpoint: 'https://idp.example.com/authorize',
  clientId: 'pbuf-ui',
  redirectUri: 'https://registry.example.com/login',
  scope: 'openid profile email',
  state: 'state-123',
  codeChallenge: challenge
})
const parsedUrl = new URL(authUrl)
assert.equal(parsedUrl.origin + parsedUrl.pathname, 'https://idp.example.com/authorize')
assert.equal(parsedUrl.searchParams.get('response_type'), 'code')
assert.equal(parsedUrl.searchParams.get('client_id'), 'pbuf-ui')
assert.equal(parsedUrl.searchParams.get('code_challenge'), challenge)
assert.equal(parsedUrl.searchParams.get('code_challenge_method'), 'S256')

// Mock fetch for discovery + token exchange
const mockFetch = async (url, opts) => {
  if (String(url).endsWith('/.well-known/openid-configuration')) {
    return {
      ok: true,
      status: 200,
      json: async () => ({
        authorization_endpoint: 'https://idp.example.com/authorize',
        token_endpoint: 'https://idp.example.com/token'
      })
    }
  }
  if (String(url) === 'https://idp.example.com/token') {
    assert.equal(opts.method, 'POST')
    assert.ok(opts.body.includes('code=auth-code-xyz'))
    return {
      ok: true,
      status: 200,
      json: async () => ({
        id_token: 'eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.c2ln',
        access_token: 'opaque-access-token'
      })
    }
  }
  throw new Error(`Unexpected URL: ${url}`)
}

const endpoints = await discoverOidcEndpoints('https://idp.example.com/', mockFetch)
assert.equal(endpoints.authorizationEndpoint, 'https://idp.example.com/authorize')
assert.equal(endpoints.tokenEndpoint, 'https://idp.example.com/token')

const token = await exchangeCodeForToken(
  {
    tokenEndpoint: endpoints.tokenEndpoint,
    clientId: 'pbuf-ui',
    code: 'auth-code-xyz',
    redirectUri: 'https://registry.example.com/login',
    codeVerifier: verifier
  },
  mockFetch
)
assert.equal(token, 'eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.c2ln')

// handleOidcCallback with mock sessionStorage
const store = new Map([
  ['pbuf_oidc_state', 'state-123'],
  ['pbuf_oidc_code_verifier', verifier],
  ['pbuf_oidc_next_url', '/modules/my-mod']
])
const fakeStorage = {
  getItem: (k) => store.get(k) ?? null,
  removeItem: (k) => store.delete(k)
}

const callbackRes = await handleOidcCallback(
  { code: 'auth-code-xyz', state: 'state-123' },
  {
    oidcIssuerUrl: 'https://idp.example.com',
    oidcClientId: 'pbuf-ui',
    oidcRedirectUri: 'https://registry.example.com/login'
  },
  fakeStorage,
  mockFetch
)
assert.equal(callbackRes.token, 'eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.c2ln')
assert.equal(callbackRes.nextUrl, '/modules/my-mod')

console.log('oidcAuth: OK')
