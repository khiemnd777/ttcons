import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = (path: string) => readFile(new URL(path, import.meta.url), 'utf8');

test('ID references resolve metadata and delegate rendering to the full record viewer', async () => {
  const adapter = await source('../src/modules/_shared/EvidenceReferenceViewer.tsx');
  const record = await source('../src/modules/_shared/EvidenceRecordViewerDialog.tsx');
  const viewer = await source('../src/modules/_shared/EvidenceViewerDialog.tsx');
  assert.match(adapter, /get<EvidenceRecord>\(`/);
  assert.match(adapter, /<EvidenceRecordViewerDialog/);
  assert.doesNotMatch(adapter, /<EvidenceViewerDialog|downloadBlob|URL\.createObjectURL/);
  assert.match(record, /Nguồn hồ sơ/);
  assert.match(record, /Gửi và rà soát/);
  assert.match(viewer, /event\.key !== 'Tab'/);
  assert.match(viewer, /data-viewer-autofocus/);
  assert.match(viewer, /previousFocus\?\.isConnected/);
});

test('construction, acceptance and handover evidence consumers use the in-app adapter', async () => {
  const construction = await source('../src/modules/operations/ConstructionEvidenceButton.tsx');
  const acceptance = await source('../src/modules/acceptance/AcceptancePage.tsx');
  const acceptanceEditor = await source('../src/modules/acceptance/AcceptanceEditor.tsx');
  const handover = await source('../src/modules/handover/HandoverChecklistSourceDialog.tsx');

  assert.match(construction, /<EvidenceReferenceViewer/);
  assert.doesNotMatch(construction, /openProtectedBlob|window\.open/);
  assert.match(acceptance, /setViewerReference/);
  assert.match(acceptance, /<EvidenceReferenceViewer/);
  assert.doesNotMatch(acceptance, /openProtectedBlob/);
  assert.match(acceptanceEditor, /onView=\{\(\) => setViewerRecord\(record\)\}/);
  assert.match(handover, /setViewerSource\(source\)/);
  assert.match(handover, /<EvidenceReferenceViewer/);
  assert.doesNotMatch(handover, /openProtectedBlob/);
});

test('evidence selectors expose a separate viewer action without changing their selected IDs', async () => {
  const material = await source('../src/modules/work-items/MaterialSnapshotPanel.tsx');
  const incoming = await source('../src/modules/work-items/IncomingAcceptancePanel.tsx');
  const surplus = await source('../src/modules/work-items/SurplusHandlingPanel.tsx');
  const subcontractor = await source('../src/modules/work-items/SubcontractorReconciliationPanel.tsx');
  const owner = await source('../src/modules/work-items/OwnerClientReconciliationPanel.tsx');

  assert.match(material, /SelectedEvidenceViewButton evidenceId=\{row\.before_evidence_id\}/);
  assert.match(material, /SelectedEvidenceViewButton evidenceId=\{row\.after_evidence_id\}/);
  assert.match(incoming, /SelectedEvidenceViewButton evidenceId=\{line\.evidence_id\}/);
  assert.match(surplus, /SelectedEvidenceViewButton evidenceId=\{draft\.evidence_id\}/);
  assert.match(subcontractor, /EvidenceReferenceViewButton reference=\{\{ id: item\.evidence_id/);
  assert.match(owner, /EvidenceReferenceViewButton reference=\{\{ id: item\.evidence_id/);
  assert.match(material, /onChange\(\{ before_evidence_id: event\.target\.value \}\)/);
  assert.match(incoming, /onChange\(\{ evidence_id: event\.target\.value \}\)/);
  assert.match(surplus, /onChange\(\{ evidence_id: event\.target\.value \}\)/);
});
