import assert from 'node:assert/strict'
import { normalizeToken, parseBool, parseConfigString, sanitizeNextUrl, validateTokenFormat } from '../src/utils/authUtils.js'

// normalizeToken
assert.equal(normalizeToken(''), '')
assert.equal(normalizeToken('  pbuf_user_abc  '), 'pbuf_user_abc')
assert.equal(normalizeToken('Bearer pbuf_user_abc'), 'pbuf_user_abc')
assert.equal(normalizeToken('bearer    pbuf_user_abc'), 'pbuf_user_abc')

// validateTokenFormat (supports pbuf_user_*, pbuf_bot_*, JWT, and SERVER_STATIC_TOKEN)
assert.equal(validateTokenFormat('pbuf_user_abc123').valid, true)
assert.equal(validateTokenFormat('pbuf_bot_xyz987').valid, true)
assert.equal(validateTokenFormat('eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.c2lnbmF0dXJl').valid, true)
assert.equal(validateTokenFormat('my-static-admin-token-2026!').valid, true)
assert.equal(validateTokenFormat('').valid, false)
assert.equal(validateTokenFormat('   ').valid, false)
assert.equal(validateTokenFormat('bad token with spaces').valid, false)
assert.equal(validateTokenFormat('bad\ntoken').valid, false)
assert.equal(validateTokenFormat('“smart-quoted-token”').valid, false)
assert.equal(validateTokenFormat('zero\u200bwidth').valid, false)

// sanitizeNextUrl
assert.equal(sanitizeNextUrl('/modules/acme/pay'), '/modules/acme/pay')
assert.equal(sanitizeNextUrl(['/modules/first', '/modules/second']), '/modules/first')
assert.equal(sanitizeNextUrl(''), '/modules')
assert.equal(sanitizeNextUrl('//evil.example.com/phish'), '/modules')
assert.equal(sanitizeNextUrl('/\\evil.example.com'), '/modules')
assert.equal(sanitizeNextUrl('https://evil.example.com'), '/modules')
assert.equal(sanitizeNextUrl('javascript:alert(1)'), '/modules')

// parseBool
assert.equal(parseBool(undefined), false)
assert.equal(parseBool(''), false)
assert.equal(parseBool('0'), false)
assert.equal(parseBool('1'), true)
assert.equal(parseBool('true'), true)
assert.equal(parseBool('YES'), true)
assert.equal(parseBool('$PUBLIC_ENABLED'), false)
assert.equal(parseBool('$OIDC_ENABLED'), false)

// parseConfigString
assert.equal(parseConfigString('$OIDC_ISSUER_URL', ''), '')
assert.equal(parseConfigString('https://idp.example.com', ''), 'https://idp.example.com')

console.log('authUtils: OK')
