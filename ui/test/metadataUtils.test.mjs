import assert from 'node:assert/strict'
import { describeMetadataField, flattenMetadataMessages } from '../src/utils/metadataUtils.js'

const nestedTree = [
  {
    name: 'Order',
    fields: [
      { name: 'id', messageType: 'string', tag: 1 },
      { name: 'tags', messageType: 'string', tag: 2, repeated: true },
      { name: 'note', messageType: 'string', tag: 3, optional: true },
      { name: 'labels', tag: 4, map: true, mapKeyType: 'string', mapValueType: 'string' },
      { name: 'payment', oneof: true, oneofNames: ['card_id', 'wire_ref'], oneofTypes: ['string', 'WireTransfer'] }
    ],
    nestedEnums: [
      { name: 'Status', values: [{ name: 'STATUS_UNSPECIFIED', tag: 0 }, { name: 'STATUS_PAID', tag: 1 }] }
    ],
    nestedMessages: [
      {
        name: 'LineItem',
        fields: [{ name: 'sku', messageType: 'string', tag: 1, required: true }],
        nestedMessages: [
          { name: 'Discount', fields: [{ name: 'cents', messageType: 'int64', tag: 1 }] }
        ]
      }
    ]
  }
]

const flat = flattenMetadataMessages(nestedTree)
assert.equal(flat.length, 3)
assert.deepEqual(
  flat.map((m) => ({ fullName: m.fullName, depth: m.depth })),
  [
    { fullName: 'Order', depth: 0 },
    { fullName: 'Order.LineItem', depth: 1 },
    { fullName: 'Order.LineItem.Discount', depth: 2 }
  ]
)
assert.equal(flat[0].nestedEnums.length, 1)
assert.equal(flat[0].nestedEnums[0].name, 'Status')

assert.deepEqual(flat[0].fields[3], {
  kind: 'map',
  tag: 4,
  typeLabel: 'map<string, string>',
  name: 'labels',
  modifiers: []
})

assert.deepEqual(flat[0].fields[4], {
  kind: 'oneof',
  name: 'payment',
  branches: [
    { name: 'card_id', type: 'string' },
    { name: 'wire_ref', type: 'WireTransfer' }
  ]
})

assert.deepEqual(flat[1].fields[0], {
  kind: 'field',
  tag: 1,
  typeLabel: 'string',
  name: 'sku',
  modifiers: ['required']
})

assert.equal(describeMetadataField(null), null)

console.log('metadataUtils: OK')
