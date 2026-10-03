export type AppRoleBoundary = {
  external_only?: boolean;
};

export const pipelineActorRoleOptions = [
  'PM',
  'Phó Quản Lý',
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

// Presentation only: persisted workflow values remain compatible with PM.
export function appRoleDisplayName(value: string) {
  const label = value.trim();
  return ['pm', 'project manager', 'quản lý dự án'].includes(label.toLocaleLowerCase('vi-VN'))
    ? 'Quản lý dự án'
    : label;
}

export const pipelineActorRoleChoices = pipelineActorRoleOptions.map((value) => ({
  value,
  label: appRoleDisplayName(value),
}));

export function appRoleChoices(values: readonly string[], selected: readonly string[] = []) {
  const choices = new Map<string, { value: string; label: string }>();
  for (const value of values) {
    const label = appRoleDisplayName(value);
    if (choices.has(label)) continue;
    // Keep the saved member value when editing an existing assignment.
    const saved = selected.find((item) => appRoleDisplayName(item) === label);
    choices.set(label, { value: saved ?? value, label });
  }
  return [...choices.values()];
}

export function isAssignableAppRole<T extends AppRoleBoundary>(role: T) {
  return role.external_only !== true;
}

export function appRoleScopeLabel(role: AppRoleBoundary) {
  return role.external_only ? 'Đối tác ngoài App' : 'Người dùng App';
}

export function uniqueRoleNames(roles?: readonly string[]) {
  return Array.from(new Set((roles ?? []).map(appRoleDisplayName).filter(Boolean)));
}

export function roleSummary(roles?: readonly string[]) {
  return uniqueRoleNames(roles).join(', ');
}
