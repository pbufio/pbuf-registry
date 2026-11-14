import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const uiDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const readUi = (relPath) => readFileSync(resolve(uiDir, relPath), 'utf8')

let utils
try {
  utils = await import('../src/utils/driftAcknowledgeUtils.js')
} catch (err) {
  throw new Error(`Failed to acknowledge drift event: cannot load driftAcknowledgeUtils (${err.code || err.message})`)
}

const {
  buildAcknowledgeDriftEventRequest,
  canAcknowledgeDriftEvent,
  replaceDriftEvent,
  getAcknowledgeDriftErrorMessage
} = utils

// buildAcknowledgeDriftEventRequest: POST path is encoded and body is optional
assert.deepEqual(buildAcknowledgeDriftEventRequest('evt-1', ''), {
  path: '/v1/drift/events/evt-1/acknowledge',
  body: {}
})
assert.deepEqual(buildAcknowledgeDriftEventRequest('evt-1', 'alice'), {
  path: '/v1/drift/events/evt-1/acknowledge',
  body: { acknowledgedBy: 'alice' }
})
assert.equal(
  buildAcknowledgeDriftEventRequest('a/b c', undefined).path,
  '/v1/drift/events/a%2Fb%20c/acknowledge'
)

// canAcknowledgeDriftEvent: only logged-in users, unacknowledged events, nothing in flight
const open = { id: 'evt-1', acknowledged: false }
assert.equal(canAcknowledgeDriftEvent({ isLoggedIn: true, event: open, pendingEventId: null }), true)
assert.equal(canAcknowledgeDriftEvent({ isLoggedIn: false, event: open, pendingEventId: null }), false)
assert.equal(
  canAcknowledgeDriftEvent({ isLoggedIn: true, event: { id: 'evt-2', acknowledged: true }, pendingEventId: null }),
  false
)
assert.equal(canAcknowledgeDriftEvent({ isLoggedIn: true, event: open, pendingEventId: 'evt-9' }), false)
assert.equal(canAcknowledgeDriftEvent({ isLoggedIn: true, event: null, pendingEventId: null }), false)

// replaceDriftEvent: replaces only the matching event in place, copying only acknowledgement fields
const events = [
  { id: 'evt-1', filename: 'a.proto', severity: 'DRIFT_SEVERITY_WARNING', acknowledged: false },
  { id: 'evt-2', filename: 'b.proto', severity: 'DRIFT_SEVERITY_INFO', acknowledged: false }
]
const replaced = replaceDriftEvent(events, 'evt-1', {
  event: { id: 'evt-1', acknowledged: true, acknowledgedBy: 'alice', acknowledgedAt: '2026-11-14T10:00:00Z' }
})
assert.deepEqual(replaced, [
  {
    id: 'evt-1',
    filename: 'a.proto',
    severity: 'DRIFT_SEVERITY_WARNING',
    acknowledged: true,
    acknowledgedBy: 'alice',
    acknowledgedAt: '2026-11-14T10:00:00Z'
  },
  events[1]
])
assert.equal(events[0].acknowledged, false, 'original array must not be mutated')

// Kratos-shaped sparse response (zero-valued fields, acknowledgedBy 'unknown') must not blank the card
const sparse = replaceDriftEvent(events, 'evt-2', {
  event: {
    id: 'evt-2',
    moduleName: '',
    tagName: '',
    filename: '',
    eventType: 'DRIFT_EVENT_TYPE_UNSPECIFIED',
    previousHash: '',
    currentHash: '',
    severity: 'DRIFT_SEVERITY_UNSPECIFIED',
    detectedAt: null,
    acknowledged: true,
    acknowledgedBy: 'unknown',
    acknowledgedAt: null
  }
})
assert.equal(sparse[1].filename, 'b.proto')
assert.equal(sparse[1].severity, 'DRIFT_SEVERITY_INFO')
assert.equal(sparse[1].acknowledged, true)
assert.equal(sparse[1].acknowledgedBy, undefined, "'unknown' must not be copied")
assert.ok(!Number.isNaN(Date.parse(sparse[1].acknowledgedAt)), 'falls back to a local timestamp when the server has none')

// Without an event in the response, the local event is still marked acknowledged
assert.equal(replaceDriftEvent(events, 'evt-2', {})[1].acknowledged, true)

// getAcknowledgeDriftErrorMessage: prefers server message, then error message, then default
assert.equal(
  getAcknowledgeDriftErrorMessage({ response: { data: { message: 'permission denied' } } }),
  'permission denied'
)
assert.equal(getAcknowledgeDriftErrorMessage(new Error('network down')), 'network down')
assert.equal(getAcknowledgeDriftErrorMessage({}), 'Failed to acknowledge drift event')
assert.equal(getAcknowledgeDriftErrorMessage(null), 'Failed to acknowledge drift event')

// nginx.conf: simulate nginx map lookup (exact entries, ~regex entries, first match wins, else default)
const nginx = readUi('nginx.conf')
const mapBody = (name) => {
  const m = nginx.match(new RegExp(`map \\$uri \\$${name} \\{([\\s\\S]*?)\\n\\}`))
  assert.ok(m, `nginx map ${name} must exist`)
  return m[1]
}
const lookup = (body, uri) => {
  const def = body.match(/default\s+(\S+?);/)[1]
  for (const m of body.matchAll(/^\s*"(~?[^"]+)"\s+(\S+?);/gm)) {
    const [, pattern, value] = m
    const hit = pattern.startsWith('~') ? new RegExp(pattern.slice(1)).test(uri) : uri === pattern
    if (hit) return value
  }
  return def
}
const ackUri = '/api/v1/drift/events/evt-1/acknowledge'
const publicUri = '/api/v1/drift/modules/events'
assert.equal(lookup(mapBody('allow_post'), ackUri), '1', 'acknowledge must be an allowed POST endpoint')
assert.equal(lookup(mapBody('allow_public_token'), ackUri), '0', 'acknowledge must not receive the public fallback token')
assert.equal(lookup(mapBody('allow_public_token'), publicUri), '1', 'public read endpoints keep the public fallback token')

console.log('driftAcknowledgeUtils: OK')
