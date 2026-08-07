import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkflowFormField } from '../src/lib/types/domain.ts';
import { effectiveWorkflowFormFields } from '../src/modules/work-items/workflowFormRules.ts';

const fields: WorkflowFormField[] = [
  {
    id: 'route', form_version_id: 'version', field_key: 'route_choice', label: 'Điểm bắt đầu',
    field_type: 'select', required: true, options_json: [{ value: 'survey' }, { value: 'direct' }],
  },
  {
    id: 'document', form_version_id: 'version', field_key: 'source_document', label: 'Hồ sơ đầu vào',
    field_type: 'evidence_reference', required: false,
    visibility_condition_json: { operator: 'equals', field_key: 'route_choice', value: 'direct' },
    validation_json: { required_when: { operator: 'equals', field_key: 'route_choice', value: 'direct' } },
  },
];

test('form rules are driven by arbitrary field keys and values', () => {
  const directFields = effectiveWorkflowFormFields(fields, { route_choice: 'direct' });
  assert.equal(directFields.length, 2);
  assert.equal(directFields[1].required, true);

  const surveyFields = effectiveWorkflowFormFields(fields, { route_choice: 'survey' });
  assert.deepEqual(surveyFields.map((field) => field.field_key), ['route_choice']);
});
