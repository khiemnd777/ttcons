import { CurrencyText } from '../../components/CurrencyText';
import { RecordSection, RecordMetrics, RecordMetric, RecordTable } from '../../components/RecordDetailLayout';
import { ProcurementStatus } from './ProcurementDetail';
import { formatDate, formatQuantity } from './procurementWorkspace';

export type ProcurementUpdateSnapshot = {
  request_code?: string; status?: string; supplier_name?: string; order_reference?: string;
  expected_delivery_date?: string; total_estimated_amount?: number;
  items?: Array<{ id: string; material_name: string; quantity_requested: number; unit: string; estimated_unit_price: number }>;
};

export function ProcurementSnapshot({ snapshot }: { snapshot: ProcurementUpdateSnapshot }) {
  return <div className="space-y-4">
    <div className="flex flex-wrap items-center gap-3"><strong>{snapshot.request_code}</strong><ProcurementStatus status={snapshot.status ?? ''} /></div>
    <RecordSection title="Thông tin tại thời điểm cập nhật"><RecordMetrics>
      <RecordMetric label="Nhà cung cấp">{snapshot.supplier_name || 'Chưa gán'}</RecordMetric>
      <RecordMetric label="PO / Đơn hàng">{snapshot.order_reference || 'Chưa lập'}</RecordMetric>
      <RecordMetric label="Giao dự kiến">{formatDate(snapshot.expected_delivery_date)}</RecordMetric>
      <RecordMetric label="Giá trị yêu cầu"><CurrencyText value={snapshot.total_estimated_amount} /></RecordMetric>
    </RecordMetrics></RecordSection>
    <RecordSection title="Dòng mua đã ghi nhận"><RecordTable rows={snapshot.items ?? []} rowKey={row => row.id} empty="Chưa có dòng mua" columns={[
      { key: 'material_name', label: 'Vật tư', render: row => row.material_name },
      { key: 'quantity_requested', label: 'Số lượng', align: 'right', render: row => `${formatQuantity(row.quantity_requested)} ${row.unit}` },
      { key: 'estimated_unit_price', label: 'Đơn giá', align: 'right', render: row => <CurrencyText value={row.estimated_unit_price} /> },
    ]} /></RecordSection>
  </div>;
}
