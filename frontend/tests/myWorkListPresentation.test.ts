import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('My Work renders compact semantic rows without nested interactive controls', async () => {
  const card = await source('../src/modules/work-items/WorkItemCard.tsx');
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(card, /aria-pressed=\{selected\}/);
  assert.match(card, /onClick=\{onSelect \?\? onOpen\}/);
  assert.match(card, /aria-label=\{openLabel\}/);
  assert.match(card, /absolute inset-y-2 left-0 w-1 rounded-r bg-blue-600/);
  assert.match(card, /title=\{context\.path\}/);
  assert.match(card, /Hồ sơ \{item\.workflow_form_required \? 'bắt buộc' : 'tùy chọn'\}/);
  assert.match(card, /Điểm chặn: \{item\.blocked_reason\}/);
  assert.doesNotMatch(card, /item\.description && !compact/);
  assert.doesNotMatch(page, /role="button" tabIndex=\{0\}/);
  assert.match(page, /<WorkItemCard key=\{item\.id\} item=\{item\} selected=\{selectedId === item\.id\}/);
});

test('My Work keeps detail copy in the desktop inspector and deduplicates header roles', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /function uniqueRoleNames\(roles\?: string\[\]\)/);
  assert.match(page, /new Set\(\(roles \?\? \[\]\)\.map/);
  assert.match(page, /description=\{`\$\{user\?\.full_name.*\$\{roleSummary/);
  assert.doesNotMatch(page, /user\.roles\.join\(', '\)/);
  assert.match(page, /hidden rounded-lg border border-line bg-white p-4 xl:sticky/);
  assert.match(page, /\{item\.description \? <p className="mt-3 text-sm leading-6 text-slate-600">/);
  assert.doesNotMatch(page, /<ContextRow label="Dự án"/);
});

test('My Work provides compact loading and stale-data states', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /function WorkListSkeleton\(\)/);
  assert.match(page, /Đang tải danh sách công việc/);
  assert.match(page, /Chưa cập nhật được dữ liệu mới; đang giữ danh sách gần nhất\./);
  assert.match(page, />Thử lại<\/button>/);
});

test('My Work tab count follows the currently displayed work query', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /const workTabCount = workItems\.isLoading[\s\S]*workItems\.isError && !firstPage[\s\S]*firstPage\?\.total \?\? 0/);
  assert.match(page, /label="Công việc" count=\{workTabCount\}/);
  assert.doesNotMatch(page, /label="Công việc" count=\{summary\.total\}/);
});

test('My Work defaults to personal responsibility and keeps approval out of ordinary scope', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /type WorkScopeKey = 'responsible' \| 'mine'/);
  assert.match(page, /key: 'responsible', label: 'Tôi phụ trách'/);
  assert.match(page, /key: 'all', label: 'Tất cả được phép xem'/);
  assert.doesNotMatch(page.slice(page.indexOf('const scopes'), page.indexOf('const attentionQueues')), /pending_approval/);
  assert.match(page, /requestedQueue === 'pending_approval' \? 'responsible' : requestedQueue/);
  assert.match(page, /key === 'queue' && value === 'responsible'/);
  assert.match(page, /updateParams\(\{ queue: 'responsible', bucket: 'need_action'/);
  assert.match(page, /function selectBucket\(nextBucket: WorkQuickBucket\) \{[\s\S]*?bucket: nextBucket/);
  assert.match(page, /<ScopeRail value=\{queue\}/);
  assert.doesNotMatch(page, /<FilterSelect value=\{queue\} label="Phạm vi công việc"/);
});

test('My Work presents source-backed quick filters inside the selected work scope', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const workMode = page.slice(page.indexOf("{mode === 'work' ? ("), page.indexOf(") : mode === 'approval' ? ("));
  const quickFilters = page.slice(page.indexOf('function WorkQuickFilters('), page.indexOf('function ModeTab('));

  assert.match(workMode, /<ScopeRail[\s\S]*<WorkQuickFilters[\s\S]*<FilterBar/);
  assert.doesNotMatch(page.slice(page.indexOf('<PageHeader'), page.indexOf('<div className="mt-4 flex')), /<WorkQuickFilters/);
  assert.match(quickFilters, /aria-label="Bộ lọc nhanh công việc"/);
  assert.match(quickFilters, /Bộ lọc nhanh · \{scopeLabel\}/);
  assert.match(quickFilters, /bucket: 'all', label: 'Tất cả', value: summary\.total/);
  assert.match(quickFilters, /bucket: 'upcoming', label: 'Sắp tới · chỉ xem'/);
  assert.doesNotMatch(quickFilters, /waiting_decision|Chờ tôi duyệt/);
});

test('My Work separates actor-decidable approvals into a dedicated compatible mode', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /type ModeKey = 'work' \| 'approval' \| 'attention'/);
  assert.match(page, /legacyApprovalMode = requestedQueue === 'pending_approval' \|\| requestedBucket === 'waiting_decision'/);
  assert.match(page, /label="Chờ tôi duyệt" count=\{approvalTabCount\}/);
  assert.match(page, /mode === 'approval'[\s\S]*<ApprovalWorkspace/);
  assert.match(page, /bucket: 'waiting_decision'/);
  assert.match(page, /<NodeApprovalLane/);
  assert.match(page, /Không có yêu cầu nào đang chờ anh duyệt\./);
});

test('My Work separates scope, list, and selected-work actions responsively', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /xl:grid-cols-\[208px_minmax\(0,1fr\)_320px\]/);
  assert.match(page, /function ScopeRail\(/);
  assert.match(page, /aria-label="Phạm vi công việc"/);
  assert.match(page, /overflow-x-auto[\s\S]*xl:flex-col/);
  assert.match(page, /<div className="min-w-0">[\s\S]*<FilterBar[\s\S]*<WorkList/);
  assert.match(page, /<NextWorkPanel item=\{selectedItem\}/);
});

test('My Work scope rail keeps only concise scope labels', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const scopeRail = page.slice(page.indexOf('function ScopeRail('), page.indexOf('function FilterBar('));

  assert.match(page, /const scopes: Array<\{ key: WorkScopeKey; label: string \}>/);
  assert.doesNotMatch(page, /Chọn nhóm công việc cần tập trung\./);
  assert.doesNotMatch(scopeRail, /item\.description/);
  assert.doesNotMatch(page, /Giao trực tiếp, theo vai trò hoặc nhóm phòng ban chưa có người nhận/);
});

test('My Work isolates actor queries and never presents placeholder zeroes as loaded quick-filter values', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /queryKey: \['my-work-items', user\?\.id, filterKey\]/);
  assert.match(page, /queryKey: \['approvals', 'pending', 'decidable', user\?\.id, project\]/);
  assert.match(page, /queryKey: \['activity-attention', 'summary', user\?\.id, project\]/);
  assert.match(page, /queryKey: \['activity-attention', 'list', user\?\.id, project\]/);
  assert.match(page, /<WorkQuickFilters[\s\S]*loading=\{workItems\.isLoading\}[\s\S]*unavailable=\{workItems\.isError && !firstPage\}/);
  assert.match(page, /\{loading \? '…' : unavailable \? '—' : card\.value\}/);
});
