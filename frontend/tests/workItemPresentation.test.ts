import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkItem } from '../src/lib/types/domain.ts';
import { workItemIsUpcoming, workItemWorkflowContext } from '../src/modules/work-items/workItemContext.ts';
import {
  completionRequirementPresentation,
  missingCompletionRequirementLabels,
  permissionRequirementPresentation,
} from '../src/modules/work-items/workItemRequirements.ts';

function workItem(values: Partial<WorkItem> = {}): WorkItem {
  return {
    id: 'work-1',
    project_id: 'project-1',
    title: 'Kiểm tra cốt thép',
    status: 'pending',
    ...values,
  } as WorkItem;
}

test('always presents the complete project, node and step path', () => {
  const context = workItemWorkflowContext(workItem({
    project_code: 'DA-01',
    project_name: 'Tòa nhà A',
    run_node_order_index: 2,
    run_node_name: 'Thi công phần thân',
    run_step_order_index: 3,
    run_step_name: 'Nghiệm thu cốt thép',
  }));

  assert.deepEqual(context, {
    project: 'DA-01 · Tòa nhà A',
    node: 'Node 2: Thi công phần thân',
    step: 'Bước 3: Nghiệm thu cốt thép',
    path: 'DA-01 · Tòa nhà A › Node 2: Thi công phần thân › Bước 3: Nghiệm thu cốt thép',
  });
});

test('keeps non-workflow items explicit instead of dropping a context level', () => {
  const context = workItemWorkflowContext(workItem({ work_type: 'task' }));
  assert.equal(context.project, 'Dự án chưa xác định');
  assert.equal(context.node, 'Node: Ngoài workflow');
  assert.match(context.step, /^Bước: /);
  assert.equal(context.path.split(' › ').length, 3);
});

test('uses server readiness as the future read-only source of truth', () => {
  assert.equal(workItemIsUpcoming(workItem({ readiness: 'upcoming' })), true);
  assert.equal(workItemIsUpcoming(workItem({ readiness: 'actionable' })), false);
});

test('uses backend completion requirements instead of inferring checklist evidence', () => {
  const item = workItem({
    completion_requirements: [
      { key: 'workflow_form', label: 'Hồ sơ nghiệp vụ bắt buộc', required: true, status: 'satisfied' },
      { key: 'checklist', label: 'Checklist của bước', required: true, status: 'satisfied', satisfied_by: 'workflow_form' },
      { key: 'document', label: 'Tài liệu', required: true, status: 'missing' },
    ],
  });

  assert.deepEqual(missingCompletionRequirementLabels(item), ['Tài liệu']);
});

test('maps a missing workflow form requirement to the first incomplete form control', () => {
  const item = workItem({
    workflow_form_instance_id: 'form-1',
    completion_requirements: [
      { key: 'workflow_form', label: 'Hồ sơ nghiệp vụ bắt buộc', required: true, status: 'missing' },
    ],
  });

  const presentation = completionRequirementPresentation(item, item.completion_requirements![0]);

  assert.equal(presentation.actionLabel, 'Hoàn tất hồ sơ');
  assert.deepEqual(presentation.target, { kind: 'workflow_form', focus: 'first_issue' });
  assert.match(presentation.guidance, /Chốt hồ sơ/);
});

test('does not expose a workflow-form action when detail has no rendered form surface', () => {
  const item = workItem({
    workflow_form_required: true,
    completion_requirements: [
      { key: 'workflow_form', label: 'Hồ sơ nghiệp vụ bắt buộc', required: true, status: 'missing' },
    ],
  });

  const presentation = completionRequirementPresentation(item, item.completion_requirements![0]);

  assert.equal(presentation.target, undefined);
  assert.match(presentation.guidance, /kiểm tra form\/version/);
});

test('opens the evidence uploader with the backend-required evidence type', () => {
  const item = workItem({
    completion_requirements: [
      { key: 'document', label: 'Tài liệu', required: true, status: 'missing' },
    ],
  });

  const presentation = completionRequirementPresentation(item, item.completion_requirements![0]);

  assert.equal(presentation.actionLabel, 'Tải tài liệu');
  assert.deepEqual(presentation.target, { kind: 'evidence', evidenceType: 'document', openUpload: true });
});

test('keeps read-only evidence navigation view-only', () => {
  const item = workItem({
    completion_requirements: [
      { key: 'photo', label: 'Hình ảnh', required: true, status: 'missing' },
    ],
  });

  const presentation = completionRequirementPresentation(item, item.completion_requirements![0], true);

  assert.equal(presentation.actionLabel, 'Xem minh chứng');
  assert.deepEqual(presentation.target, { kind: 'evidence', evidenceType: 'photo', openUpload: false });
});

test('shows instructions without a fake target when no screen control owns the requirement', () => {
  const item = workItem({
    completion_requirements: [
      { key: 'future_gate', label: 'Điều kiện mở rộng', required: true, status: 'missing' },
    ],
  });

  const presentation = completionRequirementPresentation(item, item.completion_requirements![0]);

  assert.equal(presentation.target, undefined);
  assert.equal(presentation.actionLabel, undefined);
  assert.match(presentation.guidance, /Chưa có đích thao tác/);
});

test('explains a checklist satisfied by completion payload without inventing a form field', () => {
  const item = workItem({
    completion_requirements: [
      { key: 'checklist', label: 'Checklist', required: true, status: 'satisfied', satisfied_by: 'completion_payload' },
    ],
  });

  const presentation = completionRequirementPresentation(item, item.completion_requirements![0]);

  assert.equal(presentation.target, undefined);
  assert.match(presentation.guidance, /Không có field riêng/);
});

test('presents a work-item permission with source-backed Vietnamese metadata', () => {
  const presentation = permissionRequirementPresentation(workItem({
    evidence_requirement_json: { required_permission: 'planning.edit' },
    required_permission_name: 'Cập nhật kế hoạch',
    required_permission_description: 'Cập nhật nội dung kế hoạch được phân công',
  }));

  assert.equal(presentation.description, 'Cần quyền: Cập nhật kế hoạch.');
  assert.equal(
    presentation.guidance,
    'Liên hệ Quản lý dự án hoặc Admin để kiểm tra quyền. Phạm vi quyền: Cập nhật nội dung kế hoạch được phân công.',
  );
  assert.doesNotMatch(`${presentation.description} ${presentation.guidance}`, /planning\.edit/);
});

test('hides an unknown permission key behind actionable generic guidance', () => {
  const presentation = permissionRequirementPresentation(workItem({
    evidence_requirement_json: { required_permission: 'future.unknown' },
  }));

  assert.deepEqual(presentation, {
    description: 'Cần quyền thao tác đầu việc.',
    guidance: 'Liên hệ Quản lý dự án hoặc Admin để kiểm tra quyền cần cấp.',
  });
  assert.doesNotMatch(`${presentation.description} ${presentation.guidance}`, /future\.unknown/);
});
