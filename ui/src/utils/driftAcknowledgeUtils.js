// Pure helpers for acknowledging drift events. Kept free of app aliases and
// framework imports so they can be unit tested directly with `node --test`.

export const buildAcknowledgeDriftEventRequest = (eventId, acknowledgedBy) => {
  const body = {}
  if (acknowledgedBy) body.acknowledgedBy = acknowledgedBy

  return {
    path: `/v1/drift/events/${encodeURIComponent(eventId)}/acknowledge`,
    body
  }
}

export const canAcknowledgeDriftEvent = ({ isLoggedIn, event, pendingEventId }) =>
  Boolean(isLoggedIn) && Boolean(event) && !event.acknowledged && pendingEventId == null

// Only copy the acknowledgement fields from the response. The server's event is
// a sparse, zero-valued protobuf (Kratos emits unpopulated fields), so merging
// the whole event would blank out filename, severity, hashes, etc.
export const replaceDriftEvent = (events, eventId, response) => {
  const serverEvent = response?.event || {}
  const serverBy = serverEvent.acknowledgedBy
  const acknowledgedBy = serverBy && serverBy !== 'unknown' ? serverBy : null

  return events.map((e) => {
    if (e.id !== eventId) return e
    const updated = {
      ...e,
      acknowledged: true,
      acknowledgedAt: serverEvent.acknowledgedAt || new Date().toISOString()
    }
    if (acknowledgedBy) updated.acknowledgedBy = acknowledgedBy
    return updated
  })
}

export const getAcknowledgeDriftErrorMessage = (err) =>
  err?.response?.data?.message || err?.message || 'Failed to acknowledge drift event'
