import assert from 'node:assert/strict';
import test from 'node:test';
import {
  appRoleDisplayName,
  appRoleChoices,
  pipelineActorRoleChoices,
  appRoleScopeLabel,
  isAssignableAppRole,
  pipelineActorRoleOptions,
  roleSummary,
  uniqueRoleNames,
} from '../src/lib/utils/roles.ts';

test('external-only roles are not assignable App roles', () => {
  assert.equal(isAssignableAppRole({ external_only: true }), false);
  assert.equal(isAssignableAppRole({ external_only: false }), true);
  assert.equal(appRoleScopeLabel({ external_only: true }), 'Đối tác ngoài App');
});

test('pipeline actor choices exclude the client business counterpart', () => {
  assert.equal(pipelineActorRoleOptions.includes('Chủ đầu tư'), false);
  assert.equal(pipelineActorRoleOptions.includes('PM'), true);
  assert.equal(pipelineActorRoleOptions.includes('Phó Quản Lý'), true);
  assert.equal(pipelineActorRoleOptions.includes('QA/QC'), true);
});

test('account role summaries deduplicate and normalize display names', () => {
  assert.deepEqual(uniqueRoleNames([' PM ', 'PM', '', 'QS', ' QS ']), ['Quản lý dự án', 'QS']);
  assert.equal(roleSummary([' PM ', 'PM', '', 'QS', ' QS ']), 'Quản lý dự án, QS');
  assert.equal(roleSummary(), '');
});

test('Project Manager aliases display distinctly from Deputy Manager without changing stored choices', () => {
  for (const value of ['PM', 'pm', ' Project Manager ', 'Quản lý dự án']) {
    assert.equal(appRoleDisplayName(value), 'Quản lý dự án');
  }
  assert.equal(appRoleDisplayName('Phó Quản Lý'), 'Phó Quản Lý');
  assert.equal(appRoleDisplayName('deputy_manager'), 'deputy_manager');
  assert.deepEqual(pipelineActorRoleChoices.find((choice) => choice.value === 'PM'), { value: 'PM', label: 'Quản lý dự án' });
  assert.equal(roleSummary(['PM', 'Quản lý dự án', 'Phó Quản Lý']), 'Quản lý dự án, Phó Quản Lý');
  assert.deepEqual(appRoleChoices(['Quản lý dự án', 'PM', 'QS'], ['PM']), [
    { value: 'PM', label: 'Quản lý dự án' }, { value: 'QS', label: 'QS' },
  ]);
});
