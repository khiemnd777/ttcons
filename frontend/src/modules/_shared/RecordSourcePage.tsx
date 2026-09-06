import { useQuery } from '@tanstack/react-query';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { PageHeader } from '../../components/PageHeader';
import { RecordDetailFields } from '../../components/RecordDetailDialog';
import { RecordSection, RecordSummary, RecordTable } from '../../components/RecordDetailLayout';
import { StatusBadge } from '../../components/StatusBadge';
import { ApiError, get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { dateTimeText } from '../../lib/utils/format';
import ActivityTopicDetails from '../work-items/ActivityTopicDetails';
import { RelatedRecords } from './RecordLinks';
import { canOpenRecord, isRecordID, isRecordKind, loadSourceRecord, recordKinds, RecordUnavailable, type RecordTarget } from './recordNavigation';

/** Narrow read-only destinations. Never mounts a legacy accounting or stock mutation workspace. */
export default function RecordSourcePage() {
  const { kind, recordId } = useParams();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { user, hasPermission } = useAuth();
  const projectId = params.get('project_id') ?? '';
  const allowed = ['stock_movement', 'payment', 'incoming_acceptance', 'issue'];
  const target: RecordTarget | null = isRecordKind(kind) && allowed.includes(kind) && isRecordID(recordId) && isRecordID(projectId)
    ? { kind, id: recordId, projectId } : null;
  const close = () => navigate(isRecordID(projectId) && hasPermission('project.view') ? `/projects/${projectId}` : '/my-work', { replace: true });
  return <><PageHeader title="Hồ sơ nguồn" description="Xem thông tin và truy xuất hồ sơ liên quan." />
    <Dialog open title={target ? recordKinds[target.kind].label : 'Hồ sơ không khả dụng'} onClose={close} size="xl" manageFocus
      footer={<div className="flex justify-end"><button type="button" className="rounded-md border border-slate-200 px-4 py-2 text-sm font-semibold" onClick={close}>Đóng</button></div>}>
      {!target || !user || !canOpenRecord(target, hasPermission) ? <p role="alert">Không tìm thấy hồ sơ hoặc anh không có quyền truy cập.</p>
        : target.kind === 'payment' || target.kind === 'stock_movement' ? <ActivityTopicDetails key={`${target.kind}:${target.id}`} showSource={false} topic={{ scope: target.kind, id: target.id, projectId, label: recordKinds[target.kind].label }} />
          : <SourceContent key={`${target.kind}:${target.id}`} target={target} actorId={user.id} />}
    </Dialog>
  </>;
}

function SourceContent({ target, actorId }: { target: RecordTarget; actorId: string }) {
  const query = useQuery({ queryKey: ['record-source', actorId, target.kind, target.projectId, target.id], queryFn: () => loadSourceRecord(target, get), staleTime: 0, gcTime: 0, retry: false });
  if (query.isPending || query.isFetching) return <p role="status" className="py-8 text-sm text-slate-500">Đang tải hồ sơ…</p>;
  if (query.error || !query.data) {
    const unavailable = query.error instanceof RecordUnavailable || query.error instanceof ApiError && [401, 403, 404, 410].includes(query.error.status);
    return <div role="alert" className="space-y-3 text-sm"><p>{unavailable ? 'Không tìm thấy hồ sơ hoặc anh không có quyền truy cập.' : 'Không tải được hồ sơ.'}</p>{!unavailable ? <button type="button" className="font-semibold text-blue-700" onClick={() => void query.refetch()}>Thử lại</button> : null}</div>;
  }
  const record = query.data;
  return <div className="space-y-6">
    <RecordSummary eyebrow={record.acceptance_code} title={record.title || recordKinds[target.kind].label} status={<StatusBadge status={record.status || record.result} />} />
    <RecordDetailFields record={record} fields={target.kind === 'issue' ? [
      { key: 'description', label: 'Nội dung', wide: true }, { key: 'severity', label: 'Mức độ' }, { key: 'due_date', label: 'Hạn xử lý' },
    ] : [
      { key: 'supplier_name', label: 'Nhà cung cấp' }, { key: 'delivery_reference', label: 'Mã giao hàng' },
      { key: 'received_at', label: 'Tiếp nhận lúc', render: row => dateTimeText(row.received_at) }, { key: 'comment', label: 'Ghi chú', wide: true },
    ]} />
    {target.kind === 'incoming_acceptance' ? <RecordSection title="Vật tư nghiệm thu"><RecordTable<Record<string, any>> rows={record.items ?? []} rowKey={(row, index) => row.id ?? String(index)} empty="Chưa có dòng nghiệm thu." columns={[
      { key: 'name', label: 'Vật tư', render: row => row.material_name || '—' },
      { key: 'delivered', label: 'Đã giao', align: 'right', render: row => `${Number(row.delivered_quantity ?? 0).toLocaleString('vi-VN')} ${row.unit ?? ''}` },
      { key: 'accepted', label: 'Đạt nghiệm thu', align: 'right', render: row => `${Number(row.accepted_quantity ?? 0).toLocaleString('vi-VN')} ${row.unit ?? ''}` },
      { key: 'rejected', label: 'Không đạt', align: 'right', render: row => `${Number(row.rejected_quantity ?? 0).toLocaleString('vi-VN')} ${row.unit ?? ''}` },
    ]} /></RecordSection> : null}
    <RelatedRecords kind={target.kind} record={record} />
  </div>;
}
