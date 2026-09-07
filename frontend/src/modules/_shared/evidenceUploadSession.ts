import { runEvidenceUploadQueue } from '../work-items/evidenceUploadQueue.ts';

export type EvidenceUploadStatus = 'queued' | 'uploading' | 'processing' | 'success' | 'error';
export type EvidenceUploadSnapshot = { status: EvidenceUploadStatus; progress: number; error: string };
type DraftIdentity = { id: string; file: File };
type UploadWorker<D, T> = (progress: (percent: number) => void, draft: D) => Promise<T>;
type Session = {
  draft: DraftIdentity;
  snapshot: EvidenceUploadSnapshot;
  listeners: Set<() => void>;
  promise?: Promise<unknown>;
  result?: unknown;
  completedOrder?: number;
  retry?: () => Promise<unknown>;
};

// Receipts belong to the in-memory draft, file and destination. They survive a
// failed parent save, but never cross a new file, source or form session.
const sessions = new WeakMap<File, Map<string, Session>>();
let completedSequence = 0;
const queued: EvidenceUploadSnapshot = { status: 'queued', progress: 0, error: '' };

function sessionFor(draft: DraftIdentity, scope: string): Session {
  let fileSessions = sessions.get(draft.file);
  if (!fileSessions) { fileSessions = new Map(); sessions.set(draft.file, fileSessions); }
  const key = JSON.stringify([draft.id, scope]);
  let session = fileSessions.get(key);
  if (!session) { session = { draft, snapshot: queued, listeners: new Set() }; fileSessions.set(key, session); }
  session.draft = draft;
  return session;
}

export function evidenceUploadSnapshot(draft: DraftIdentity, scope: string) {
  return sessionFor(draft, scope).snapshot;
}

export function subscribeEvidenceUpload(draft: DraftIdentity, scope: string, listener: () => void) {
  const session = sessionFor(draft, scope);
  session.listeners.add(listener);
  return () => { session.listeners.delete(listener); };
}

function publish(session: Session, snapshot: EvidenceUploadSnapshot) {
  session.snapshot = snapshot;
  session.listeners.forEach((listener) => listener());
}

export function uploadEvidenceDraft<D extends DraftIdentity, T>(draft: D, scope: string, worker: UploadWorker<D, T>): Promise<T> {
  const session = sessionFor(draft, scope);
  if (session.snapshot.status === 'success') return Promise.resolve(session.result as T);
  if (session.promise) return session.promise as Promise<T>;
  session.retry = () => uploadEvidenceDraft(session.draft as D, scope, worker);
  publish(session, { status: 'uploading', progress: 0, error: '' });
  const promise = Promise.resolve().then(() => worker((percent) => {
    const progress = Math.max(0, Math.min(100, percent));
    publish(session, { status: progress >= 100 ? 'processing' : 'uploading', progress, error: '' });
  }, session.draft as D)).then((result) => {
    session.result = result;
    session.completedOrder = ++completedSequence;
    publish(session, { status: 'success', progress: 100, error: '' });
    return result;
  }).catch((error: unknown) => {
    publish(session, { status: 'error', progress: 0, error: error instanceof Error ? error.message : 'Không tải được tệp. Vui lòng thử lại.' });
    throw error;
  }).finally(() => { session.promise = undefined; });
  session.promise = promise;
  return promise;
}

export function retryEvidenceUpload(draft: DraftIdentity, scope: string) {
  return sessionFor(draft, scope).retry?.();
}

export function latestEvidenceUploadResult<T>(drafts: readonly DraftIdentity[], scope: string): T | undefined {
  const completed = drafts.map((draft) => sessionFor(draft, scope)).filter((session) => session.snapshot.status === 'success');
  return completed.sort((left, right) => (right.completedOrder ?? 0) - (left.completedOrder ?? 0))[0]?.result as T | undefined;
}

export async function uploadEvidenceDraftBatch<D extends DraftIdentity, T>(drafts: readonly D[], scope: string, worker: (draft: D, progress: (percent: number) => void) => Promise<T>, concurrency = 3): Promise<T[]> {
  const results = await runEvidenceUploadQueue(drafts, (draft) => uploadEvidenceDraft(draft, scope, (progress, currentDraft) => worker(currentDraft, progress)), concurrency);
  const failed = results.filter((result) => result.status === 'rejected');
  if (failed.length) {
    const reason: unknown = (failed[0] as PromiseRejectedResult).reason;
    throw new Error(`${failed.length} tệp chưa tải được. Các tệp đã tải được giữ lại; thử lại dòng lỗi rồi lưu tiếp. ${reason instanceof Error ? reason.message : ''}`.trim());
  }
  return results.map((result) => (result as PromiseFulfilledResult<T>).value);
}
