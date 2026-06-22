import type { Partner } from '../../lib/types/domain';

export const partnerTypeOptions = [
  { value: 'client', label: 'Khách hàng / CĐT' },
  { value: 'supplier', label: 'Nhà cung cấp' },
  { value: 'subcontractor', label: 'Thầu phụ' },
  { value: 'labor_team', label: 'Đội thi công' },
  { value: 'transport', label: 'Vận chuyển' },
  { value: 'equipment_rental', label: 'Thuê thiết bị' },
  { value: 'service_provider', label: 'Dịch vụ' },
  { value: 'other', label: 'Khác' },
];

const partnerTypeLabelByValue = new Map(partnerTypeOptions.map((option) => [option.value, option.label]));

export function partnerTypeLabel(value?: string) {
  if (!value) return '-';
  return partnerTypeLabelByValue.get(value) ?? value;
}

export function partnerTypesText(partner: Pick<Partner, 'partner_types'>) {
  return (partner.partner_types ?? []).map(partnerTypeLabel).join(', ');
}

export function partnerOptionLabel(partner: Partner) {
  const code = partner.partner_code ? `${partner.partner_code} · ` : '';
  const taxCode = partner.tax_code ? ` · MST ${partner.tax_code}` : '';
  return `${code}${partner.name}${taxCode}`;
}

export function partnerHasAnyType(partner: Partner, allowedTypes: string[]) {
  if (!allowedTypes.length) return true;
  return allowedTypes.some((type) => partner.partner_types?.includes(type));
}

export function partnerOptions(partners: Partner[], allowedTypes: string[]) {
  return partners
    .filter((partner) => partner.active !== false && partnerHasAnyType(partner, allowedTypes))
    .map((partner) => ({ label: partnerOptionLabel(partner), value: partner.id }));
}

export const vendorPartnerTypes = ['supplier', 'subcontractor', 'labor_team', 'transport', 'equipment_rental', 'service_provider', 'other'];
