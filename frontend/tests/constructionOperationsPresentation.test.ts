import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  constructionCheckFilterWorkItems,
  constructionImpactHelperText,
} from '../src/modules/operations/constructionWorkspaceUtils.ts';
import { statusLabel } from '../src/lib/utils/status.ts';

test('construction check filter keeps eligible items and historical check sources only', () => {
  const workspace = {
    work_items: [
      { id: 'eligible', title: 'Báo cáo thi công', report_eligible: true },
      { id: 'historical', title: 'Kiểm tra đã lập', report_eligible: false },
      { id: 'future', title: 'Nghiệm thu tương lai', report_eligible: false },
    ],
    checks: [{ work_item_id: 'historical' }],
  };

  assert.deepEqual(
    constructionCheckFilterWorkItems(workspace as Parameters<typeof constructionCheckFilterWorkItems>[0]).map((item) => item.id),
    ['eligible', 'historical'],
  );
});

test('construction action copy separates technical impact from commercial proposal', () => {
  assert.equal(
    constructionImpactHelperText,
    'Chỉ phân loại ảnh hưởng kỹ thuật. Muốn mở luồng thương mại, dùng “Đề xuất phát sinh”.',
  );
});

test('linked construction variation uses the shared Vietnamese status label', () => {
  assert.equal(`PS-001 · ${statusLabel('qs_review')}`, 'PS-001 · QS đang rà soát');
  assert.equal(`PS-002 · ${statusLabel('in_progress')}`, 'PS-002 · Đang xử lý');
});

test('construction evidence draft preview recreates object URLs inside an effect for StrictMode', async () => {
  const source = await readFile(new URL('../src/modules/_shared/EvidenceCapture.tsx', import.meta.url), 'utf8');

  assert.doesNotMatch(source, /useMemo\([^)]*URL\.createObjectURL/);
  assert.match(source, /useEffect\(\(\) => \{[\s\S]*const objectURL = URL\.createObjectURL\(draft\.file\)/);
  assert.match(source, /return \(\) => URL\.revokeObjectURL\(objectURL\)/);
  assert.match(source, /aria-label=\{`Xem ảnh \$\{draft\.name\}`\}/);
  assert.match(source, /Không đọc được ảnh xem trước/);
  assert.match(source, /<EvidenceDraftViewerDialog/);
  assert.match(source, /onPreviewPhoto\?: \(draft: ConstructionEvidenceUploadDraft, previewURL: string\)/);
  assert.doesNotMatch(source, /<a href=\{previewURL\}[^>]+Xem ảnh/);
});

test('construction report exposes protected saved evidence preview and the approved daily-work title', async () => {
  const source = await readFile(new URL('../src/modules/operations/ConstructionReportFormDialog.tsx', import.meta.url), 'utf8');

  assert.match(source, /title="Công việc trong ngày"/);
  assert.doesNotMatch(source, /title="Checklist báo cáo"/);
  assert.match(source, /openProtectedBlob\(\(\) => downloadBlob\(`\/evidence\/\$\{record\.id\}\/file`\)\)/);
  assert.match(source, /<ExistingEvidenceOption/);
  assert.match(source, /Thử lại thumbnail/);
  assert.match(source, /<EvidenceRecordViewerDialog/);
  assert.match(source, /isImageEvidence\(record\) \|\| isInlinePdfEvidence\(record\)/);
  assert.match(source, /isInlineWordEvidence\(record\)/);
  assert.match(source, /isInlineSpreadsheetEvidence\(record\)/);
  assert.match(source, /pdf \? 'Xem PDF'/);
  assert.match(source, /word \? 'Xem Word'/);
  assert.match(source, /spreadsheet \? 'Xem Excel'/);
  assert.match(source, /<EvidenceDraftViewerDialog/);
  assert.match(source, /showWorkItem/);
  assert.match(source, /onOpenFile=\{viewerRecord \? \(\) => void openEvidenceFile\(viewerRecord\)/);
  assert.match(source, /onPreviewPhoto=\{\(draft, previewURL\) => setEvidenceViewer/);
  assert.match(source, /onRetry=\{viewerRecord \? \(\) => setEvidencePreviewRetryKey/);
  assert.doesNotMatch(source, /import \{ EvidenceViewerDialog \}/);
  assert.doesNotMatch(source, /window\.open\(previewURL/);
});

test('shared evidence viewer keeps images in-app with loading, retry, decode failure and nested-dialog escape safety', async () => {
  const viewer = await readFile(new URL('../src/modules/_shared/EvidenceViewerDialog.tsx', import.meta.url), 'utf8');
  const recordViewer = await readFile(new URL('../src/modules/_shared/EvidenceRecordViewerDialog.tsx', import.meta.url), 'utf8');
  const gallery = await readFile(new URL('../src/modules/work-items/EvidenceGallery.tsx', import.meta.url), 'utf8');

  assert.match(viewer, /role="dialog"/);
  assert.match(viewer, /z-\[80\]/);
  assert.match(viewer, /max-w-\[min\(1180px,calc\(100vw-2rem\)\)\]/);
  assert.match(viewer, /object-contain/);
  assert.match(viewer, /Đang tải ảnh minh chứng/);
  assert.match(viewer, /Không thể hiển thị định dạng ảnh này/);
  assert.match(viewer, /Thử lại/);
  assert.match(viewer, /mediaType === 'photo'/);
  assert.match(viewer, /mediaType === 'pdf'/);
  assert.match(viewer, /<EvidencePdfViewer/);
  assert.match(viewer, /mediaAction \?/);
  assert.match(viewer, /documentUnavailableText/);
  assert.doesNotMatch(viewer, /Tài liệu được mở bằng hành động Mở tệp/);
  assert.match(viewer, /event\.stopImmediatePropagation\(\)/);
  assert.match(viewer, /addEventListener\('keydown', onKeyDown, true\)/);
  assert.match(recordViewer, /<EvidenceViewerDialog/);
  assert.match(recordViewer, /headerStatus=\{<StatusBadge status=\{record\.status\} \/>\}/);
  assert.match(recordViewer, /mediaAction=\{mediaAction\}/);
  assert.match(recordViewer, /Nguồn hồ sơ/);
  assert.match(recordViewer, /Gửi và rà soát/);
  assert.match(recordViewer, /<details className=/);
  assert.match(recordViewer, /Chi tiết kỹ thuật/);
  assert.match(recordViewer, /Chấp nhận/);
  assert.match(recordViewer, /Phạm vi hiển thị/);
  assert.match(recordViewer, /Người gửi/);
  assert.match(recordViewer, /Nơi lưu trữ/);
  assert.match(recordViewer, /Mã kiểm tra SHA-256/);
  assert.match(recordViewer, /Chưa upload/);
  assert.match(recordViewer, /Thông tin người gửi, nơi lưu trữ và mã kiểm tra sẽ có sau khi upload/);
  const draftViewer = recordViewer.slice(recordViewer.indexOf('export function EvidenceDraftViewerDialog'), recordViewer.indexOf('function CloseButton'));
  assert.doesNotMatch(draftViewer, /<EvidenceMeta label="Người gửi"/);
  assert.doesNotMatch(draftViewer, /<EvidenceMeta label="Nơi lưu trữ"/);
  assert.doesNotMatch(draftViewer, /<EvidenceMeta label="Mã kiểm tra SHA-256"/);
  assert.match(gallery, /import \{[\s\S]*EvidenceRecordViewerDialog[\s\S]*\} from '\.\.\/_shared\/EvidenceRecordViewerDialog'/);
  assert.match(gallery, /<EvidenceRecordViewerDialog/);
  assert.doesNotMatch(gallery, /function EvidenceDetailDialog/);
  assert.match(gallery, /canPreviewEvidenceInApp\(record\) \? setSelected\(record\) : void openEvidence\(record\)/);
  assert.match(gallery, /Xem PDF/);
  const persistedFooter = recordViewer.slice(recordViewer.indexOf('footer={('), recordViewer.indexOf('details={('));
  assert.doesNotMatch(persistedFooter, /Mở tệp/);
});

test('construction report resources stay in one column and edit through inline tables', async () => {
  const source = await readFile(new URL('../src/modules/operations/ConstructionReportFormDialog.tsx', import.meta.url), 'utf8');
  const resourceStart = source.indexOf('function CrewEditor(');
  const resourceEnd = source.indexOf('function ResourceEditorFrame(', resourceStart);
  const resources = source.slice(resourceStart, resourceEnd);

  assert.match(source, /<div className="mt-3 space-y-4"><CrewEditor/);
  assert.doesNotMatch(source, /lg:grid-cols-2"><CrewEditor/);
  assert.equal((resources.match(/<table className=/g) ?? []).length, 2);
  assert.equal((resources.match(/<span className="sr-only">Thao tác<\/span>/g) ?? []).length, 2);
  assert.match(resources, /Tên tổ \/ đội/);
  assert.match(resources, /Tên vật tư/);
  assert.match(resources, /Kéo ngang trong bảng để xem và chỉnh sửa đủ các cột/);
});
