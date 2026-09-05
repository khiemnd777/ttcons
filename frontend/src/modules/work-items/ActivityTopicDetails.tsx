import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Loader2, RefreshCw } from 'lucide-react';
import { ApiError, get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { BOQItem } from '../../lib/types/domain';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { RecordDetailFields, type DetailField } from '../../components/RecordDetailDialog';
import { dateText, dateTimeText } from '../../lib/utils/format';
import { enumLabel } from '../../lib/utils/status';
import { activityTopicPath, activityTopicRecord, ActivityTopicUnavailable, type ActivityTopic, type ActivityTopicScope } from './activityTopicDomain';

export default function ActivityTopicDetails({ topic }: { topic: ActivityTopic }) {
  const { user } = useAuth();
  const [tab, setTab] = useState<'overview' | 'prices' | 'quotes' | 'procurement'>('overview');
  const detail = useQuery({
    queryKey: ['activity-topic-detail', user?.id, topic.projectId, topic.scope, topic.id],
    queryFn: async () => {
      const record = activityTopicRecord(topic, await get<unknown>(activityTopicPath(topic)));
      // Import source viewers only after the authorized read. Chunk failures use the same retry UI.
      const boq = topic.scope === 'boq_item' ? (await import('../boq/BOQPage')).BOQDetail : null;
      const procurement = topic.scope === 'procurement_request' ? (await import('../procurement/ProcurementPage')).ProcurementDetail : null;
      return { record, boq, procurement };
    },
    enabled: Boolean(user?.id),
    staleTime: 0,
    gcTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: false,
    retry: false,
  });
  if (!user) return <Unavailable message="Phiên đăng nhập đã kết thúc." />;
  if (detail.isFetching || detail.isPending) return <div role="status" className="flex items-center justify-center gap-2 py-10 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />Đang tải hồ sơ…</div>;
  if (detail.error) {
    const denied = detail.error instanceof ApiError && [401, 403].includes(detail.error.status);
    const missing = detail.error instanceof ActivityTopicUnavailable || (detail.error instanceof ApiError && [404, 410].includes(detail.error.status));
    return <Unavailable message={denied ? 'Anh không có quyền xem hồ sơ này.' : missing ? 'Hồ sơ không còn tồn tại hoặc không còn khả dụng trong công trình này.' : 'Không tải được hồ sơ.'} onRetry={denied || missing ? undefined : () => void detail.refetch()} />;
  }
  if (!detail.data) return <Unavailable message="Không tìm thấy hồ sơ." />;
  const { record, boq: BOQDetail, procurement: ProcurementDetail } = detail.data;
  if (BOQDetail) return <BOQDetail item={record as BOQItem} tab={tab} onTabChange={setTab} projectId={topic.projectId} canViewProcurement={false} />;
  if (ProcurementDetail) return <div className="space-y-4"><RecordDetailFields record={record} fields={[status, money('total_estimated_amount', 'Giá trị yêu cầu')]} /><ProcurementDetail record={record} /></div>;
  return <div className="space-y-4">
    <RecordDetailFields record={record} fields={topicFields[topic.scope] ?? []} />
    {topic.scope === 'acceptance' ? <Checklist rows={record.checklist} fields={acceptanceFields} /> : null}
    {topic.scope === 'handover' ? <Checklist rows={record.checklist_items} fields={handoverFields} /> : null}
  </div>;
}

function Unavailable({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div role="alert" className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
    <p>{message}</p>
    {onRetry ? <button type="button" className="mt-3 inline-flex items-center gap-1 rounded-md border border-amber-300 bg-white px-3 py-2 font-semibold hover:bg-amber-100" onClick={onRetry}><RefreshCw className="h-4 w-4" />Thử lại</button> : null}
  </div>;
}

type Field = DetailField<Record<string, any>>;
const text = (key: string, label: string): Field => ({ key, label });
const date = (key: string, label: string): Field => ({ key, label, render: row => dateText(row[key]) });
const timestamp = (key: string, label: string): Field => ({ key, label, render: row => dateTimeText(row[key]) });
const money = (key: string, label: string): Field => ({ key, label, render: row => row[key] == null ? '—' : <CurrencyText value={row[key]} /> });
const status: Field = { key: 'status', label: 'Trạng thái', render: row => row.status ? <StatusBadge status={row.status} /> : '—' };
const result: Field = { key: 'result', label: 'Kết quả', render: row => row.result ? <StatusBadge status={row.result} /> : '—' };
const topicFields: Partial<Record<ActivityTopicScope, Field[]>> = {
  work_item: [text('title', 'Đầu việc'), status, text('run_node_name', 'Giai đoạn'), text('run_step_name', 'Bước'), text('assignee_name', 'Người phụ trách'), text('description', 'Mô tả'), timestamp('due_at', 'Hạn xử lý'), timestamp('completed_at', 'Hoàn tất')],
  stock_movement: [text('movement_code', 'Mã luân chuyển'), { key: 'movement_type', label: 'Loại', render: row => enumLabel('movement_type', row.movement_type) }, text('material_name', 'Vật tư'), text('quantity', 'Số lượng'), text('unit', 'Đơn vị'), text('warehouse_name', 'Kho'), text('target_warehouse_name', 'Kho đến'), text('document_code', 'Phiếu kho'), text('reason', 'Lý do'), timestamp('created_at', 'Ngày ghi nhận')],
  acceptance: [text('acceptance_code', 'Mã biên bản'), text('title', 'Nghiệm thu'), status, result, text('location', 'Vị trí'), date('acceptance_date', 'Ngày nghiệm thu'), text('client_representative', 'Đại diện chủ đầu tư'), text('contractor_representative', 'Đại diện nhà thầu'), text('scope_summary', 'Phạm vi'), text('notes', 'Ghi chú')],
  rework: [text('title', 'Sửa lỗi'), text('description', 'Mô tả'), date('due_date', 'Hạn xử lý'), status],
  handover: [status, date('handover_date', 'Ngày dự kiến'), text('final_acceptance_title', 'Nghiệm thu cuối'), text('client_representative', 'Đại diện chủ đầu tư'), text('company_representative', 'Đại diện nhà thầu'), text('notes', 'Ghi chú'), timestamp('completed_at', 'Hoàn tất')],
  payable: [text('payable_code', 'Mã phải trả'), text('vendor_name', 'Nhà cung cấp / đối tác'), status, money('gross_amount', 'Giá trị'), money('paid_amount', 'Đã trả'), money('remaining_amount', 'Còn phải trả'), date('due_date', 'Hạn thanh toán'), text('notes', 'Ghi chú')],
  receivable: [text('receivable_code', 'Mã phải thu'), text('client_name', 'Khách hàng'), status, text('milestone_name', 'Đợt thanh toán'), money('gross_amount', 'Giá trị'), money('received_amount', 'Đã thu'), money('remaining_amount', 'Còn phải thu'), date('due_date', 'Hạn thanh toán')],
  payment: [text('payment_code', 'Mã phiếu'), { key: 'payment_type', label: 'Loại phiếu', render: row => enumLabel('payment_type', row.payment_type) }, status, money('amount', 'Số tiền'), { key: 'payment_method', label: 'Phương thức', render: row => enumLabel('payment_method', row.payment_method) }, timestamp('paid_at', 'Đã thanh toán'), timestamp('created_at', 'Ngày lập')],
};
const acceptanceFields: Field[] = [text('title', 'Tiêu chí'), text('description', 'Mô tả'), { key: 'decision', label: 'Kết luận', render: row => row.decision ? <StatusBadge status={row.decision} /> : '—' }, text('required_action', 'Yêu cầu xử lý'), date('due_date', 'Hạn xử lý'), text('note', 'Ghi chú')];
const handoverFields: Field[] = [text('title', 'Nội dung'), status, text('notes', 'Ghi chú'), timestamp('verified_at', 'Đã xác minh')];

function Checklist({ rows, fields }: { rows: unknown; fields: Field[] }) {
  const items = Array.isArray(rows) ? rows.filter(row => row && typeof row === 'object') : [];
  return <section className="space-y-3"><h3 className="text-sm font-semibold text-ink">Checklist hồ sơ</h3>
    {items.length ? items.map((row, index) => <RecordDetailFields key={row.id ?? index} record={row} fields={fields} />) : <p className="text-sm text-slate-500">Chưa có mục checklist.</p>}
  </section>;
}
