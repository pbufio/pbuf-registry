import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildBearerValue,
} from '../src/utils/proxyAuthUtils.js'

import { selectProxyAuthorizationHeader } from '../src/utils/proxyAuthUtils.js'

test('selectProxyAuthorizationHeader: prefers incoming Authorization header', () => {
  const result = selectProxyAuthorizationHeader({
    incomingAuthorization: 'Bearer from-browser',
    apiToken: 'pbuf_user_from_env',
    publicEnabled: false
  })

  assert.equal(result, 'Bearer from-browser')
})

test('selectProxyAuthorizationHeader: falls back to API token only in public mode', () => {
  const result = selectProxyAuthorizationHeader({
    incomingAuthorization: '',
    apiToken: 'pbuf_user_from_env',
    publicEnabled: true
  })

  assert.equal(result, 'Bearer pbuf_user_from_env')
})

test('selectProxyAuthorizationHeader: does not fall back to API token in private mode', () => {
  const result = selectProxyAuthorizationHeader({
    incomingAuthorization: '',
    apiToken: 'pbuf_user_from_env',
    publicEnabled: false
  })

  assert.equal(result, '')
})

assert.equal(buildBearerValue(''), '')
assert.equal(buildBearerValue('   '), '')
assert.equal(buildBearerValue('pbuf_user_abc'), 'Bearer pbuf_user_abc')
assert.equal(buildBearerValue('Bearer pbuf_user_abc'), 'Bearer pbuf_user_abc')

assert.equal(
    selectProxyAuthorizationHeader({
      incomingAuthorization: 'Bearer from-client',
      apiToken: 'pbuf_user_env',
      publicEnabled: false
    }),
    'Bearer from-client'
)
assert.equal(
    selectProxyAuthorizationHeader({
      incomingAuthorization: '',
      apiToken: 'pbuf_user_env',
      publicEnabled: true
    }),
    'Bearer pbuf_user_env'
)
assert.equal(
    selectProxyAuthorizationHeader({
      incomingAuthorization: undefined,
      apiToken: '',
      publicEnabled: true
    }),
    ''
)

console.log('proxyAuthUtils: OK')
