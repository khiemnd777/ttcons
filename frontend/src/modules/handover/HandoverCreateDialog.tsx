import { useMemo } from 'react';
import { ArrowRight, Loader2, RefreshCw } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { dateText } from '../../lib/utils/format';
import { SimpleForm, type FieldDef, type FormValues } from '../_shared/SimpleForm';
import type { FinalAcceptanceOption } from './handoverTypes';

export function HandoverCreateDialog({ open, projectId, acceptances, loading, error, onRetry, onClose, onCreate }: { open: boolean; projectId: string; acceptances: FinalAcceptanceOption[]; loading: boolean; error: unknown; onRetry: () => void; onClose: () => void; onCreate: (values: FormValues) => Promise<unknown> }) {
  const fields = useMemo<FieldDef[]>(() => [
    { name: 'handover_code', label: 'Mã hồ sơ', defaultValue: `HO-${Date.now().toString().slice(-6)}` },
    { name: 'final_acceptance_id', label: 'Nghiệm thu cuối đạt', type: 'select', required: true, options: acceptances.map((row) => ({ value: row.id, label: `${row.title} · ${resultLabel(row.result)}` })), helperText: 'Nguồn được khóa vào hồ sơ và được kiểm tra lại khi hoàn tất.' },
    { name: 'handover_date', label: 'Ngày dự kiến bàn giao', type: 'date' },
    { name: 'client_representative', label: 'Đại diện chủ đầu tư', required: true },
    { name: 'company_representative', label: 'Đại diện nhà thầu', required: true },
    { name: 'notes', label: 'Ghi chú', type: 'textarea' },
  ], [acceptances]);

  return <Dialog open={open} title="Tạo hồ sơ bàn giao" description="Khóa đúng nghiệm thu cuối và đại diện hai bên trước khi hệ thống sinh checklist bàn giao." onClose={onClose} size="xl">
    {loading ? <div className="flex min-h-48 items-center justify-center gap-2 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />Đang tải nguồn nghiệm thu...</div> : error ? <div className="flex min-h-48 flex-col items-center justify-center gap-3 rounded-lg border border-red-200 bg-red-50 p-6 text-sm text-red-700"><span>Không tải được nghiệm thu cuối đủ điều kiện.</span><button type="button" className="inline-flex items-center gap-1 font-semibold underline" onClick={onRetry}><RefreshCw className="h-4 w-4" />Thử lại</button></div> : acceptances.length === 0 ? <div className="flex min-h-48 flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-line p-6 text-center"><div><div className="font-semibold text-ink">Chưa có nghiệm thu cuối hợp lệ</div><p className="mt-1 text-sm text-slate-500">Cần một nghiệm thu cuối đã hoàn tất với kết quả Đạt hoặc Đạt có điều kiện.</p></div><Link className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white" to={`/projects/${projectId}/acceptance`}>Mở Nghiệm thu<ArrowRight className="h-4 w-4" /></Link></div> : <SimpleForm
      fields={fields}
      layout="two-column"
      sections={[
        { title: 'Nguồn nghiệm thu', description: 'Chọn đúng nghiệm thu cuối của công trình; thông tin đại diện sẽ được gợi ý từ nguồn.', fields: ['final_acceptance_id'] },
        { title: 'Thông tin bàn giao', fields: ['handover_code', 'handover_date', 'client_representative', 'company_representative', 'notes'] },
      ]}
      onFieldChange={(name, value, values) => {
        if (name !== 'final_acceptance_id') return undefined;
        const selected = acceptances.find((item) => item.id === value);
        if (!selected) return undefined;
        return {
          client_representative: selected.client_representative || values.client_representative,
          company_representative: selected.contractor_representative || values.company_representative,
        };
      }}
      renderAside={(values) => <CreateSummary acceptance={acceptances.find((item) => item.id === values.final_acceptance_id)} handoverDate={values.handover_date} />}
      submitLabel="Tạo hồ sơ nháp"
      onCancel={onClose}
      onSubmit={onCreate}
    />}
  </Dialog>;
}

function CreateSummary({ acceptance, handoverDate }: { acceptance?: FinalAcceptanceOption; handoverDate?: string }) {
  return <div>
    <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">Nguồn được chọn</div>
    {acceptance ? <div className="mt-3 space-y-3 text-sm">
      <div><div className="font-semibold text-ink">{acceptance.title}</div><div className="mt-1 text-xs text-slate-500">{resultLabel(acceptance.result)}</div></div>
      <SummaryRow label="Ngày nghiệm thu" value={dateText(acceptance.acceptance_date)} />
      <SummaryRow label="Đại diện CĐT" value={acceptance.client_representative || '-'} />
      <SummaryRow label="Đại diện nhà thầu" value={acceptance.contractor_representative || '-'} />
    </div> : <div className="mt-3 rounded-md border border-dashed border-line bg-white p-3 text-xs text-slate-500">Chọn nghiệm thu cuối để xem thông tin nguồn.</div>}
    <div className="my-4 border-t border-line" />
    <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">Sau khi tạo</div>
    <div className="mt-3 space-y-2 text-sm text-slate-600">
      <SummaryRow label="Trạng thái" value="Nháp" />
      <SummaryRow label="Checklist" value="Sinh tự động" />
      <SummaryRow label="Readiness" value="Chưa kiểm tra" />
      <SummaryRow label="Ngày dự kiến" value={dateText(handoverDate)} />
    </div>
  </div>;
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return <div className="flex items-start justify-between gap-3"><span className="text-slate-500">{label}</span><span className="text-right font-medium text-ink">{value}</span></div>;
}

function resultLabel(value: string) {
  return value === 'conditional_pass' ? 'Đạt có điều kiện' : value === 'pass' ? 'Đạt' : value;
}
