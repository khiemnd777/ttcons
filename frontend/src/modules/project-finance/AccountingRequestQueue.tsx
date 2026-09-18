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
    if (!enabled)
        return null;
    return <section className="space-y-2 rounded border border-line p-3"><h3 className="font-semibold">Đề nghị theo đợt cần xử lý</h3>{query.isError ? <p role="alert">Không tải được đề nghị. <button onClick={() => void query.refetch()}>Thử lại</button></p> : query.isFetching ? <p>Đang tải…</p> : <>{!query.data?.length && <p className="text-sm">Chưa có đề nghị cần xử lý.</p>}{query.data?.filter(r => !projectId || r.project_id === projectId).map(r => <div key={r.id} className="flex flex-wrap items-center justify-between gap-2 border-t py-2 text-sm"><span>{r.project_code} · {r.title} · {requestStatuses[r.status]}</span><AccountingOperationsButton projectId={r.project_id} requestId={r.id}/></div>)}</>}</section>;
}
