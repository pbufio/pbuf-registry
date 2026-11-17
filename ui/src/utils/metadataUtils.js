/**
 * Flattens nested protobuf message definitions into display entries while preserving hierarchy depth.
 */
export const flattenMetadataMessages = (messages = [], prefix = '', depth = 0) => {
  if (!Array.isArray(messages) || messages.length === 0) return []
  const entries = []
  for (const msg of messages) {
    if (!msg?.name) continue
    const fullName = prefix ? `${prefix}.${msg.name}` : msg.name
    const rawFields = Array.isArray(msg.fields) ? msg.fields : []
    entries.push({
      key: fullName,
      name: msg.name,
      fullName,
      depth,
      fields: rawFields.map(describeMetadataField).filter(Boolean),
      nestedEnums: Array.isArray(msg.nestedEnums) ? msg.nestedEnums : []
    })
    if (Array.isArray(msg.nestedMessages) && msg.nestedMessages.length > 0) {
      entries.push(...flattenMetadataMessages(msg.nestedMessages, fullName, depth + 1))
    }
  }
  return entries
}

/**
 * Formats a protobuf metadata Field message (scalar/message, map<K,V>, or oneof) into structured display metadata.
 */
export const describeMetadataField = (field) => {
  if (!field) return null

  if (field.oneof) {
    const names = Array.isArray(field.oneofNames) ? field.oneofNames : []
    const types = Array.isArray(field.oneofTypes) ? field.oneofTypes : []
    const branches = names.map((branchName, idx) => ({
      name: branchName,
      type: types[idx] || 'unknown'
    }))
    return {
      kind: 'oneof',
      name: field.name || '',
      branches
    }
  }

  if (field.map) {
    const keyType = field.mapKeyType || 'string'
    const valueType = field.mapValueType || 'bytes'
    return {
      kind: 'map',
      tag: field.tag,
      typeLabel: `map<${keyType}, ${valueType}>`,
      name: field.name || '',
      modifiers: []
    }
  }

  const modifiers = []
  if (field.repeated) modifiers.push('repeated')
  if (field.optional) modifiers.push('optional')
  if (field.required) modifiers.push('required')

  return {
    kind: 'field',
    tag: field.tag,
    typeLabel: field.messageType || '',
    name: field.name || '',
    modifiers
  }
}
