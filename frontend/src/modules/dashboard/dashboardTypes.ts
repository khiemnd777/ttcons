export type DashboardPeriodType = 'month' | 'quarter' | 'year';

export type DashboardActivity = {
  id: string;
  project_id: string;
  project_code: string;
  project_name: string;
  work_item_id: string;
  target_label: string;
  actor_name: string;
  actor_account_deleted: boolean;
  activity_type: string;
  comment: string;
  from_status: string;
  to_status: string;
  created_at: string;
  form_version_id?: string;
};

export type DashboardActivitiesPage = {
  items: DashboardActivity[];
  has_more: boolean;
  next_cursor: string;
  can_view_activity: boolean;
  as_of: string;
};

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

export type DashboardData = {
  meta: DashboardPeriodMeta;
  project_snapshot: DashboardProjectSnapshot;
};

export type DashboardProjectItem = {
  id: string;
  code: string;
  name: string;
  status: string;
  progress_percent: number;
  open_issues: number;
  pending_approvals: number;
};

export type DashboardProjectsPage = {
  items: DashboardProjectItem[];
  total: number;
  has_more: boolean;
  next_cursor?: string;
};
