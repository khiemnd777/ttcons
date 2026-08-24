import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

test('Planning handoff activation presents one fixed evidence policy instead of an editable toggle', async () => {
  const panel = await readFile(new URL('../src/modules/planning/ConstructionHandoffPanel.tsx', import.meta.url), 'utf8');

  assert.match(panel, /Mọi hạng mục bắt buộc ít nhất 1 ảnh hoặc tài liệu khi ghi kết quả kiểm tra\./);
  assert.match(panel, /evidence_required: true/);
  assert.doesNotMatch(panel, /Bắt buộc minh chứng khi ghi kết quả/);
  assert.doesNotMatch(panel, /updateEvidenceRequired/);
});
