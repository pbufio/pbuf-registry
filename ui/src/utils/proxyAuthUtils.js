import { normalizeToken, parseBool } from './authUtils.js'

const PUBLIC_POST_ENDPOINTS = new Set([
  '/api/v1/modules/get',
  '/api/v1/modules/pull',
  '/api/v1/modules/dependencies',
  '/api/v1/metadata',
  '/api/v1/drift/modules/events',
  '/api/v1/drift/modules/dependencies/status'
])

const PUBLIC_PATH_REGEX = /^\/api\/v1\/(modules|metadata|drift)(\/|$)/

export const isPublicReadEndpoint = (method = 'GET', url = '/api/v1/modules') => {
  const m = String(method || 'GET').toUpperCase()
  const path = String(url || '').split('?')[0]
  if (!PUBLIC_PATH_REGEX.test(path)) return false
  if (m === 'GET' || m === 'HEAD' || m === 'OPTIONS') return true
  if (m === 'POST') return PUBLIC_POST_ENDPOINTS.has(path)
  return false
}

export const buildBearerValue = (token) => {
  const normalized = normalizeToken(token)
  return normalized ? `Bearer ${normalized}` : ''
}

export const selectProxyAuthorizationHeader = ({
  incomingAuthorization,
  apiToken,
  publicEnabled,
  method = 'GET',
  url = '/api/v1/modules'
}) => {
  if (incomingAuthorization) return String(incomingAuthorization)
  if (!parseBool(publicEnabled)) return ''
  if (!isPublicReadEndpoint(method, url)) return ''
  return buildBearerValue(apiToken)
}
