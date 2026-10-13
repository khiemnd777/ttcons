export type EvidenceCaptureMetadata = {
  captured_at?: string;
  location?: string;
  photo_phase?: '' | 'before' | 'during' | 'after' | 'defect';
  handover_note?: string;
};

// A file's modification/upload time does not establish when a site photo was taken.
export function captureDateTimeInput(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const pad = (number: number) => String(number).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

export function captureDateTimeValue(value: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toISOString();
}

export function appendEvidenceCaptureMetadata(formData: FormData, metadata?: EvidenceCaptureMetadata) {
  if (!metadata) return;
  const capture_metadata: Record<string, string> = {};
  for (const field of ['captured_at', 'location', 'photo_phase', 'handover_note'] as const) {
    const value = metadata[field]?.trim();
    if (value) capture_metadata[field] = value;
  }
  if (Object.keys(capture_metadata).length) {
    const raw = formData.get('payload');
    const payload = typeof raw === 'string' ? JSON.parse(raw) : {};
    formData.set('payload', JSON.stringify({ ...payload, capture_metadata }));
  }
}
