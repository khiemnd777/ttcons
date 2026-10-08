import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Loader2, RefreshCw } from 'lucide-react';
import { ApiError, get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { BOQItem } from '../../lib/types/domain';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { RecordDetailFields, type DetailField } from '../../components/RecordDetailDialog';
import { RecordSummary, RecordSection, RecordEmpty } from '../../components/RecordDetailLayout';
import { dateText, dateTimeText } from '../../lib/utils/format';
import { enumLabel } from '../../lib/utils/status';
import { activityTopicPath, activityTopicRecord, ActivityTopicUnavailable, type ActivityTopic, type ActivityTopicScope } from './activityTopicDomain';
import { RecordLink, RelatedRecords } from '../_shared/RecordLinks';
import { HandoverChecklistSourceDialog } from '../handover/HandoverChecklistSourceDialog';
import { checklistSourceMode } from '../handover/handoverSourcePresentation';
import type { ChecklistItem, HandoverDetail } from '../handover/handoverTypes';

export default function ActivityTopicDetails({ topic, showSource = true, onClose }: { topic: ActivityTopic; showSource?: boolean; onClose?: () => void }) {
  const { user, hasPermission } = useAuth();
  const [tab, setTab] = useState<'overview' | 'prices' | 'quotes' | 'procurement'>('overview');
  const [handoverSource, setHandoverSource] = useState<ChecklistItem | null>(null);
  const detail = useQuery({
    queryKey: ['activity-topic-detail', user?.id, topic.projectId, topic.scope, topic.id],
    queryFn: async () => {
      const record = activityTopicRecord(topic, await get<unknown>(activityTopicPath(topic)));
      // Import source viewers only after the authorized read. Chunk failures use the same retry UI.
      const boq = topic.scope === 'boq_item' ? (await import('../boq/BOQPage')).BOQDetail : null;
      const procurement = topic.scope === 'procurement_request' ? (await import('../procurement/ProcurementDetail')).ProcurementDetail : null;
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
  const content = BOQDetail ? <BOQDetail item={record as BOQItem} tab={tab} onTabChange={setTab} projectId={topic.projectId} canViewProcurement={hasPermission('procurement.view')} />
    : ProcurementDetail ? <ProcurementDetail record={record} /> : <div className="space-y-6">
    <TopicRecordSummary topic={topic} record={record} />
    {(topicSections[topic.scope] ?? []).map(section => <RecordSection key={section.title} title={section.title}>
      <RecordDetailFields record={record} fields={(topicFields[topic.scope] ?? []).filter(field => section.keys.includes(field.key))} />
    </RecordSection>)}
    {topic.scope === 'acceptance' ? <Checklist rows={record.checklist} fields={acceptanceFields} /> : null}
    {topic.scope === 'handover' ? <><Checklist rows={record.checklist_items} fields={handoverFields} onViewSource={setHandoverSource} /><HandoverChecklistSourceDialog open={Boolean(handoverSource)} item={handoverSource} detail={record as HandoverDetail} onClose={() => setHandoverSource(null)} /></> : null}
    <RelatedRecords kind={topic.scope} record={record} />
  </div>;
  return <div className="space-y-6">{content}
    {showSource ? <div className="flex flex-wrap justify-end gap-3 border-t border-slate-200 pt-4">
      {onClose ? <button type="button" className="rounded-md border border-slate-200 px-4 py-2 text-sm font-semibold" onClick={onClose}>Đóng</button> : null}
      <RecordLink primary target={{ kind: topic.scope, id: topic.id, projectId: topic.projectId }}>Mở hồ sơ gốc</RecordLink>
    </div> : null}
  </div>;
}

function Unavailable({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div role="alert" className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
    <p>{message}</p>
    {onRetry ? <button type="button" className="mt-3 inline-flex items-center gap-1 rounded-md border border-amber-300 bg-white px-3 py-2 font-semibold hover:bg-amber-100" onClick={onRetry}><RefreshCw className="h-4 w-4" />Thử lại</button> : null}
  </div>;
}

type Field = DetailField<Record<string, any>>;
const text = (key: string, label: string): Field => ({ key, label, wide: ['description', 'notes', 'reason', 'scope_summary', 'required_action', 'note'].includes(key) });
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
};
const acceptanceFields: Field[] = [text('title', 'Tiêu chí'), text('description', 'Mô tả'), { key: 'decision', label: 'Kết luận', render: row => row.decision ? <StatusBadge status={row.decision} /> : '—' }, text('required_action', 'Yêu cầu xử lý'), date('due_date', 'Hạn xử lý'), text('note', 'Ghi chú')];
const handoverFields: Field[] = [text('title', 'Nội dung'), status, text('notes', 'Ghi chú'), timestamp('verified_at', 'Đã xác minh')];

const topicSections: Partial<Record<ActivityTopicScope, { title: string; keys: string[] }[]>> = {
  work_item: [{ title: 'Phụ trách & thời hạn', keys: ['assignee_name', 'due_at', 'completed_at'] }, { title: 'Nội dung công việc', keys: ['description', 'run_node_name', 'run_step_name'] }],
  stock_movement: [{ title: 'Luồng vật tư', keys: ['warehouse_name', 'target_warehouse_name', 'document_code', 'created_at', 'reason'] }],
  acceptance: [{ title: 'Phạm vi nghiệm thu', keys: ['location', 'acceptance_date', 'scope_summary'] }, { title: 'Các bên tham gia', keys: ['client_representative', 'contractor_representative'] }, { title: 'Ghi chú', keys: ['notes'] }],
  rework: [{ title: 'Nội dung cần xử lý', keys: ['description', 'due_date'] }],
  handover: [{ title: 'Nguồn & thời hạn bàn giao', keys: ['final_acceptance_title', 'handover_date', 'completed_at'] }, { title: 'Các bên tham gia', keys: ['client_representative', 'company_representative'] }, { title: 'Ghi chú', keys: ['notes'] }],
};

export function TopicRecordSummary({ topic, record }: { topic: ActivityTopic; record: Record<string, any> }) {
  const code = record.acceptance_code || record.movement_code;
  const movement = topic.scope === 'stock_movement';
  return <RecordSummary eyebrow={code} title={movement ? record.material_name || topic.label : record.title || topic.label}
    subtitle={movement ? enumLabel('movement_type', record.movement_type) : undefined}
    status={record.status || (topic.scope === 'acceptance' && record.result) ? <div className="flex flex-wrap gap-2">{record.status ? <StatusBadge status={record.status} /> : null}{topic.scope === 'acceptance' && record.result ? <StatusBadge status={record.result} /> : null}</div> : undefined}
    value={movement ? `${record.quantity == null ? '—' : Number(record.quantity).toLocaleString('vi-VN', { maximumFractionDigits: 3 })} ${record.unit ?? ''}` : undefined}
    valueLabel={movement ? 'Số lượng luân chuyển' : undefined} />;
}

function Checklist({ rows, fields, onViewSource }: { rows: unknown; fields: Field[]; onViewSource?: (row: ChecklistItem) => void }) {
  const items = Array.isArray(rows) ? rows.filter(row => row && typeof row === 'object') : [];
  return <RecordSection title="Checklist hồ sơ" aside={`${items.length} mục`}>
    {items.length ? <div className="divide-y divide-slate-200">{items.map((row, index) => <details key={row.id ?? index} open={index === 0} className="group py-3">
      <summary className="cursor-pointer text-sm font-semibold text-slate-800"><span className="ml-1 mr-3 text-slate-400">{String(index + 1).padStart(2, '0')}</span>{row.title || 'Nội dung checklist'}<span className="ml-3 inline-flex">{row.decision || row.status ? <StatusBadge status={row.decision || row.status} /> : null}</span></summary>
      <div className="pl-6 pt-2"><RecordDetailFields record={row} fields={fields.filter(field => !['title', 'decision', 'status'].includes(field.key))} /></div>
      {onViewSource && ['automatic', 'linked'].includes(checklistSourceMode(row)) ? <button type="button" className="ml-6 mt-3 text-sm font-semibold text-blue-700 underline underline-offset-4" onClick={() => onViewSource(row)}>Xem hồ sơ nguồn</button> : null}
    </details>)}</div> : <RecordEmpty>Chưa có mục checklist.</RecordEmpty>}
  </RecordSection>;
}
