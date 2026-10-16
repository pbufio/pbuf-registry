export const normalizeToken = (raw) => {
  if (!raw) return ''
  return String(raw).trim().replace(/^bearer\s+/i, '').trim()
}

/**
 * Validates that a token has the expected pbuf format.
 * Valid tokens: pbuf_user_... or pbuf_bot_...
 * @param {string} token - The normalized token (without "Bearer " prefix)
 * @returns {{ valid: boolean, error?: string }}
 */
export const validateTokenFormat = (token) => {
  if (!token) {
    return { valid: false, error: 'Token is required' }
  }

  const trimmed = token.trim()
  if (!trimmed) {
    return { valid: false, error: 'Token is required' }
  }

  // Check for valid pbuf token format (pbuf_<type>_<base64url>) or OIDC JWT (<header>.<payload>.<signature>)
  const pbufPattern = /^pbuf_(user|bot)_[A-Za-z0-9_-]+$/
  const jwtPattern = /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/
  if (!pbufPattern.test(trimmed) && !jwtPattern.test(trimmed)) {
    return {
      valid: false,
      error: 'Invalid token format. Expected: pbuf_user_..., pbuf_bot_..., or a valid JWT'
    }
  }

  return { valid: true }
}

export const parseBool = (value) => {
  if (value === true) return true
  if (value === false) return false

  if (value == null) return false
  const str = String(value).trim().toLowerCase()

  if (!str) return false
  // Protect dev builds where the template placeholder was not substituted.
  if (str.startsWith('$')) return false

  return str === '1' || str === 'true' || str === 'yes' || str === 'on'
}

export const parseConfigString = (value, fallback = '') => {
  if (value == null) return fallback
  const str = String(value).trim()
  if (!str || str.startsWith('$')) return fallback
  return str
}
