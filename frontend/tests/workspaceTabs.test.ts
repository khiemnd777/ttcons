import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('shared workspace tabs own the flat member-tab presentation and accessible state', async () => {
  const tabs = await source('../src/components/WorkspaceTabs.tsx');

  assert.match(tabs, /role="tablist" aria-label=\{ariaLabel\}/);
  assert.match(tabs, /role="tab"/);
  assert.match(tabs, /aria-selected=\{active\}/);
  assert.match(tabs, /border-b-2 px-3 py-2 text-sm font-semibold/);
  assert.match(tabs, /active \? 'border-blue-600 text-blue-700'/);
  assert.match(tabs, /border-transparent text-slate-600 hover:border-slate-300 hover:text-ink/);
  assert.match(tabs, /<Icon className="h-4 w-4 shrink-0" aria-hidden="true" \/>/);
  assert.match(tabs, /typeof tab\.count === 'number' \|\| typeof tab\.count === 'string'/);
  assert.match(tabs, /overflow-x-auto/);
  assert.match(tabs, /flex min-w-max gap-1/);
});

test('project and domain workspaces reuse the same workspace tab owner', async () => {
  const consumers = await Promise.all([
    source('../src/modules/projects/ProjectDetailPage.tsx'),
    source('../src/modules/boq/BOQPage.tsx'),
    source('../src/modules/procurement/ProcurementPage.tsx'),
    source('../src/modules/warehouse/WarehousePage.tsx'),
    source('../src/modules/operations/OperationsPage.tsx'),
    source('../src/modules/project-finance/AccountingHandoffPage.tsx'),
    source('../src/modules/planning/PreparationPage.tsx'),
    source('../src/modules/planning/PlanningPage.tsx'),
  ]);

  for (const consumer of consumers) {
    assert.match(consumer, /import \{ WorkspaceTabs/);
    assert.match(consumer, /<WorkspaceTabs/);
  }

  assert.doesNotMatch(consumers[1], /function TabButton\(/);
  assert.doesNotMatch(consumers[2], /function ViewButton\(/);
  assert.doesNotMatch(consumers[3], /function TabButton\(/);
  assert.equal((consumers[0].match(/<WorkspaceTabs/g) ?? []).length, 2);
  assert.match(consumers[0], /id: 'open', label: 'Cần xử lý', icon: AlertTriangle, count: counts\.open/);
  assert.match(consumers[0], /id: 'blocked', label: 'Bị chặn', icon: ShieldAlert, count: counts\.blocked/);
  assert.match(consumers[0], /id: 'skipped', label: 'Đã bỏ qua', icon: Ban, count: counts\.skipped/);
  assert.doesNotMatch(consumers[0], /aria-pressed=\{filter === option\.id\}/);
  assert.match(consumers[0], /const \[filter, setFilter\] = useState<WorkflowWorkFilter>\('open'\)/);
  assert.match(consumers[0], /const defaultFilter: WorkflowWorkFilter = selectedNodeSkipped \? 'skipped' : 'open'/);
  assert.match(consumers[0], /setFilter\(defaultFilter\);\s*setSearch\(''\)/);
  assert.match(consumers[0], /if \(filter === 'waiting'\) return item\.status === 'waiting_approval' \|\| item\.status === 'waiting_review'/);
  assert.match(consumers[0], /return item\.status === 'cancelled'/);
  assert.match(consumers[4], /label: 'Kiểm tra', icon: ClipboardCheck, count: pendingChecks/);
  assert.match(consumers[4], /label: 'Cần xử lý', icon: AlertTriangle, count: activeActions/);
  assert.match(consumers[5], /count: countKey \? data\?\.summary\[countKey\] \?\? 0 : undefined/);
  assert.match(consumers[6], /id: 'need_action', label: 'Cần xử lý', icon: AlertTriangle, count: counts\.needAction/);
  assert.match(consumers[6], /id: 'done', label: 'Hoàn tất', icon: CheckCircle2, count: counts\.done/);
  assert.match(consumers[7], /id: 'waiting', label: 'Chờ duyệt', icon: Send, count: metrics\.waiting/);
  assert.match(consumers[7], /id: 'all', label: 'Tất cả', icon: ListFilter, count: metrics\.total/);
  assert.doesNotMatch(consumers[6], /aria-pressed=\{filter === option\.value\}/);
  assert.doesNotMatch(consumers[7], /aria-pressed=\{queueFilter === tab\.id\}/);
});
