import assert from 'node:assert/strict';
import test from 'node:test';
import type { ApprovalRequest } from '../src/lib/types/domain.ts';
import { approvalCanDecide, approvalDecisionLabel, approvalRequiresComment } from '../src/components/approvalPresentation.ts';

function approval(values: Partial<ApprovalRequest> = {}): ApprovalRequest {
  return {
    id: 'approval-1',
    project_id: 'project-1',
    target_type: 'pipeline_run_node',
    target_id: 'node-1',
    target_name: 'Họp báo cáo khảo sát',
    status: 'pending',
    capabilities: { can_decide: true },
    available_actions: ['approve', 'request-change', 'reject'],
    decision_context: {
      current_stage: 0,
      required_roles: ['PM'],
      quorum: 1,
      comment_required: true,
    },
    ...values,
  };
}

test('uses server capabilities and actions as the decision source of truth', () => {
  assert.equal(approvalCanDecide(approval()), true);
  assert.equal(approvalCanDecide(approval({ capabilities: { can_decide: false } })), false);
  assert.equal(approvalCanDecide(approval({ available_actions: [] })), false);
});

test('presents the current policy stage without inferring actor permissions', () => {
  assert.equal(approvalDecisionLabel(approval()), 'Stage 1 · PM · cần 1 phê duyệt');
  assert.equal(approvalRequiresComment(approval()), true);
  assert.equal(approvalRequiresComment(approval({ decision_context: { comment_required: false } })), false);
});
