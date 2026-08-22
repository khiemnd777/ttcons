CREATE INDEX IF NOT EXISTS idx_projects_pm_staff_reference
  ON projects(pm_staff_id)
  WHERE pm_staff_id IS NOT NULL AND status<>'cancelled';

CREATE INDEX IF NOT EXISTS idx_project_members_staff_active
  ON project_members(staff_id,project_id)
  WHERE active;

CREATE INDEX IF NOT EXISTS idx_issues_assigned_to_active
  ON issues(assigned_to)
  WHERE assigned_to IS NOT NULL AND status NOT IN ('resolved','closed','cancelled','deleted');

CREATE INDEX IF NOT EXISTS idx_rework_requests_assigned_to_active
  ON rework_requests(assigned_to)
  WHERE assigned_to IS NOT NULL AND status NOT IN ('done','completed','closed','cancelled','deleted');

CREATE INDEX IF NOT EXISTS idx_acceptance_records_inspected_by_active
  ON acceptance_records(inspected_by)
  WHERE inspected_by IS NOT NULL AND status NOT IN ('approved','completed','closed','cancelled','deleted');

CREATE INDEX IF NOT EXISTS idx_construction_action_assignee_active
  ON construction_action_items(assigned_user_id)
  WHERE assigned_user_id IS NOT NULL AND status IN ('open','waiting_confirmation');

CREATE INDEX IF NOT EXISTS idx_recurring_plan_assignee_active
  ON construction_recurring_plans(assigned_user_id)
  WHERE assigned_user_id IS NOT NULL AND status='active';

CREATE INDEX IF NOT EXISTS idx_recurring_occurrence_assignee_active
  ON construction_recurring_occurrences(assigned_user_id)
  WHERE assigned_user_id IS NOT NULL AND status IN ('pending','in_progress','overdue');

CREATE INDEX IF NOT EXISTS idx_construction_handoff_item_assignee_active
  ON construction_handoff_items(assigned_user_id)
  WHERE assigned_user_id IS NOT NULL AND status='activated';
