import type { EvidenceRecord } from '../../lib/types/domain';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage.ts';

export type EvidenceMediaType = 'photo' | 'pdf' | 'word' | 'spreadsheet' | 'document';

export function evidenceMediaType(record: EvidenceRecord): EvidenceMediaType {
  if (isPdfEvidence(record)) return 'pdf';
  if (isWordEvidence(record)) return 'word';
  if (isSpreadsheetEvidence(record)) return 'spreadsheet';
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
  if (
    isPdfEvidence(record)
    || isWordEvidence(record)
    || isSpreadsheetEvidence(record)
    || isLegacyWordEvidence(record)
    || isLegacySpreadsheetEvidence(record)
  ) return false;
  const mimeType = String(record.mime_type ?? '').trim().toLowerCase();
  return mimeType.startsWith('image/') || record.evidence_type === 'photo';
}

export function isInlinePdfEvidence(record: EvidenceRecord) {
  return isPdfEvidence(record)
    && isProtectedStorageProvider(record.storage_provider)
    && Boolean(record.storage_key);
}

export function isWordEvidence(record: EvidenceRecord) {
  const mimeType = normalizedMimeType(record);
  if (mimeType === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document') return true;
  return evidenceFilenames(record).some((value) => hasFilenameExtension(value, '.docx'));
}

export function isSpreadsheetEvidence(record: EvidenceRecord) {
  const mimeType = normalizedMimeType(record);
  if (mimeType === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet') return true;
  return evidenceFilenames(record).some((value) => hasFilenameExtension(value, '.xlsx'));
}

export function isLegacyWordEvidence(record: EvidenceRecord) {
  const mimeType = normalizedMimeType(record);
  if (mimeType === 'application/msword') return true;
  return evidenceFilenames(record).some((value) => hasFilenameExtension(value, '.doc'));
}

export function isLegacySpreadsheetEvidence(record: EvidenceRecord) {
  const mimeType = normalizedMimeType(record);
  if (mimeType === 'application/vnd.ms-excel') return true;
  return evidenceFilenames(record).some((value) => hasFilenameExtension(value, '.xls'));
}

export function isInlineWordEvidence(record: EvidenceRecord) {
  return isWordEvidence(record)
    && isProtectedStorageProvider(record.storage_provider)
    && Boolean(record.storage_key);
}

export function isInlineSpreadsheetEvidence(record: EvidenceRecord) {
  return isSpreadsheetEvidence(record)
    && isProtectedStorageProvider(record.storage_provider)
    && Boolean(record.storage_key);
}

export function originalFilename(record: EvidenceRecord) {
  const payload = record.payload_json;
  if (!payload || typeof payload !== 'object') return '';
  return String(payload.original_filename ?? '').trim();
}

function hasPdfFilename(value: unknown) {
  return hasFilenameExtension(value, '.pdf');
}

function evidenceFilenames(record: EvidenceRecord) {
  return [
    record.document_name,
    originalFilename(record),
    record.file_url,
    record.name,
    record.title,
  ];
}

function normalizedMimeType(record: EvidenceRecord) {
  return String(record.mime_type ?? '').trim().toLowerCase().split(';', 1)[0];
}

function hasFilenameExtension(value: unknown, extension: string) {
  const filename = String(value ?? '').trim().toLowerCase().split(/[?#]/, 1)[0];
  return filename.endsWith(extension);
}
