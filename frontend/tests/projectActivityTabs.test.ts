import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('project discussion route exposes only three activity kinds, opens discussion first, and keeps one page composer', async () => {
  const [thread, workItemDetail, projectDetail, projectDiscussion] = await Promise.all([
    source('../src/modules/work-items/ActivityThread.tsx'),
    source('../src/modules/work-items/WorkItemDetailDialog.tsx'),
    source('../src/modules/projects/ProjectDetailPage.tsx'),
    source('../src/modules/projects/ProjectDiscussionPage.tsx'),
  ]);
  const tabs = thread.slice(thread.indexOf('const kindTabs'), thread.indexOf('const attentionItems'));

  assert.match(thread, /type ActivityKind = 'discussion' \| 'decision' \| 'system';/);
  assert.match(thread, /const \[kind, setKind\] = useState<ActivityKind>\('discussion'\);/);
  assert.match(thread, /params\.set\('kind', kind\);/);
  assert.doesNotMatch(thread, /if \(kind\) params\.set\('kind', kind\);/);
  assert.doesNotMatch(tabs, /Tất cả/);
  assert.match(tabs, /label: 'Thảo luận'/);
  assert.match(tabs, /label: 'Thảo luận', countKey: 'discussion_unread'/);
  assert.match(tabs, /label: 'Quyết định'/);
  assert.match(tabs, /label: 'Nhật ký hệ thống'/);
  assert.equal((thread.match(/<ActivityComposer/g) ?? []).length, 1);
  assert.match(thread, /kind === 'discussion'[\s\S]*variant="chat"/);
  assert.doesNotMatch(thread, /kind !== 'discussion'/);
  assert.match(workItemDetail, /<ActivityComposer[\s\S]*mode="work_item"[\s\S]*defaultExpanded/);
  assert.doesNotMatch(projectDetail, /id: 'activity'/);
  assert.match(projectDiscussion, /<ActivityThread/);
  assert.match(projectDiscussion, /buildActivityAttachmentOptions/);
  assert.doesNotMatch(projectDetail, /activity_summary\?\.discussion\) - numberValue\(data\.activity_summary\?\.decisions/);
  assert.doesNotMatch(projectDetail, /activity_summary\?\.total/);
});

test('project discussion leads with the lifecycle action while preserving the personal work queue', async () => {
  const [thread, actionCard, projectDetail, projectDiscussion] = await Promise.all([
    source('../src/modules/work-items/ActivityThread.tsx'),
    source('../src/modules/work-items/DiscussionWorkflowActionCard.tsx'),
    source('../src/modules/projects/ProjectDetailPage.tsx'),
    source('../src/modules/projects/ProjectDiscussionPage.tsx'),
  ]);

  assert.match(thread, /<DiscussionWorkflowActionCard/);
  assert.ok(thread.indexOf('<DiscussionWorkflowActionCard') < thread.indexOf('<DiscussionWorkRail'));
  assert.match(thread, /onDiscuss=\{discussWork\}/);
  assert.match(actionCard, /queryKey: \['work-item', primaryWorkId\]/);
  assert.match(actionCard, /Hoàn tất thiết lập công trình/);
  assert.match(actionCard, /Trao đổi về việc này/);
  assert.match(actionCard, /discussionWorkflowActionLabel\(currentWork\)/);
  assert.doesNotMatch(actionCard, /\bpost\(|\bput\(/);

  assert.match(projectDiscussion, /<ProjectStartupWizard/);
  assert.match(projectDiscussion, /workflowAction=\{\{/);
  assert.match(projectDiscussion, /currentNode,/);
  assert.match(projectDiscussion, /primaryWork,/);
  assert.doesNotMatch(projectDetail, /post\(`\/projects\/\$\{id\}\/start-pipeline`/);
  assert.match(projectDetail, /<ProjectOverviewActionBand/);
  assert.doesNotMatch(projectDetail, /function HeaderActions/);
});

test('project discussion starts with operational content without a visible local title block', async () => {
  const projectDiscussion = await source('../src/modules/projects/ProjectDiscussionPage.tsx');

  assert.match(projectDiscussion, /<h1 className="sr-only">Trao đổi · \{project\.code/);
  assert.match(projectDiscussion, /<\/h1>\s*<ActivityThread/);
  assert.doesNotMatch(projectDiscussion, /<h2 className="font-semibold text-ink">Trao đổi<\/h2>/);
  assert.doesNotMatch(projectDiscussion, /Phối hợp, quyết định và theo dõi các vấn đề của công trình\./);
  assert.doesNotMatch(projectDiscussion, /h-10 w-64 animate-pulse/);
});
