import { useEffect, useMemo, useState } from 'react';
import { AlertCircle, AlertTriangle, RefreshCw, Trash2 } from 'lucide-react';
import { ApiError, post } from '../../lib/api/client';
import { Dialog } from '../../components/Dialog';

type DeletionIssue = {
  code: string;
  message: string;
  count?: number;
};

type ImpactItem = {
  reference_id: string;
  reference_type: string;
  code?: string;
  label: string;
  status?: string;
};

type StaffDeletionImpact = {
  project_manager: ImpactItem[];
  project_memberships: ImpactItem[];
  work_items: ImpactItem[];
  other_responsibilities: ImpactItem[];
};

type StaffDeletionPreviewRow = {
  staff_id: string;
  user_id?: string;
  full_name: string;
  email: string;
  will_delete_account: boolean;
  can_delete: boolean;
  blockers: DeletionIssue[];
  warnings: DeletionIssue[];
  impact: StaffDeletionImpact;
};

export type StaffDeletionPreview = {
  total_count: number;
  deletable_count: number;
  blocked_count: number;
  warning_count: number;
  preview_hash: string;
  impact_summary: Record<string, number>;
  rows: StaffDeletionPreviewRow[];
};

export type StaffDeletionResult = {
  deletion_id: string;
  deleted_count: number;
  memberships_deactivated: number;
  sessions_revoked: number;
  tokens_invalidated: number;
  roles_removed: number;
};

export function StaffDeleteDialog({
  open,
  staffIDs,
  onClose,
  onSuccess,
  onError,
}: {
  open: boolean;
  staffIDs: string[];
  onClose: () => void;
  onSuccess: (result: StaffDeletionResult) => Promise<void> | void;
  onError: (title: string, error: unknown) => void;
}) {
  const [preview, setPreview] = useState<StaffDeletionPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const selectionKey = useMemo(() => [...staffIDs].sort().join(','), [staffIDs]);

  useEffect(() => {
    if (!open || !staffIDs.length) return;
    setConfirmed(false);
    setIdempotencyKey(crypto.randomUUID());
    void loadPreview();
  }, [open, selectionKey]);

  async function loadPreview() {
    setLoading(true);
    setError(null);
    setPreview(null);
    setConfirmed(false);
    try {
      const result = await post<StaffDeletionPreview>('/staff/deletions/preview', { staff_ids: staffIDs });
      setPreview(result);
    } catch (requestError) {
      setError(requestError);
    } finally {
      setLoading(false);
    }
  }

  async function execute() {
    if (!preview?.preview_hash || preview.blocked_count > 0 || !confirmed || executing) return;
    setExecuting(true);
    setError(null);
    try {
      const result = await post<StaffDeletionResult>('/staff/deletions/execute', {
        staff_ids: staffIDs,
        preview_hash: preview.preview_hash,
        idempotency_key: idempotencyKey,
      });
      await onSuccess(result);
    } catch (requestError) {
      setError(requestError);
      onError('Không xóa được nhân sự', requestError);
      if (requestError instanceof ApiError && requestError.status === 409) {
        setPreview(null);
        setConfirmed(false);
      }
    } finally {
      setExecuting(false);
    }
  }

  const busy = loading || executing;
  const canExecute = Boolean(preview?.preview_hash) && preview?.blocked_count === 0 && confirmed && !busy;

  return (
    <Dialog
      open={open}
      title="Xác nhận xóa nhân sự"
      description="Tài khoản sẽ bị vô hiệu hóa; dữ liệu nghiệp vụ và lịch sử tham chiếu vẫn được giữ lại."
      size="xl"
      onClose={() => { if (!busy) onClose(); }}
      footer={(
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">
          <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" onClick={onClose} disabled={busy}>Hủy</button>
          <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={execute} disabled={!canExecute}>
            <Trash2 className="h-4 w-4" />{executing ? 'Đang xóa...' : `Xóa ${preview?.total_count ?? staffIDs.length} nhân sự`}
          </button>
        </div>
      )}
    >
      {loading ? <LoadingState /> : error && !preview ? <PreviewError error={error} onRetry={loadPreview} /> : preview ? (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Metric label="Đã chọn" value={preview.total_count} />
            <Metric label="Có thể xóa" value={preview.deletable_count} tone="success" />
            <Metric label="Bị chặn" value={preview.blocked_count} tone={preview.blocked_count ? 'danger' : 'default'} />
            <Metric label="Cảnh báo liên kết" value={preview.warning_count} tone={preview.warning_count ? 'warning' : 'default'} />
          </div>

          {preview.blocked_count > 0 ? (
            <div className="flex items-start gap-3 rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
              <AlertCircle className="mt-0.5 h-5 w-5 shrink-0" />
              <div><p className="font-semibold">Chưa thể thực hiện xóa hàng loạt</p><p className="mt-1 text-xs leading-5">Bỏ chọn các dòng bị chặn, sau đó kiểm tra lại danh sách.</p></div>
            </div>
          ) : preview.warning_count > 0 ? (
            <div className="flex items-start gap-3 rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
              <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0" />
              <div><p className="font-semibold">Có liên kết cần phân công lại</p><p className="mt-1 text-xs leading-5">Các liên kết được giữ nguyên và sẽ hiển thị “tài khoản đã xoá”. Membership đang hoạt động sẽ được vô hiệu hóa.</p></div>
            </div>
          ) : null}

          <div className="overflow-hidden rounded-lg border border-line">
            <div className="consflow-table-viewport max-h-[42vh]">
              <table className="min-w-full divide-y divide-line">
                <thead className="bg-slate-50">
                  <tr>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Nhân sự</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Tài khoản</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Ảnh hưởng</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Kết quả kiểm tra</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line bg-white">
                  {preview.rows.map((row) => <PreviewRow key={row.staff_id} row={row} />)}
                </tbody>
              </table>
            </div>
          </div>

          {error ? <PreviewError error={error} onRetry={loadPreview} compact /> : null}

          {preview.blocked_count === 0 ? (
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-line bg-slate-50 p-4">
              <input type="checkbox" className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} disabled={busy} />
              <span className="text-sm leading-5 text-slate-700"><strong className="font-semibold text-ink">Tôi xác nhận xóa {preview.total_count} nhân sự.</strong><br /><span className="text-xs text-slate-500">Tài khoản đăng nhập, phiên đăng nhập và vai trò sẽ bị thu hồi; thao tác được ghi audit và không thể hoàn tác trên giao diện.</span></span>
            </label>
          ) : null}
        </div>
      ) : null}
    </Dialog>
  );
}

function PreviewRow({ row }: { row: StaffDeletionPreviewRow }) {
  const impactGroups = [
    ['Công trình phụ trách', row.impact.project_manager],
    ['Membership', row.impact.project_memberships],
    ['Công việc', row.impact.work_items],
    ['Trách nhiệm khác', row.impact.other_responsibilities],
  ] as const;
  const impactCount = impactGroups.reduce((total, [, items]) => total + items.length, 0);
  return (
    <tr className={row.can_delete ? '' : 'bg-rose-50/60'}>
      <td className="px-3 py-3 align-top"><p className="text-sm font-semibold text-ink">{row.full_name}</p><p className="mt-1 text-xs text-slate-500">{row.email}</p></td>
      <td className="px-3 py-3 align-top text-xs text-slate-600">{row.will_delete_account ? 'Vô hiệu hóa tài khoản' : 'Giữ tài khoản liên kết'}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-600">
        {impactCount ? (
          <details><summary className="cursor-pointer font-semibold text-amber-700">{impactCount} liên kết</summary><ul className="mt-2 space-y-1">{impactGroups.flatMap(([label, items]) => items.map((item) => <li key={`${label}-${item.reference_type}-${item.reference_id}`}><span className="font-medium">{label}:</span> {[item.code, item.label].filter(Boolean).join(' · ')}</li>))}</ul></details>
        ) : <span className="text-slate-400">Không có liên kết đang hoạt động</span>}
      </td>
      <td className="px-3 py-3 align-top text-xs">
        {row.blockers.length ? <ul className="space-y-1 text-rose-700">{row.blockers.map((item) => <li key={item.code}>{item.message}</li>)}</ul> : row.warnings.length ? <span className="font-semibold text-amber-700">Có thể xóa · cần phân công lại</span> : <span className="font-semibold text-emerald-700">Có thể xóa</span>}
      </td>
    </tr>
  );
}

function Metric({ label, value, tone = 'default' }: { label: string; value: number; tone?: 'default' | 'success' | 'warning' | 'danger' }) {
  const color = tone === 'success' ? 'text-emerald-700' : tone === 'warning' ? 'text-amber-700' : tone === 'danger' ? 'text-rose-700' : 'text-ink';
  return <div className="rounded-lg border border-line bg-white p-3"><p className="text-xs font-medium text-slate-500">{label}</p><p className={`mt-1 text-xl font-bold ${color}`}>{value}</p></div>;
}

function LoadingState() {
  return <div className="flex items-center gap-3 py-8 text-sm text-slate-600" role="status"><div className="h-5 w-5 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />Đang kiểm tra tài khoản và các liên kết nghiệp vụ...</div>;
}

function PreviewError({ error, onRetry, compact = false }: { error: unknown; onRetry: () => void; compact?: boolean }) {
  const message = error instanceof Error ? error.message : 'Không kiểm tra được danh sách nhân sự.';
  return <div className={`flex items-start justify-between gap-3 rounded-lg border border-rose-200 bg-rose-50 ${compact ? 'p-3' : 'p-4'}`}><div className="flex min-w-0 items-start gap-2 text-sm text-rose-800"><AlertCircle className="mt-0.5 h-4 w-4 shrink-0" /><div><p className="font-semibold">Không thể tiếp tục</p><p className="mt-1 text-xs leading-5">{message}</p></div></div><button type="button" className="inline-flex shrink-0 items-center gap-1 rounded-md border border-rose-200 bg-white px-2 py-1.5 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" />Kiểm tra lại</button></div>;
}
