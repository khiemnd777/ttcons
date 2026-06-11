export type AnyRecord = Record<string, any>;

export type User = {
  id: string;
  email: string;
  full_name: string;
  roles: string[];
  permissions?: string[];
};

export type Project = AnyRecord & {
  id: string;
  name: string;
  code: string;
  client_name: string;
  status: string;
  progress_percent: number;
};

export type PipelineNode = AnyRecord & {
  id: string;
  name: string;
  description?: string;
  type: string;
  order_index: number;
  position_x?: number;
  position_y?: number;
  responsible_role_ids?: string[];
  approval_rule?: string;
  required_permission?: string;
  permissions?: string[];
  exit_conditions?: AnyRecord;
  fail_path?: string;
  rework_enabled?: boolean;
  replanning_enabled?: boolean;
  status?: string;
  steps?: AnyRecord[];
};

export type PipelineTemplateEdge = AnyRecord & {
  id: string;
  template_id?: string;
  from_node_id: string;
  to_node_id: string;
  condition_type?: string;
  condition_config?: AnyRecord;
  label?: string;
  priority?: number;
};

export type PipelineEdgeConditionDefinition = {
  type: string;
  label: string;
  description: string;
  default_label: string;
  category: string;
};

export type PipelineTemplate = AnyRecord & {
  id: string;
  name: string;
  description?: string;
  project_type?: string;
  status: string;
  version: number;
  node_count?: number;
  nodes?: PipelineNode[];
  edges?: PipelineTemplateEdge[];
};
