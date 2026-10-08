import type { AnyRecord } from './common';

export type ProjectScopeOption = {
  id: string;
  code: string;
  name: string;
};

export type ProjectScopeOptionsPage = {
  items: ProjectScopeOption[];
  has_more: boolean;
  next_cursor?: string;
  selected: ProjectScopeOption | null;
};

export type ProjectScopeOptions = {
  projects: ProjectScopeOptionsPage;
};

export type ReportProjectOption = ProjectScopeOption;

export type ReportProjectOptionsPage = ProjectScopeOptionsPage;

export type ReportOptions = ProjectScopeOptions;

export type ProjectReportRow = AnyRecord & {
  project_id: string;
  name: string;
  is_sample?: boolean;
  status: string;
  progress_percent: number;
  open_issues: number;
  pending_approvals: number;
};

export type WarehouseReportRow = AnyRecord & {
  material_id: string;
  name: string;
  category?: string;
  stock_quantity: number;
  surplus_quantity: number;
};

export type ProcurementReportRow = AnyRecord & {
  project_id: string;
  project_code: string;
  project_name: string;
  status: string;
  budget_status: string;
  total: number;
  amount: number;
};

export type StaffReportRow = AnyRecord & {
  staff_id: string;
  full_name: string;
  department?: string;
  position?: string;
  project_count: number;
};
