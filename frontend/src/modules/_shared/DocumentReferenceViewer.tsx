import { useState } from 'react';
import { createPortal } from 'react-dom';
import { useAuth } from '../../lib/auth/AuthContext';
import { ProtectedDocumentViewerDialog } from './EvidenceRecordViewerDialog';

export type DocumentFileReference = {
  id: string;
  name: string;
  mime_type?: string;
  file_size?: number;
  uploaded_by_name?: string;
  created_at?: string;
};

export function DocumentReferenceViewButton({ file, sourceLabel, label = 'Xem', className }: {
  file: DocumentFileReference;
  sourceLabel?: string;
  label?: string;
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const { user } = useAuth();
  return <>
    <button type="button" disabled={!file.id} className={className ?? 'shrink-0 rounded-md border border-line px-2.5 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50'} onClick={() => setOpen(true)}>{label}</button>
    {open && user?.id ? createPortal(<div className="relative z-[120]">
      <ProtectedDocumentViewerDialog key={`${user.id}:${file.id}`} name={file.name}
        protectedFilePath={`/documents/${encodeURIComponent(file.id)}/file`}
        mimeType={file.mime_type} fileSize={file.file_size} sourceLabel={sourceLabel}
        uploadedBy={file.uploaded_by_name} createdAt={file.created_at} onClose={() => setOpen(false)} />
    </div>, document.body) : null}
  </>;
}
