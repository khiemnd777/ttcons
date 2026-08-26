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

  assert.match(page, /import \{ roleSummary as summarizeRoles \} from '\.\.\/\.\.\/lib\/utils\/roles'/);
  assert.match(page, /const roleSummary = summarizeRoles\(user\?\.roles\)/);
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

  assert.match(page, /const workTabCount = workItems\.isLoading[\s\S]*workItems\.isError && !firstPage[\s\S]*\(firstPage\?\.total \?\? 0\) \+ workflowActionTotal/);
  assert.match(page, /label="Công việc" count=\{workTabCount\}/);
  assert.doesNotMatch(page, /label="Công việc" count=\{summary\.total\}/);
});

test('My Work defaults to personal responsibility and keeps approval out of ordinary scope', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const scopeOptions = page.slice(page.indexOf('const scopes'), page.indexOf('const attentionQueues'));

  assert.match(page, /type WorkScopeKey = 'responsible' \| 'mine'/);
  assert.match(page, /key: 'responsible', label: 'Tôi phụ trách'/);
  assert.match(page, /key: 'all', label: 'Tất cả được phép xem'/);
  assert.doesNotMatch(scopeOptions, /key: 'mine'|Giao trực tiếp|pending_approval/);
  assert.match(page, /\(requestedQueue === 'pending_approval' \|\| requestedQueue === 'mine'\) \? 'responsible' : requestedQueue/);
  assert.match(page, /if \(requestedQueue !== 'mine'\) return;[\s\S]*next\.delete\('queue'\);[\s\S]*replace: true/);
  assert.match(page, /key === 'queue' && value === 'responsible'/);
  assert.match(page, /updateParams\(\{ queue: 'responsible', bucket: 'need_action'/);
  assert.match(page, /function selectBucket\(nextBucket: WorkQuickBucket\) \{[\s\S]*?bucket: nextBucket/);
  assert.match(page, /<ScopeRail\s+value=\{queue\}/);
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
  assert.match(quickFilters, /onClick=\{\(\) => onSelect\('all'\)\}/);
  assert.match(quickFilters, /Xem tất cả/);
  assert.match(quickFilters, /summary\.total/);
  assert.doesNotMatch(quickFilters.slice(quickFilters.indexOf('const cards'), quickFilters.indexOf('return (')), /bucket: 'all'/);
  assert.match(quickFilters, /bucket: 'upcoming', label: 'Sắp tới · chỉ xem'/);
  assert.match(quickFilters, /bucket: 'completed', label: 'Hoàn tất', value: summary\.completed/);
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
  assert.match(page, /<WorkReportReviewLane/);
  assert.match(page, /get<WorkReportSummary>\(`\/work-reports\/summary/);
  assert.match(page, /get<WorkReportListItem\[]>\(`\/work-reports\?/);
  assert.match(page, /summary\.waiting_decision \+ nodeApprovalCount \+ reportReviewCount/);
  assert.match(page, /navigate\(`\/reports\?scope=review&report=\$\{report\.source_id\}`\)/);
  assert.match(page, /searchParams\.get\('approval'\)/);
  assert.match(page, /data-approval-id=\{request\.id\}/);
});

test('My Work splits approval sources into URL-backed responsive tabs', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const workspace = page.slice(page.indexOf('function ApprovalWorkspace('), page.indexOf('function NodeApprovalLane('));

  assert.match(page, /type ApprovalLaneKey = 'work' \| 'node' \| 'report'/);
  assert.match(page, /searchParams\.get\('approval_type'\)/);
  assert.match(page, /requestedApproval\s*\? 'node'/);
  assert.match(page, /function firstPopulatedApprovalLane[\s\S]*workCount > 0[\s\S]*nodeCount > 0[\s\S]*reportVisible && reportCount > 0/);
  assert.match(page, /updateParams\(\{ approval_type: value, approval: value === 'node' \? requestedApproval \|\| null : null \}\)/);
  assert.match(workspace, /role="tablist" aria-label="Nhóm yêu cầu chờ duyệt"/);
  assert.match(workspace, /lane="work"[\s\S]*label="Phê duyệt công việc"[\s\S]*count=\{workCount\}/);
  assert.match(workspace, /lane="node"[\s\S]*label="Phê duyệt giai đoạn quy trình"[\s\S]*count=\{nodeCount\}/);
  assert.match(workspace, /reportVisible \? <ApprovalLaneTab lane="report"[\s\S]*label="Báo cáo chờ xác nhận"/);
  assert.match(workspace, /activeLane === 'work'[\s\S]*<WorkList[\s\S]*<NextWorkPanel/);
  assert.match(workspace, /activeLane === 'node'[\s\S]*<NodeApprovalLane/);
  assert.match(workspace, /reportVisible && activeLane === 'report'[\s\S]*<WorkReportReviewLane/);
  assert.match(workspace, /overflow-x-auto border-b border-line/);
});

test('My Work separates scope, list, and selected-work actions responsively', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /xl:grid-cols-\[208px_minmax\(0,1fr\)_320px\]/);
  assert.match(page, /function ScopeRail\(/);
  assert.match(page, /aria-label="Phạm vi công việc"/);
  assert.match(page, /overflow-x-auto[\s\S]*xl:flex-col/);
  assert.match(page, /<div className="min-w-0">[\s\S]*<FilterBar[\s\S]*<WorkList/);
  assert.match(page, /<NextQueuePanel[\s\S]*item=\{selectedItem\}[\s\S]*workflowAction=\{selectedWorkflowAction\}/);
});

test('My Work scope rail keeps only concise scope labels', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const scopeRail = page.slice(page.indexOf('function ScopeRail('), page.indexOf('function FilterBar('));

  assert.match(page, /const scopes: Array<\{ key: WorkScopeKey; label: string \}>/);
  assert.doesNotMatch(page, /Chọn nhóm công việc cần tập trung\./);
  assert.doesNotMatch(scopeRail, /item\.description/);
  assert.doesNotMatch(page, /Giao trực tiếp, theo vai trò hoặc nhóm phòng ban chưa có người nhận/);
});

test('My Work scope rail shows only positive need-action counts as chips', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const realtime = await source('../src/modules/notifications/ActorRealtimeProvider.tsx');
  const scopeRail = page.slice(page.indexOf('function ScopeCountChip('), page.indexOf('function FilterBar('));

  assert.match(page, /type MyWorkScopeSummary = Record<WorkScopeKey, MyWorkScopeSummaryRow>/);
  assert.match(page, /queryKey: \['my-work-scope-summary', user\?\.id, project\]/);
  assert.match(page, /get<MyWorkScopeSummary>\(buildWorkScopeSummaryPath\(project\)\)/);
  assert.match(page, /<ScopeRail[\s\S]*counts=\{scopeSummary\.data\}[\s\S]*loading=\{scopeSummary\.isLoading\}/);
  assert.match(scopeRail, /function ScopeCountChip[\s\S]*if \(count <= 0\) return null/);
  assert.match(scopeRail, /rounded-full[\s\S]*active \? 'bg-blue-600 text-white ring-blue-600' : 'bg-slate-100 text-slate-600 ring-slate-200'/);
  assert.match(scopeRail, /<ScopeCountChip count=\{countValue\(item\.key\)\} active=\{active\} \/>/);
  assert.doesNotMatch(scopeRail, /countLabel|\? '…'|\? '—'/);
  assert.match(scopeRail, /tabular-nums/);
  assert.match(realtime, /\['my-work-scope-summary'\]/);
});

test('My Work isolates actor queries and never presents placeholder zeroes as loaded quick-filter values', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');

  assert.match(page, /queryKey: \['my-work-items', user\?\.id, filterKey\]/);
  assert.match(page, /queryKey: \['my-workflow-actions', user\?\.id, queue, project, search\]/);
  assert.match(page, /queryKey: \['approvals', 'pending', 'decidable', user\?\.id, project\]/);
  assert.match(page, /queryKey: \['activity-attention', 'summary', user\?\.id, project\]/);
  assert.match(page, /queryKey: \['activity-attention', 'list', user\?\.id, project\]/);
  assert.match(page, /<WorkQuickFilters[\s\S]*loading=\{workItems\.isLoading \|\| workflowActions\.isLoading \|\| startupActions\.isLoading\}[\s\S]*unavailable=\{\(workItems\.isError && !firstPage\)/);
  assert.match(page, /\{loading \? '…' : unavailable \? '—' : card\.value\}/);
});

test('My Work surfaces generic ready stage transitions as one-click business actions', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const domain = await source('../src/lib/types/domain.ts');
  const workList = page.slice(page.indexOf('function WorkList('), page.indexOf('function WorkListSkeleton('));

  assert.match(domain, /export type MyWorkflowAction =/);
  assert.match(page, /return `\/me\/workflow-actions\?\$\{params\.toString\(\)\}`/);
  assert.match(page, /post\(`\/pipeline-run-nodes\/\$\{action\.run_node_id\}\/advance`, \{\}\)/);
  assert.match(page, /summary\.need_action \+ workflowActionSummaryTotal/);
  assert.match(workList, /workflowActions\.map[\s\S]*<WorkflowActionCard[\s\S]*rows\.map/);
  assert.match(page, /Cần chốt giai đoạn/);
  assert.match(page, /Đã đủ điều kiện để tiếp tục quy trình\./);
  assert.match(page, /function NextQueuePanel\(/);
  assert.match(page, /pending \? 'Đang chuyển…' : action\.action_label/);
  assert.doesNotMatch(page, /Xác nhận chuyển giai đoạn/);
});

test('My Work coordinates project startup without creating placeholder work items', async () => {
  const page = await source('../src/modules/my-work/MyWorkPage.tsx');
  const projects = await source('../src/modules/projects/ProjectsPage.tsx');
  const wizard = await source('../src/modules/projects/ProjectStartupWizard.tsx');
  const realtime = await source('../src/modules/notifications/ActorRealtimeProvider.tsx');

  assert.match(page, /queryKey: \['my-project-startup-actions', user\?\.id, project, search\]/);
  assert.match(page, /searchParams\.get\('setup'\)/);
  assert.match(page, /function ProjectStartupActionCard\(/);
  assert.match(page, /Cần thiết lập công trình/);
  assert.match(page, /Sắp tới · chỉ xem/);
  assert.match(page, /<ProjectStartupWizard/);
  assert.match(projects, /<ProjectStartupWizard/);
  assert.match(wizard, /\/projects\/\$\{activeProjectID\}\/startup-readiness/);
  assert.match(wizard, /\/projects\/\$\{activeProjectID\}\/project-manager/);
  assert.match(wizard, /\/projects\/\$\{activeProjectID\}\/start-pipeline/);
  assert.match(wizard, /Khi bắt đầu, hệ thống mới tạo Work Item thật/);
  assert.doesNotMatch(projects, /navigate\(`\/my-work\?setup=/);
  assert.match(realtime, /\['my-project-startup-actions'\]/);
});
