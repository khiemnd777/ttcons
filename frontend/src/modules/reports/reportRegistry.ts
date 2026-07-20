export type ReportKey = 'profit' | 'cashflow' | 'tax' | 'accounting' | 'projects' | 'procurement' | 'warehouse' | 'staff';
export type ReportGroup = 'finance' | 'operations' | 'resources';

export type ReportDefinition = {
  key: ReportKey;
  label: string;
  description: string;
  group: ReportGroup;
  permissions: string[];
};

export const reportGroupLabels: Record<ReportGroup, string> = {
  finance: 'Tài chính',
  operations: 'Vận hành',
  resources: 'Nguồn lực',
};

export const reportDefinitions: ReportDefinition[] = [
  { key: 'profit', label: 'Lợi nhuận (P&L)', description: 'Doanh thu, chi phí và biên lợi nhuận theo hóa đơn.', group: 'finance', permissions: ['report.finance.view', 'accounting.view'] },
  { key: 'cashflow', label: 'Dòng tiền', description: 'Thu, chi và số dư từ sổ phiếu thanh toán.', group: 'finance', permissions: ['report.finance.view', 'accounting.view'] },
  { key: 'tax', label: 'Thuế / VAT', description: 'VAT đầu vào, đầu ra và cảnh báo hóa đơn.', group: 'finance', permissions: ['report.finance.view', 'accounting.view'] },
  { key: 'accounting', label: 'Công nợ & ngân sách', description: 'Phải trả, phải thu và ngân sách đã sử dụng.', group: 'finance', permissions: ['report.finance.view', 'accounting.view'] },
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
