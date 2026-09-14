import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

test('project-detail tabs pair every project workspace surface with its Lucide icon', async () => {
  const projectDetail = await readFile(new URL('../src/modules/projects/ProjectDetailPage.tsx', import.meta.url), 'utf8');

  assert.match(projectDetail, /\{ id: 'overview', label: 'Tổng quan', icon: LayoutDashboard \}/);
  assert.match(projectDetail, /\{ id: 'workflow', label: 'Quy trình', icon: GitBranch, count: timeline\.length \}/);
  assert.match(projectDetail, /\{ id: 'work', label: 'Danh sách công việc', icon: ListTodo, count: allWorkItems\.length \}/);
  assert.match(projectDetail, /\{ id: 'activity', label: 'Trao đổi', icon: MessageSquare, count: activityDiscussionCount \}/);
  assert.match(projectDetail, /\{ id: 'files', label: 'Hồ sơ', icon: FileText, count: numberValue\(data\.evidence_summary\?\.total \?\? latestEvidence\.length\) \}/);
  assert.match(projectDetail, /\{ id: 'audit', label: 'Nhật ký', icon: ScrollText \}/);
  assert.match(projectDetail, /\{ id: 'members', label: 'Thành viên', icon: Users, count: members\.data\?\.summary\.active \?\? 0 \}/);
  assert.match(projectDetail, /<Icon className="h-4 w-4 shrink-0" aria-hidden="true" \/>/);
});

test('project overview uses one compact toolbar without repeating project identity or lifecycle actions', async () => {
  const projectDetail = await readFile(new URL('../src/modules/projects/ProjectDetailPage.tsx', import.meta.url), 'utf8');

  assert.match(projectDetail, /<h1 className="sr-only">\{activeTabLabel\} · \{project\.code/);
  assert.match(projectDetail, /data-project-overview-toolbar/);
  assert.match(projectDetail, /role="group" aria-label="Nội dung công trình"/);
  assert.match(projectDetail, /aria-pressed=\{active\}/);
  assert.match(projectDetail, /actions=\{activeTab === 'workflow' \? undefined/);
  assert.match(projectDetail, />\s*Mở kế hoạch\s*<\/Link>/);
  assert.match(projectDetail, /<ProjectOverviewActionBand/);
  assert.doesNotMatch(projectDetail, /function HeaderActions/);
  assert.doesNotMatch(projectDetail, /title=\{`\$\{project\.code/);
});
