import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { get, post } from '../../lib/api/client';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { PageHeader } from '../../components/PageHeader';
import { useToast } from '../../components/ToastProvider';

type RequestStatus = 'pending' | 'issued' | 'completed' | 'rejected';
type RecoveryRequest = {
  id: string;
  login_email: string;
  full_name: string;
  verified_contact_email?: string | null;
  status: RequestStatus;
  requested_at: string;
  issued_at?: string | null;
  completed_at?: string | null;
  rejected_at?: string | null;
};
type RequestPage = { items: RecoveryRequest[]; has_more: boolean; page: number };
type IssuedLink = { link: string; expires_at: string };

const statuses: { value: RequestStatus; label: string }[] = [
  { value: 'pending', label: 'Chờ xử lý' },
  { value: 'issued', label: 'Đã cấp liên kết' },
  { value: 'completed', label: 'Hoàn tất' },
  { value: 'rejected', label: 'Đã từ chối' },
];
const dateTime = (value?: string | null) => value ? new Date(value).toLocaleString('vi-VN') : '—';

export function RecoveryRequestsPage() {
  const [status, setStatus] = useState<RequestStatus>('pending');
  const [page, setPage] = useState(0);
  const [params, setParams] = useSearchParams();
  const selectedID = params.get('request') ?? '';
  const [confirmReject, setConfirmReject] = useState(false);
  const [issuedLink, setIssuedLink] = useState<IssuedLink | null>(null);
  const toast = useToast();
  const queryClient = useQueryClient();
  const list = useQuery({
    queryKey: ['developer-recovery-requests', status, page],
    queryFn: () => get<RequestPage>(`/account-recovery/requests?status=${status}&page=${page}`),
  });
  const detail = useQuery({
    queryKey: ['developer-recovery-request', selectedID],
    queryFn: () => get<RecoveryRequest>(`/account-recovery/requests/${selectedID}`),
    enabled: Boolean(selectedID),
  });
  const refresh = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['developer-recovery-requests'] }),
      queryClient.invalidateQueries({ queryKey: ['developer-recovery-request', selectedID] }),
      queryClient.invalidateQueries({ queryKey: ['notification-summary'] }),
      queryClient.invalidateQueries({ queryKey: ['notifications'] }),
    ]);
  };
  const issue = useMutation({
    mutationFn: () => post<IssuedLink>(`/account-recovery/requests/${selectedID}/issue`, {}),
    onSuccess: async (result) => { setIssuedLink(result); setConfirmReject(false); await refresh(); toast.success('Đã cấp liên kết tạm'); },
    onError: (error) => toast.error('Không cấp được liên kết', error),
  });
  const reject = useMutation({
    mutationFn: () => post(`/account-recovery/requests/${selectedID}/reject`, {}),
    onSuccess: async () => { await refresh(); closeDetail(); toast.success('Đã từ chối yêu cầu'); },
    onError: (error) => toast.error('Không từ chối được yêu cầu', error),
  });

  function openDetail(id: string) { setIssuedLink(null); setConfirmReject(false); setParams({ request: id }); }
  function closeDetail() { setParams({}); setIssuedLink(null); setConfirmReject(false); }
  function selectStatus(next: RequestStatus) { setStatus(next); setPage(0); }
  async function copyLink() {
    if (!issuedLink) return;
    try { await navigator.clipboard.writeText(issuedLink.link); toast.success('Đã sao chép liên kết'); }
    catch { toast.error('Không sao chép được liên kết'); }
  }
  function viewAction(item: RecoveryRequest) {
    return <button type="button" className="rounded-md border border-line bg-white px-3 py-1.5 text-sm font-semibold text-blue-700 hover:bg-blue-50" aria-label={`Xem yêu cầu của ${item.login_email}`} onClick={() => openDetail(item.id)}>Xem</button>;
  }

  const request = detail.data;
  return <div className="space-y-4 p-4 sm:p-6">
    <PageHeader title="Yêu cầu khôi phục" description="Chỉ Developer hệ thống được xử lý." />
    <div className="flex flex-wrap gap-2" aria-label="Trạng thái yêu cầu">
      {statuses.map((item) => <button key={item.value} type="button" aria-pressed={status === item.value} className={`rounded-md px-3 py-2 text-sm font-semibold ${status === item.value ? 'bg-blue-600 text-white' : 'border border-line bg-white text-slate-700'}`} onClick={() => selectStatus(item.value)}>{item.label}</button>)}
    </div>
    <DataTable
      rows={list.data?.items ?? []}
      loading={list.isLoading}
      error={list.error}
      onRetry={() => { void list.refetch(); }}
      empty="Không có yêu cầu trong trạng thái này."
      getRowKey={(item) => item.id}
      columns={[
        { key: 'open', header: 'Thao tác', render: viewAction },
        { key: 'login_email', header: 'Email đăng nhập', render: (item) => <span className="font-semibold text-ink">{item.login_email}</span> },
        { key: 'full_name', header: 'Họ tên' },
        { key: 'requested_at', header: 'Yêu cầu lúc', render: (item) => dateTime(item.requested_at) },
        { key: 'status', header: 'Trạng thái', render: (item) => <span className="font-medium text-blue-700">{statuses.find((entry) => entry.value === item.status)?.label}</span> },
      ]}
      renderMobileCard={(item) => <article className="space-y-3 rounded-lg border border-line bg-white p-4 text-sm">
        <div className="min-w-0 break-all font-semibold text-ink">{item.login_email}</div>
        <div className="text-slate-600">{item.full_name}</div>
        <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500"><span>{dateTime(item.requested_at)}</span><span className="font-semibold text-blue-700">{statuses.find((entry) => entry.value === item.status)?.label}</span></div>
        {viewAction(item)}
      </article>}
    />
    {list.data ? <div className="flex items-center justify-end gap-3 text-sm"><button type="button" disabled={page === 0} className="rounded-md border border-line px-3 py-2 disabled:opacity-40" onClick={() => setPage((value) => value - 1)}>Trước</button><span>Trang {page + 1}</span><button type="button" disabled={!list.data.has_more} className="rounded-md border border-line px-3 py-2 disabled:opacity-40" onClick={() => setPage((value) => value + 1)}>Tiếp</button></div> : null}
    <Dialog open={Boolean(selectedID)} title="Yêu cầu khôi phục mật khẩu" onClose={() => { if (!issue.isPending && !reject.isPending) closeDetail(); }}>
      {detail.isLoading ? <p className="text-sm text-slate-600">Đang tải chi tiết...</p> : null}
      {detail.isError ? <div className="space-y-2 text-sm text-rose-700"><p>Không tải được chi tiết yêu cầu.</p><button type="button" className="font-semibold underline" onClick={() => detail.refetch()}>Thử lại</button></div> : null}
      {request ? <div className="space-y-4 text-sm">
        <dl className="grid gap-2 rounded-md bg-slate-50 p-4 sm:grid-cols-2">
          <div><dt className="text-slate-500">Email đăng nhập</dt><dd className="font-semibold text-ink">{request.login_email}</dd></div>
          <div><dt className="text-slate-500">Họ tên</dt><dd className="font-semibold text-ink">{request.full_name}</dd></div>
          <div><dt className="text-slate-500">Email liên hệ đã xác minh</dt><dd>{request.verified_contact_email ?? 'Chưa có'}</dd></div>
          <div><dt className="text-slate-500">Yêu cầu lúc</dt><dd>{dateTime(request.requested_at)}</dd></div>
          <div><dt className="text-slate-500">Trạng thái</dt><dd>{statuses.find((item) => item.value === request.status)?.label}</dd></div>
          {request.issued_at ? <div><dt className="text-slate-500">Cấp liên kết lúc</dt><dd>{dateTime(request.issued_at)}</dd></div> : null}
        </dl>
        {request.status === 'pending' || request.status === 'issued' ? <>
          <p className="rounded-md bg-amber-50 p-3 text-amber-800">Chỉ gửi liên kết cho đúng người yêu cầu qua kênh liên hệ tin cậy. Liên kết có hiệu lực tối đa 15 phút và chỉ sử dụng một lần.</p>
          <div className="flex flex-wrap gap-2"><button type="button" disabled={issue.isPending || reject.isPending} className="rounded-md bg-blue-600 px-4 py-2 font-semibold text-white disabled:opacity-50" onClick={() => issue.mutate()}>Tạo liên kết tạm</button><button type="button" disabled={issue.isPending || reject.isPending} className="rounded-md border border-rose-300 px-4 py-2 font-semibold text-rose-700 disabled:opacity-50" onClick={() => setConfirmReject(true)}>Từ chối yêu cầu</button></div>
          {confirmReject ? <div className="space-y-2 rounded-md border border-rose-200 bg-rose-50 p-3 text-rose-800"><p>Từ chối yêu cầu và vô hiệu liên kết đã cấp?</p><div className="flex gap-2"><button type="button" disabled={reject.isPending} className="rounded-md bg-rose-700 px-3 py-2 font-semibold text-white" onClick={() => reject.mutate()}>Xác nhận từ chối</button><button type="button" className="rounded-md border border-line bg-white px-3 py-2" onClick={() => setConfirmReject(false)}>Hủy</button></div></div> : null}
        </> : null}
        {issuedLink ? <div className="space-y-2 rounded-md border border-blue-200 bg-blue-50 p-3"><p className="font-semibold text-blue-900">Liên kết chỉ hiển thị đến khi đóng cửa sổ này.</p><input readOnly aria-label="Liên kết đặt lại mật khẩu" value={issuedLink.link} className="w-full rounded-md border border-blue-200 bg-white px-3 py-2 font-mono text-xs" /><p>Hết hạn: {dateTime(issuedLink.expires_at)}</p><button type="button" className="rounded-md bg-blue-600 px-3 py-2 font-semibold text-white" onClick={copyLink}>Sao chép liên kết</button></div> : null}
      </div> : null}
    </Dialog>
  </div>;
}
