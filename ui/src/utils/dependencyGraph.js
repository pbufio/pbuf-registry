const SEVERITY_RANK = {
  HEALTHY: 0,
  DRIFT_SEVERITY_UNSPECIFIED: 0,
  INFO: 1,
  DRIFT_SEVERITY_INFO: 1,
  WARNING: 2,
  DRIFT_SEVERITY_WARNING: 2,
  CRITICAL: 3,
  DRIFT_SEVERITY_CRITICAL: 3
}

export const GRAPH_LAYOUT = {
  nodeWidth: 268,
  nodeHeight: 78,
  colGap: 112,
  rowGap: 26,
  padX: 28,
  padY: 28
}

export function normalizeGraphSeverity(severity) {
  switch (severity) {
    case 'DRIFT_SEVERITY_CRITICAL':
    case 'CRITICAL':
      return 'CRITICAL'
    case 'DRIFT_SEVERITY_WARNING':
    case 'WARNING':
      return 'WARNING'
    case 'DRIFT_SEVERITY_INFO':
    case 'INFO':
      return 'INFO'
    case 'UNKNOWN':
      return 'UNKNOWN'
    default:
      return 'HEALTHY'
  }
}

export function dependencyNodeId(name, tag = '') {
  return `${name}@${tag || ''}`
}

export function buildDependencyVersionRoute(name, tag = '') {
  if (!name) return '/modules'
  if (!tag) return `/modules/${name}`
  return `/modules/${name}/tags/${encodeURIComponent(tag)}`
}

export function truncateGraphLabel(value, maxLen = 27) {
  if (!value || value.length <= maxLen) return value || ''
  return `${value.slice(0, maxLen - 1)}…`
}

export function dedupeDriftStatuses(statuses = []) {
  const seen = new Set()
  const result = []
  for (const status of statuses || []) {
    if (!status?.dependencyName) continue
    const key = `${status.dependencyName}:${status.currentTag || ''}:${status.targetTag || ''}`
    if (seen.has(key)) continue
    seen.add(key)
    result.push(status)
  }
  return result
}

function indexDriftStatuses(driftStatuses = []) {
  const byKey = new Map()
  for (const status of dedupeDriftStatuses(driftStatuses)) {
    const key = dependencyNodeId(status.dependencyName, status.currentTag)
    if (!byKey.has(key)) byKey.set(key, [])
    byKey.get(key).push(status)
  }
  return byKey
}

function highestSeverityForStatuses(statuses = []) {
  let best = 'HEALTHY'
  for (const status of statuses) {
    const normalized = normalizeGraphSeverity(status?.severity)
    if ((SEVERITY_RANK[normalized] ?? 0) > (SEVERITY_RANK[best] ?? 0)) {
      best = normalized
    }
  }
  return best
}

/**
 * Builds a layered left-to-right DAG layout for a module version and its direct + transitive dependencies.
 */
export function buildDependencyGraph({
  rootModule,
  rootTag = '',
  dependencies = [],
  driftStatuses = [],
  edgesByParent = {},
  directDriftUnavailable = false,
  transitiveDriftUnavailable = false
}) {
  if (!rootModule) {
    return { width: 0, height: 0, nodes: [], edges: [], counts: { direct: 0, transitive: 0 } }
  }

  const rootId = dependencyNodeId(rootModule, rootTag)
  const driftByKey = indexDriftStatuses(driftStatuses)
  const nodeMap = new Map()

  nodeMap.set(rootId, {
    id: rootId,
    name: rootModule,
    tag: rootTag,
    isRoot: true,
    dependencyType: 'root',
    severity: 'ROOT',
    driftStatuses: [],
    route: null
  })

  const ensureDepNode = (dep, fallbackType = 'direct') => {
    if (!dep?.name) return null
    const id = dependencyNodeId(dep.name, dep.tag)
    if (id === rootId) return rootId

    const depType = dep.dependencyType === 'transitive'
      ? 'transitive'
      : dep.dependencyType === 'direct'
        ? 'direct'
        : fallbackType

    const existing = nodeMap.get(id)
    if (existing) {
      if (depType === 'direct' && existing.dependencyType === 'transitive') {
        existing.dependencyType = 'direct'
        if (existing.driftStatuses.length === 0) {
          existing.severity = directDriftUnavailable ? 'UNKNOWN' : 'HEALTHY'
        }
      }
      return id
    }

    const statuses = driftByKey.get(id) || driftByKey.get(dependencyNodeId(dep.name, '')) || []
    const unavailable = depType === 'transitive' ? transitiveDriftUnavailable : directDriftUnavailable
    const severity = statuses.length > 0
      ? highestSeverityForStatuses(statuses)
      : unavailable
        ? 'UNKNOWN'
        : 'HEALTHY'

    nodeMap.set(id, {
      id,
      name: dep.name,
      tag: dep.tag || '',
      isRoot: false,
      dependencyType: depType,
      severity,
      driftStatuses: statuses,
      route: buildDependencyVersionRoute(dep.name, dep.tag)
    })
    return id
  }

  for (const dep of dependencies || []) {
    ensureDepNode(dep, 'direct')
  }

  const rawEdges = new Map()
  const addEdge = (from, to, isFallback = false) => {
    if (!from || !to || from === to) return
    const key = `${from}->${to}`
    const prev = rawEdges.get(key)
    if (!prev || (prev.isFallback && !isFallback)) {
      rawEdges.set(key, { id: key, from, to, isFallback })
    }
  }

  for (const dep of dependencies || []) {
    if (!dep?.name || dep.dependencyType === 'transitive') continue
    const depId = ensureDepNode(dep, 'direct')
    addEdge(rootId, depId, false)
  }

  for (const [parentKey, children] of Object.entries(edgesByParent || {})) {
    if (!nodeMap.has(parentKey) || !Array.isArray(children)) continue
    const isRootParent = parentKey === rootId
    for (const child of children) {
      if (!child?.name) continue
      const childId = ensureDepNode(
        {
          name: child.name,
          tag: child.tag,
          dependencyType: isRootParent ? 'direct' : 'transitive'
        },
        isRootParent ? 'direct' : 'transitive'
      )
      addEdge(parentKey, childId, false)
    }
  }

  const incomingCount = new Map()
  for (const edge of rawEdges.values()) {
    incomingCount.set(edge.to, (incomingCount.get(edge.to) || 0) + 1)
  }

  for (const node of nodeMap.values()) {
    if (node.isRoot) continue
    if (!incomingCount.get(node.id)) {
      const isTransitive = node.dependencyType === 'transitive'
      addEdge(rootId, node.id, isTransitive)
      incomingCount.set(node.id, 1)
    }
  }

  const adj = new Map()
  for (const edge of rawEdges.values()) {
    if (!adj.has(edge.from)) adj.set(edge.from, [])
    adj.get(edge.from).push(edge)
  }

  // Assign DAG column depth via cycle-safe longest-path BFS from root.
  const depthById = new Map([[rootId, 0]])
  const maxDepth = Math.max(1, nodeMap.size - 1)
  const queue = [{ id: rootId, path: new Set([rootId]) }]

  while (queue.length > 0) {
    const { id, path } = queue.shift()
    const currentDepth = depthById.get(id) || 0
    for (const edge of adj.get(id) || []) {
      if (path.has(edge.to)) continue
      const minStep = edge.from === rootId && edge.isFallback ? 2 : 1
      const nextDepth = Math.min(maxDepth, currentDepth + minStep)
      if (nextDepth > (depthById.get(edge.to) ?? -1)) {
        depthById.set(edge.to, nextDepth)
        const nextPath = new Set(path)
        nextPath.add(edge.to)
        queue.push({ id: edge.to, path: nextPath })
      }
    }
  }

  for (const node of nodeMap.values()) {
    if (!depthById.has(node.id)) {
      depthById.set(node.id, node.dependencyType === 'transitive' ? 2 : 1)
    }
  }

  const columns = new Map()
  for (const node of nodeMap.values()) {
    const d = depthById.get(node.id) || 0
    if (!columns.has(d)) columns.set(d, [])
    columns.get(d).push(node)
  }

  const sortedDepths = [...columns.keys()].sort((a, b) => a - b)
  const { nodeWidth, nodeHeight, colGap, rowGap, padX, padY } = GRAPH_LAYOUT
  const maxRows = Math.max(1, ...[...columns.values()].map((col) => col.length))
  const numCols = Math.max(1, sortedDepths.length)
  const width = padX * 2 + numCols * nodeWidth + (numCols - 1) * colGap
  const height = Math.max(200, padY * 2 + maxRows * nodeHeight + (maxRows - 1) * rowGap)

  let directCount = 0
  let transitiveCount = 0
  const positionedNodes = []

  sortedDepths.forEach((depthKey, colIndex) => {
    const colNodes = columns.get(depthKey)
    const colContentHeight = colNodes.length * nodeHeight + (colNodes.length - 1) * rowGap
    const startY = Math.round((height - colContentHeight) / 2)
    const x = padX + colIndex * (nodeWidth + colGap)

    colNodes.forEach((node, rowIndex) => {
      if (!node.isRoot) {
        if (node.dependencyType === 'transitive') transitiveCount += 1
        else directCount += 1
      }
      const y = startY + rowIndex * (nodeHeight + rowGap)
      positionedNodes.push({
        ...node,
        depth: colIndex,
        x,
        y,
        width: nodeWidth,
        height: nodeHeight,
        displayName: truncateGraphLabel(node.name, 30),
        displayTag: truncateGraphLabel(
          node.tag || 'latest',
          node.dependencyType === 'transitive' ? 11 : 20
        )
      })
    })
  })

  const positionedById = new Map(positionedNodes.map((n) => [n.id, n]))
  const positionedEdges = []

  for (const edge of rawEdges.values()) {
    const source = positionedById.get(edge.from)
    const target = positionedById.get(edge.to)
    if (!source || !target) continue

    const x1 = source.x + source.width
    const y1 = Math.round(source.y + source.height / 2)
    const x2 = target.x
    const y2 = Math.round(target.y + target.height / 2)
    const dx = Math.max(44, Math.round(Math.abs(x2 - x1) * 0.5))
    const path = `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`

    positionedEdges.push({
      id: edge.id,
      from: edge.from,
      to: edge.to,
      severity: target.severity,
      dashed: edge.isFallback || target.dependencyType === 'transitive',
      path
    })
  }

  return {
    width,
    height,
    nodes: positionedNodes,
    edges: positionedEdges,
    counts: {
      direct: directCount,
      transitive: transitiveCount
    }
  }
}
