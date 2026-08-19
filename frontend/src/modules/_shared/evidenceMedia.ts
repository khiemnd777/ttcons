import type { EvidenceRecord } from '../../lib/types/domain';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage.ts';

export type EvidenceMediaType = 'photo' | 'pdf' | 'document';

export function evidenceMediaType(record: EvidenceRecord): EvidenceMediaType {
  if (isPdfEvidence(record)) return 'pdf';
  if (isImageEvidence(record)) return 'photo';
  return 'document';
}

export function isPdfEvidence(record: EvidenceRecord) {
  const mimeType = String(record.mime_type ?? '').trim().toLowerCase();
  if (mimeType.includes('pdf')) return true;

  return [
    record.document_name,
    originalFilename(record),
    record.file_url,
    record.name,
    record.title,
  ].some(hasPdfFilename);
}

export function isImageEvidence(record: EvidenceRecord) {
  if (isPdfEvidence(record)) return false;
  const mimeType = String(record.mime_type ?? '').trim().toLowerCase();
  return mimeType.startsWith('image/') || record.evidence_type === 'photo';
}

export function isInlinePdfEvidence(record: EvidenceRecord) {
  return isPdfEvidence(record)
    && isProtectedStorageProvider(record.storage_provider)
    && Boolean(record.storage_key);
}

export function originalFilename(record: EvidenceRecord) {
  const payload = record.payload_json;
  if (!payload || typeof payload !== 'object') return '';
  return String(payload.original_filename ?? '').trim();
}

function hasPdfFilename(value: unknown) {
  const filename = String(value ?? '').trim().toLowerCase().split(/[?#]/, 1)[0];
  return filename.endsWith('.pdf');
}
