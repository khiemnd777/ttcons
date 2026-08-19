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

function dimensionItemsField(): WorkflowFormField {
  return {
    form_version_id: 'version',
    field_key: 'dimension_items',
    label: 'Kích thước chính',
    field_type: 'structured_list',
    required: true,
    validation_json: {
      min_items: 1,
      item_fields: [
        { key: 'area', label: 'Khu vực', type: 'text', required: true, min_length: 2, max_length: 160 },
        { key: 'dimension', label: 'Kích thước', type: 'text', required: true },
        { key: 'unit', label: 'ĐVT', type: 'text', required: true },
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

test('makes only survey work-package area optional without mutating the published schema', () => {
  const stored = dimensionItemsField();
  const [effective] = effectiveWorkflowStructuredListFields([stored], 'client_survey_work_package');
  const effectiveSpecs = effective.validation_json?.item_fields as Array<Record<string, unknown>>;
  const storedSpecs = stored.validation_json?.item_fields as Array<Record<string, unknown>>;

  assert.deepEqual(effectiveSpecs[0], {
    key: 'area',
    label: 'Khu vực',
    type: 'text',
    required: false,
    max_length: 160,
  });
  assert.equal(effectiveSpecs[1].required, true);
  assert.equal(effectiveSpecs[2].required, true);
  assert.equal(storedSpecs[0].required, true);
  assert.equal(storedSpecs[0].min_length, 2);
});

test('keeps survey dimension area required for unrelated forms', () => {
  const stored = dimensionItemsField();
  const [effective] = effectiveWorkflowStructuredListFields([stored], 'another_form');
  assert.strictEqual(effective, stored);
});
