CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  full_name TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS staff (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES users(id),
  full_name TEXT NOT NULL,
  email TEXT NOT NULL,
  phone TEXT,
  department TEXT,
  position TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL,
  description TEXT,
  system_role BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS permissions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT,
  module TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS role_permissions (
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
  PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_roles (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  project_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (user_id, role_id, project_id)
);

CREATE TABLE IF NOT EXISTS pipeline_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  description TEXT,
  project_type TEXT,
  version INTEGER NOT NULL DEFAULT 1,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  code TEXT NOT NULL UNIQUE,
  client_name TEXT NOT NULL,
  address TEXT,
  planned_start_date DATE,
  planned_end_date DATE,
  estimated_budget NUMERIC(18,2) NOT NULL DEFAULT 0,
  contract_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',
  pipeline_template_id UUID REFERENCES pipeline_templates(id),
  current_pipeline_run_id UUID,
  pm_staff_id UUID REFERENCES staff(id),
  progress_percent INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS project_members (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  staff_id UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
  role_ids JSONB NOT NULL DEFAULT '[]',
  assigned_nodes JSONB NOT NULL DEFAULT '[]',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pipeline_template_nodes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES pipeline_templates(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  type TEXT NOT NULL DEFAULT 'custom',
  position_x INTEGER NOT NULL DEFAULT 0,
  position_y INTEGER NOT NULL DEFAULT 0,
  order_index INTEGER NOT NULL DEFAULT 0,
  responsible_role_ids JSONB NOT NULL DEFAULT '[]',
  required_permission TEXT,
  entry_conditions JSONB NOT NULL DEFAULT '{}',
  exit_conditions JSONB NOT NULL DEFAULT '{}',
  approval_rule TEXT,
  rework_enabled BOOLEAN NOT NULL DEFAULT false,
  replanning_enabled BOOLEAN NOT NULL DEFAULT false,
  fail_path TEXT NOT NULL DEFAULT 'previous',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pipeline_template_edges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES pipeline_templates(id) ON DELETE CASCADE,
  from_node_id UUID NOT NULL REFERENCES pipeline_template_nodes(id) ON DELETE CASCADE,
  to_node_id UUID NOT NULL REFERENCES pipeline_template_nodes(id) ON DELETE CASCADE,
  condition_type TEXT NOT NULL DEFAULT 'always',
  condition_config JSONB NOT NULL DEFAULT '{}',
  label TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pipeline_template_steps (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  node_id UUID NOT NULL REFERENCES pipeline_template_nodes(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  step_type TEXT NOT NULL DEFAULT 'task',
  required_role_ids JSONB NOT NULL DEFAULT '[]',
  required_permission TEXT,
  checklist_required BOOLEAN NOT NULL DEFAULT true,
  document_required BOOLEAN NOT NULL DEFAULT false,
  photo_required BOOLEAN NOT NULL DEFAULT false,
  order_index INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pipeline_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  template_id UUID NOT NULL REFERENCES pipeline_templates(id),
  current_node_id UUID,
  status TEXT NOT NULL DEFAULT 'in_progress',
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS pipeline_run_nodes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  template_node_id UUID REFERENCES pipeline_template_nodes(id),
  name TEXT NOT NULL,
  description TEXT,
  type TEXT NOT NULL DEFAULT 'custom',
  order_index INTEGER NOT NULL DEFAULT 0,
  responsible_role_ids JSONB NOT NULL DEFAULT '[]',
  required_permission TEXT,
  exit_conditions JSONB NOT NULL DEFAULT '{}',
  approval_rule TEXT,
  fail_path TEXT NOT NULL DEFAULT 'previous',
  status TEXT NOT NULL DEFAULT 'pending',
  block_reason TEXT,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS pipeline_run_steps (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_node_id UUID NOT NULL REFERENCES pipeline_run_nodes(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  template_step_id UUID REFERENCES pipeline_template_steps(id),
  name TEXT NOT NULL,
  description TEXT,
  step_type TEXT NOT NULL DEFAULT 'task',
  required_role_ids JSONB NOT NULL DEFAULT '[]',
  required_permission TEXT,
  checklist_required BOOLEAN NOT NULL DEFAULT true,
  document_required BOOLEAN NOT NULL DEFAULT false,
  photo_required BOOLEAN NOT NULL DEFAULT false,
  order_index INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending',
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  reviewed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS approval_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  target_type TEXT NOT NULL,
  target_id UUID,
  name TEXT NOT NULL,
  rule_type TEXT NOT NULL,
  required_role_ids JSONB NOT NULL DEFAULT '[]',
  amount_min NUMERIC(18,2),
  amount_max NUMERIC(18,2),
  budget_exceeded_required BOOLEAN NOT NULL DEFAULT false,
  client_required BOOLEAN NOT NULL DEFAULT false,
  config_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS approval_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  target_type TEXT NOT NULL,
  target_id UUID,
  project_id UUID REFERENCES projects(id),
  requested_by UUID REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'pending',
  reason TEXT,
  required_roles JSONB NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS approval_actions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  approval_request_id UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id),
  role_id UUID REFERENCES roles(id),
  action TEXT NOT NULL,
  comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS boq_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  category TEXT NOT NULL,
  work_item_name TEXT NOT NULL,
  description TEXT,
  unit TEXT NOT NULL,
  quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  unit_price NUMERIC(18,2) NOT NULL DEFAULT 0,
  total_amount NUMERIC(18,2) GENERATED ALWAYS AS (quantity * unit_price) STORED,
  material_id UUID,
  node_id UUID,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS procurement_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  request_code TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  requested_by UUID REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'draft',
  total_estimated_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  budget_status TEXT NOT NULL DEFAULT 'within_budget',
  approval_request_id UUID REFERENCES approval_requests(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS procurement_request_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  request_id UUID NOT NULL REFERENCES procurement_requests(id) ON DELETE CASCADE,
  material_id UUID,
  boq_item_id UUID REFERENCES boq_items(id),
  node_id UUID,
  quantity_planned NUMERIC(18,3) NOT NULL DEFAULT 0,
  quantity_requested NUMERIC(18,3) NOT NULL DEFAULT 0,
  unit TEXT NOT NULL,
  estimated_unit_price NUMERIC(18,2) NOT NULL DEFAULT 0,
  estimated_total NUMERIC(18,2) GENERATED ALWAYS AS (quantity_requested * estimated_unit_price) STORED,
  supplier_name TEXT,
  delivery_status TEXT NOT NULL DEFAULT 'pending',
  approval_status TEXT NOT NULL DEFAULT 'pending',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS warehouses (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  location TEXT,
  type TEXT NOT NULL DEFAULT 'central',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS materials (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  unit TEXT NOT NULL,
  category TEXT,
  default_supplier TEXT,
  standard_price NUMERIC(18,2) NOT NULL DEFAULT 0,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS stock_movements (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  movement_code TEXT NOT NULL UNIQUE,
  project_id UUID REFERENCES projects(id),
  warehouse_id UUID NOT NULL REFERENCES warehouses(id),
  material_id UUID NOT NULL REFERENCES materials(id),
  movement_type TEXT NOT NULL,
  quantity NUMERIC(18,3) NOT NULL,
  unit TEXT NOT NULL,
  reason TEXT,
  reference_type TEXT,
  reference_id UUID,
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS site_material_balances (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  material_id UUID NOT NULL REFERENCES materials(id),
  planned_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  issued_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  used_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  surplus_quantity NUMERIC(18,3) NOT NULL DEFAULT 0,
  surplus_status TEXT NOT NULL DEFAULT 'pending',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (project_id, material_id)
);

CREATE TABLE IF NOT EXISTS operation_daily_reports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  report_date DATE NOT NULL,
  reported_by UUID REFERENCES users(id),
  weather TEXT,
  work_summary TEXT,
  manpower_summary TEXT,
  material_usage_summary TEXT,
  issues_summary TEXT,
  progress_percent INTEGER NOT NULL DEFAULT 0,
  photos JSONB NOT NULL DEFAULT '[]',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS issues (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  node_id UUID,
  title TEXT NOT NULL,
  description TEXT,
  severity TEXT NOT NULL DEFAULT 'medium',
  status TEXT NOT NULL DEFAULT 'open',
  reported_by UUID REFERENCES users(id),
  assigned_to UUID REFERENCES staff(id),
  requires_rework BOOLEAN NOT NULL DEFAULT false,
  requires_replanning BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS acceptance_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  node_id UUID,
  type TEXT NOT NULL DEFAULT 'partial',
  title TEXT NOT NULL,
  requested_by UUID REFERENCES users(id),
  inspected_by UUID REFERENCES staff(id),
  approved_by UUID REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'draft',
  result TEXT,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS acceptance_checklist_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  acceptance_id UUID NOT NULL REFERENCES acceptance_records(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  description TEXT,
  is_passed BOOLEAN NOT NULL DEFAULT false,
  note TEXT,
  photo_required BOOLEAN NOT NULL DEFAULT false,
  photo_url TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS rework_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  issue_id UUID REFERENCES issues(id),
  acceptance_id UUID REFERENCES acceptance_records(id),
  node_id UUID,
  title TEXT NOT NULL,
  description TEXT,
  assigned_to UUID REFERENCES staff(id),
  status TEXT NOT NULL DEFAULT 'draft',
  due_date DATE,
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS replanning_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  issue_id UUID REFERENCES issues(id),
  node_id UUID,
  reason TEXT NOT NULL,
  impact_scope TEXT NOT NULL,
  budget_impact NUMERIC(18,2) NOT NULL DEFAULT 0,
  timeline_impact INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS handover_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'draft',
  final_acceptance_id UUID REFERENCES acceptance_records(id),
  handover_date DATE,
  client_representative TEXT,
  company_representative TEXT,
  documents_ready BOOLEAN NOT NULL DEFAULT false,
  warehouse_closed BOOLEAN NOT NULL DEFAULT false,
  payables_recorded BOOLEAN NOT NULL DEFAULT false,
  receivables_recorded BOOLEAN NOT NULL DEFAULT false,
  notes TEXT,
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS accounting_budgets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  budget_code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  planned_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  approved_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  used_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  remaining_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'draft',
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payables (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  payable_code TEXT NOT NULL UNIQUE,
  vendor_type TEXT NOT NULL,
  vendor_name TEXT NOT NULL,
  source_type TEXT,
  source_id UUID,
  gross_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  advance_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  retention_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  deduction_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  paid_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  remaining_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  due_date DATE,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS receivables (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  receivable_code TEXT NOT NULL UNIQUE,
  client_name TEXT NOT NULL,
  source_type TEXT,
  source_id UUID,
  contract_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  milestone_name TEXT,
  gross_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  received_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  remaining_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  due_date DATE,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  payment_code TEXT NOT NULL UNIQUE,
  payment_type TEXT NOT NULL,
  target_type TEXT,
  target_id UUID,
  amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  payment_method TEXT NOT NULL DEFAULT 'bank_transfer',
  status TEXT NOT NULL DEFAULT 'draft',
  paid_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS advance_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  request_code TEXT NOT NULL UNIQUE,
  requested_by UUID REFERENCES users(id),
  amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  reason TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  approved_by UUID REFERENCES users(id),
  settled_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  target_type TEXT,
  target_id UUID,
  name TEXT NOT NULL,
  file_url TEXT,
  document_type TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  uploaded_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  actor_id UUID REFERENCES users(id),
  actor_name TEXT,
  event_type TEXT NOT NULL,
  target_type TEXT,
  target_id UUID,
  description TEXT NOT NULL,
  metadata_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS system_settings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  key TEXT NOT NULL UNIQUE,
  value_json JSONB NOT NULL DEFAULT '{}',
  description TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_pipeline_nodes_template_order ON pipeline_template_nodes(template_id, order_index);
CREATE INDEX IF NOT EXISTS idx_pipeline_run_nodes_run_order ON pipeline_run_nodes(run_id, order_index);
CREATE INDEX IF NOT EXISTS idx_audit_logs_project_created ON audit_logs(project_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_approval_requests_status ON approval_requests(status);
