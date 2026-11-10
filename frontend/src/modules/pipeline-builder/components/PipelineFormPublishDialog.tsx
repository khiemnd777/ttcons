import { Dialog } from '../../../components/Dialog';
import { LoadingSpinner } from '../../../components/LoadingSpinner';

export type PipelineFormPublishItem = {
  version_id: string;
  definition_hash: string;
  version_status: string;
  version_no: number;
  template_name: string;
  template_status: string;
  step_usage_count: number;
  field_count: number;
};

export type PipelineFormPublishPreview = {
  template_id: string;
  template_name: string;
  template_status: string;
  forms: PipelineFormPublishItem[];
  publishable_count: number;
  step_occurrences: number;
};

export function PipelineFormPublishDialog({
  open, preview, loading, error, pending, onClose, onRetry, onConfirm,
}: {
  open: boolean;
  preview?: PipelineFormPublishPreview;
  loading: boolean;
  error: boolean;
  pending: boolean;
  onClose: () => void;
  onRetry: () => void;
  onConfirm: () => void;
}) {
  const publishable = preview?.forms.filter((form) => form.version_status === 'draft' && form.template_status === 'active' && form.field_count > 0) ?? [];
  const blocked = preview?.forms.filter((form) => form.template_status !== 'active' || (form.version_status !== 'draft' && form.version_status !== 'published') || (form.version_status === 'draft' && form.field_count === 0)) ?? [];
  const canConfirm = !loading && !error && !pending && preview?.template_status === 'draft' && publishable.length > 0 && blocked.length === 0;

  return (
    <Dialog
      open={open}
      title="Phát hành biểu mẫu liên quan"
      description={preview ? `Pipeline: ${preview.template_name}` : undefined}
      onClose={() => { if (!pending) onClose(); }}
      size="lg"
      manageFocus
      footer={
        <div className="flex flex-wrap justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-white disabled:opacity-50" onClick={onClose} disabled={pending}>Hủy</button>
          <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={onConfirm} disabled={!canConfirm}>
            {pending ? 'Đang phát hành…' : `Phát hành ${publishable.length} biểu mẫu`}
          </button>
        </div>
      }
    >
      {loading ? <div className="flex justify-center py-8"><LoadingSpinner /></div> : error ? (
        <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
          Không tải được danh sách biểu mẫu. <button className="ml-2 font-semibold underline" onClick={onRetry}>Thử lại</button>
        </div>
      ) : preview ? (
        <div className="space-y-4 text-sm">
          <p className="rounded-md border border-blue-200 bg-blue-50 p-3 text-blue-900">
            {publishable.length} phiên bản nháp · được dùng tại {preview.step_occurrences} Gói công việc. Chỉ các phiên bản nháp dưới đây được phát hành.
          </p>
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="w-full min-w-[540px] text-left">
              <thead className="bg-slate-50 text-xs uppercase text-slate-500"><tr><th className="px-3 py-2">Biểu mẫu</th><th className="px-3 py-2">Phiên bản</th><th className="px-3 py-2">Gói sử dụng</th><th className="px-3 py-2">Trạng thái</th></tr></thead>
              <tbody>{preview.forms.map((form) => (
                <tr key={form.version_id} className="border-t border-line">
                  <td className="px-3 py-2 font-medium text-ink">{form.template_name}</td>
                  <td className="px-3 py-2">v{form.version_no}</td>
                  <td className="px-3 py-2">{form.step_usage_count}</td>
                  <td className="px-3 py-2">{form.template_status !== 'active' ? 'Không active' : form.version_status === 'draft' && form.field_count === 0 ? 'Thiếu trường' : form.version_status === 'draft' ? 'Nháp' : form.version_status === 'published' ? 'Đã phát hành' : form.version_status}</td>
                </tr>
              ))}</tbody>
            </table>
          </div>
          {blocked.length > 0 ? <p className="rounded-md border border-rose-200 bg-rose-50 p-3 text-rose-800">Có {blocked.length} biểu mẫu chưa đủ điều kiện. Hãy xử lý chúng trước khi phát hành hàng loạt.</p> : null}
          <p className="text-slate-600">Thao tác này không phát hành Pipeline. Sau khi hoàn tất, hệ thống sẽ tải lại dữ liệu để anh kiểm tra Pipeline.</p>
        </div>
      ) : null}
    </Dialog>
  );
}
