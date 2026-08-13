import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, Loader2, Plus, Stamp, Upload } from 'lucide-react';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post, postForm } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { SealAsset } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { SealAssetPreview } from '../work-items/SealAssetPreview';

const maxSealFileSize = 10 * 1024 * 1024;
const acceptedSealTypes = new Set(['image/png', 'image/jpeg', 'image/webp']);

export function SettingsPage() {
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasPermission } = useAuth();
  const canManageSeals = hasPermission('seal.manage');
  const [uploadOpen, setUploadOpen] = useState(false);
  const [archiveCandidate, setArchiveCandidate] = useState<SealAsset | null>(null);
  const [name, setName] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [errors, setErrors] = useState<{ name?: string; file?: string }>({});
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => get<Record<string, any>[]>('/settings') });
  const seals = useQuery({
    queryKey: ['seal-assets', 'all'],
    queryFn: () => get<SealAsset[]>('/seal-assets?include_archived=true'),
    enabled: canManageSeals,
    retry: false,
  });

  const upload = useMutation({
    mutationFn: async () => {
      const body = new FormData();
      body.append('name', name.trim());
      if (file) body.append('file', file);
      return postForm<SealAsset>('/seal-assets/upload', body);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['seal-assets'] });
      closeUpload();
      toast.success('Đã lưu con dấu vào tài nguyên dùng chung');
    },
    onError: (error) => toast.error('Không tải được ảnh con dấu', error),
  });

  const archiveSeal = useMutation({
    mutationFn: (assetId: string) => post<SealAsset>(`/seal-assets/${assetId}/archive`, {}),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['seal-assets'] });
      setArchiveCandidate(null);
      toast.success('Đã ngừng sử dụng con dấu');
    },
    onError: (error) => toast.error('Không ngừng sử dụng được con dấu', error),
  });

  function validateAndUpload() {
    const nextErrors: { name?: string; file?: string } = {};
    if (!name.trim()) nextErrors.name = 'Vui lòng nhập tên con dấu.';
    if (!file) nextErrors.file = 'Vui lòng chọn ảnh con dấu.';
    else if (!acceptedSealTypes.has(file.type)) nextErrors.file = 'Chỉ chấp nhận PNG, JPG hoặc WebP; không hỗ trợ SVG.';
    else if (file.size > maxSealFileSize) nextErrors.file = 'Ảnh con dấu không được vượt quá 10 MB.';
    setErrors(nextErrors);
    if (Object.keys(nextErrors).length) {
      toast.warn({ title: 'Chưa thể tải con dấu', description: Object.values(nextErrors).join(' ') });
      return;
    }
    upload.mutate();
  }

  function closeUpload() {
    if (upload.isPending) return;
    setUploadOpen(false);
    setName('');
    setFile(null);
    setErrors({});
  }

  return (
    <div className="space-y-6">
      <PageHeader title="Cấu hình" description="Thiết lập hệ thống và quản lý tài nguyên dùng chung có kiểm soát quyền." />

      {canManageSeals ? (
        <section className="rounded-lg border border-line bg-white">
          <div className="flex flex-col gap-3 border-b border-line px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <div className="flex items-center gap-2"><Stamp className="h-5 w-5 text-blue-600" /><h2 className="text-base font-semibold text-ink">Tài nguyên con dấu</h2></div>
              <p className="mt-1 text-sm text-slate-600">Tải một lần, tái sử dụng khi xác nhận hồ sơ workflow. Con dấu đã ngừng dùng vẫn được giữ trong lịch sử.</p>
            </div>
            <button type="button" className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setUploadOpen(true)}>
              <Plus className="h-4 w-4" />Thêm con dấu
            </button>
          </div>
          <div className="p-4">
            {seals.isLoading ? (
              <div className="flex items-center justify-center gap-2 py-10 text-sm text-slate-500"><Loader2 className="h-5 w-5 animate-spin" />Đang tải tài nguyên con dấu</div>
            ) : seals.isError ? (
              <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">
                <p className="font-semibold">Không tải được tài nguyên con dấu</p>
                <button type="button" className="mt-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold" onClick={() => seals.refetch()}>Thử lại</button>
              </div>
            ) : seals.data?.length ? (
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {seals.data.map((asset) => (
                  <article key={asset.id} className="rounded-md border border-line bg-slate-50 p-3">
                    <SealAssetPreview path={`/seal-assets/${asset.id}/file`} alt={asset.name} className="h-36 w-full" />
                    <div className="mt-3 flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-ink">{asset.name}</p>
                        <p className="mt-1 text-xs text-slate-500">{asset.width_px}×{asset.height_px}px · {fileSizeText(asset.file_size)}</p>
                        <p className="mt-1 text-xs text-slate-500">Tải bởi {asset.uploaded_by_name} · {dateText(asset.created_at)}</p>
                      </div>
                      <StatusBadge status={asset.status} label={asset.status === 'active' ? 'Đang dùng' : 'Ngừng dùng'} />
                    </div>
                    {asset.status === 'active' ? (
                      <button type="button" className="mt-3 inline-flex w-full items-center justify-center gap-2 rounded-md border border-amber-200 bg-white px-3 py-2 text-xs font-semibold text-amber-700 hover:bg-amber-50" onClick={() => setArchiveCandidate(asset)}>
                        <Archive className="h-3.5 w-3.5" />Ngừng sử dụng
                      </button>
                    ) : <p className="mt-3 text-xs text-slate-500">Không còn xuất hiện trong danh sách chọn mới.</p>}
                  </article>
                ))}
              </div>
            ) : <EmptyState title="Chưa có tài nguyên con dấu. Tải ảnh đầu tiên để tái sử dụng trong hồ sơ workflow." />}
          </div>
        </section>
      ) : null}

      <section className="space-y-3">
        <div>
          <h2 className="text-base font-semibold text-ink">Thiết lập hệ thống</h2>
          <p className="mt-1 text-sm text-slate-600">Điều kiện pipeline, payment gate và xử lý vật tư dư.</p>
        </div>
        <DataTable rows={settings.data ?? []} columns={[
          { key: 'key', header: 'Key', render: (row) => <code className="rounded bg-slate-100 px-2 py-1 text-xs">{row.key}</code> },
          { key: 'description', header: 'Mô tả' },
          { key: 'value_json', header: 'Giá trị', render: (row) => <pre className="max-w-lg whitespace-pre-wrap text-xs">{JSON.stringify(row.value_json, null, 2)}</pre> },
        ]} />
      </section>

      <Dialog
        open={uploadOpen}
        title="Thêm tài nguyên con dấu"
        description="Chấp nhận PNG, JPG hoặc WebP tối đa 10 MB. Hệ thống chuẩn hóa ảnh sang PNG để dùng ổn định trong PDF."
        onClose={closeUpload}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" disabled={upload.isPending} onClick={closeUpload}>Hủy</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={upload.isPending} onClick={validateAndUpload}>
              {upload.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}Tải và lưu
            </button>
          </div>
        )}
      >
        <div className="space-y-4">
          <label className="block">
            <span className="text-sm font-semibold text-slate-700">Tên con dấu *</span>
            <input className={`mt-1 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 ${errors.name ? 'border-rose-300 bg-rose-50 focus:ring-rose-100' : 'border-line focus:ring-blue-100'}`} value={name} maxLength={160} disabled={upload.isPending} onChange={(event) => { setName(event.target.value); setErrors((current) => ({ ...current, name: undefined })); }} placeholder="Ví dụ: Dấu tròn Công ty TTCons" />
            {errors.name ? <p className="mt-1 text-xs font-medium text-rose-600">{errors.name}</p> : null}
          </label>
          <label className="block">
            <span className="text-sm font-semibold text-slate-700">Ảnh con dấu *</span>
            <input className={`mt-1 block w-full rounded-md border p-2 text-sm ${errors.file ? 'border-rose-300 bg-rose-50' : 'border-line bg-white'}`} type="file" accept="image/png,image/jpeg,image/webp" disabled={upload.isPending} onChange={(event) => { const next = event.target.files?.[0] ?? null; setFile(next); if (next && !name.trim()) setName(next.name.replace(/\.[^.]+$/, '')); setErrors((current) => ({ ...current, file: undefined })); }} />
            {errors.file ? <p className="mt-1 text-xs font-medium text-rose-600">{errors.file}</p> : <p className="mt-1 text-xs text-slate-500">Không hỗ trợ SVG. Nên dùng ảnh nền trong suốt, cắt sát viền con dấu.</p>}
          </label>
        </div>
      </Dialog>

      <Dialog
        open={Boolean(archiveCandidate)}
        title="Ngừng sử dụng con dấu?"
        description="Con dấu sẽ không còn xuất hiện trong danh sách chọn mới; các hồ sơ đã đóng dấu vẫn giữ nguyên."
        size="sm"
        onClose={() => { if (!archiveSeal.isPending) setArchiveCandidate(null); }}
        footer={(
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" disabled={archiveSeal.isPending} onClick={() => setArchiveCandidate(null)}>Hủy</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-amber-600 px-3 py-2 text-sm font-semibold text-white hover:bg-amber-700 disabled:opacity-60" disabled={archiveSeal.isPending || !archiveCandidate} onClick={() => archiveCandidate && archiveSeal.mutate(archiveCandidate.id)}>
              {archiveSeal.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Archive className="h-4 w-4" />}Ngừng sử dụng
            </button>
          </div>
        )}
      >
        <p className="text-sm text-slate-700">Tài nguyên: <span className="font-semibold">{archiveCandidate?.name}</span></p>
      </Dialog>
    </div>
  );
}

function fileSizeText(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
