import { strict as assert } from 'node:assert'

const { getLoginValidationErrorMessage } = await import('../src/utils/loginValidationUtils.js')

assert.equal(
  getLoginValidationErrorMessage({ response: { status: 401 } }),
  'Invalid or expired token. Please check your token and try again.'
)
assert.equal(
  getLoginValidationErrorMessage({ response: { status: 403 } }),
  'Token is valid but lacks read permission for the registry.'
)
assert.equal(
  getLoginValidationErrorMessage({ response: { status: 500 } }),
  'Unable to verify token (HTTP 500)'
)
assert.equal(
  getLoginValidationErrorMessage({}),
  'Unable to verify token. Please check your network connection and try again.'
)
assert.equal(
  getLoginValidationErrorMessage(null),
  'Unable to verify token. Please check your network connection and try again.'
)

console.log('loginValidationUtils: OK')
