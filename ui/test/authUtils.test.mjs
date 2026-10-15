import assert from 'node:assert/strict'
import { normalizeToken, parseBool, parseConfigString, validateTokenFormat } from '../src/utils/authUtils.js'

// normalizeToken
assert.equal(normalizeToken(''), '')
assert.equal(normalizeToken('  pbuf_user_abc  '), 'pbuf_user_abc')
assert.equal(normalizeToken('Bearer pbuf_user_abc'), 'pbuf_user_abc')
assert.equal(normalizeToken('bearer    pbuf_user_abc'), 'pbuf_user_abc')

// validateTokenFormat
assert.equal(validateTokenFormat('pbuf_user_abc123').valid, true)
assert.equal(validateTokenFormat('pbuf_bot_xyz987').valid, true)
assert.equal(validateTokenFormat('eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.c2lnbmF0dXJl').valid, true)
assert.equal(validateTokenFormat('invalid-token').valid, false)

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
