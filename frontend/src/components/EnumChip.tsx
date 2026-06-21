import clsx from 'clsx';
import { enumLabel, type EnumLabelGroup } from '../lib/utils/status';

const defaultTone = 'bg-slate-100 text-slate-700 ring-slate-200';

const toneByValue: Partial<Record<EnumLabelGroup, Record<string, string>>> = {
  result: {
    pass: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    fail: 'bg-rose-50 text-rose-700 ring-rose-200',
    conditional_pass: 'bg-amber-50 text-amber-800 ring-amber-200',
  },
  budget_status: {
    within_budget: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    budget_exceeded: 'bg-rose-50 text-rose-700 ring-rose-200',
  },
  severity: {
    low: 'bg-sky-50 text-sky-700 ring-sky-200',
    medium: 'bg-amber-50 text-amber-800 ring-amber-200',
    high: 'bg-orange-50 text-orange-800 ring-orange-200',
    critical: 'bg-red-50 text-red-700 ring-red-200',
  },
  acceptance_type: {
    partial: 'bg-blue-50 text-blue-700 ring-blue-200',
    final: 'bg-violet-50 text-violet-700 ring-violet-200',
  },
  payment_type: {
    supplier_payment: 'bg-blue-50 text-blue-700 ring-blue-200',
    subcontractor_payment: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
    client_receipt: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    inbound: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    outbound: 'bg-blue-50 text-blue-700 ring-blue-200',
    advance: 'bg-amber-50 text-amber-800 ring-amber-200',
    advance_settlement: 'bg-teal-50 text-teal-700 ring-teal-200',
    other: defaultTone,
  },
  payment_method: {
    bank_transfer: 'bg-blue-50 text-blue-700 ring-blue-200',
    cash: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  },
  vendor_type: {
    supplier: 'bg-blue-50 text-blue-700 ring-blue-200',
    subcontractor: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
    labor_team: 'bg-teal-50 text-teal-700 ring-teal-200',
    transport: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
    equipment_rental: 'bg-violet-50 text-violet-700 ring-violet-200',
    service_provider: 'bg-sky-50 text-sky-700 ring-sky-200',
    other: defaultTone,
  },
  impact_scope: {
    design: 'bg-violet-50 text-violet-700 ring-violet-200',
    boq: 'bg-blue-50 text-blue-700 ring-blue-200',
    budget: 'bg-amber-50 text-amber-800 ring-amber-200',
    timeline: 'bg-orange-50 text-orange-800 ring-orange-200',
    material: 'bg-teal-50 text-teal-700 ring-teal-200',
    subcontractor: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
    client_requirement: 'bg-rose-50 text-rose-700 ring-rose-200',
  },
  movement_type: {
    receive: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    issue: 'bg-blue-50 text-blue-700 ring-blue-200',
    transfer: 'bg-violet-50 text-violet-700 ring-violet-200',
    loss: 'bg-rose-50 text-rose-700 ring-rose-200',
    return_to_warehouse: 'bg-teal-50 text-teal-700 ring-teal-200',
    return_to_subcontractor: 'bg-amber-50 text-amber-800 ring-amber-200',
    'return-to-warehouse': 'bg-teal-50 text-teal-700 ring-teal-200',
    'return-to-subcontractor': 'bg-amber-50 text-amber-800 ring-amber-200',
  },
  surplus_status: {
    pending: 'bg-amber-50 text-amber-800 ring-amber-200',
    store_pending: 'bg-blue-50 text-blue-700 ring-blue-200',
    handled: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  },
  node_type: {
    survey: 'bg-sky-50 text-sky-700 ring-sky-200',
    design: 'bg-violet-50 text-violet-700 ring-violet-200',
    boq: 'bg-blue-50 text-blue-700 ring-blue-200',
    budget_approval: 'bg-amber-50 text-amber-800 ring-amber-200',
    procurement: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
    warehouse_issue: 'bg-teal-50 text-teal-700 ring-teal-200',
    construction: 'bg-orange-50 text-orange-800 ring-orange-200',
    acceptance: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    rework: 'bg-rose-50 text-rose-700 ring-rose-200',
    replanning: 'bg-violet-50 text-violet-700 ring-violet-200',
    handover: 'bg-teal-50 text-teal-700 ring-teal-200',
    financial_closure: 'bg-slate-100 text-slate-700 ring-slate-200',
    custom: defaultTone,
  },
  fail_path: {
    none: defaultTone,
    previous: 'bg-amber-50 text-amber-800 ring-amber-200',
    rework: 'bg-rose-50 text-rose-700 ring-rose-200',
    replanning: 'bg-violet-50 text-violet-700 ring-violet-200',
  },
  condition_type: {
    always: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    order_index: 'bg-blue-50 text-blue-700 ring-blue-200',
  },
  step_type: {
    task: 'bg-slate-100 text-slate-700 ring-slate-200',
    checklist: 'bg-blue-50 text-blue-700 ring-blue-200',
    approval: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    document_upload: 'bg-violet-50 text-violet-700 ring-violet-200',
    photo_upload: 'bg-sky-50 text-sky-700 ring-sky-200',
    budget_check: 'bg-amber-50 text-amber-800 ring-amber-200',
    material_check: 'bg-teal-50 text-teal-700 ring-teal-200',
    daily_report: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
    meeting_minutes: 'bg-violet-50 text-violet-700 ring-violet-200',
    acceptance_check: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    handover_check: 'bg-teal-50 text-teal-700 ring-teal-200',
    accounting_check: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
  },
  work_type: {
    task: 'bg-slate-100 text-slate-700 ring-slate-200',
    checklist: 'bg-blue-50 text-blue-700 ring-blue-200',
    approval: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    document_upload: 'bg-violet-50 text-violet-700 ring-violet-200',
    photo_upload: 'bg-sky-50 text-sky-700 ring-sky-200',
    design_review: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
    budget_check: 'bg-amber-50 text-amber-800 ring-amber-200',
    material_check: 'bg-teal-50 text-teal-700 ring-teal-200',
    daily_report: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
    meeting_minutes: 'bg-violet-50 text-violet-700 ring-violet-200',
    acceptance_check: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    rework_task: 'bg-rose-50 text-rose-700 ring-rose-200',
    handover_check: 'bg-teal-50 text-teal-700 ring-teal-200',
    accounting_check: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
  },
  priority: {
    low: 'bg-sky-50 text-sky-700 ring-sky-200',
    normal: defaultTone,
    high: 'bg-orange-50 text-orange-800 ring-orange-200',
    urgent: 'bg-red-50 text-red-700 ring-red-200',
  },
  visibility: {
    internal: 'bg-slate-100 text-slate-700 ring-slate-200',
    project: 'bg-blue-50 text-blue-700 ring-blue-200',
    client: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
  },
  target_type: {
    project: 'bg-blue-50 text-blue-700 ring-blue-200',
    pipeline_template: 'bg-slate-100 text-slate-700 ring-slate-200',
    pipeline_run: 'bg-blue-50 text-blue-700 ring-blue-200',
    procurement_request: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
    replanning_request: 'bg-violet-50 text-violet-700 ring-violet-200',
    pipeline_run_node: 'bg-blue-50 text-blue-700 ring-blue-200',
    pipeline_run_step: 'bg-sky-50 text-sky-700 ring-sky-200',
    work_item: 'bg-blue-50 text-blue-700 ring-blue-200',
    workflow_form_action_item: 'bg-violet-50 text-violet-700 ring-violet-200',
    boq_item: 'bg-amber-50 text-amber-800 ring-amber-200',
    stock_movement: 'bg-teal-50 text-teal-700 ring-teal-200',
    issue: 'bg-orange-50 text-orange-800 ring-orange-200',
    acceptance: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    rework: 'bg-rose-50 text-rose-700 ring-rose-200',
    rework_request: 'bg-rose-50 text-rose-700 ring-rose-200',
    handover: 'bg-cyan-50 text-cyan-700 ring-cyan-200',
    accounting_budget: 'bg-amber-50 text-amber-800 ring-amber-200',
    payable: 'bg-orange-50 text-orange-800 ring-orange-200',
    receivable: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    payment: 'bg-blue-50 text-blue-700 ring-blue-200',
    evidence_record: 'bg-violet-50 text-violet-700 ring-violet-200',
  },
  event_type: {
    review_requested: 'bg-amber-50 text-amber-800 ring-amber-200',
    material_approved: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    material_ordered: 'bg-indigo-50 text-indigo-700 ring-indigo-200',
    warehouse_received: 'bg-teal-50 text-teal-700 ring-teal-200',
    warehouse_issued: 'bg-blue-50 text-blue-700 ring-blue-200',
    warehouse_transferred: 'bg-violet-50 text-violet-700 ring-violet-200',
    acceptance_passed: 'bg-emerald-50 text-emerald-700 ring-emerald-200',
    acceptance_failed: 'bg-rose-50 text-rose-700 ring-rose-200',
    rework_requested: 'bg-rose-50 text-rose-700 ring-rose-200',
    handover_completed: 'bg-teal-50 text-teal-700 ring-teal-200',
  },
};

export function EnumChip({
  group,
  value,
  className,
}: {
  group: EnumLabelGroup;
  value?: unknown;
  className?: string;
}) {
  const key = value === null || value === undefined || value === '' ? '' : String(value);
  const tone = toneByValue[group]?.[key] ?? defaultTone;
  return (
    <span
      className={clsx(
        'inline-flex max-w-full items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset',
        tone,
        className,
      )}
    >
      {enumLabel(group, value)}
    </span>
  );
}
