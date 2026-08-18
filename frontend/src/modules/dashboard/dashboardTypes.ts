export type DashboardPeriodType = 'month' | 'quarter' | 'year';

export type DashboardPeriod = {
  periodType: DashboardPeriodType;
  year: number;
  periodNo: number;
};

export type DashboardPeriodMeta = {
  period_type: DashboardPeriodType;
  year: number;
  period_no?: number;
  from: string;
  to: string;
  as_of: string;
  group_by: 'day' | 'month';
  is_current_period: boolean;
  can_move_forward: boolean;
};

export type DashboardStatusMetric = {
  status: string;
  total: number;
};

export type DashboardProjectSnapshot = {
  total_projects: number;
  active_projects: number;
  completed_projects: number;
  blocked_projects: number;
  open_issues: number;
  pending_approvals: number;
  average_progress: number;
  status_counts: DashboardStatusMetric[];
};

export type DashboardCashflowSummary = {
  opening_balance: number;
  total_inflow: number;
  total_outflow: number;
  net_flow: number;
  closing_balance: number;
  payment_count: number;
  pending_payment_count: number;
  pending_payment_total: number;
  unclassified_count: number;
  non_cash_count: number;
};

export type DashboardCashflowSeries = {
  period_start: string;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
  running_balance: number;
  payment_count: number;
};

export type DashboardCashflowProject = {
  project_id: string;
  project_name: string;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
  payment_count: number;
};

export type DashboardData = {
  meta: DashboardPeriodMeta;
  capabilities: {
    finance_visible: boolean;
  };
  project_snapshot: DashboardProjectSnapshot;
  cashflow?: {
    summary: DashboardCashflowSummary;
    series: DashboardCashflowSeries[];
    projects: DashboardCashflowProject[];
  };
};

export type DashboardProjectItem = {
  id: string;
  code: string;
  name: string;
  status: string;
  progress_percent: number;
  open_issues: number;
  pending_approvals: number;
  finance_visible: boolean;
  inflow_amount?: number;
  outflow_amount?: number;
  net_amount?: number;
};

export type DashboardProjectsPage = {
  items: DashboardProjectItem[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
};
