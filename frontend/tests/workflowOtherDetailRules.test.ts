import assert from 'node:assert/strict';
import test from 'node:test';
import {
  workflowStructuredOtherDetailRequired,
  workflowTopLevelOtherDetailDependentKeys,
  workflowTopLevelOtherDetailRequired,
} from '../src/modules/work-items/workflowOtherDetailRules.ts';

test('requires the existing owner-checklist scope detail only when its scope is Khác', () => {
  assert.equal(workflowTopLevelOtherDetailRequired(
    'builtin_owner_client_checklist',
    'scope_detail',
    { handover_scope: 'other' },
  ), true);
  assert.equal(workflowTopLevelOtherDetailRequired(
    'builtin_owner_client_checklist',
    'scope_detail',
    { handover_scope: 'area' },
  ), false);
  assert.equal(workflowTopLevelOtherDetailRequired(
    'unrelated_template',
    'scope_detail',
    { handover_scope: 'other' },
  ), false);
  assert.deepEqual(
    workflowTopLevelOtherDetailDependentKeys('builtin_owner_client_checklist', 'handover_scope'),
    ['scope_detail'],
  );
  assert.deepEqual(workflowTopLevelOtherDetailDependentKeys('unrelated_template', 'handover_scope'), []);
});

test('requires the existing material note only for construction-report rows whose source is Khác', () => {
  assert.equal(workflowStructuredOtherDetailRequired(
    'builtin_construction_daily_report',
    'material_usage_items',
    { source: 'other' },
    'note',
  ), true);
  assert.equal(workflowStructuredOtherDetailRequired(
    'builtin_construction_daily_report',
    'material_usage_items',
    { source: 'site_balance' },
    'note',
  ), false);
  assert.equal(workflowStructuredOtherDetailRequired(
    'builtin_construction_daily_report',
    'blockers',
    { impact: 'other' },
    'note',
  ), false);
});
