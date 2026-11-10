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

export const replaceDriftEvent = (events, eventId, response) => {
  const updated = response?.event || { acknowledged: true }
  return events.map((e) => (e.id === eventId ? { ...e, ...updated } : e))
}

export const getAcknowledgeDriftErrorMessage = (err) =>
  err?.response?.data?.message || err?.message || 'Failed to acknowledge drift event'
