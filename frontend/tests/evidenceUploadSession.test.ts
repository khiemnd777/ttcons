import assert from 'node:assert/strict';
import test from 'node:test';
import { evidenceUploadSnapshot, latestEvidenceUploadResult, retryEvidenceUpload, subscribeEvidenceUpload, uploadEvidenceDraft, uploadEvidenceDraftBatch } from '../src/modules/_shared/evidenceUploadSession.ts';

const draft = (id: string) => ({ id, file: new File(['pdf'], `${id}.pdf`, { type: 'application/pdf' }), name: id, caption: '' });
const scope = '/work-items/source/evidence/upload';

test('100% byte progress stays processing until the server receipt arrives; duplicate submits join the request', async () => {
  const item = draft('progress');
  let finish!: (value: string) => void;
  let calls = 0;
  const states: string[] = [];
  const unsubscribe = subscribeEvidenceUpload(item, scope, () => states.push(evidenceUploadSnapshot(item, scope).status));
  const worker = async (progress: (percent: number) => void) => { calls++; progress(100); return new Promise<string>((resolve) => { finish = resolve; }); };
  const first = uploadEvidenceDraft(item, scope, worker);
  const second = uploadEvidenceDraft(item, scope, worker);
  await Promise.resolve();
  assert.equal(evidenceUploadSnapshot(item, scope).status, 'processing');
  assert.equal(calls, 1);
  finish('stored-id');
  assert.deepEqual(await Promise.all([first, second]), ['stored-id', 'stored-id']);
  assert.deepEqual(states, ['uploading', 'processing', 'success']);
  unsubscribe();
});

test('partial failure and a later parent-save failure retain successful IDs in original order', async () => {
  const items = [draft('first'), draft('second'), draft('third')];
  const counts = [0, 0, 0];
  const worker = async (item: typeof items[number]) => {
    const index = items.findIndex((candidate) => candidate.id === item.id);
    counts[index]++;
    if (index === 1 && counts[index] === 1) throw new Error('network failure');
    return `stored-${item.id}`;
  };
  await assert.rejects(uploadEvidenceDraftBatch(items, scope, worker), /1 tệp chưa tải được/);
  assert.equal(evidenceUploadSnapshot(items[0], scope).status, 'success');
  assert.equal(evidenceUploadSnapshot(items[1], scope).status, 'error');
  assert.deepEqual(await uploadEvidenceDraftBatch(items, scope, worker), ['stored-first', 'stored-second', 'stored-third']);
  // Re-running after the business save fails must not upload again.
  await uploadEvidenceDraftBatch(items, scope, worker);
  assert.deepEqual(counts, [1, 2, 1]);
});

test('retry uses edited name and caption and report snapshots follow completion order', async () => {
  const first = draft('first');
  const second = draft('second');
  let revision = 0;
  let fail = true;
  const seen: string[] = [];
  const worker = async (item: typeof first) => {
    if (item.id === first.id && fail) { fail = false; throw new Error('retry'); }
    seen.push(item.name + ':' + item.caption);
    return { revision: ++revision };
  };
  await assert.rejects(uploadEvidenceDraftBatch([first, second], scope, worker, 1));
  const edited = { ...first, name: 'Edited', caption: 'New caption' };
  await retryEvidenceUpload(edited, scope);
  assert.deepEqual(seen, ['second:', 'Edited:New caption']);
  assert.deepEqual(latestEvidenceUploadResult([edited, second], scope), { revision: 2 });
  await uploadEvidenceDraftBatch([edited, second], scope, worker, 1);
  assert.equal(revision, 2);
});

test('new source, replacement file and new form draft never reuse a previous receipt', async () => {
  const item = draft('same');
  let calls = 0;
  const worker = async () => ++calls;
  assert.equal(await uploadEvidenceDraft(item, scope, worker), 1);
  assert.equal(await uploadEvidenceDraft({ ...item }, scope, worker), 1);
  assert.equal(await uploadEvidenceDraft(item, '/work-items/other/evidence/upload', worker), 2);
  assert.equal(await uploadEvidenceDraft(draft('same'), scope, worker), 3);
  assert.equal(await uploadEvidenceDraft({ ...item, id: 'new-form-draft' }, scope, worker), 4);
});

test('removed subscribers receive no late upload updates', async () => {
  const item = draft('closed');
  let notifications = 0;
  const stop = subscribeEvidenceUpload(item, scope, () => notifications++);
  stop();
  await uploadEvidenceDraft(item, scope, async (progress) => { progress(50); return 'receipt'; });
  assert.equal(notifications, 0);
});
