import assert from 'node:assert/strict'
import test from 'node:test'
import {
  buildVendorSnippet,
  computeLineDiff,
  diffProtoFileSets,
  pickDefaultBaseTag,
  resolveRegistryAddr
} from '../src/utils/protoDiff.js'

test('resolveRegistryAddr and buildVendorSnippet emit valid registry.addr YAML', () => {
  assert.equal(resolveRegistryAddr(''), 'pbuf.io')
  assert.equal(resolveRegistryAddr('registry.acme.internal'), 'registry.acme.internal')
  assert.equal(resolveRegistryAddr('localhost:6777'), 'localhost:6777')
  assert.equal(resolveRegistryAddr('::1'), '::1')
  assert.equal(resolveRegistryAddr('[::1]'), '::1')
  assert.equal(resolveRegistryAddr('[::1]:6777'), '[::1]:6777')

  const basic = buildVendorSnippet({
    moduleName: 'acme/orders',
    tag: 'v1.2.0',
    hostname: 'registry.acme.internal'
  })
  assert.match(basic, /registry:\n  addr: registry\.acme\.internal\nmodules:\n  - name: acme\/orders\n    tag: v1\.2\.0/)
  assert.doesNotMatch(basic, /domain:/)

  const ipv6 = buildVendorSnippet({
    moduleName: 'acme/orders',
    tag: 'v1.2.0',
    hostname: '[::1]'
  })
  assert.match(ipv6, /registry:\n  addr: '::1'\n/)

  const hostPort = buildVendorSnippet({
    moduleName: 'acme/orders',
    tag: 'v1.2.0',
    hostname: 'localhost:6777'
  })
  assert.match(hostPort, /registry:\n  addr: 'localhost:6777'\n/)

  const withGen = buildVendorSnippet({
    moduleName: 'acme/orders',
    tag: 'v1.2.0',
    genOut: 'gen',
    hostname: 'pbuf.io'
  })
  assert.match(withGen, /gen_out: gen {2}# patches go_package option/)
})

test('pickDefaultBaseTag selects immediately preceding tag or first non-self tag', () => {
  const tags = ['v2.0.0', 'v1.1.0', 'v1.0.0']
  assert.equal(pickDefaultBaseTag(tags, 'v2.0.0'), 'v1.1.0')
  assert.equal(pickDefaultBaseTag(tags, 'v1.1.0'), 'v1.0.0')
  assert.equal(pickDefaultBaseTag(tags, 'v1.0.0'), 'v2.0.0')
  assert.equal(pickDefaultBaseTag(tags, 'v2.1.0-rc1'), 'v2.0.0')
  assert.equal(pickDefaultBaseTag(['v1.0.0'], 'v1.0.0'), '')
})

test('computeLineDiff and diffProtoFileSets classify added, modified, deleted, and unchanged files', () => {
  const before = 'syntax = "proto3";\npackage acme.v1;\nmessage Order {\n  string id = 1;\n}\n'
  const after = 'syntax = "proto3";\npackage acme.v1;\nmessage Order {\n  string id = 1;\n  string status = 2;\n}\n'

  const lines = computeLineDiff(before, after)
  assert.equal(lines.filter((l) => l.type === 'add').length, 1)
  assert.equal(lines.filter((l) => l.type === 'del').length, 0)
  assert.equal(lines.find((l) => l.type === 'add').text, '  string status = 2;')

  const diff = diffProtoFileSets(
    [
      { filename: 'order.proto', content: before },
      { filename: 'legacy.proto', content: 'syntax = "proto3";\n' },
      { filename: 'same.proto', content: 'syntax = "proto3";\r\n' },
      { filename: 'eof.proto', content: 'syntax = "proto3";' }
    ],
    [
      { filename: 'order.proto', content: after },
      { filename: 'new.proto', content: 'syntax = "proto3";\nmessage New {}\n' },
      { filename: 'same.proto', content: 'syntax = "proto3";\n' },
      { filename: 'eof.proto', content: 'syntax = "proto3";\n' }
    ]
  )

  assert.equal(diff.added, 1)
  assert.equal(diff.modified, 1)
  assert.equal(diff.deleted, 1)
  assert.equal(diff.unchanged, 2)
  assert.deepEqual(
    diff.files.map((f) => ({ filename: f.filename, status: f.status, additions: f.additions, deletions: f.deletions })),
    [
      { filename: 'legacy.proto', status: 'deleted', additions: 0, deletions: 1 },
      { filename: 'new.proto', status: 'added', additions: 2, deletions: 0 },
      { filename: 'order.proto', status: 'modified', additions: 1, deletions: 0 }
    ]
  )
})

test('computeLineDiff handles >250k LCS cell fallback with contiguous line numbers', () => {
  const oldBody = Array.from({ length: 510 }, (_, i) => `  string old_field_${i} = ${i + 1};`).join('\n')
  const newBody = Array.from({ length: 510 }, (_, i) => `  string new_field_${i} = ${i + 1};`).join('\n')
  const before = `syntax = "proto3";\nmessage Big {\n${oldBody}\n}\n`
  const after = `syntax = "proto3";\nmessage Big {\n${newBody}\n}\n`

  const lines = computeLineDiff(before, after)
  assert.equal(lines[0].type, 'context')
  assert.equal(lines[0].oldLine, 1)
  assert.equal(lines[1].oldLine, 2)
  assert.equal(lines.filter((l) => l.type === 'del').length, 510)
  assert.equal(lines.filter((l) => l.type === 'add').length, 510)

  const last = lines[lines.length - 1]
  assert.equal(last.type, 'context')
  assert.equal(last.text, '}')
  assert.equal(last.oldLine, 513)
  assert.equal(last.newLine, 513)
})
