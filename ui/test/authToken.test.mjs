import { strict as assert } from 'node:assert'

const createSessionStorage = () => {
  const store = new Map()
  return {
    getItem: (key) => (store.has(key) ? store.get(key) : null),
    setItem: (key, value) => store.set(key, String(value)),
    removeItem: (key) => store.delete(key)
  }
}

globalThis.window = {
  sessionStorage: createSessionStorage()
}

const { authToken } = await import('../src/auth/authToken.js')

assert.equal(authToken.isLoggedIn.value, false)
assert.equal(authToken.getAuthorizationHeaderValue(), '')

authToken.setToken('Bearer pbuf_user_abc')
assert.equal(authToken.token.value, 'pbuf_user_abc')
assert.equal(authToken.isLoggedIn.value, true)
assert.equal(authToken.getAuthorizationHeaderValue(), 'Bearer pbuf_user_abc')
assert.equal(globalThis.window.sessionStorage.getItem('pbuf_registry_ui_token'), 'pbuf_user_abc')

authToken.clearToken()
assert.equal(authToken.isLoggedIn.value, false)
assert.equal(authToken.getAuthorizationHeaderValue(), '')
assert.equal(globalThis.window.sessionStorage.getItem('pbuf_registry_ui_token'), null)

console.log('authToken: OK')
