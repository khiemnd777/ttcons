import assert from 'node:assert/strict';
import test from 'node:test';
import { appRoleScopeLabel, isAssignableAppRole, pipelineActorRoleOptions } from '../src/lib/utils/roles.ts';

test('external-only roles are not assignable App roles', () => {
  assert.equal(isAssignableAppRole({ external_only: true }), false);
  assert.equal(isAssignableAppRole({ external_only: false }), true);
  assert.equal(appRoleScopeLabel({ external_only: true }), 'Đối tác ngoài App');
});

test('pipeline actor choices exclude the client business counterpart', () => {
  assert.equal(pipelineActorRoleOptions.includes('Chủ đầu tư'), false);
  assert.equal(pipelineActorRoleOptions.includes('PM'), true);
  assert.equal(pipelineActorRoleOptions.includes('QA/QC'), true);
});
