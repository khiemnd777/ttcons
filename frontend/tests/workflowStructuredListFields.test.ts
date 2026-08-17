import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkflowFormField } from '../src/lib/types/domain.ts';
import { effectiveWorkflowStructuredListFields } from '../src/modules/work-items/workflowStructuredListFields.ts';

function riskItemsField(): WorkflowFormField {
  return {
    form_version_id: 'version',
    field_key: 'risk_items',
    label: 'Danh sách rủi ro',
    field_type: 'structured_list',
    required: true,
    validation_json: {
      item_fields: [
        { key: 'risk', label: 'Rủi ro', type: 'textarea', required: true },
        { key: 'category', label: 'Nhóm', type: 'select', required: true },
        { key: 'severity', label: 'Mức độ', type: 'select', required: true },
        { key: 'mitigation', label: 'Hướng xử lý', type: 'textarea', required: false },
      ],
    },
  };
}

test('hides the site-risk category without mutating the published field definition', () => {
  const stored = riskItemsField();
  const [effective] = effectiveWorkflowStructuredListFields([stored], 'client_site_risk_assessment');
  const effectiveSpecs = effective.validation_json?.item_fields as Array<Record<string, unknown>>;
  const storedSpecs = stored.validation_json?.item_fields as Array<Record<string, unknown>>;

  assert.deepEqual(effectiveSpecs.map((spec) => spec.key), ['risk', 'severity', 'mitigation']);
  assert.deepEqual(storedSpecs.map((spec) => spec.key), ['risk', 'category', 'severity', 'mitigation']);
});

test('keeps structured-list fields unchanged outside the approved site-risk form', () => {
  const stored = riskItemsField();
  const [effective] = effectiveWorkflowStructuredListFields([stored], 'another_form');
  assert.strictEqual(effective, stored);
});
