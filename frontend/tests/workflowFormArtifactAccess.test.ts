import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkflowFormDetail } from '../src/lib/types/domain.ts';
import { workflowFormArtifactAccess, workflowFormNextAction } from '../src/modules/work-items/workflowFormArtifactAccess.ts';

function form(status: string, capabilities: WorkflowFormDetail['capabilities'] = {}): WorkflowFormDetail {
  return {
    available: true,
    instance: {
      id: 'form',
      project_id: 'project',
      work_item_id: 'task',
      form_type: 'task',
      title: 'Hồ sơ',
      status,
    },
    capabilities,
  };
}

test('submitted workflow forms use backend-owned artifact capabilities', () => {
  assert.deepEqual(workflowFormArtifactAccess(form('submitted', {
    can_add_confirmation: true,
    can_generate_pdf: true,
    can_delete_pdf: false,
  })), {
    canAddConfirmation: true,
    canGeneratePDF: true,
    canDeletePDF: false,
  });
  assert.deepEqual(workflowFormArtifactAccess(form('draft', {
    can_add_confirmation: true,
    can_generate_pdf: true,
    can_delete_pdf: true,
  })), {
    canAddConfirmation: false,
    canGeneratePDF: false,
    canDeletePDF: false,
  });
  assert.deepEqual(workflowFormArtifactAccess(form('submitted')), {
    canAddConfirmation: false,
    canGeneratePDF: false,
    canDeletePDF: false,
  });
});

test('next-action copy never advertises a blocked artifact mutation', () => {
  const locked = workflowFormArtifactAccess(form('submitted'));
  assert.equal(workflowFormNextAction({ submitted: true, validationIssueCount: 0, confirmationCount: 0, pdfCount: 0, access: locked }), 'Hồ sơ đã chốt · Chỉ xem');

  const enabled = workflowFormArtifactAccess(form('submitted', {
    can_add_confirmation: true,
    can_generate_pdf: true,
  }));
  assert.equal(workflowFormNextAction({ submitted: true, validationIssueCount: 0, confirmationCount: 0, pdfCount: 0, access: enabled }), 'Thêm xác nhận hồ sơ');
  assert.equal(workflowFormNextAction({ submitted: true, validationIssueCount: 0, confirmationCount: 1, pdfCount: 0, access: enabled }), 'Tạo PDF');
});
