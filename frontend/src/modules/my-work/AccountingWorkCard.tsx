import clsx from 'clsx';
import { Link } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import type { AccountingWorkItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';

function ReferenceAmount({ item }: { item: AccountingWorkItem }) {
  return <span>{item.reference_amount == null ? '—' : new Intl.NumberFormat('vi-VN', { style: 'currency', currency: item.currency || 'VND' }).format(item.reference_amount)}</span>;
}

export function AccountingWorkCard({ item, selected, onSelect, onAcknowledge }: {
  item: AccountingWorkItem; selected: boolean; onSelect: () => void; onAcknowledge?: () => void;
}) {
  return <article className={clsx('border-b border-line last:border-b-0 sm:flex sm:items-stretch', selected ? 'bg-blue-50/70 shadow-[inset_3px_0_0_0_#2563eb]' : 'bg-white hover:bg-slate-50')}>
    <button type="button" className="min-w-0 flex-1 px-4 py-3 text-left" aria-pressed={selected} onClick={onSelect}>
      <div className="flex flex-wrap items-center gap-2"><span className="rounded bg-violet-50 px-2 py-0.5 text-xs font-semibold text-violet-700">Hồ sơ kế toán</span><StatusBadge status={item.status} /><span className="text-sm font-semibold text-ink">{item.source_code} · {item.source_title}</span></div>
      <p className="mt-1 text-xs text-slate-600">{item.project_code} · {item.project_name}</p>
      <p className="mt-1 text-xs text-slate-600">{item.counterparty_name || 'Chưa xác định đối tác'} · Tham chiếu: <ReferenceAmount item={item} /></p>
      <p className="mt-1 text-xs text-slate-500">Nhóm Kế toán · hàng chờ chung</p>
    </button>
    <button type="button" className="mx-4 my-3 flex shrink-0 items-center justify-center gap-1 rounded-md border border-blue-200 px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 sm:h-9 sm:self-center sm:py-0" onClick={onAcknowledge ?? onSelect}>
      {onAcknowledge ? 'Tiếp nhận' : 'Xem hồ sơ'}<ChevronRight className="h-4 w-4" />
    </button>
  </article>;
}

export function AccountingWorkDetails({ item, canAcknowledge, onAction }: {
  item: AccountingWorkItem; canAcknowledge: boolean; onAction: (action: 'acknowledge' | 'return') => void;
}) {
  return <div className="space-y-3 text-sm">
    <p className="text-xs font-semibold text-violet-700">Hồ sơ kế toán · {item.source_type === 'procurement_request' ? 'Yêu cầu mua' : 'Mốc hợp đồng'}</p>
    <h2 className="font-semibold text-ink">{item.source_code} · {item.source_title}</h2>
    <StatusBadge status={item.status} />
    <p className="text-slate-600">{item.status === 'handed_off' ? 'Đối chiếu nguồn, sau đó tiếp nhận hoặc trả lại để bổ sung.' : item.status === 'returned' ? 'Đang chờ bộ phận nguồn bổ sung và bàn giao lại.' : 'Đã tiếp nhận hồ sơ. Có thể tra cứu nguồn và thông tin xử lý.'}</p>
    <dl className="space-y-2 break-words text-xs text-slate-600">
      <div><dt>Công trình</dt><dd className="font-semibold">{item.project_code} · {item.project_name}</dd></div>
      <div><dt>Đối tác</dt><dd className="font-semibold">{item.counterparty_name || '—'}</dd></div>
      <div><dt>Nhóm hồ sơ</dt><dd>{item.direction === 'payable' ? 'Hồ sơ mua' : 'Hồ sơ thu'}</dd></div>
      <div><dt>Giá trị tham chiếu từ nguồn</dt><dd className="font-semibold"><ReferenceAmount item={item} /></dd></div>
      <div><dt>Phụ trách</dt><dd>Nhóm Kế toán · hàng chờ chung</dd></div>
      <div><dt>Bàn giao</dt><dd>{item.handed_off_by_name || '—'} · {dateText(item.handed_off_at)}</dd></div>
      {item.acknowledged_at ? <div><dt>Tiếp nhận</dt><dd>{item.acknowledged_by_name} · {dateText(item.acknowledged_at)}</dd></div> : null}
      {item.return_reason ? <div className="rounded border border-amber-200 bg-amber-50 p-2"><dt>Nội dung cần bổ sung</dt><dd>{item.return_reason}</dd></div> : null}
      {item.returned_at ? <div><dt>Trả bổ sung</dt><dd>{item.returned_by_name} · {dateText(item.returned_at)}</dd></div> : null}
      {item.external_reference ? <div><dt>Tham chiếu MISA</dt><dd>{item.external_reference}</dd></div> : null}
      {item.note ? <div><dt>Ghi chú</dt><dd>{item.note}</dd></div> : null}
    </dl>
    {canAcknowledge && item.status === 'handed_off' ? <div className="flex flex-wrap gap-2"><button className="rounded bg-blue-600 px-3 py-2 font-semibold text-white" onClick={() => onAction('acknowledge')}>Tiếp nhận</button><button className="rounded border border-line px-3 py-2 font-semibold" onClick={() => onAction('return')}>Trả lại</button></div> : null}
    <div className="flex flex-col gap-2 text-blue-700"><Link to={item.source_route}>Mở nguồn</Link><Link to={`/projects/${item.project_id}/accounting`}>Bàn giao kế toán của công trình</Link></div>
  </div>;
}
