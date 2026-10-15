import { normalizeToken, parseBool } from './authUtils.js'

export const buildBearerValue = (token) => {
  const normalized = normalizeToken(token)
  return normalized ? `Bearer ${normalized}` : ''
}

export const selectProxyAuthorizationHeader = ({ incomingAuthorization, apiToken, publicEnabled }) => {
  if (incomingAuthorization) return String(incomingAuthorization)
  if (!parseBool(publicEnabled)) return ''
  return buildBearerValue(apiToken)
}
