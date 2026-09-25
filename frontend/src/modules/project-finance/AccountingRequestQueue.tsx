import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { AccountingOperationsButton } from './AccountingOperationsDialog';
import { requestStatuses } from './accountingOperations';
export function AccountingRequestQueue({ projectId = '' }: {
    projectId?: string;
}) {
    const { user, hasPermission } = useAuth();
    const enabled = hasPermission('accounting_request.view');
    const query = useQuery({ queryKey: ['accounting-request-queue', user?.id], queryFn: () => get<{
            id: string;
            project_id: string;
            title: string;
            project_code: string;
            status: string;
        }[]>('/me/accounting-requests'), enabled, staleTime: 0, refetchOnMount: 'always' });
    if (!enabled) return null;
    const requests = query.data?.filter((request) => !projectId || request.project_id === projectId) ?? [];
    if (!query.isPending && !query.isError && requests.length === 0) return null;
    return <section className="mb-3 space-y-2 rounded-lg border border-line bg-white p-3" aria-label="Đề nghị theo đợt cần xử lý">
        <h3 className="text-sm font-semibold">Đề nghị theo đợt cần xử lý</h3>
        {query.isError && requests.length === 0 ? <p role="alert" className="text-sm">Không tải được đề nghị. <button className="font-semibold text-blue-700 underline" onClick={() => void query.refetch()}>Thử lại</button></p> : null}
        {query.isPending ? <p className="text-sm text-slate-500">Đang tải…</p> : null}
        {requests.map((request) => <div key={request.id} className="flex flex-wrap items-center justify-between gap-2 border-t pt-2 text-sm"><span className="min-w-0 flex-1">{request.project_code} · {request.title} · {requestStatuses[request.status]}</span><AccountingOperationsButton projectId={request.project_id} requestId={request.id}/></div>)}
        {query.isError && requests.length > 0 ? <p role="alert" className="text-xs text-amber-700">Chưa cập nhật được đề nghị mới. <button className="font-semibold underline" onClick={() => void query.refetch()}>Thử lại</button></p> : null}
    </section>;
}
