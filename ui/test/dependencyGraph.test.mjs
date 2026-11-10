import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildDependencyGraph,
  buildDependencyVersionRoute,
  normalizeGraphSeverity,
  truncateGraphLabel
} from '../src/utils/dependencyGraph.js'

test('buildDependencyVersionRoute: formats version and fallback module routes', () => {
  assert.equal(buildDependencyVersionRoute('', 'v1.0.0'), '/modules')
  assert.equal(buildDependencyVersionRoute('acme/payment-service', ''), '/modules/acme/payment-service')
  assert.equal(
    buildDependencyVersionRoute('acme/payment-service', 'v1.2.0'),
    '/modules/acme/payment-service/tags/v1.2.0'
  )
})

test('normalizeGraphSeverity and truncateGraphLabel: normalizes enum values and truncates long labels', () => {
  assert.equal(normalizeGraphSeverity('DRIFT_SEVERITY_CRITICAL'), 'CRITICAL')
  assert.equal(normalizeGraphSeverity('DRIFT_SEVERITY_WARNING'), 'WARNING')
  assert.equal(normalizeGraphSeverity('DRIFT_SEVERITY_INFO'), 'INFO')
  assert.equal(normalizeGraphSeverity('DRIFT_SEVERITY_UNSPECIFIED'), 'HEALTHY')
  assert.equal(normalizeGraphSeverity(undefined), 'HEALTHY')

  assert.equal(truncateGraphLabel('acme/short', 20), 'acme/short')
  assert.equal(truncateGraphLabel('open-telemetry/opentelemetry-proto', 20), 'open-telemetry/open…')
})

test('buildDependencyGraph: lays out diamond DAG with direct + transitive edges and highest drift severities', () => {
  const graph = buildDependencyGraph({
    rootModule: 'acme/checkout-service',
    rootTag: 'v2.0.0',
    dependencies: [
      { name: 'acme/order-service', tag: 'v1.1.0', dependencyType: 'direct' },
      { name: 'acme/payment-service', tag: 'v1.0.0', dependencyType: 'direct' },
      { name: 'googleapis/common-protos', tag: 'v1.0.0', dependencyType: 'transitive' }
    ],
    driftStatuses: [
      {
        dependencyName: 'acme/order-service',
        currentTag: 'v1.1.0',
        targetTag: 'v1.2.0',
        severity: 'DRIFT_SEVERITY_INFO'
      },
      {
        dependencyName: 'acme/order-service',
        currentTag: 'v1.1.0',
        targetTag: 'v2.0.0',
        severity: 'DRIFT_SEVERITY_CRITICAL'
      },
      {
        dependencyName: 'googleapis/common-protos',
        currentTag: 'v1.0.0',
        targetTag: 'v1.1.0',
        severity: 'DRIFT_SEVERITY_WARNING'
      }
    ],
    edgesByParent: {
      'acme/order-service@v1.1.0': [{ name: 'googleapis/common-protos', tag: 'v1.0.0' }],
      'acme/payment-service@v1.0.0': [{ name: 'googleapis/common-protos', tag: 'v1.0.0' }]
    }
  })

  assert.equal(graph.counts.direct, 2)
  assert.equal(graph.counts.transitive, 1)
  assert.equal(graph.nodes.length, 4)
  assert.equal(graph.edges.length, 4)

  const byId = new Map(graph.nodes.map((n) => [n.id, n]))
  const root = byId.get('acme/checkout-service@v2.0.0')
  const order = byId.get('acme/order-service@v1.1.0')
  const payment = byId.get('acme/payment-service@v1.0.0')
  const common = byId.get('googleapis/common-protos@v1.0.0')

  assert.equal(root.depth, 0)
  assert.equal(order.depth, 1)
  assert.equal(payment.depth, 1)
  assert.equal(common.depth, 2)
  assert.ok(order.x > root.x)
  assert.ok(common.x > order.x)

  assert.equal(order.severity, 'CRITICAL')
  assert.equal(payment.severity, 'HEALTHY')
  assert.equal(common.severity, 'WARNING')
  assert.equal(order.route, '/modules/acme/order-service/tags/v1.1.0')
})

test('buildDependencyGraph: renders fallback dashed edge for transitive dep without parent lookup and handles cycles', () => {
  const fallbackGraph = buildDependencyGraph({
    rootModule: 'acme/app',
    rootTag: 'v1.0.0',
    dependencies: [
      { name: 'acme/direct', tag: 'v1.0.0', dependencyType: 'direct' },
      { name: 'acme/orphan-transitive', tag: 'v1.0.0', dependencyType: 'transitive' }
    ]
  })

  const orphan = fallbackGraph.nodes.find((n) => n.name === 'acme/orphan-transitive')
  const orphanEdge = fallbackGraph.edges.find((e) => e.to === orphan.id)
  assert.equal(orphan.depth, 2)
  assert.equal(orphanEdge.from, 'acme/app@v1.0.0')
  assert.equal(orphanEdge.dashed, true)

  const cyclicGraph = buildDependencyGraph({
    rootModule: 'acme/root',
    rootTag: 'v1.0.0',
    dependencies: [
      { name: 'acme/a', tag: 'v1.0.0', dependencyType: 'direct' },
      { name: 'acme/b', tag: 'v1.0.0', dependencyType: 'transitive' }
    ],
    edgesByParent: {
      'acme/a@v1.0.0': [{ name: 'acme/b', tag: 'v1.0.0' }],
      'acme/b@v1.0.0': [{ name: 'acme/a', tag: 'v1.0.0' }]
    }
  })

  assert.equal(cyclicGraph.nodes.length, 3)
  assert.equal(cyclicGraph.edges.length, 3)
})
