import type { EvidenceRecord } from '../../lib/types/domain';
import { enumLabel } from '../../lib/utils/status';

export function evidenceOptionLabel(record: EvidenceRecord) {
  const title = record.name ?? record.title ?? record.document_name ?? record.id;
  const kind = enumLabel('evidence_type', record.evidence_type);
  return `${title} · ${kind} · ${record.status}`;
}

export function textValue(value: unknown) {
  if (value === null || value === undefined) return '';
  return String(value);
}
