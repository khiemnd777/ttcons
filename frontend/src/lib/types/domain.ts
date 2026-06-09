export type AnyRecord = Record<string, any>;

export type User = {
  id: string;
  email: string;
  full_name: string;
  roles: string[];
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
  responsible_role_ids?: string[];
  approval_rule?: string;
  status?: string;
  steps?: AnyRecord[];
};

export type PipelineTemplate = AnyRecord & {
  id: string;
  name: string;
  status: string;
  version: number;
  nodes?: PipelineNode[];
};
