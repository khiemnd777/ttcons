import { Check, ChevronRight } from 'lucide-react';
import type { WorkflowFormSignature } from '../../lib/types/domain';
import { confirmationRolesText, signatureDateTime, signaturePurposeLabel } from './workflowConfirmationPresentation';
import { WorkflowSignatureVisual, useWorkflowSignaturePreview } from './WorkflowSignatureVisual';

export function WorkflowSignatureRecord({ signature, onViewDetail, onOpenVisual }: {
  signature: WorkflowFormSignature;
  onViewDetail: () => void;
  onOpenVisual: () => void;
}) {
  const preview = useWorkflowSignaturePreview(signature);
  return (
    <article className="grid min-w-0 grid-cols-1 gap-3 rounded-md border border-line bg-white p-3 [@container(min-width:360px)]:grid-cols-[160px_minmax(0,1fr)]">
      <WorkflowSignatureVisual preview={preview} signerName={signature.signer_name} variant="thumbnail" onEnlarge={onOpenVisual} />
      <div className="flex min-w-0 flex-col">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <p className="min-w-0 break-all text-sm font-semibold text-ink">{signature.signer_name || 'Chưa có thông tin người ký'}</p>
          <span className="inline-flex shrink-0 items-center gap-1 rounded bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700"><Check className="h-3.5 w-3.5" />Đã ký</span>
        </div>
        <p className="mt-1 break-words text-xs text-slate-600">{confirmationRolesText(signature.signer_roles_json)}</p>
        <p className="mt-3 text-xs text-slate-600">{signatureDateTime(signature.signed_at)}</p>
        <p className="mt-1 text-xs text-slate-500">{signaturePurposeLabel(signature.purpose)} · Hồ sơ phiên bản {signature.form_revision_no}</p>
        <div className="mt-auto flex justify-end pt-2">
          <button type="button" aria-label={`Xem chi tiết chữ ký của ${signature.signer_name || 'người ký'}`} className="inline-flex items-center gap-1 rounded-md px-2 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50" onClick={onViewDetail}>Xem chi tiết<ChevronRight className="h-4 w-4" /></button>
        </div>
      </div>
    </article>
  );
}
