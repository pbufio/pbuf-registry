import { computed, ref } from 'vue'
import { normalizeToken } from '../utils/authUtils.js'

export { normalizeToken, validateTokenFormat } from '../utils/authUtils.js'

const STORAGE_KEY = 'pbuf_registry_ui_token'

const loadFromSession = () => {
  if (typeof window === 'undefined') return ''
  return normalizeToken(window.sessionStorage.getItem(STORAGE_KEY) || '')
}

const tokenRef = ref(loadFromSession())

const persist = (token) => {
  if (typeof window === 'undefined') return
  if (!token) {
    window.sessionStorage.removeItem(STORAGE_KEY)
    return
  }
  window.sessionStorage.setItem(STORAGE_KEY, token)
}

export const authToken = {
  token: tokenRef,
  isLoggedIn: computed(() => Boolean(tokenRef.value)),
  setToken(raw) {
    const normalized = normalizeToken(raw)
    tokenRef.value = normalized
    persist(normalized)
  },
  clearToken() {
    tokenRef.value = ''
    persist('')
  },
  getAuthorizationHeaderValue() {
    if (!tokenRef.value) return ''
    return `Bearer ${tokenRef.value}`
  }
}
