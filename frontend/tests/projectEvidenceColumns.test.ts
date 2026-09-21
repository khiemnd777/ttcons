import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const gallerySource = await readFile(new URL('../src/modules/work-items/EvidenceGallery.tsx', import.meta.url), 'utf8');

test('project evidence list separates file, source or stage, and work item columns', () => {
  assert.match(gallerySource, /<span>Hồ sơ<\/span>\s*<span>Nguồn \/ Giai đoạn<\/span>\s*<span>Đầu việc<\/span>/);
  assert.match(gallerySource, /<ProjectEvidenceTextCell label="Nguồn \/ Giai đoạn" value=\{record\.design_release_id \? `Thiết kế/);
  assert.match(gallerySource, /<ProjectEvidenceTextCell label="Đầu việc" value=\{record\.work_item_title\} \/>/);
  assert.match(gallerySource, /showWorkItem && listContext !== 'project'/);
});

test('project evidence long text truncates with an accessible hover tooltip', () => {
  assert.match(gallerySource, /function ProjectEvidenceTextCell/);
  assert.match(gallerySource, /className="block min-w-0 truncate[^\"]*"/);
  assert.match(gallerySource, /title=\{displayValue\}/);
  assert.match(gallerySource, /aria-label=\{`\$\{label\}: \$\{displayValue\}`\}/);
  assert.match(gallerySource, /tabIndex=\{0\}/);
  assert.match(gallerySource, /title=\{listContext === 'project' \? title : undefined\}/);
  assert.match(gallerySource, /title=\{listContext === 'project' \? fileMeta : undefined\}/);
});

test('project evidence uses seven desktop tracks and stacked labels below xl', () => {
  assert.match(gallerySource, /PROJECT_EVIDENCE_LIST_GRID_CLASS = 'xl:grid-cols-\[[^']+\]'/);
  assert.match(gallerySource, /text-slate-400 xl:hidden/);
  assert.match(gallerySource, /grid-cols-\[6rem_minmax\(0,1fr\)\]/);
});

test('shared evidence presentations expose one explicit open action without a duplicate detail button', () => {
  assert.doesNotMatch(gallerySource, />\s*Chi tiết\s*</);
  assert.equal((gallerySource.match(/onClick=\{onSelect\}/g) ?? []).length, 2);
  assert.match(gallerySource, /aria-label=\{`Xem chi tiết hồ sơ \$\{title\}`\}/);
  assert.match(gallerySource, /function evidenceOpenLabel/);
  assert.match(gallerySource, /return 'Xem ảnh'/);
  assert.match(gallerySource, /return 'Xem PDF'/);
  assert.match(gallerySource, /return 'Xem Word'/);
  assert.match(gallerySource, /return 'Xem Excel'/);
  assert.match(gallerySource, /return 'Mở tệp'/);
});
