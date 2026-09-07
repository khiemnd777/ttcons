import { RecordMetric, RecordMetrics } from '../../components/RecordDetailLayout';
import { dateTimeText } from '../../lib/utils/format';

type PlanningContext = {
  run_node_name?: string; run_step_name?: string; updated_at?: string;
  generated_work_item_count?: number; signature_count?: number; pdf_export_count?: number; evidence_count?: number;
};

export function PlanningRecordContext({ record }: { record: PlanningContext }) {
  return <details className="rounded-lg border border-slate-200 px-4 py-3">
    <summary className="cursor-pointer text-sm font-semibold text-slate-600">Nguồn quy trình & tài liệu</summary>
    <div className="pt-4"><RecordMetrics>
      <RecordMetric label="Giai đoạn">{record.run_node_name || '—'}</RecordMetric>
      <RecordMetric label="Bước">{record.run_step_name || '—'}</RecordMetric>
      <RecordMetric label="Cập nhật">{dateTimeText(record.updated_at)}</RecordMetric>
      {record.generated_work_item_count != null ? <RecordMetric label="Đầu việc đã sinh">{record.generated_work_item_count}</RecordMetric> : null}
      <RecordMetric label="Tài liệu">{record.signature_count ?? 0} chữ ký · {record.pdf_export_count ?? 0} PDF · {record.evidence_count ?? 0} minh chứng</RecordMetric>
    </RecordMetrics></div>
  </details>;
}
