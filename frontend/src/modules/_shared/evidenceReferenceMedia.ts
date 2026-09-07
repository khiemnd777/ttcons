import type { EvidenceMediaType } from './evidenceMedia';

export function evidenceReferenceMediaType(name?: string, mimeType?: string, evidenceType?: string): EvidenceMediaType {
  const mime = String(mimeType ?? '').trim().toLowerCase().split(';', 1)[0];
  const filename = String(name ?? '').trim().toLowerCase().split(/[?#]/, 1)[0];
  if (mime.includes('pdf') || filename.endsWith('.pdf')) return 'pdf';
  if (mime === 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' || filename.endsWith('.docx')) return 'word';
  if (mime === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' || filename.endsWith('.xlsx')) return 'spreadsheet';
  if (mime.startsWith('image/') || /\.(png|jpe?g|webp|gif)$/.test(filename) || evidenceType === 'photo') return 'photo';
  return 'document';
}

export function safeEvidenceFileUrl(value?: string) {
  const candidate = String(value ?? '').trim();
  if (!candidate) return undefined;
  if (candidate.startsWith('/') && !candidate.startsWith('//')) return candidate;
  try {
    const parsed = new URL(candidate, typeof window === 'undefined' ? 'http://localhost' : window.location.origin);
    if (parsed.protocol === 'http:' || parsed.protocol === 'https:' || parsed.protocol === 'blob:') return parsed.href;
    if (parsed.protocol === 'data:' && candidate.toLowerCase().startsWith('data:image/')) return candidate;
  } catch {
    return undefined;
  }
  return undefined;
}
