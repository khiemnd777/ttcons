export type RoleRow = Record<string, any> & {
  id: string;
  name: string;
  display_name: string;
  external_only?: boolean;
};

export type StaffImportIssue = { field: string; message: string };

export type StaffImportPreviewRow = {
  row_number: number;
  full_name: string;
  login_email: string;
  phone: string;
  department: string;
  position: string;
  role_code: string;
  role_name: string;
  has_recovery_email: boolean;
  recovery_email_masked?: string;
  password_valid: boolean;
  valid: boolean;
  errors: StaffImportIssue[];
  warnings: StaffImportIssue[];
};

export type StaffImportPreview = {
  source_filename: string;
  total_count: number;
  valid_count: number;
  invalid_count: number;
  warning_count: number;
  preview_hash: string;
  rows: StaffImportPreviewRow[];
};

export type StaffImportResult = {
  import_id: string;
  source_filename: string;
  created_count: number;
  activated_count: number;
  role_assignment_count: number;
  must_change_password_count: number;
  idempotent_replay: boolean;
};
