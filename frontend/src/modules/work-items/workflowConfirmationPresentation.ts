import type { WorkflowFormSealApplication, WorkflowFormSignature } from '../../lib/types/domain';

export function confirmationRolesText(roles: WorkflowFormSignature['signer_roles_json'] | WorkflowFormSealApplication['applied_by_roles_json']) {
  if (!Array.isArray(roles) || !roles.length) return '-';
  return roles
    .map((role) => {
      if (typeof role === 'string') return role;
      if (!role || typeof role !== 'object') return '';
      const label = role.display_name ?? role.role_name ?? role.name ?? role.code ?? role.id;
      return label ? String(label) : JSON.stringify(role);
    })
    .filter(Boolean)
    .join(', ') || '-';
}

export function fileSizeText(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

export function signaturePurposeLabel(purpose?: string) {
  switch (purpose) {
    case 'prepared_by': return 'Người lập';
    case 'reviewed_by': return 'Người rà soát';
    case 'approved_by': return 'Người phê duyệt';
    case 'received_by': return 'Người nhận';
    case 'internal_ack':
    default: return 'Xác nhận nội bộ';
  }
}

export function signatureDateTime(value: string) {
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return '—';
  return new Intl.DateTimeFormat('vi-VN', {
    hour: '2-digit', minute: '2-digit', day: '2-digit', month: '2-digit', year: 'numeric',
  }).format(date);
}

export function signatureIsHistorical(signature: WorkflowFormSignature, currentRevision: number) {
  return !(signature.is_current_revision ?? signature.form_revision_no === currentRevision);
}
