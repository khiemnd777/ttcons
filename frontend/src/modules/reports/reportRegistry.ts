export type ReportKey = 'projects' | 'procurement' | 'warehouse' | 'staff';
export type ReportGroup = 'operations' | 'resources';

export type ReportDefinition = {
  key: ReportKey;
  label: string;
  description: string;
  group: ReportGroup;
  permissions: string[];
};

export const reportGroupLabels: Record<ReportGroup, string> = {
  operations: 'Vận hành',
  resources: 'Nguồn lực',
};

export const reportDefinitions: ReportDefinition[] = [
  { key: 'projects', label: 'Công trình & tiến độ', description: 'Tiến độ, vấn đề mở và yêu cầu chờ duyệt.', group: 'operations', permissions: ['report.view'] },
  { key: 'procurement', label: 'Mua sắm', description: 'Giá trị yêu cầu theo trạng thái và ngân sách.', group: 'operations', permissions: ['report.view'] },
  { key: 'warehouse', label: 'Vật tư / Kho', description: 'Tồn vật tư và khối lượng dư trong phạm vi được xem.', group: 'operations', permissions: ['report.view'] },
  { key: 'staff', label: 'Nhân sự', description: 'Phân bổ nhân sự trong các công trình được phép xem.', group: 'resources', permissions: ['report.view'] },
];

export function visibleReportDefinitions(hasAnyPermission: (permissions?: string[]) => boolean) {
  return reportDefinitions.filter((report) => hasAnyPermission(report.permissions));
}

export function isReportKey(value?: string): value is ReportKey {
  return reportDefinitions.some((report) => report.key === value);
}

export function reportPath(key: ReportKey) {
  return `/reports/${key}`;
}
