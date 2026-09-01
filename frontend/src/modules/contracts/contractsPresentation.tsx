import clsx from 'clsx';
import type { Contract, ContractMilestone, ContractVariation, ContractVersion } from '../../lib/types/domain';
import { currency } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';

export type ActionDialog = {
  title: string;
  description: string;
  endpoint: string;
  success: string;
  commentRequired?: boolean;
} | null;

export function SignedCurrency({ value }: { value: number }) {
  const numericValue = Number(value ?? 0);
  return <span className={numericValue > 0 ? 'text-emerald-700' : numericValue < 0 ? 'text-rose-700' : undefined}>{numericValue > 0 ? '+' : ''}{currency(numericValue)}</span>;
}

export function versionSubmitAction(row: ContractVersion): NonNullable<ActionDialog> {
  return { title: 'Gửi rà soát phiên bản', description: row.version_label, endpoint: `/contract-versions/${row.id}/submit`, success: 'Đã gửi rà soát phiên bản' };
}

export function versionApproveAction(row: ContractVersion): NonNullable<ActionDialog> {
  return { title: 'Duyệt phiên bản hợp đồng', description: row.version_label, endpoint: `/contract-versions/${row.id}/approve`, success: 'Đã duyệt phiên bản', commentRequired: true };
}

export function milestoneEligibleAction(row: ContractMilestone): NonNullable<ActionDialog> {
  return { title: 'Xác nhận mốc đủ điều kiện', description: row.milestone_name, endpoint: `/contract-milestones/${row.id}/eligible`, success: 'Đã xác nhận mốc đủ điều kiện' };
}

export function milestoneObligationAction(row: ContractMilestone): NonNullable<ActionDialog> {
  return { title: 'Mở công nợ từ mốc thanh toán', description: `${row.milestone_name} · sau giữ lại ${currency(row.net_amount)}`, endpoint: `/contract-milestones/${row.id}/open-obligation`, success: 'Đã mở công nợ từ mốc thanh toán' };
}

export function variationSubmitAction(row: ContractVariation): NonNullable<ActionDialog> {
  return { title: 'Gửi QS rà soát phát sinh', description: row.title, endpoint: `/contract-variations/${row.id}/submit`, success: 'Đã gửi phát sinh cho QS' };
}

export function variationQSAction(row: ContractVariation): NonNullable<ActionDialog> {
  return { title: 'QS rà soát phát sinh', description: row.title, endpoint: `/contract-variations/${row.id}/qs-review`, success: 'QS đã xác nhận và chuyển Mua hàng', commentRequired: true };
}

export function variationPurchaseAction(row: ContractVariation): NonNullable<ActionDialog> {
  return { title: 'Mua hàng xác nhận đã lấy giá', description: row.title, endpoint: `/contract-variations/${row.id}/purchase`, success: 'Đã chuyển phát sinh sang PM/Giám đốc duyệt', commentRequired: true };
}

export function variationApproveAction(row: ContractVariation): NonNullable<ActionDialog> {
  return { title: 'Duyệt phụ lục', description: row.title, endpoint: `/contract-variations/${row.id}/approve`, success: 'Đã duyệt phụ lục', commentRequired: true };
}

export function variationIncorporateAction(row: ContractVariation): NonNullable<ActionDialog> {
  return {
    title: 'Tạo phiên bản điều chỉnh từ phụ lục',
    description: 'Tạo phiên bản nháp mới; vẫn phải rà soát, duyệt và ký.',
    endpoint: `/contract-variations/${row.id}/incorporate`,
    success: 'Đã tạo phiên bản điều chỉnh từ phụ lục',
  };
}

export function variationProcurementAction(row: ContractVariation): NonNullable<ActionDialog> {
  return {
    title: 'Tạo yêu cầu mua từ phát sinh',
    description: 'Yêu cầu mua nháp được tạo sau khi phát sinh đã qua đủ QS, Mua hàng và phê duyệt.',
    endpoint: `/contract-variations/${row.id}/generate-procurement`,
    success: 'Đã tạo yêu cầu mua từ phát sinh',
  };
}

export function readinessCount(contract: Contract) {
  const readiness = contract.execution_readiness;
  if (!readiness) return 0;
  const executionReady = readiness.contract_active
    && readiness.app_signed
    && readiness.paper_dispatched
    && readiness.returned_scan_received;
  return [readiness.app_signed, readiness.paper_dispatched, readiness.returned_scan_received, executionReady].filter(Boolean).length;
}

export function PriorityBadge({ priority }: { priority?: string }) {
  const urgent = priority === 'urgent';
  return (
    <span className={clsx(
      'inline-flex rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset',
      urgent ? 'bg-rose-50 text-rose-700 ring-rose-200' : 'bg-slate-100 text-slate-700 ring-slate-200',
    )}>
      {urgent ? 'Gấp' : 'Không gấp'}
    </span>
  );
}

export function dispatchMethodLabel(method?: string) {
  return dispatchMethodOptions.find((option) => option.value === method)?.label ?? method ?? 'Chưa xác định';
}

export function acceptanceLabel(row: ContractMilestone) {
  if (!row.acceptance_required) return 'Không yêu cầu';
  return row.acceptance_id ? 'Có nghiệm thu liên kết' : 'Thiếu hồ sơ nghiệm thu';
}

export function obligationLabel(row: ContractMilestone) {
  if (!row.obligation_id) return null;
  return `${directionLabel(row.obligation_type)} · Đã mở công nợ`;
}

export function contractStatusLabel(status?: string) {
  if (status === 'active') return 'Đang hiệu lực';
  if (status === 'terminated') return 'Đã chấm dứt';
  return statusLabel(status);
}

export function contractTypeLabel(type?: string) {
  return contractTypeOptions.find((option) => option.value === type)?.label ?? 'Loại hợp đồng khác';
}

export function directionLabel(direction?: string) {
  if (direction === 'receivable') return 'Phải thu';
  if (direction === 'payable') return 'Phải trả';
  return 'Chưa xác định';
}

export function currencyLabel(value?: string) {
  return value === 'VND' ? 'Việt Nam đồng' : value || 'Chưa xác định tiền tệ';
}

export function milestoneTypeLabel(type?: string) {
  return milestoneTypeOptions.find((option) => option.value === type)?.label ?? 'Loại mốc khác';
}

export function signedDays(value: number) {
  const days = Number(value ?? 0);
  return `${days > 0 ? '+' : ''}${days} ngày`;
}

export const contractTypeOptions = [
  { value: 'main_client', label: 'Hợp đồng chủ đầu tư' },
  { value: 'subcontractor', label: 'Hợp đồng thầu phụ' },
  { value: 'supplier', label: 'Hợp đồng nhà cung cấp' },
  { value: 'service', label: 'Hợp đồng dịch vụ' },
  { value: 'other', label: 'Hợp đồng khác' },
];

export const directionOptions = [
  { value: 'receivable', label: 'Phải thu · Đầu ra' },
  { value: 'payable', label: 'Phải trả · Đầu vào' },
];

export const milestoneTypeOptions = [
  { value: 'advance', label: 'Tạm ứng' },
  { value: 'progress', label: 'Thanh toán tiến độ' },
  { value: 'final', label: 'Quyết toán' },
  { value: 'retention_release', label: 'Giải phóng khoản giữ lại' },
  { value: 'other', label: 'Loại khác' },
];

export const amountBasisOptions = [
  { value: 'percentage', label: 'Tỷ lệ hợp đồng' },
  { value: 'fixed', label: 'Giá trị cố định' },
  { value: 'quantity_certificate', label: 'Chứng nhận khối lượng' },
];

export const variationTypeOptions = [
  { value: 'scope', label: 'Phạm vi' },
  { value: 'price', label: 'Giá trị' },
  { value: 'time', label: 'Tiến độ' },
  { value: 'mixed', label: 'Kết hợp' },
];

export const variationPriorityOptions = [
  { value: 'normal', label: 'Không gấp' },
  { value: 'urgent', label: 'Gấp' },
];

export const dispatchMethodOptions = [
  { value: 'direct', label: 'Bàn giao trực tiếp' },
  { value: 'courier', label: 'Chuyển phát' },
  { value: 'postal', label: 'Bưu điện' },
  { value: 'other', label: 'Khác' },
];
