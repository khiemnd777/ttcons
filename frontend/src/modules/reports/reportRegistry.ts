export type ReportKey = 'projects' | 'procurement' | 'warehouse' | 'staff';
export type ReportGroup = 'operations' | 'resources';

export type ReportDefinition = {
  key: ReportKey;
  label: string;
  description: string;
  basis: string;
  fileName: string;
  group: ReportGroup;
  permissions: string[];
};

export const reportGroupLabels: Record<ReportGroup, string> = {
  operations: 'Vận hành',
  resources: 'Nguồn lực',
};

export const reportDefinitions: ReportDefinition[] = [
  {
    key: 'projects',
    label: 'Công trình & tiến độ',
    description: 'Tiến độ, vấn đề mở và yêu cầu chờ duyệt.',
    basis: 'Tiến độ công trình, số vấn đề đang mở và yêu cầu đang chờ duyệt trong phạm vi được phép xem.',
    fileName: 'thong-ke-cong-trinh',
    group: 'operations',
    permissions: ['statistics.view'],
  },
  {
    key: 'procurement',
    label: 'Mua sắm',
    description: 'Giá trị yêu cầu theo trạng thái và ngân sách.',
    basis: 'Số lượng và giá trị yêu cầu mua sắm theo trạng thái xử lý và trạng thái ngân sách.',
    fileName: 'thong-ke-mua-sam',
    group: 'operations',
    permissions: ['statistics.view'],
  },
  {
    key: 'warehouse',
    label: 'Vật tư / Kho',
    description: 'Tồn vật tư và khối lượng dư trong phạm vi được xem.',
    basis: 'Tồn vật tư được tổng hợp độc lập với khối lượng dư để tránh nhân chéo số liệu.',
    fileName: 'thong-ke-vat-tu-kho',
    group: 'operations',
    permissions: ['statistics.view'],
  },
  {
    key: 'staff',
    label: 'Nhân sự',
    description: 'Phân bổ nhân sự trong các công trình được phép xem.',
    basis: 'Chỉ hiển thị nhân sự thuộc các công trình người dùng được phép xem. Danh sách toàn công ty nằm ở module Nhân sự.',
    fileName: 'thong-ke-nhan-su-cong-trinh',
    group: 'resources',
    permissions: ['statistics.view'],
  },
];

export function visibleReportDefinitions(hasAnyPermission: (permissions?: string[]) => boolean) {
  return reportDefinitions.filter((report) => hasAnyPermission(report.permissions));
}

export function isReportKey(value?: string): value is ReportKey {
  return reportDefinitions.some((report) => report.key === value);
}

export function reportPath(key: ReportKey) {
  return `/statistics/${key}`;
}
