export type EvidenceFileKind = 'photo' | 'pdf' | 'word' | 'excel';
export type EvidenceUploadBusinessType = 'photo' | 'document';

export type EvidenceFileDescriptor = {
  name?: string | null;
  type?: string | null;
};

export type EvidenceFileClassification = {
  kind: EvidenceFileKind;
  label: 'Ảnh' | 'PDF' | 'Word' | 'Excel';
  evidenceType: EvidenceUploadBusinessType;
};

const classificationByMime: Record<string, EvidenceFileClassification> = {
  'image/jpeg': classification('photo', 'Ảnh', 'photo'),
  'image/jpg': classification('photo', 'Ảnh', 'photo'),
  'image/png': classification('photo', 'Ảnh', 'photo'),
  'image/webp': classification('photo', 'Ảnh', 'photo'),
  'image/gif': classification('photo', 'Ảnh', 'photo'),
  'application/pdf': classification('pdf', 'PDF', 'document'),
  'application/msword': classification('word', 'Word', 'document'),
  'application/vnd.ms-word': classification('word', 'Word', 'document'),
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document': classification('word', 'Word', 'document'),
  'application/vnd.ms-excel': classification('excel', 'Excel', 'document'),
  'application/msexcel': classification('excel', 'Excel', 'document'),
  'application/x-msexcel': classification('excel', 'Excel', 'document'),
  'application/x-excel': classification('excel', 'Excel', 'document'),
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet': classification('excel', 'Excel', 'document'),
};

const classificationByExtension: Record<string, EvidenceFileClassification> = {
  '.jpg': classificationByMime['image/jpeg'],
  '.jpeg': classificationByMime['image/jpeg'],
  '.png': classificationByMime['image/png'],
  '.webp': classificationByMime['image/webp'],
  '.gif': classificationByMime['image/gif'],
  '.pdf': classificationByMime['application/pdf'],
  '.doc': classificationByMime['application/msword'],
  '.docx': classificationByMime['application/vnd.openxmlformats-officedocument.wordprocessingml.document'],
  '.xls': classificationByMime['application/vnd.ms-excel'],
  '.xlsx': classificationByMime['application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'],
};

const genericMimeTypes = new Set([
  '',
  'application/octet-stream',
  'application/zip',
  'application/x-zip-compressed',
  'application/cdfv2',
  'application/x-cfb',
  'application/x-ole-storage',
  'application/vnd.ms-office',
]);

export const evidenceFileAccept = [
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
  'application/pdf',
  'application/msword',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  'application/vnd.ms-excel',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  '.png',
  '.jpg',
  '.jpeg',
  '.webp',
  '.gif',
  '.pdf',
  '.doc',
  '.docx',
  '.xls',
  '.xlsx',
].join(',');

export const evidenceSupportedFileHint = 'PNG/JPG/JPEG/WebP/GIF, PDF, Word (DOC/DOCX), Excel (XLS/XLSX)';

export function classifyEvidenceFile(file: EvidenceFileDescriptor): EvidenceFileClassification | null {
  const mimeType = String(file.type ?? '').trim().toLowerCase().split(';', 1)[0];
  const mimeClassification = classificationByMime[mimeType];
  if (mimeClassification) return mimeClassification;

  if (!genericMimeTypes.has(mimeType)) return null;
  return classificationByExtension[fileExtension(file.name)] ?? null;
}

export function evidenceFileKindLabel(kind: EvidenceFileKind | null | undefined) {
  if (!kind) return 'Chưa xác định';
  return kind === 'photo' ? 'Ảnh' : kind === 'pdf' ? 'PDF' : kind === 'word' ? 'Word' : 'Excel';
}

function classification(kind: EvidenceFileKind, label: EvidenceFileClassification['label'], evidenceType: EvidenceUploadBusinessType): EvidenceFileClassification {
  return { kind, label, evidenceType };
}

function fileExtension(value: string | null | undefined) {
  const filename = String(value ?? '').trim().toLowerCase().split(/[?#]/, 1)[0];
  const index = filename.lastIndexOf('.');
  return index >= 0 ? filename.slice(index) : '';
}
