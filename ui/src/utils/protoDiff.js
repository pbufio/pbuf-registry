const MAX_LCS_CELLS = 250000

/**
 * Resolves the registry gRPC host for pbuf.yaml snippets from a browser hostname.
 */
export function resolveRegistryAddr(hostname = '') {
  const raw = String(hostname || '').trim()
  const fallback = raw || 'pbuf.io'
  if (fallback.startsWith('[') && fallback.endsWith(']')) {
    return fallback.slice(1, -1)
  }
  return fallback
}

/**
 * Builds a valid pbuf.yaml snippet using registry.addr (host[:port]).
 */
export function buildVendorSnippet({ moduleName, tag = 'v1.0.0', genOut = '', hostname = '' } = {}) {
  const addr = resolveRegistryAddr(
    hostname || (typeof window !== 'undefined' && window.location ? window.location.hostname : '')
  )
  const formattedAddr = addr.includes(':') || addr.startsWith('[') ? `'${addr}'` : addr
  const lines = [
    '# Add to your pbuf.yaml:',
    'version: v1',
    'registry:',
    `  addr: ${formattedAddr}`,
    'modules:',
    `  - name: ${moduleName}`,
    `    tag: ${tag}`,
    '    out: third_party'
  ]
  if (genOut) {
    lines.push(`    gen_out: ${genOut}  # patches go_package option`)
  }
  lines.push('', '# Then run:', '# pbuf vendor')
  return lines.join('\n')
}

/**
 * Picks the default baseline tag to compare against (immediately preceding tag when present).
 */
export function pickDefaultBaseTag(tags = [], currentTag = '') {
  if (!Array.isArray(tags) || tags.length === 0) return ''
  const idx = tags.indexOf(currentTag)
  if (idx >= 0 && idx + 1 < tags.length) {
    return tags[idx + 1]
  }
  return tags.find((t) => t && t !== currentTag) || ''
}

function splitLines(text) {
  if (!text) return []
  const normalized = String(text).replace(/\r\n|\r/g, '\n')
  const parts = normalized.split('\n')
  if (parts.length > 0 && parts[parts.length - 1] === '') {
    parts.pop()
  }
  return parts
}

/**
 * Formats a .proto file string for display alongside 1-based line numbers without phantom trailing blank lines.
 */
export function formatProtoViewer(content = '') {
  const code = String(content ?? '').replace(/\r\n|\r/g, '\n')
  const lines = splitLines(code)
  if (lines.length === 0) {
    return { code: '', lineNumbers: '1' }
  }
  return {
    code,
    lineNumbers: Array.from({ length: lines.length }, (_, i) => i + 1).join('\n')
  }
}

/**
 * Copies text to the clipboard via navigator.clipboard in secure contexts, falling back to execCommand on plain HTTP.
 */
export async function copyTextToClipboard(text = '') {
  const value = String(text ?? '')
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      return true
    } catch {
      // fall back to execCommand when clipboard permission is unavailable
    }
  }
  if (typeof document === 'undefined' || !document.body) return false
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)
  try {
    textarea.select()
    return Boolean(document.execCommand && document.execCommand('copy'))
  } catch {
    return false
  } finally {
    document.body.removeChild(textarea)
  }
}

/**
 * Computes a line-level unified diff between two strings.
 */
export function computeLineDiff(beforeText = '', afterText = '') {
  const oldLines = splitLines(beforeText)
  const newLines = splitLines(afterText)

  let start = 0
  while (
    start < oldLines.length
    && start < newLines.length
    && oldLines[start] === newLines[start]
  ) {
    start += 1
  }

  let oldEnd = oldLines.length
  let newEnd = newLines.length
  while (
    oldEnd > start
    && newEnd > start
    && oldLines[oldEnd - 1] === newLines[newEnd - 1]
  ) {
    oldEnd -= 1
    newEnd -= 1
  }

  const out = []
  for (let i = 0; i < start; i += 1) {
    out.push({ type: 'context', oldLine: i + 1, newLine: i + 1, text: oldLines[i] })
  }

  const midOld = oldLines.slice(start, oldEnd)
  const midNew = newLines.slice(start, newEnd)
  const m = midOld.length
  const n = midNew.length

  if (m > 0 || n > 0) {
    // shortcut: O(m*n) LCS capped at 250k cells, upgrade to Myers diff if multi-thousand-line proto rewrites need fine-grained hunks
    if (m === 0 || n === 0 || m * n > MAX_LCS_CELLS) {
      for (let i = 0; i < m; i += 1) {
        out.push({ type: 'del', oldLine: start + i + 1, newLine: null, text: midOld[i] })
      }
      for (let j = 0; j < n; j += 1) {
        out.push({ type: 'add', oldLine: null, newLine: start + j + 1, text: midNew[j] })
      }
    } else {
      const width = n + 1
      const dp = new Uint32Array((m + 1) * width)
      for (let i = m - 1; i >= 0; i -= 1) {
        for (let j = n - 1; j >= 0; j -= 1) {
          if (midOld[i] === midNew[j]) {
            dp[i * width + j] = dp[(i + 1) * width + j + 1] + 1
          } else {
            const down = dp[(i + 1) * width + j]
            const right = dp[i * width + j + 1]
            dp[i * width + j] = down >= right ? down : right
          }
        }
      }

      let i = 0
      let j = 0
      while (i < m && j < n) {
        if (midOld[i] === midNew[j]) {
          out.push({
            type: 'context',
            oldLine: start + i + 1,
            newLine: start + j + 1,
            text: midOld[i]
          })
          i += 1
          j += 1
        } else if (dp[(i + 1) * width + j] >= dp[i * width + j + 1]) {
          out.push({ type: 'del', oldLine: start + i + 1, newLine: null, text: midOld[i] })
          i += 1
        } else {
          out.push({ type: 'add', oldLine: null, newLine: start + j + 1, text: midNew[j] })
          j += 1
        }
      }
      while (i < m) {
        out.push({ type: 'del', oldLine: start + i + 1, newLine: null, text: midOld[i] })
        i += 1
      }
      while (j < n) {
        out.push({ type: 'add', oldLine: null, newLine: start + j + 1, text: midNew[j] })
        j += 1
      }
    }
  }

  for (let k = 0; k < oldLines.length - oldEnd; k += 1) {
    out.push({
      type: 'context',
      oldLine: oldEnd + k + 1,
      newLine: newEnd + k + 1,
      text: oldLines[oldEnd + k]
    })
  }

  return out
}

/**
 * Compares two sets of { filename, content } proto files and returns changed file diffs plus summary counts.
 */
export function diffProtoFileSets(baseFiles = [], targetFiles = []) {
  const baseMap = new Map()
  for (const f of baseFiles || []) {
    if (f?.filename) baseMap.set(f.filename, String(f.content ?? ''))
  }
  const targetMap = new Map()
  for (const f of targetFiles || []) {
    if (f?.filename) targetMap.set(f.filename, String(f.content ?? ''))
  }

  const allNames = Array.from(new Set([...baseMap.keys(), ...targetMap.keys()])).sort()
  const files = []
  let added = 0
  let modified = 0
  let deleted = 0
  let unchanged = 0

  for (const filename of allNames) {
    const inBase = baseMap.has(filename)
    const inTarget = targetMap.has(filename)
    const before = inBase ? baseMap.get(filename) : ''
    const after = inTarget ? targetMap.get(filename) : ''

    if (inBase && inTarget && before.replace(/\r\n/g, '\n') === after.replace(/\r\n/g, '\n')) {
      unchanged += 1
      continue
    }

    const lines = computeLineDiff(before, after)
    let additions = 0
    let deletions = 0
    for (const line of lines) {
      if (line.type === 'add') additions += 1
      else if (line.type === 'del') deletions += 1
    }

    if (inBase && inTarget && additions === 0 && deletions === 0) {
      unchanged += 1
      continue
    }

    let status = 'modified'
    if (!inBase) {
      status = 'added'
      added += 1
    } else if (!inTarget) {
      status = 'deleted'
      deleted += 1
    } else {
      modified += 1
    }

    files.push({ filename, status, additions, deletions, lines })
  }

  return { added, modified, deleted, unchanged, files }
}
