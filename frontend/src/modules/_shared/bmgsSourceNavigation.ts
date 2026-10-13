import { isRecordID, RecordUnavailable } from './recordNavigation.ts';

export function parseBMGSRevision(value: string | null): number | null | undefined {
  if (value === null) return undefined;
  if (!/^[1-9]\d*$/.test(value) || !Number.isSafeInteger(Number(value))) return null;
  return Number(value);
}

/** Only a freshly authorized parent contract can resolve its exact persisted variation. */
export function resolveBMGSContractVariation(params: URLSearchParams, contract: Record<string, any>, projectId: string): Record<string, any> | null {
  const requested = params.get('variation');
  if (requested === null) return null;
  if (!isRecordID(requested) || params.get('record_type') !== 'contract' || params.get('record') !== contract.id || contract.project_id !== projectId) throw new RecordUnavailable();
  const variation = (contract.variations ?? []).find((row: Record<string, any>) => row.id === requested && row.contract_id === contract.id && !['deleted', 'cancelled'].includes(row.status) && !row.deleted_at);
  if (!variation) throw new RecordUnavailable();
  return variation;
}
