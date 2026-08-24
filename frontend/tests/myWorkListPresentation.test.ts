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
