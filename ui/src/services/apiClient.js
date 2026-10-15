import axios from 'axios'
import { appConfig } from '@/config/appConfig'
import { authToken } from '@/auth/authToken'

// API calls are proxied through the same-origin `/api` endpoint.
// If the user logged in via token, we attach `Authorization: Bearer <token>`.
// In production, nginx may also inject its own Authorization header (server-side)
// when `API_TOKEN` is configured.
export const apiClient = axios.create({
  baseURL: '/api',
  headers: {
    'Content-Type': 'application/json'
  }
})

apiClient.interceptors.request.use((config) => {
  const auth = authToken.getAuthorizationHeaderValue()
  if (auth) {
    config.headers = config.headers || {}
    if (!config.headers.Authorization && !config.headers.authorization) {
      config.headers.Authorization = auth
    }
  }
  return config
})

const redirectToLogin = () => {
  if (typeof window === 'undefined') return
  if (window.location.pathname === '/login') return

  const next = `${window.location.pathname}${window.location.search}${window.location.hash}`
  window.location.assign(`/login?next=${encodeURIComponent(next)}`)
}

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error?.response?.status

    if (status === 401) {
      authToken.clearToken()
      if (!appConfig.publicEnabled) {
        redirectToLogin()
      }
    }

    return Promise.reject(error)
  }
)
