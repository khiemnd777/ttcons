import assert from 'node:assert/strict';
import test from 'node:test';

import { enumLabel, enumOptions, nodeTypeValues } from '../src/lib/utils/status.ts';

const officialNodeTypeLabels = {
  contract: 'Hợp đồng',
  construction_planning: 'Kế hoạch thi công',
  incoming_acceptance: 'Nghiệm thu đầu vào',
} as const;

test('official workflow node types use Vietnamese presentation labels', () => {
  for (const [value, expectedLabel] of Object.entries(officialNodeTypeLabels)) {
    assert.equal(enumLabel('node_type', value), expectedLabel);
    assert.ok(nodeTypeValues.includes(value as (typeof nodeTypeValues)[number]));
  }
});

test('node inspector options expose the same shared Vietnamese labels', () => {
  const options = new Map(enumOptions('node_type', nodeTypeValues).map((option) => [option.value, option.label]));

  for (const [value, expectedLabel] of Object.entries(officialNodeTypeLabels)) {
    assert.equal(options.get(value), expectedLabel);
  }
});

test('unknown future node types remain visible through the technical fallback', () => {
  assert.equal(enumLabel('node_type', 'future_node_type'), 'future_node_type');
});
