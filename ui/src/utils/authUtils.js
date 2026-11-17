export const normalizeToken = (raw) => {
  if (!raw) return ''
  return String(raw).trim().replace(/^bearer\s+/i, '').trim()
}

/**
 * Validates that a token is a non-empty single-token credential without whitespace or control characters.
 * Supports pbuf_user_..., pbuf_bot_..., OIDC JWTs, and SERVER_STATIC_TOKEN secrets.
 * @param {string} token - The normalized token (without "Bearer " prefix)
 * @returns {{ valid: boolean, error?: string }}
 */
export const validateTokenFormat = (token) => {
  if (!token) {
    return { valid: false, error: 'Token is required' }
  }

  const trimmed = String(token).trim()
  if (!trimmed) {
    return { valid: false, error: 'Token is required' }
  }

  if (!/^[\x21-\x7e]+$/.test(trimmed)) {
    return {
      valid: false,
      error: 'Invalid token format: token must contain only printable non-space ASCII characters'
    }
  }

  return { valid: true }
}

/**
 * Sanitizes a post-login redirect target so only safe in-app relative paths are allowed.
 * Rejects protocol-relative URLs (//host), backslash prefixes (/\host), external URLs, and control chars.
 */
export const sanitizeNextUrl = (raw, fallback = '/modules') => {
  const candidate = Array.isArray(raw) ? raw[0] : raw
  if (!candidate) return fallback
  const str = String(candidate).trim()
  if (!str.startsWith('/') || str.startsWith('//') || str.startsWith('/\\') || /[\x00-\x1f\x7f]/.test(str)) {
    return fallback
  }
  return str
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
