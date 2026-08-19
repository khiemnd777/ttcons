import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import type { WorkflowFormField } from '../src/lib/types/domain.ts';
import {
  workflowSelectOptionChangeValue,
  workflowSelectOptionValue,
  workflowSelectOtherTextChangeValue,
  workflowSelectOtherTextConfig,
  workflowSelectOtherTextIssue,
  workflowSelectOtherTextValue,
  workflowSelectSupportsOtherText,
} from '../src/modules/work-items/workflowSelectOtherText.ts';

const supportedPairs = [
  ['client_design_style_brief', 'style_direction'],
  ['client_survey_work_package', 'style_direction'],
  ['client_boq_owner_submission', 'sent_channel'],
  ['client_hardcopy_dispatch', 'delivery_method'],
  ['builtin_construction_daily_report', 'construction_scope'],
  ['builtin_construction_daily_report', 'weather'],
  ['builtin_subcontractor_checklist', 'work_package'],
  ['client_subcontractor_acceptance_package', 'work_package'],
  ['builtin_daily_report', 'planning_scope'],
  ['builtin_checklist', 'planning_scope'],
  ['client_owner_soft_file_submission', 'delivery_channel'],
  ['client_procurement_takeoff_review', 'takeoff_source'],
] as const;

function selectField(fieldKey: string, includeOther = true): WorkflowFormField {
  return {
    form_version_id: 'version',
    field_key: fieldKey,
    label: fieldKey,
    field_type: 'select',
    required: true,
    options_json: [
      { value: 'catalog', label: 'Trong danh mục' },
      ...(includeOther ? [{ value: 'other', label: 'Khác' }] : []),
    ],
  };
}

test('supports free text only for the exact reviewed template and field pairs', () => {
  for (const [templateCode, fieldKey] of supportedPairs) {
    const field = selectField(fieldKey);
    assert.equal(workflowSelectSupportsOtherText(templateCode, field), true, `${templateCode}.${fieldKey}`);
    assert.equal(workflowSelectOtherTextConfig(templateCode, field)?.maxLength, 300);
  }

  assert.equal(workflowSelectSupportsOtherText('unrelated_template', selectField('style_direction')), false);
  assert.equal(workflowSelectSupportsOtherText('client_survey_work_package', selectField('unrelated_field')), false);
  assert.equal(workflowSelectSupportsOtherText('client_survey_work_package', selectField('style_direction', false)), false);
});

test('keeps the select on Khác while storing and restoring its custom text', () => {
  const templateCode = 'client_survey_work_package';
  const field = selectField('style_direction');

  assert.equal(workflowSelectOptionValue(templateCode, field, 'other'), 'other');
  assert.equal(workflowSelectOtherTextValue(templateCode, field, 'other'), '');
  assert.equal(workflowSelectOtherTextIssue(templateCode, field, 'other'), 'phong cách khác là bắt buộc');

  const customValue = workflowSelectOtherTextChangeValue(templateCode, field, 'Japandi tông gỗ sáng');
  assert.equal(customValue, 'Japandi tông gỗ sáng');
  assert.equal(workflowSelectOptionValue(templateCode, field, customValue), 'other');
  assert.equal(workflowSelectOtherTextValue(templateCode, field, customValue), 'Japandi tông gỗ sáng');
  assert.equal(workflowSelectOtherTextIssue(templateCode, field, customValue), '');
  assert.equal(workflowSelectOptionChangeValue(templateCode, field, customValue, 'catalog'), 'catalog');
  assert.equal(workflowSelectOptionChangeValue(templateCode, field, customValue, 'other'), customValue);
  assert.match(workflowSelectOtherTextIssue(templateCode, field, 'x'.repeat(301)), /300 ký tự/);
});

test('workflow form panel renders the configured conditional text input', async () => {
  const panel = await readFile(new URL('../src/modules/work-items/WorkflowFormPanel.tsx', import.meta.url), 'utf8');

  assert.match(panel, /workflowSelectOtherTextConfig\(templateCode, field\)/);
  assert.match(panel, /\{otherTextConfig\?\.inputLabel\}/);
  assert.match(panel, /placeholder=\{otherTextConfig\?\.placeholder\}/);
  assert.match(panel, /maxLength=\{otherTextConfig\?\.maxLength\}/);
  assert.match(panel, /aria-invalid=\{invalid \|\| undefined\}/);
});
