import assert from 'node:assert/strict';
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
