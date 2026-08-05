export type AppRoleBoundary = {
  external_only?: boolean;
};

export const pipelineActorRoleOptions = [
  'PM',
  'QS',
  'Thiết kế',
  'Giám sát',
  'Kế toán',
  'Pháp lý',
  'Bộ phận HĐ',
  'Thủ kho',
  'QA/QC',
  'Giám đốc',
  'Mua hàng',
  'Đội thi công',
];

export function isAssignableAppRole<T extends AppRoleBoundary>(role: T) {
  return role.external_only !== true;
}

export function appRoleScopeLabel(role: AppRoleBoundary) {
  return role.external_only ? 'Đối tác ngoài App' : 'Người dùng App';
}
