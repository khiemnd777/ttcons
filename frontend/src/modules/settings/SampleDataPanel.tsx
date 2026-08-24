import { useEffect, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Database, Loader2, RefreshCw, UserPlus, Users } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';

type SampleProfile = 'draft' | 'planning' | 'completed';

type SampleDataBlocker = {
  code: string;
  message: string;
};

type SampleRoleCoverage = {
  role_id: string;
  role_key: string;
  role_name: string;
  email: string;
  status: 'ready' | 'missing' | 'blocked';
  message: string;
  staff_count: number;
  ready: boolean;
};

type SamplePipelineOption = {
  id: string;
  name: string;
  revision: number;
  definition_hash: string;
  engine_version: number;
  node_count: number;
  selected_step_count: number;
  structure_compatible: boolean;
  sample_users_ready: boolean;
  sample_users_provisionable: boolean;
  compatible: boolean;
  blockers: SampleDataBlocker[];
  role_coverage: SampleRoleCoverage[];
  missing_roles: string[];
};

type SampleAccountingBookOption = {
  id: string;
  code: string;
  name: string;
  base_currency: string;
  compatible: boolean;
  blockers: SampleDataBlocker[];
};

type SampleDataOptions = {
  ready: boolean;
  blueprint_version: number;
  progress_presets: number[];
  pipelines: SamplePipelineOption[];
  accounting_books: SampleAccountingBookOption[];
  environment_policy: string;
};

type SampleDataResult = {
  id: string;
  name: string;
  code: string;
  status: string;
  progress_percent: number;
  sample_profile: SampleProfile;
  pipeline_template_id: string;
  accounting_book_id: string;
  current_pipeline_run_id?: string;
  replayed?: boolean;
};

type SampleUserProvisionResult = {
  batch_id: string;
  created_count: number;
  existing_count: number;
  role_count: number;
  ready: boolean;
  message: string;
};

const profileCards: Array<{ value: SampleProfile; title: string; description: string }> = [
  { value: 'draft', title: 'Công trình mới', description: 'Tạo hồ sơ công trình ở trạng thái Nháp, chưa khởi chạy quy trình.' },
  { value: 'planning', title: 'Đang lập kế hoạch', description: 'Khởi chạy nhánh Có thiết kế, giao việc đúng vai trò và đặt tiến độ đã chọn.' },
  { value: 'completed', title: 'Đã hoàn thành', description: 'Tạo đầy đủ quy trình, biểu mẫu, phê duyệt, tài chính và minh chứng lịch sử.' },
];

function BlockerList({ blockers }: { blockers: SampleDataBlocker[] }) {
  if (!blockers.length) return null;
  return (
    <ul className="mt-2 list-disc space-y-1 pl-5 text-xs text-rose-700">
      {blockers.map((blocker) => <li key={`${blocker.code}:${blocker.message}`}>{blocker.message}</li>)}
    </ul>
  );
}

export function SampleDataPanel() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [profile, setProfile] = useState<SampleProfile>('planning');
  const [progress, setProgress] = useState(40);
  const [pipelineID, setPipelineID] = useState('');
  const [accountingBookID, setAccountingBookID] = useState('');
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');
  const [created, setCreated] = useState<SampleDataResult | null>(null);

  const options = useQuery({
    queryKey: ['settings', 'sample-data', 'options'],
    queryFn: () => get<SampleDataOptions>('/settings/sample-data/options'),
    retry: false,
  });

  useEffect(() => {
    if (!options.data) return;
    const compatible = options.data.pipelines.filter((item) => item.sample_users_provisionable || item.compatible);
    setPipelineID((current) => options.data?.pipelines.some((item) => item.id === current)
      ? current
      : compatible.length === 1 ? compatible[0].id : '');
  }, [options.data]);

  useEffect(() => {
    if (!options.data) return;
    const compatible = options.data.accounting_books.filter((item) => item.compatible);
    setAccountingBookID((current) => options.data?.accounting_books.some((item) => item.id === current)
      ? current
      : compatible.length === 1 ? compatible[0].id : '');
  }, [options.data]);

  const createSample = useMutation({
    mutationFn: () => post<SampleDataResult>('/settings/sample-data/projects', {
      profile,
      progress_percent: profile === 'draft' ? 0 : profile === 'completed' ? 100 : progress,
      pipeline_template_id: pipelineID,
      accounting_book_id: profile === 'completed' ? accountingBookID : '',
      idempotency_key: idempotencyKey,
    }),
    onSuccess: async (result) => {
      setCreated(result);
      setConfirmOpen(false);
      setIdempotencyKey('');
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['projects'] }),
        queryClient.invalidateQueries({ queryKey: ['project-explorer'] }),
        queryClient.invalidateQueries({ queryKey: ['my-work-items'] }),
      ]);
      toast.success(result.replayed ? 'Đã trả lại kết quả của yêu cầu tạo trước đó' : 'Đã tạo công trình dữ liệu mẫu');
    },
    onError: (error) => toast.error('Không tạo được công trình dữ liệu mẫu', error),
  });

  const createSampleUsers = useMutation({
    mutationFn: () => post<SampleUserProvisionResult>('/settings/sample-data/users', { pipeline_template_id: pipelineID }),
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['settings', 'sample-data', 'options'] }),
        queryClient.invalidateQueries({ queryKey: ['staff'] }),
      ]);
      if (result.created_count === 0) {
        toast.success(result.message);
      } else {
        toast.success({
          title: `Đã tạo ${result.created_count} tài khoản mẫu`,
          description: `${result.role_count}/${result.role_count} vai trò đã sẵn sàng.`,
        });
      }
    },
    onError: (error) => toast.error('Không tạo được tài khoản mẫu', error),
  });

  if (options.isLoading) {
    return <div className="flex items-center justify-center gap-2 rounded-lg border border-line bg-white py-16 text-sm text-slate-500"><Loader2 className="h-5 w-5 animate-spin" />Đang kiểm tra điều kiện tạo dữ liệu mẫu</div>;
  }
  if (options.error || !options.data) {
    return (
      <div className="rounded-lg border border-rose-200 bg-rose-50 p-5 text-sm text-rose-700">
        <p className="font-semibold">Không tải được cấu hình dữ liệu mẫu</p>
        <p className="mt-1">Hãy kiểm tra quyền Nhà phát triển và các nguồn dữ liệu đã phát hành trong môi trường hiện tại.</p>
        <button className="mt-3 inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold" onClick={() => options.refetch()}><RefreshCw className="h-4 w-4" />Thử lại</button>
      </div>
    );
  }

  const data = options.data;
  const selectedPipeline = data.pipelines.find((item) => item.id === pipelineID);
  const selectedAccountingBook = data.accounting_books.find((item) => item.id === accountingBookID);
  const hasProjectAccountingBook = data.accounting_books.length > 0;
  const canCreate = Boolean(selectedPipeline?.compatible)
    && hasProjectAccountingBook
    && (profile !== 'completed' || Boolean(selectedAccountingBook?.compatible));
  const finalProgress = profile === 'draft' ? 0 : profile === 'completed' ? 100 : progress;
  const selectedProfile = profileCards.find((item) => item.value === profile)!;
  const completedSteps = selectedPipeline ? Math.floor(finalProgress * selectedPipeline.selected_step_count / 100) : 0;
  const missingSampleUsers = selectedPipeline?.role_coverage.filter((role) => role.status === 'missing').length ?? 0;

  const requestCreate = () => {
    if (!selectedPipeline) {
      toast.warn({ title: 'Chưa chọn Pipeline', description: 'Hãy chọn Pipeline đã phát hành dùng để tạo công trình mẫu.' });
      return;
    }
    if (!selectedPipeline.compatible) {
      toast.warn({ title: 'Pipeline chưa tương thích', description: selectedPipeline.blockers.map((item) => item.message).join('; ') });
      return;
    }
    if (!hasProjectAccountingBook) {
      toast.warn({ title: 'Chưa có sổ kế toán', description: 'Môi trường hiện tại cần ít nhất một sổ kế toán đang hoạt động để gắn với công trình.' });
      return;
    }
    if (profile === 'completed' && !selectedAccountingBook?.compatible) {
      toast.warn({ title: 'Sổ kế toán chưa đủ điều kiện', description: selectedAccountingBook?.blockers.map((item) => item.message).join('; ') || 'Hãy chọn sổ kế toán tương thích.' });
      return;
    }
    setCreated(null);
    setIdempotencyKey(crypto.randomUUID());
    setConfirmOpen(true);
  };

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-3 rounded-lg border border-line bg-white p-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-base font-semibold text-ink">Tạo dữ liệu Công trình mẫu</h2>
            <span className="rounded bg-violet-50 px-2 py-0.5 text-xs font-semibold text-violet-700">Chỉ Nhà phát triển</span>
          </div>
          <p className="mt-1 text-sm text-slate-600">Chọn nguồn đã có trong môi trường hiện tại. Hệ thống kiểm tra cấu trúc nghiệp vụ, không phụ thuộc ID giữa local và production.</p>
        </div>
        <span className={`inline-flex shrink-0 items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold ${canCreate ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700'}`}>
          {canCreate ? <CheckCircle2 className="h-4 w-4" /> : <AlertTriangle className="h-4 w-4" />}
          {canCreate ? 'Sẵn sàng' : 'Cần chọn đủ nguồn'}
        </span>
      </div>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="space-y-4">
          <div className="rounded-lg border border-line bg-white p-4">
            <h3 className="text-sm font-semibold text-ink">1. Chọn Pipeline đã phát hành</h3>
            <p className="mt-1 text-xs text-slate-500">Pipeline import lên production có ID riêng; hãy chọn đúng bản cần dùng và kiểm tra trạng thái tương thích.</p>
            <select className="mt-3 w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-800" value={pipelineID} onChange={(event) => setPipelineID(event.target.value)}>
              <option value="">Chọn Pipeline</option>
              {data.pipelines.map((item) => <option key={item.id} value={item.id}>{item.name} · Rev{item.revision} · {item.compatible ? 'Sẵn sàng' : item.sample_users_provisionable ? 'Cần tài khoản mẫu' : 'Không tương thích'}</option>)}
            </select>
            {data.pipelines.length === 0 ? <p className="mt-2 text-xs text-rose-700">Chưa có Pipeline nào ở trạng thái Đã phát hành.</p> : null}
            {selectedPipeline ? (
              <div className={`mt-3 rounded-md border p-3 ${selectedPipeline.compatible ? 'border-emerald-200 bg-emerald-50' : selectedPipeline.sample_users_provisionable ? 'border-amber-200 bg-amber-50' : 'border-rose-200 bg-rose-50'}`}>
                <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                  <span className={`font-semibold ${selectedPipeline.compatible ? 'text-emerald-800' : selectedPipeline.sample_users_provisionable ? 'text-amber-800' : 'text-rose-800'}`}>{selectedPipeline.compatible ? 'Sẵn sàng' : selectedPipeline.sample_users_provisionable ? 'Cần tạo tài khoản mẫu' : 'Không tương thích'}</span>
                  <span className="text-slate-600">{selectedPipeline.node_count} node · {selectedPipeline.selected_step_count} bước nhánh Có thiết kế</span>
                </div>
                <p className="mt-1 break-all text-[11px] text-slate-500">Hash: {selectedPipeline.definition_hash || 'Chưa có'}</p>
                <BlockerList blockers={selectedPipeline.blockers} />
              </div>
            ) : null}
          </div>

          <div className="rounded-lg border border-line bg-white p-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <h3 className="text-sm font-semibold text-ink">2. Tài khoản mẫu theo vai trò</h3>
                <p className="mt-1 text-xs leading-5 text-slate-500">Chỉ tạo các vai trò Pipeline cần dùng. Tên tài khoản là tên vai trò, email có dạng &lt;role&gt;@dev.local và không được gán vai trò toàn hệ thống.</p>
              </div>
              {selectedPipeline?.sample_users_ready ? <span className="inline-flex shrink-0 items-center gap-1 rounded bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700"><CheckCircle2 className="h-3.5 w-3.5" />Đủ {selectedPipeline.role_coverage.length}/{selectedPipeline.role_coverage.length}</span> : null}
            </div>
            {!selectedPipeline ? <p className="mt-3 rounded-md bg-slate-50 p-3 text-xs text-slate-500">Chọn Pipeline để xác định các vai trò cần tạo.</p> : (
              <div className="mt-3 space-y-2">
                {selectedPipeline.role_coverage.map((role) => (
                  <div key={`${role.role_id}:${role.role_name}`} className="flex flex-col gap-2 rounded-md border border-line p-3 sm:flex-row sm:items-center sm:justify-between">
                    <div className="min-w-0"><p className="truncate text-sm font-semibold text-slate-800">{role.role_name}</p><p className="mt-0.5 truncate text-xs text-slate-500">{role.email || role.message}</p></div>
                    <span className={`shrink-0 rounded px-2 py-1 text-xs font-semibold ${role.status === 'ready' ? 'bg-emerald-50 text-emerald-700' : role.status === 'missing' ? 'bg-amber-50 text-amber-700' : 'bg-rose-50 text-rose-700'}`}>{role.status === 'ready' ? 'Sẵn sàng' : role.status === 'missing' ? 'Chưa tạo' : 'Bị chặn'}</span>
                  </div>
                ))}
                {selectedPipeline.sample_users_ready ? <p className="rounded-md bg-emerald-50 p-3 text-xs font-semibold text-emerald-800">Đã có đủ tài khoản mẫu cho {selectedPipeline.role_coverage.length}/{selectedPipeline.role_coverage.length} vai trò.</p> : null}
                {!selectedPipeline.sample_users_ready && selectedPipeline.sample_users_provisionable ? (
                  <div className="flex flex-col gap-3 rounded-md border border-blue-100 bg-blue-50 p-3 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-xs leading-5 text-blue-900">Mật khẩu đăng nhập chung: <code className="rounded bg-white px-1.5 py-0.5 font-semibold">Dev240489.</code></p>
                    <button type="button" className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={createSampleUsers.isPending || missingSampleUsers === 0} onClick={() => createSampleUsers.mutate()}>{createSampleUsers.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <UserPlus className="h-4 w-4" />}Tạo {missingSampleUsers} tài khoản còn thiếu</button>
                  </div>
                ) : null}
              </div>
            )}
          </div>

          <div className="rounded-lg border border-line bg-white p-4">
            <h3 className="text-sm font-semibold text-ink">3. Chọn loại hồ sơ</h3>
            <div className="mt-3 grid gap-3 md:grid-cols-3">
              {profileCards.map((item) => (
                <button key={item.value} type="button" className={`rounded-lg border p-4 text-left transition ${profile === item.value ? 'border-blue-400 bg-blue-50 ring-2 ring-blue-100' : 'border-line bg-white hover:border-blue-200 hover:bg-slate-50'}`} aria-pressed={profile === item.value} onClick={() => setProfile(item.value)}>
                  <span className="block text-sm font-semibold text-ink">{item.title}</span>
                  <span className="mt-1 block text-xs leading-5 text-slate-600">{item.description}</span>
                  <span className="mt-3 block text-xs font-bold text-blue-700">{item.value === 'draft' ? '0%' : item.value === 'completed' ? '100%' : `${progress}%`}</span>
                </button>
              ))}
            </div>
          </div>

          {profile === 'planning' ? (
            <div className="rounded-lg border border-line bg-white p-4">
              <h3 className="text-sm font-semibold text-ink">4. Chọn tiến độ lập kế hoạch</h3>
              <p className="mt-1 text-xs text-slate-500">Tiến độ được ánh xạ theo số bước thực tế của Pipeline đã chọn; 100% chỉ dành cho hồ sơ Đã hoàn thành.</p>
              <div className="mt-3 flex flex-wrap gap-2">
                {data.progress_presets.map((value) => <button key={value} type="button" aria-pressed={progress === value} className={`min-w-14 rounded-md border px-3 py-2 text-sm font-semibold ${progress === value ? 'border-blue-600 bg-blue-600 text-white' : 'border-line bg-white text-slate-700 hover:bg-slate-50'}`} onClick={() => setProgress(value)}>{value}%</button>)}
              </div>
              {selectedPipeline ? <p className="mt-3 text-xs font-medium text-slate-700">Dự kiến hoàn tất {completedSteps}/{selectedPipeline.selected_step_count} bước.</p> : null}
            </div>
          ) : null}

          {profile === 'completed' ? (
            <div className="rounded-lg border border-line bg-white p-4">
              <h3 className="text-sm font-semibold text-ink">4. Chọn sổ kế toán</h3>
              <p className="mt-1 text-xs text-slate-500">Hồ sơ hoàn thành sinh dữ liệu tài chính; sổ phải có tài khoản 1121, 154, 5111 và kỳ kế toán đang mở.</p>
              <select className="mt-3 w-full rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-800" value={accountingBookID} onChange={(event) => setAccountingBookID(event.target.value)}>
                <option value="">Chọn sổ kế toán</option>
                {data.accounting_books.map((item) => <option key={item.id} value={item.id}>{item.code} · {item.name} · {item.compatible ? 'Tương thích' : 'Không tương thích'}</option>)}
              </select>
              {selectedAccountingBook ? (
                <div className={`mt-3 rounded-md border p-3 text-xs ${selectedAccountingBook.compatible ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-rose-200 bg-rose-50 text-rose-800'}`}>
                  <p className="font-semibold">{selectedAccountingBook.compatible ? 'Tương thích cho dữ liệu tài chính' : 'Chưa đủ điều kiện'} · {selectedAccountingBook.base_currency}</p>
                  <BlockerList blockers={selectedAccountingBook.blockers} />
                </div>
              ) : null}
            </div>
          ) : null}

          {created ? (
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4">
              <div className="flex items-start gap-3"><CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-700" /><div><p className="text-sm font-semibold text-emerald-900">Đã tạo {created.code}</p><p className="mt-1 text-sm text-emerald-800">{created.name} · {created.progress_percent}%</p></div></div>
              <div className="mt-4 flex flex-wrap gap-2"><Link className="rounded-md bg-emerald-700 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-800" to={`/projects/${created.id}`}>Mở công trình</Link><Link className="rounded-md border border-emerald-300 bg-white px-3 py-2 text-sm font-semibold text-emerald-800 hover:bg-emerald-100" to="/my-work">Mở Bàn làm việc</Link></div>
            </div>
          ) : null}

          <div className="flex justify-end"><button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" disabled={!canCreate || createSample.isPending} onClick={requestCreate}><Database className="h-4 w-4" />Tạo dữ liệu mẫu</button></div>
        </div>

        <aside className="h-fit rounded-lg border border-line bg-white p-4">
          <div className="flex items-center gap-2"><Users className="h-5 w-5 text-blue-700" /><h3 className="text-sm font-semibold text-ink">Điều kiện nguồn đã chọn</h3></div>
          <dl className="mt-3 space-y-2 border-b border-line pb-3 text-xs">
            <div className="flex justify-between gap-3"><dt className="text-slate-500">Pipeline</dt><dd className="text-right font-semibold text-slate-700">{selectedPipeline?.name || 'Chưa chọn'}</dd></div>
            <div className="flex justify-between gap-3"><dt className="text-slate-500">Revision</dt><dd className="font-semibold text-slate-700">{selectedPipeline ? `Rev${selectedPipeline.revision}` : '—'}</dd></div>
            <div className="flex justify-between gap-3"><dt className="text-slate-500">Blueprint sinh mẫu</dt><dd className="font-semibold text-slate-700">v{data.blueprint_version}</dd></div>
          </dl>
          <div className="mt-3 space-y-2">
            {selectedPipeline?.role_coverage.map((role) => <div key={role.role_name} className="flex items-center justify-between gap-3 text-xs"><span className="truncate text-slate-600">{role.role_name}</span><span className={`shrink-0 rounded px-2 py-0.5 font-semibold ${role.ready ? 'bg-emerald-50 text-emerald-700' : role.status === 'missing' ? 'bg-amber-50 text-amber-700' : 'bg-rose-50 text-rose-700'}`}>{role.ready ? '1 tài khoản mẫu' : role.status === 'missing' ? 'Chưa tạo' : 'Bị chặn'}</span></div>)}
            {!selectedPipeline ? <p className="text-xs text-slate-500">Chọn Pipeline để xem độ phủ tài khoản mẫu theo vai trò.</p> : null}
          </div>
          <p className="mt-4 text-xs leading-5 text-slate-500">Công trình mẫu chỉ nhận tài khoản có cờ dữ liệu mẫu; tài khoản người dùng thật không được thêm hoặc giao việc. Đầu việc hiện tại hiển thị tại Bàn làm việc; bước sau hiển thị ở Sắp tới.</p>
        </aside>
      </div>

      <Dialog open={confirmOpen} title="Xác nhận tạo dữ liệu mẫu" description="Thao tác sẽ ghi dữ liệu thật vào demo site và xuất hiện trong báo cáo." size="sm" onClose={() => { if (!createSample.isPending) setConfirmOpen(false); }} footer={<div className="flex justify-end gap-2"><button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" disabled={createSample.isPending} onClick={() => setConfirmOpen(false)}>Hủy</button><button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" disabled={createSample.isPending} onClick={() => createSample.mutate()}>{createSample.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Database className="h-4 w-4" />}Tạo công trình</button></div>}>
        <dl className="space-y-3 text-sm">
          <div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Pipeline nguồn</dt><dd className="mt-1 font-semibold text-ink">{selectedPipeline?.name} · Rev{selectedPipeline?.revision}</dd></div>
          <div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Loại hồ sơ</dt><dd className="mt-1 font-semibold text-ink">{selectedProfile.title}</dd></div>
          <div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tiến độ</dt><dd className="mt-1 font-semibold text-ink">{finalProgress}% · {completedSteps}/{selectedPipeline?.selected_step_count || 0} bước hoàn tất</dd></div>
          {profile === 'completed' ? <div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Sổ kế toán</dt><dd className="mt-1 font-semibold text-ink">{selectedAccountingBook?.code} · {selectedAccountingBook?.name}</dd></div> : null}
          <div><dt className="text-xs font-semibold uppercase tracking-wide text-slate-500">Dữ liệu sinh kèm</dt><dd className="mt-1 leading-5 text-slate-700">{profile === 'draft' ? 'Thông tin công trình mẫu và thành viên mẫu.' : profile === 'planning' ? 'Pipeline đã chọn, thành viên mẫu, giao việc, biểu mẫu và minh chứng ở các bước đã hoàn tất.' : 'Toàn bộ Pipeline đã chọn, tài khoản mẫu, biểu mẫu, phê duyệt, nghiệp vụ, tài chính, kho, nghiệm thu, bàn giao và minh chứng lịch sử.'}</dd></div>
          <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 text-xs font-medium leading-5 text-emerald-900">Không tài khoản người dùng thật nào được thêm hoặc giao việc trong công trình mẫu này.</div>
        </dl>
      </Dialog>
    </section>
  );
}
