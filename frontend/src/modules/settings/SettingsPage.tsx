import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, Database, KeyRound, Loader2, LockKeyhole, Plus, RefreshCw, RotateCcw, Save, Settings2, Stamp, Upload } from 'lucide-react';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { PageHeader } from '../../components/PageHeader';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { get, post, postForm, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { SealAsset } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { firstInvalidFieldKey, useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { SealAssetPreview } from '../work-items/SealAssetPreview';
import { SampleDataPanel } from './SampleDataPanel';
import { SettingsBackupPanel } from './SettingsBackupPanel';

const maxSealFileSize = 10 * 1024 * 1024;
const acceptedSealTypes = new Set(['image/png', 'image/jpeg', 'image/webp']);

type SettingsTab = 'seals' | 'system' | 'sample-data';
type SettingOption = { value: string; label: string };
type SystemSetting = {
  key: string; category: string; label: string; description: string;
  value_type: 'boolean' | 'integer' | 'number' | 'duration' | 'string' | 'enum' | 'multi_select' | 'secret';
  value?: unknown; unit?: string; options?: SettingOption[]; min?: number | null; max?: number | null;
  sensitive: boolean; configured: boolean; requires_restart: boolean; restart_pending: boolean;
  version: number; source: string; updated_at?: string; updated_by?: string;
};
type CoreSetting = {
  key: string; label: string; description: string; value?: unknown; sensitive: boolean;
  configured: boolean; source: '.env'; readonly: true;
};
type SettingsResponse = {
  capabilities: { can_update: boolean };
  categories: Array<{ key: string; label: string; order: number }>;
  settings: SystemSetting[];
  core_settings: CoreSetting[];
  restart_required_count: number;
};

export function SettingsPage() {
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasPermission } = useAuth();
  const canManageSeals = hasPermission('seal.manage');
  const canCreateSampleData = hasPermission('sample_data.create');
  const [tab, setTab] = useState<SettingsTab>(canManageSeals ? 'seals' : 'system');
  const [uploadOpen, setUploadOpen] = useState(false);
  const [archiveCandidate, setArchiveCandidate] = useState<SealAsset | null>(null);
  const [name, setName] = useState('');
  const [file, setFile] = useState<File | null>(null);
  const [errors, setErrors] = useState<{ name?: string; file?: string }>({});
  const uploadContentRef = useRef<HTMLDivElement | null>(null);
  const navigateUploadValidation = useValidationNavigation(uploadContentRef);

  const settings = useQuery({ queryKey: ['settings'], queryFn: () => get<SettingsResponse>('/settings'), retry: false });
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
      navigateUploadValidation(firstInvalidFieldKey(nextErrors, ['name', 'file']));
      return;
    }
    upload.mutate();
  }
  function closeUpload() {
    if (upload.isPending) return;
    setUploadOpen(false); setName(''); setFile(null); setErrors({});
  }

  return (
    <div className="space-y-5">
      <PageHeader title="Cấu hình" />
      <div className="flex flex-wrap gap-2 border-b border-line" role="tablist" aria-label="Nhóm cấu hình">
        {canManageSeals ? <TabButton active={tab === 'seals'} onClick={() => setTab('seals')} icon={<Stamp className="h-4 w-4" />}>Con dấu</TabButton> : null}
        <TabButton active={tab === 'system'} onClick={() => setTab('system')} icon={<Settings2 className="h-4 w-4" />}>Thiết lập hệ thống</TabButton>
        {canCreateSampleData ? <TabButton active={tab === 'sample-data'} onClick={() => setTab('sample-data')} icon={<Database className="h-4 w-4" />}>Dữ liệu mẫu</TabButton> : null}
      </div>
      {tab === 'seals' && canManageSeals ? <SealTable rows={seals.data ?? []} loading={seals.isLoading} error={seals.error} onRetry={() => seals.refetch()} onUpload={() => setUploadOpen(true)} onArchive={setArchiveCandidate} /> : null}
      {tab === 'system' ? <SystemSettingsPanel data={settings.data} loading={settings.isLoading} error={settings.error} onRetry={() => settings.refetch()} /> : null}
      {tab === 'sample-data' && canCreateSampleData ? <SampleDataPanel /> : null}

      <Dialog open={uploadOpen} title="Thêm tài nguyên con dấu" description="PNG, JPG hoặc WebP · tối đa 10 MB." onClose={closeUpload} footer={(
        <div className="flex justify-end gap-2">
          <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" disabled={upload.isPending} onClick={closeUpload}>Hủy</button>
          <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={upload.isPending} onClick={validateAndUpload}>{upload.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}Tải và lưu</button>
        </div>
      )}>
        <div ref={uploadContentRef} className="space-y-4">
          <label data-validation-key="name" className="block scroll-mt-24 rounded-md"><span className="text-sm font-semibold text-slate-700">Tên con dấu *</span><input name="name" aria-invalid={Boolean(errors.name)} className={`mt-1 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 ${errors.name ? 'border-rose-300 bg-rose-50 focus:ring-rose-100' : 'border-line focus:ring-blue-100'}`} value={name} maxLength={160} disabled={upload.isPending} onChange={(event) => { setName(event.target.value); setErrors((current) => ({ ...current, name: undefined })); }} placeholder="Ví dụ: Dấu tròn Công ty TTCons" />{errors.name ? <p role="alert" className="mt-1 text-xs font-medium text-rose-600">{errors.name}</p> : null}</label>
          <label data-validation-key="file" className="block scroll-mt-24 rounded-md"><span className="text-sm font-semibold text-slate-700">Ảnh con dấu *</span><input name="file" aria-invalid={Boolean(errors.file)} className={`mt-1 block w-full rounded-md border p-2 text-sm ${errors.file ? 'border-rose-300 bg-rose-50' : 'border-line bg-white'}`} type="file" accept="image/png,image/jpeg,image/webp" disabled={upload.isPending} onChange={(event) => { const next = event.target.files?.[0] ?? null; setFile(next); if (next && !name.trim()) setName(next.name.replace(/\.[^.]+$/, '')); setErrors((current) => ({ ...current, file: undefined })); }} />{errors.file ? <p role="alert" className="mt-1 text-xs font-medium text-rose-600">{errors.file}</p> : <p className="mt-1 text-xs text-slate-500">Không hỗ trợ SVG. Nên dùng ảnh nền trong suốt, cắt sát viền con dấu.</p>}</label>
        </div>
      </Dialog>
      <Dialog open={Boolean(archiveCandidate)} title="Ngừng sử dụng con dấu?" description="Con dấu sẽ không còn xuất hiện trong danh sách chọn mới; các hồ sơ đã đóng dấu vẫn giữ nguyên." size="sm" onClose={() => { if (!archiveSeal.isPending) setArchiveCandidate(null); }} footer={(
        <div className="flex justify-end gap-2"><button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" disabled={archiveSeal.isPending} onClick={() => setArchiveCandidate(null)}>Hủy</button><button type="button" className="inline-flex items-center gap-2 rounded-md bg-amber-600 px-3 py-2 text-sm font-semibold text-white hover:bg-amber-700 disabled:opacity-60" disabled={archiveSeal.isPending || !archiveCandidate} onClick={() => archiveCandidate && archiveSeal.mutate(archiveCandidate.id)}>{archiveSeal.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Archive className="h-4 w-4" />}Ngừng sử dụng</button></div>
      )}><p className="text-sm text-slate-700">Tài nguyên: <span className="font-semibold">{archiveCandidate?.name}</span></p></Dialog>
    </div>
  );
}

function SealTable({ rows, loading, error, onRetry, onUpload, onArchive }: { rows: SealAsset[]; loading: boolean; error: unknown; onRetry: () => void; onUpload: () => void; onArchive: (asset: SealAsset) => void }) {
  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between"><div><h2 className="text-base font-semibold text-ink">Tài nguyên con dấu</h2><p className="mt-1 text-sm text-slate-600">Con dấu ngừng dùng vẫn được giữ để bảo toàn lịch sử hồ sơ.</p></div><button type="button" className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={onUpload}><Plus className="h-4 w-4" />Thêm con dấu</button></div>
      <DataTable<SealAsset>
        rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={12} searchPlaceholder="Tìm theo tên con dấu, người tải..." getRowKey={(row) => row.id}
        columns={[
          { key: 'actions', header: 'Thao tác', action: true, className: 'w-24', render: (row) => <RowActions>{row.status === 'active' ? <button type="button" title="Ngừng sử dụng" aria-label="Ngừng sử dụng" className="rounded-md border border-amber-200 p-1.5 text-amber-700 hover:bg-amber-50" onClick={() => onArchive(row)}><Archive className="h-4 w-4" /></button> : <span className="text-xs font-medium text-slate-400">Đã khóa</span>}</RowActions> },
          { key: 'thumbnail', header: 'Hình con dấu', className: 'w-36', render: (row) => <SealAssetPreview path={`/seal-assets/${row.id}/file`} alt={row.name} className="h-16 w-28" /> },
          { key: 'name', header: 'Tên con dấu', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.name}</span> },
          { key: 'dimensions', header: 'Kích thước', render: (row) => <span className="whitespace-nowrap">{row.width_px} × {row.height_px} px<br /><span className="text-xs text-slate-500">{fileSizeText(row.file_size)}</span></span> },
          { key: 'uploaded_by_name', header: 'Người tải', sortable: true, render: (row) => identityReferenceText(row.uploaded_by_name, row.uploaded_by_account_deleted) },
          { key: 'created_at', header: 'Ngày tải', sortable: true, render: (row) => dateText(row.created_at) },
          { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} label={row.status === 'active' ? 'Đang dùng' : 'Ngừng dùng'} /> },
        ]}
        empty="Chưa có tài nguyên con dấu."
        emptyAction={<button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={onUpload}>Thêm con dấu đầu tiên</button>}
        renderMobileCard={(row) => <article className="rounded-lg border border-line bg-white p-3"><div className="flex gap-3"><SealAssetPreview path={`/seal-assets/${row.id}/file`} alt={row.name} className="h-20 w-24 shrink-0" /><div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold text-ink">{row.name}</p><p className="mt-1 text-xs text-slate-500">{row.width_px} × {row.height_px} px · {fileSizeText(row.file_size)}</p><div className="mt-2"><StatusBadge status={row.status} label={row.status === 'active' ? 'Đang dùng' : 'Ngừng dùng'} /></div></div>{row.status === 'active' ? <button className="self-start rounded-md border border-amber-200 p-2 text-amber-700" aria-label="Ngừng sử dụng" onClick={() => onArchive(row)}><Archive className="h-4 w-4" /></button> : null}</div><p className="mt-3 text-xs text-slate-500">Tải bởi {identityReferenceText(row.uploaded_by_name, row.uploaded_by_account_deleted)} · {dateText(row.created_at)}</p></article>}
      />
    </section>
  );
}

function SystemSettingsPanel({ data, loading, error, onRetry }: { data?: SettingsResponse; loading: boolean; error: unknown; onRetry: () => void }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [category, setCategory] = useState('business');
  const [drafts, setDrafts] = useState<Record<string, unknown>>({});
  const [clearSecrets, setClearSecrets] = useState<Record<string, boolean>>({});
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [showCore, setShowCore] = useState(false);
  const settingsPanelRef = useRef<HTMLElement | null>(null);
  const navigateSettingsValidation = useValidationNavigation(settingsPanelRef);

  useEffect(() => {
    if (!data) return;
    const next: Record<string, unknown> = {};
    data.settings.forEach((setting) => { next[setting.key] = setting.sensitive ? '' : setting.value; });
    setDrafts(next); setClearSecrets({}); setFieldErrors({});
    if (!data.categories.some((item) => item.key === category)) setCategory(data.categories[0]?.key ?? 'business');
  }, [data]);

  const changed = useMemo(() => {
    if (!data) return [];
    return data.settings.filter((setting) => setting.sensitive
      ? Boolean(clearSecrets[setting.key] || String(drafts[setting.key] ?? '').length)
      : JSON.stringify(drafts[setting.key]) !== JSON.stringify(setting.value));
  }, [clearSecrets, drafts, data]);

  const save = useMutation({
    mutationFn: async () => {
      if (!data) return;
      await put('/settings', { updates: changed.map((setting) => ({ key: setting.key, value: normalizedDraftValue(setting, drafts[setting.key]), expected_version: setting.version, clear_secret: Boolean(clearSecrets[setting.key]) })) });
    },
    onSuccess: async () => { await queryClient.invalidateQueries({ queryKey: ['settings'] }); toast.success('Đã lưu thiết lập hệ thống'); },
    onError: (mutationError) => toast.error('Không lưu được thiết lập hệ thống', mutationError),
  });

  const requestSave = () => {
    const nextErrors: Record<string, string> = {};
    for (const setting of changed) {
      const value = drafts[setting.key];
      if ((setting.value_type === 'integer' || setting.value_type === 'number') && !Number.isFinite(Number(value))) nextErrors[setting.key] = 'Phải là số hợp lệ.';
      if (setting.value_type === 'multi_select' && (!Array.isArray(value) || value.length === 0)) nextErrors[setting.key] = 'Chọn ít nhất một phương án.';
    }
    setFieldErrors(nextErrors);
    if (Object.keys(nextErrors).length) {
      toast.warn({ title: 'Chưa thể lưu thiết lập', description: `${Object.keys(nextErrors).length} trường chưa hợp lệ.` });
      const firstKey = firstInvalidFieldKey(nextErrors, data?.settings.map((setting) => setting.key) ?? []);
      navigateSettingsValidation({
        key: firstKey,
        prepare: () => {
          const invalidSetting = data?.settings.find((setting) => setting.key === firstKey);
          if (invalidSetting) setCategory(invalidSetting.category);
        },
      });
      return;
    }
    save.mutate();
  };

  if (loading) return <div className="flex items-center justify-center gap-2 rounded-lg border border-line bg-white py-16 text-sm text-slate-500"><Loader2 className="h-5 w-5 animate-spin" />Đang tải thiết lập hệ thống</div>;
  if (error || !data) return <div className="rounded-lg border border-rose-200 bg-rose-50 p-5 text-sm text-rose-700"><p className="font-semibold">Không tải được thiết lập hệ thống</p><button className="mt-3 inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold" onClick={onRetry}><RefreshCw className="h-4 w-4" />Thử lại</button></div>;

  const canUpdate = data.capabilities.can_update;
  const visibleSettings = data.settings.filter((setting) => setting.category === category);
  return (
    <section ref={settingsPanelRef} className="space-y-4">
      <SettingsBackupPanel canRestore={canUpdate} />
      {!canUpdate ? <div className="flex items-start gap-3 rounded-lg border border-blue-200 bg-blue-50 p-4 text-sm text-blue-900"><LockKeyhole className="mt-0.5 h-5 w-5 shrink-0" /><div><p className="font-semibold">Chế độ chỉ xem</p><p className="mt-1 text-blue-800">Admin có thể xem toàn bộ thiết lập. Chỉ vai trò Nhà phát triển được phép chỉnh sửa và lưu.</p></div></div> : null}
      {data.restart_required_count > 0 ? <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900"><span className="font-semibold">{data.restart_required_count} thiết lập đang chờ khởi động lại backend.</span> Giá trị đã được lưu và sẽ có hiệu lực ở lần khởi động tiếp theo.</div> : null}
      <div className="grid gap-4 lg:grid-cols-[230px_minmax(0,1fr)]">
        <nav className="h-fit rounded-lg border border-line bg-white p-2" aria-label="Nhóm thiết lập hệ thống">
          {data.categories.map((item) => { const count = data.settings.filter((setting) => setting.category === item.key).length; return <button key={item.key} className={`flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm font-semibold ${category === item.key ? 'bg-blue-50 text-blue-700' : 'text-slate-600 hover:bg-slate-50'}`} onClick={() => setCategory(item.key)}><span>{item.label}</span><span className="rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-500">{count}</span></button>; })}
        </nav>
        <div className="min-w-0 space-y-3">{visibleSettings.map((setting) => <SettingField key={setting.key} setting={setting} value={drafts[setting.key]} clearSecret={Boolean(clearSecrets[setting.key])} disabled={!canUpdate || save.isPending} error={fieldErrors[setting.key]} onChange={(value) => { setDrafts((current) => ({ ...current, [setting.key]: value })); setClearSecrets((current) => ({ ...current, [setting.key]: false })); setFieldErrors((current) => ({ ...current, [setting.key]: '' })); }} onClearSecret={() => { setDrafts((current) => ({ ...current, [setting.key]: '' })); setClearSecrets((current) => ({ ...current, [setting.key]: true })); }} />)}</div>
      </div>
      {canUpdate ? <div className="sticky bottom-3 z-10 flex flex-col gap-3 rounded-lg border border-line bg-white/95 px-4 py-3 shadow-lg backdrop-blur sm:flex-row sm:items-center sm:justify-between"><p className="text-sm text-slate-600">{changed.length ? `${changed.length} thay đổi chưa lưu` : 'Không có thay đổi chưa lưu'}</p><div className="flex gap-2"><button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 disabled:opacity-50" disabled={!changed.length || save.isPending} onClick={() => { const next: Record<string, unknown> = {}; data.settings.forEach((setting) => { next[setting.key] = setting.sensitive ? '' : setting.value; }); setDrafts(next); setClearSecrets({}); setFieldErrors({}); }}><RotateCcw className="h-4 w-4" />Hoàn tác</button><button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={!changed.length || save.isPending} onClick={requestSave}>{save.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}Lưu thay đổi</button></div></div> : null}
      <div className="rounded-lg border border-line bg-white">
        <button className="flex w-full items-center justify-between gap-3 px-4 py-4 text-left" onClick={() => setShowCore((value) => !value)}><span className="flex items-center gap-2"><Database className="h-5 w-5 text-slate-500" /><span><span className="block text-sm font-semibold text-ink">Cấu hình lõi từ .env</span><span className="mt-0.5 block text-xs font-normal text-slate-500">Các field bootstrap, kết nối và root secret chỉ đọc; không quản lý qua database.</span></span></span><span className="text-xs font-semibold text-blue-700">{showCore ? 'Thu gọn' : 'Xem danh sách'}</span></button>
        {showCore ? <div className="border-t border-line"><DataTable<CoreSetting> rows={data.core_settings} getRowKey={(row) => row.key} searchPlaceholder="Tìm field lõi..." pageSize={20} columns={[
          { key: 'key', header: 'Field', render: (row) => <code className="text-xs font-semibold text-slate-700">{row.key}</code> },
          { key: 'label', header: 'Mục đích', render: (row) => <div><p className="font-semibold text-ink">{row.label}</p><p className="mt-1 text-xs text-slate-500">{row.description}</p></div> },
          { key: 'value', header: 'Giá trị / trạng thái', render: (row) => row.sensitive ? <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-600"><KeyRound className="h-3.5 w-3.5" />{row.configured ? 'Đã cấu hình' : 'Chưa cấu hình'}</span> : <code className="break-all text-xs">{displayValue(row.value)}</code> },
          { key: 'source', header: 'Nguồn', render: () => <span className="rounded bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">.env · chỉ đọc</span> },
        ]} /></div> : null}
      </div>
    </section>
  );
}

function SettingField({ setting, value, clearSecret, disabled, error, onChange, onClearSecret }: { setting: SystemSetting; value: unknown; clearSecret: boolean; disabled: boolean; error?: string; onChange: (value: unknown) => void; onClearSecret: () => void }) {
  const inputClass = `mt-2 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 disabled:bg-slate-50 disabled:text-slate-500 ${error ? 'border-rose-300 bg-rose-50 focus:ring-rose-100' : 'border-line focus:ring-blue-100'}`;
  return (
    <article data-validation-key={setting.key} data-validation-error={error ? 'true' : undefined} className="scroll-mt-24 rounded-lg border border-line bg-white p-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between"><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><h3 className="text-sm font-semibold text-ink">{setting.label}</h3>{setting.restart_pending ? <span className="rounded bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-800">Chờ restart</span> : setting.requires_restart ? <span className="rounded bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">Cần restart</span> : <span className="rounded bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700">Hiệu lực ngay</span>}</div><p className="mt-1 text-sm text-slate-600">{setting.description}</p><code className="mt-1 block text-[11px] text-slate-400">{setting.key}</code></div><span className="shrink-0 text-xs text-slate-400">v{setting.version}</span></div>
      {setting.value_type === 'boolean' ? <label className="mt-3 inline-flex items-center gap-2 text-sm font-semibold text-slate-700"><input type="checkbox" className="h-4 w-4 rounded border-line text-blue-600" checked={Boolean(value)} disabled={disabled} onChange={(event) => onChange(event.target.checked)} />{value ? 'Đang bật' : 'Đang tắt'}</label> : null}
      {setting.value_type === 'enum' ? <select className={inputClass} value={String(value ?? '')} disabled={disabled} onChange={(event) => onChange(event.target.value)}>{setting.options?.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select> : null}
      {setting.value_type === 'multi_select' ? <div className="mt-3 flex flex-wrap gap-2">{setting.options?.map((option) => { const values = Array.isArray(value) ? value.map(String) : []; return <label key={option.value} className={`inline-flex items-center gap-2 rounded-md border px-3 py-2 text-sm ${values.includes(option.value) ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line text-slate-600'}`}><input type="checkbox" checked={values.includes(option.value)} disabled={disabled} onChange={(event) => onChange(event.target.checked ? [...values, option.value] : values.filter((item) => item !== option.value))} />{option.label}</label>; })}</div> : null}
      {setting.value_type === 'integer' || setting.value_type === 'number' ? <div className="relative"><input type="number" step={setting.value_type === 'integer' ? 1 : 'any'} min={setting.min ?? undefined} max={setting.max ?? undefined} className={inputClass} value={String(value ?? '')} disabled={disabled} onChange={(event) => onChange(event.target.value)} />{setting.unit ? <span className="pointer-events-none absolute bottom-2 right-3 text-xs font-semibold text-slate-400">{setting.unit}</span> : null}</div> : null}
      {setting.value_type === 'duration' || setting.value_type === 'string' ? <input className={inputClass} value={String(value ?? '')} disabled={disabled} onChange={(event) => onChange(event.target.value)} placeholder={setting.value_type === 'duration' ? 'Ví dụ: 15s, 5m, 24h' : undefined} /> : null}
      {setting.value_type === 'secret' ? <div className="mt-2 space-y-2"><input type="password" className={inputClass.replace('mt-2 ', '')} value={String(value ?? '')} disabled={disabled} onChange={(event) => onChange(event.target.value)} placeholder={setting.configured ? 'Để trống để giữ nguyên secret hiện tại' : 'Nhập secret mới'} autoComplete="new-password" /><div className="flex flex-wrap items-center justify-between gap-2 text-xs"><span className={clearSecret ? 'font-semibold text-rose-700' : 'text-slate-500'}>{clearSecret ? 'Secret sẽ bị xóa khi lưu.' : setting.configured ? 'Đã có secret; giá trị hiện tại không được trả về trình duyệt.' : 'Chưa cấu hình secret.'}</span>{setting.configured && !disabled ? <button className="font-semibold text-rose-700 hover:text-rose-800" onClick={onClearSecret}>Xóa secret khi lưu</button> : null}</div></div> : null}
      {error ? <p role="alert" className="mt-2 text-xs font-semibold text-rose-600">{error}</p> : null}
      <p className="mt-3 text-[11px] text-slate-400">Nguồn: {sourceLabel(setting.source)}{setting.updated_by ? ` · Cập nhật bởi ${setting.updated_by}` : ''}{setting.updated_at ? ` · ${dateText(setting.updated_at)}` : ''}</p>
    </article>
  );
}

function TabButton({ active, onClick, icon, children }: { active: boolean; onClick: () => void; icon: React.ReactNode; children: React.ReactNode }) { return <button role="tab" aria-selected={active} className={`inline-flex items-center gap-2 border-b-2 px-4 py-3 text-sm font-semibold ${active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-slate-800'}`} onClick={onClick}>{icon}{children}</button>; }
function normalizedDraftValue(setting: SystemSetting, value: unknown) { return setting.value_type === 'integer' || setting.value_type === 'number' ? Number(value) : value; }
function displayValue(value: unknown) { if (Array.isArray(value)) return value.join(', '); if (typeof value === 'boolean') return value ? 'true' : 'false'; if (value == null || value === '') return '—'; return String(value); }
function sourceLabel(source: string) { if (source === 'legacy_env') return '.env được import lần đầu'; if (source === 'database_ui') return 'Database UI'; if (source === 'business_default') return 'Mặc định nghiệp vụ'; return 'Mặc định ứng dụng'; }
function fileSizeText(value: number) { if (value < 1024) return `${value} B`; if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`; return `${(value / 1024 / 1024).toFixed(1)} MB`; }
