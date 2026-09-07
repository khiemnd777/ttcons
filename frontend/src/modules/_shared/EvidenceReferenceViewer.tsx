import { useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Eye } from 'lucide-react';
import { get } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { EvidenceRecordViewerDialog, EvidenceReferenceStateDialog } from './EvidenceRecordViewerDialog';
import { useEvidenceFileOpener } from '../work-items/EvidenceGallery';
export { evidenceReferenceMediaType } from './evidenceReferenceMedia';

export type EvidenceFileReference = {
  id?: string;
  record?: EvidenceRecord;
  name?: string;
  filePath?: string;
  fileUrl?: string;
  mimeType?: string;
  evidenceType?: string;
  metadata?: string[];
  headerStatus?: ReactNode;
};

// Cards and dialogs resolve the same protected record, never fabricated metadata.
export function useEvidenceReferenceRecord(reference: EvidenceFileReference | null) {
  const id = reference?.id || reference?.record?.id;
  const supplied = reference?.record;
  // Construction/acceptance summaries may contain er.* (including created_at)
  // without the source and identity joins required by the full viewer.
  const known = supplied && supplied.id === id && ['work_item_title', 'run_node_name', 'submitted_by_name', 'reviewed_by_name'].every((field) => Object.prototype.hasOwnProperty.call(supplied, field)) ? supplied : undefined;
  const detail = useQuery({
    queryKey: ['evidence-detail', id],
    queryFn: () => get<EvidenceRecord>(`/evidence/${id}`),
    enabled: Boolean(reference && id && !known),
    retry: false,
    staleTime: 0,
    gcTime: 0,
  });
  const record = known ?? (!detail.isFetching && !detail.isError ? detail.data : undefined);
  return {
    id,
    record,
    loading: Boolean(id) && !record && !detail.isError,
    error: id && !known && detail.isError ? (detail.error instanceof Error ? detail.error.message : 'Không tải được thông tin minh chứng.') : undefined,
    retry: () => { void detail.refetch(); },
  };
}

// Reference adapters resolve identity only. Media/details belong to the full viewer.
export function EvidenceReferenceViewer({ reference, onClose }: { reference: EvidenceFileReference | null; onClose: () => void }) {
  const { id, record, loading, error, retry } = useEvidenceReferenceRecord(reference);
  const openEvidenceFile = useEvidenceFileOpener(reference?.filePath ? () => reference.filePath! : undefined);
  if (!reference) return null;
  if (record) return <EvidenceRecordViewerDialog key={record.id + (reference.filePath ?? '')} record={record} protectedFilePath={reference.filePath} showWorkItem onOpenFile={() => { void openEvidenceFile(record); }} onClose={onClose} />;
  return <EvidenceReferenceStateDialog name={reference.name || 'Minh chứng'} fileUrl={!id ? reference.fileUrl : undefined} mimeType={reference.mimeType} evidenceType={reference.evidenceType} loading={loading} error={error} onRetry={id ? retry : undefined} onClose={onClose} />;
}

export function SelectedEvidenceViewButton({ evidenceId, evidence, label = 'Xem' }: { evidenceId?: string; evidence: EvidenceRecord[]; label?: string }) {
  const record = evidence.find((item) => String(item.id) === String(evidenceId));
  if (!evidenceId) return null;
  return <EvidenceReferenceViewButton label={label} reference={{ id: String(evidenceId), record, name: record?.name || record?.title || String(evidenceId) }} />;
}

export function EvidenceReferenceViewButton({ reference, label = 'Xem' }: { reference: EvidenceFileReference; label?: string }) {
  const [open, setOpen] = useState(false);
  const name = reference.name || reference.id || 'minh chứng';
  return <>
    <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold text-blue-700 hover:border-blue-200 hover:bg-blue-50" onClick={() => setOpen(true)} aria-label={`Xem minh chứng ${name}`}><Eye className="h-3.5 w-3.5" /> {label}</button>
    {open ? <EvidenceReferenceViewer reference={reference} onClose={() => setOpen(false)} /> : null}
  </>;
}
