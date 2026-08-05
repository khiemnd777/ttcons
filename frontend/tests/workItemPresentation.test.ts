import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkItem } from '../src/lib/types/domain.ts';
import { workItemIsUpcoming, workItemWorkflowContext } from '../src/modules/work-items/workItemContext.ts';

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
