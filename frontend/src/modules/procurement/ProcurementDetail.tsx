import { ShoppingCart } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { RecordSummary, RecordSection, RecordMetrics, RecordMetric, RecordTable } from '../../components/RecordDetailLayout';
import { RecordLink, RelatedRecords } from '../_shared/RecordLinks';
import { procurementStatusLabels, numeric, formatQuantity, formatDate, type ProcurementRow } from './procurementWorkspace';

export function ProcurementStatus({ status }: { status: string }) { return <span className="inline-flex rounded-full border border-slate-200 bg-slate-50 px-2 py-1 text-xs font-semibold text-slate-700">{procurementStatusLabels[status] ?? status}</span>; }

export function ProcurementDetail({ record }: { record: Record<string, any> }) {
  const items = record.items ?? [];
  return <div className="space-y-6">
    <RecordSummary eyebrow={record.request_code} title={record.title || 'Yêu cầu mua'} status={<ProcurementStatus status={record.status} />} value={record.total_estimated_amount == null ? '—' : <CurrencyText value={record.total_estimated_amount} />} valueLabel="Giá trị yêu cầu" icon={<ShoppingCart className="h-5 w-5" />} />
    <RecordSection title="Thông tin đặt hàng"><RecordMetrics>
      <RecordMetric label="Nhà cung cấp"><RecordLink target={record.supplier_partner_id ? { kind: 'partner', id: record.supplier_partner_id, partnerGroup: 'output' } : null}>{record.supplier_name || 'Chưa gán'}</RecordLink></RecordMetric>
      <RecordMetric label="PO / Đơn hàng">{record.order_reference || 'Chưa lập'}</RecordMetric>
      <RecordMetric label="Giao dự kiến">{formatDate(record.expected_delivery_date)}</RecordMetric>
    </RecordMetrics></RecordSection>
    <RecordSection title="Vật tư & tiếp nhận" aside={`${items.length} dòng hàng`} description="Khối lượng đạt nghiệm thu và khối lượng nhập kho được theo dõi riêng.">
      <RecordTable<ProcurementRow> rows={items} rowKey={(item, index) => String(item.id ?? index)} empty="Yêu cầu chưa có dòng hàng; chưa đủ điều kiện gửi duyệt." columns={[
        { key: 'material', label: 'Vật tư / căn cứ giá', render: item => <div className="min-w-0 space-y-2">
          <div className="break-words font-semibold text-slate-900">{item.material_name || item.work_item_name}</div>
          {item.boq_item_id ? <div className="text-xs"><RecordLink target={{ kind: 'boq_item', id: item.boq_item_id, projectId: record.project_id }}>Dòng BOQ nguồn</RecordLink></div> : null}
          {item.supplier_name ? <p className="text-xs text-slate-500">{item.supplier_name}</p> : null}
          <DeliveryStatus status={item.delivery_status} />
          {item.quotation_code ? <div className="text-xs leading-5 text-slate-600"><RecordLink target={item.quotation_id ? { kind: 'supplier_quotation', id: item.quotation_id, projectId: record.project_id } : null}>Báo giá {item.quotation_code}</RecordLink>{item.valid_until ? <div>Hiệu lực đến {formatDate(item.valid_until)}</div> : null}</div>
            : <details className="max-w-prose text-xs leading-5 text-amber-800"><summary className="cursor-pointer font-medium">Căn cứ miễn báo giá</summary><p className="mt-1 whitespace-pre-wrap break-words">{item.price_waiver_reason || 'Chưa có căn cứ'}</p></details>}
        </div> },
        { key: 'requested', label: 'Yêu cầu', align: 'right', render: item => <span className="whitespace-nowrap">{formatQuantity(item.quantity_requested)} {item.unit}</span> },
        { key: 'accepted', label: 'Đạt nghiệm thu', align: 'right', render: item => <span className="whitespace-nowrap">{formatQuantity(item.accepted_quantity)} {item.unit}</span> },
        { key: 'received', label: 'Đã nhập kho', align: 'right', render: item => <span className="whitespace-nowrap">{formatQuantity(item.stock_received_quantity)} {item.unit}</span> },
        { key: 'awaiting', label: 'Chờ nhập kho', align: 'right', render: item => <span className="whitespace-nowrap font-semibold text-blue-800">{formatQuantity(item.awaiting_stock_quantity)} {item.unit}</span> },
      ]} />
    </RecordSection>
    <RelatedRecords kind="procurement_request" record={{ ...record, supplier_partner_id: undefined }} />
  </div>;
}

function DeliveryStatus({ status }: { status: string }) {
  const label = ({ pending: 'Chờ tiếp nhận', partially_received: 'Nghiệm thu một phần', received: 'Đã nghiệm thu đủ' } as Record<string, string>)[status] ?? procurementStatusLabels[status] ?? status;
  return <span className="inline-flex rounded-md bg-slate-100 px-2 py-1 text-xs font-medium text-slate-600">{label || 'Chưa có trạng thái'}</span>;
}
