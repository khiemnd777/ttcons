export const evidenceUploadBatchLimit = 10;
export const evidenceUploadConcurrency = 3;

export type EvidenceUploadProgressInput = {
  fileSize: number;
  progress: number;
};

export function evidenceUploadOverallProgress(rows: readonly EvidenceUploadProgressInput[]) {
  const totalBytes = rows.reduce((sum, row) => sum + Math.max(0, row.fileSize), 0);
  if (totalBytes <= 0) return 0;

  const uploadedBytes = rows.reduce((sum, row) => {
    const progress = Math.min(100, Math.max(0, row.progress));
    return sum + Math.max(0, row.fileSize) * (progress / 100);
  }, 0);
  return Math.round((uploadedBytes / totalBytes) * 100);
}

export async function runEvidenceUploadQueue<T, R>(
  items: readonly T[],
  worker: (item: T, index: number) => Promise<R>,
  concurrency = evidenceUploadConcurrency,
): Promise<PromiseSettledResult<R>[]> {
  if (!items.length) return [];

  const results: PromiseSettledResult<R>[] = new Array(items.length);
  const workerCount = Math.min(items.length, Math.max(1, Math.trunc(concurrency) || 1));
  let cursor = 0;

  async function consume() {
    while (cursor < items.length) {
      const index = cursor;
      cursor += 1;
      try {
        results[index] = { status: 'fulfilled', value: await worker(items[index], index) };
      } catch (reason) {
        results[index] = { status: 'rejected', reason };
      }
    }
  }

  await Promise.all(Array.from({ length: workerCount }, consume));
  return results;
}

export function failedEvidenceUploadRowIds<T extends { id: string }>(
  rows: readonly T[],
  results: readonly PromiseSettledResult<unknown>[],
) {
  const failedRowIds = new Set<string>();
  rows.forEach((row, index) => {
    if (results[index]?.status !== 'fulfilled') failedRowIds.add(row.id);
  });
  return failedRowIds;
}
