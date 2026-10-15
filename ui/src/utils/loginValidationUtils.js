export const getLoginValidationErrorMessage = (error) => {
  const status = error?.response?.status
  const message = error?.response?.data?.message || error?.message

  if (status === 401) {
    return 'Invalid or expired token. Please check your token and try again.'
  }
  if (status === 403) {
    return 'Token is valid but lacks read permission for the registry.'
  }
  if (status === 404) {
    return 'Registry API not found. Please check the server configuration.'
  }
  if (status === 502 || status === 503 || status === 504) {
    return 'Registry backend is unavailable. Please try again later.'
  }

  if (typeof status === 'number') {
    const detail = message ? `: ${message}` : ''
    return `Unable to verify token (HTTP ${status})${detail}`
  }

  // Network errors (no response)
  if (error?.code === 'ECONNREFUSED' || error?.code === 'ERR_NETWORK') {
    return 'Cannot connect to the registry. Please check your network connection.'
  }

  return 'Unable to verify token. Please check your network connection and try again.'
}
