import { useEffect, useMemo, useState } from 'react';
import { AlertCircle, AlertTriangle, RefreshCw, Trash2 } from 'lucide-react';
import { ApiError, post } from '../lib/api/client';
import { Dialog } from './Dialog';

type DeletionKind = 'partner' | 'project';
type DeletionIssue = { code: string; message: string; count?: number };
type PreviewRow = Record<string, any> & {
  can_delete: boolean;
  blockers: DeletionIssue[];
  warnings: DeletionIssue[];
  impact: Record<string, number>;
};
type DeletionPreview = {
  total_count: number;
  deletable_count: number;
  blocked_count: number;
  warning_count: number;
  preview_hash: string;
  impact_summary: Record<string, number>;
  rows: PreviewRow[];
};
export type BulkDeletionResult = {
  deletion_id: string;
  deleted_count: number;
  items: Record<string, any>[];
  idempotent_replay?: boolean;
};

const config: Record<DeletionKind, {
  title: string;
  description: string;
  itemLabel: string;
  confirmVerb: string;
  endpoint: string;
  idField: 'partner_ids' | 'project_ids';
  loading: string;
  warning: string;
}> = {
  partner: {
    title: 'Xác nhận xóa đối tác',
    description: 'Đối tác sẽ chuyển sang Inactive; dữ liệu nghiệp vụ và snapshot lịch sử vẫn được giữ lại.',
    itemLabel: 'đối tác',
    confirmVerb: 'vô hiệu hóa',
    endpoint: '/partners/deletions',
    idField: 'partner_ids',
    loading: 'Đang kiểm tra phạm vi danh mục và các liên kết nghiệp vụ...',
    warning: 'Đối tác đa loại sẽ bị ẩn khỏi cả Đầu vào và Đầu ra. Mọi liên kết lịch sử vẫn được giữ nguyên.',
  },
  project: {
    title: 'Xác nhận xóa công trình',
    description: 'Công trình sẽ chuyển sang Đã hủy; toàn bộ lịch sử workflow và nghiệp vụ vẫn được giữ lại.',
    itemLabel: 'công trình',
    confirmVerb: 'hủy',
    endpoint: '/projects/deletions',
    idField: 'project_ids',
    loading: 'Đang kiểm tra quyền và các hồ sơ nghiệp vụ của công trình...',
    warning: 'Workflow, tài chính, kho và nghiệm thu không bị xóa. Công trình đã hủy sẽ rời khỏi danh sách hoạt động.',
  },
};

export function BulkDeleteDialog({
  kind,
  open,
  ids,
  onClose,
  onSuccess,
  onError,
}: {
  kind: DeletionKind;
  open: boolean;
  ids: string[];
  onClose: () => void;
  onSuccess: (result: BulkDeletionResult) => Promise<void> | void;
  onError: (title: string, error: unknown) => void;
}) {
  const settings = config[kind];
  const [preview, setPreview] = useState<DeletionPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [executing, setExecuting] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const selectionKey = useMemo(() => [...ids].sort().join(','), [ids]);

  useEffect(() => {
    if (!open || !ids.length) return;
    setConfirmed(false);
    setIdempotencyKey(crypto.randomUUID());
    void loadPreview();
  }, [kind, open, selectionKey]);

  async function loadPreview() {
    setLoading(true);
    setError(null);
    setPreview(null);
    setConfirmed(false);
    try {
      const result = await post<DeletionPreview>(`${settings.endpoint}/preview`, { [settings.idField]: ids });
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
      const result = await post<BulkDeletionResult>(`${settings.endpoint}/execute`, {
        [settings.idField]: ids,
        preview_hash: preview.preview_hash,
        idempotency_key: idempotencyKey,
      });
      await onSuccess(result);
    } catch (requestError) {
      setError(requestError);
      onError(`Không xóa được ${settings.itemLabel}`, requestError);
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
      title={settings.title}
      description={settings.description}
      size="xl"
      onClose={() => { if (!busy) onClose(); }}
      footer={(
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-end">
          <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" onClick={onClose} disabled={busy}>Hủy</button>
          <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={execute} disabled={!canExecute}>
            <Trash2 className="h-4 w-4" />{executing ? 'Đang xử lý...' : `${kind === 'partner' ? 'Xóa' : 'Hủy'} ${preview?.total_count ?? ids.length} ${settings.itemLabel}`}
          </button>
        </div>
      )}
    >
      {loading ? <LoadingState message={settings.loading} /> : error && !preview ? <PreviewError error={error} itemLabel={settings.itemLabel} onRetry={loadPreview} /> : preview ? (
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <Metric label="Đã chọn" value={preview.total_count} />
            <Metric label="Có thể xử lý" value={preview.deletable_count} tone="success" />
            <Metric label="Bị chặn" value={preview.blocked_count} tone={preview.blocked_count ? 'danger' : 'default'} />
            <Metric label="Cảnh báo" value={preview.warning_count} tone={preview.warning_count ? 'warning' : 'default'} />
          </div>

          {preview.blocked_count > 0 ? (
            <Notice tone="danger" title="Chưa thể thực hiện" message="Bỏ chọn các dòng bị chặn, sau đó kiểm tra lại danh sách." />
          ) : preview.warning_count > 0 ? (
            <Notice tone="warning" title="Có dữ liệu cần lưu ý" message={settings.warning} />
          ) : null}

          <div className="overflow-hidden rounded-lg border border-line">
            <div className="consflow-table-viewport max-h-[42vh]">
              <table className="min-w-full divide-y divide-line">
                <thead className="bg-slate-50">
                  <tr>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">{kind === 'partner' ? 'Đối tác' : 'Công trình'}</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Phạm vi</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Ảnh hưởng</th>
                    <th className="px-3 py-2.5 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">Kết quả kiểm tra</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-line bg-white">
                  {preview.rows.map((row) => <PreviewRow key={row.partner_id ?? row.project_id} kind={kind} row={row} />)}
                </tbody>
              </table>
            </div>
          </div>

          {error ? <PreviewError error={error} itemLabel={settings.itemLabel} onRetry={loadPreview} compact /> : null}

          {preview.blocked_count === 0 ? (
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-line bg-slate-50 p-4">
              <input type="checkbox" className="mt-0.5 h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} disabled={busy} />
              <span className="text-sm leading-5 text-slate-700">
                <strong className="font-semibold text-ink">Tôi xác nhận {settings.confirmVerb} {preview.total_count} {settings.itemLabel}.</strong><br />
                <span className="text-xs text-slate-500">Thao tác được ghi audit, không xóa lịch sử nghiệp vụ và không thể hoàn tác trực tiếp từ hộp thoại này.</span>
              </span>
            </label>
          ) : null}
        </div>
      ) : null}
    </Dialog>
  );
}

function PreviewRow({ kind, row }: { kind: DeletionKind; row: PreviewRow }) {
  const impactEntries = Object.entries(row.impact ?? {}).filter(([, value]) => Number(value) > 0);
  const impactCount = impactEntries.reduce((total, [, value]) => total + Number(value), 0);
  const name = kind === 'partner' ? row.name : `${row.code} · ${row.name}`;
  const sub = kind === 'partner' ? row.partner_code : row.status;
  const scope = kind === 'partner'
    ? (row.catalog_groups ?? []).map((group: string) => group === 'input' ? 'Đầu vào' : 'Đầu ra').join(' + ')
    : 'Chuyển sang Đã hủy';
  return (
    <tr className={row.can_delete ? '' : 'bg-rose-50/60'}>
      <td className="px-3 py-3 align-top"><p className="text-sm font-semibold text-ink">{name}</p><p className="mt-1 text-xs text-slate-500">{sub}</p></td>
      <td className="px-3 py-3 align-top text-xs font-medium text-slate-600">{scope}</td>
      <td className="px-3 py-3 align-top text-xs text-slate-600">
        {impactCount ? <details><summary className="cursor-pointer font-semibold text-amber-700">{impactCount} liên kết</summary><ul className="mt-2 space-y-1">{impactEntries.map(([key, value]) => <li key={key}>{impactLabel(key)}: {Number(value)}</li>)}</ul></details> : <span className="text-slate-400">Không có liên kết đang hoạt động</span>}
      </td>
      <td className="px-3 py-3 align-top text-xs">
        {row.blockers?.length ? <ul className="space-y-1 text-rose-700">{row.blockers.map((item) => <li key={item.code}>{item.message}</li>)}</ul> : row.warnings?.length ? <ul className="space-y-1 text-amber-700">{row.warnings.map((item) => <li key={item.code}>{item.message}</li>)}</ul> : <span className="font-semibold text-emerald-700">Có thể xử lý</span>}
      </td>
    </tr>
  );
}

function impactLabel(key: string) {
  const labels: Record<string, string> = {
    project_count: 'Công trình', catalog_reference_count: 'Danh mục', procurement_reference_count: 'Mua hàng',
    contract_reference_count: 'Hợp đồng', finance_reference_count: 'Tài chính', acceptance_reference_count: 'Nghiệm thu',
    workflow_count: 'Quy trình', open_work_item_count: 'Công việc mở', open_issue_count: 'Vướng mắc mở',
    finance_record_count: 'Hồ sơ tài chính', stock_movement_count: 'Biến động kho', acceptance_count: 'Hồ sơ nghiệm thu',
  };
  return labels[key] ?? key;
}

function Metric({ label, value, tone = 'default' }: { label: string; value: number; tone?: 'default' | 'success' | 'warning' | 'danger' }) {
  const color = tone === 'success' ? 'text-emerald-700' : tone === 'warning' ? 'text-amber-700' : tone === 'danger' ? 'text-rose-700' : 'text-ink';
  return <div className="rounded-lg border border-line bg-white p-3"><p className="text-xs font-medium text-slate-500">{label}</p><p className={`mt-1 text-xl font-bold ${color}`}>{value}</p></div>;
}

function LoadingState({ message }: { message: string }) {
  return <div className="flex items-center gap-3 py-8 text-sm text-slate-600" role="status"><div className="h-5 w-5 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />{message}</div>;
}

function Notice({ tone, title, message }: { tone: 'warning' | 'danger'; title: string; message: string }) {
  const danger = tone === 'danger';
  const Icon = danger ? AlertCircle : AlertTriangle;
  return <div className={`flex items-start gap-3 rounded-lg border p-4 text-sm ${danger ? 'border-rose-200 bg-rose-50 text-rose-800' : 'border-amber-200 bg-amber-50 text-amber-900'}`}><Icon className="mt-0.5 h-5 w-5 shrink-0" /><div><p className="font-semibold">{title}</p><p className="mt-1 text-xs leading-5">{message}</p></div></div>;
}

function PreviewError({ error, itemLabel, onRetry, compact = false }: { error: unknown; itemLabel: string; onRetry: () => void; compact?: boolean }) {
  const message = error instanceof Error ? error.message : `Không kiểm tra được danh sách ${itemLabel}.`;
  return <div className={`flex items-start justify-between gap-3 rounded-lg border border-rose-200 bg-rose-50 ${compact ? 'p-3' : 'p-4'}`}><div className="flex min-w-0 items-start gap-2 text-sm text-rose-800"><AlertCircle className="mt-0.5 h-4 w-4 shrink-0" /><div><p className="font-semibold">Không thể tiếp tục</p><p className="mt-1 text-xs leading-5">{message}</p></div></div><button type="button" className="inline-flex shrink-0 items-center gap-1 rounded-md border border-rose-200 bg-white px-2 py-1.5 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" />Kiểm tra lại</button></div>;
}
