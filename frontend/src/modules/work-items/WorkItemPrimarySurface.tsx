import { ReactNode, type Ref } from 'react';
import { Link } from 'react-router-dom';
import { AlertTriangle, CheckCircle2, ClipboardCheck, ExternalLink, FileCheck2, FileText, PackageCheck, ScrollText, ShieldCheck, Upload } from 'lucide-react';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import type { EvidenceRecord, WorkItem } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { EvidencePanel } from './EvidencePanel';
import { IncomingAcceptancePanel } from './IncomingAcceptancePanel';
import { MaterialSnapshotPanel } from './MaterialSnapshotPanel';
import { OwnerClientReconciliationPanel } from './OwnerClientReconciliationPanel';
import { OutgoingAcceptancePanel } from './OutgoingAcceptancePanel';
import { SubcontractorReconciliationPanel } from './SubcontractorReconciliationPanel';
import { SurplusHandlingPanel } from './SurplusHandlingPanel';
import {
  WorkflowFormPanel,
  type WorkflowFormActionHandle,
  type WorkflowFormActionState,
} from './WorkflowFormPanel';
import type { WorkItemRequirementNavigationRequest } from './workItemRequirements';
import { departmentLabel, hasWorkflowFormSummary, workTypeLabel, workflowFormChipText } from './workItemPresentation';

export type WorkItemPrimarySurfaceKind =
  | 'workflow_form'
  | 'evidence'
  | 'approval'
  | 'incoming_acceptance'
  | 'material_snapshot'
  | 'surplus_handling'
  | 'outgoing_acceptance'
  | 'owner_client_reconciliation'
  | 'subcontractor_reconciliation'
  | 'contract'
  | 'quality'
  | 'finance'
  | 'handover'
  | 'generic';

export type WorkItemPrimarySurfaceDefinition = {
  kind: WorkItemPrimarySurfaceKind;
  title: string;
  description: string;
};

export function WorkItemPrimarySurface({
  item,
  evidence,
  surface = resolveWorkItemPrimarySurface(item),
  readOnly = false,
  navigationRequest,
  workflowFormActionRef,
  onWorkflowFormActionStateChange,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  surface?: WorkItemPrimarySurfaceDefinition;
  readOnly?: boolean;
  navigationRequest?: WorkItemRequirementNavigationRequest | null;
  workflowFormActionRef?: Ref<WorkflowFormActionHandle>;
  onWorkflowFormActionStateChange?: (state: WorkflowFormActionState | null) => void;
}) {
  return (
    <section className="space-y-3">
      {surface.kind !== 'workflow_form' ? <PrimarySurfaceHeader surface={surface} item={item} /> : null}
      {item.business_source ? <BusinessSourceCard item={item} /> : null}
      {renderPrimaryPanel(
        surface.kind,
        item,
        evidence,
        readOnly,
        navigationRequest,
        workflowFormActionRef,
        onWorkflowFormActionStateChange,
      )}
    </section>
  );
}

function BusinessSourceCard({ item }: { item: WorkItem }) {
  const source = item.business_source;
  if (!source) return null;
  const ready = Boolean(source.ready);
  const checks = source.checks ?? [];
  const readyCheckCount = checks.filter((check) => check.ready).length;
  return (
    <section className={ready
      ? 'rounded-md border border-emerald-200 bg-emerald-50 p-3'
      : 'rounded-md border border-amber-200 bg-amber-50 p-3'}>
      <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
        <div className="flex min-w-0 items-start gap-2.5">
          <span className={ready ? 'mt-0.5 rounded-md bg-white p-1.5 text-emerald-700' : 'mt-0.5 rounded-md bg-white p-1.5 text-amber-700'}>
            {ready ? <CheckCircle2 className="h-4 w-4" /> : <AlertTriangle className="h-4 w-4" />}
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
              {checks.length > 0 ? `Điều kiện nguồn · ${readyCheckCount}/${checks.length}` : 'Dữ liệu từ module nguồn'}
            </p>
            <h3 className="mt-0.5 text-sm font-semibold text-ink">{source.label || 'Nguồn nghiệp vụ của bước'}</h3>
            <p className="mt-1 text-xs leading-5 text-slate-600">{source.message || (ready ? 'Nguồn nghiệp vụ đã sẵn sàng.' : 'Hoàn tất dữ liệu tại module nguồn trước khi chốt bước.')}</p>
          </div>
        </div>
        {(item.execution_target?.route || source.route) ? (
          <Link className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md border border-current bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" to={item.execution_target?.route || source.route || '#'}>
            {item.execution_target?.action_label || 'Mở sổ nghiệp vụ'} <ExternalLink className="h-4 w-4" />
          </Link>
        ) : null}
      </div>
      {checks.length > 0 ? (
        <ul className="mt-3 grid gap-2 border-t border-current/10 pt-3 md:grid-cols-2">
          {checks.map((check) => (
            <li className="flex min-w-0 items-start gap-2 rounded-md bg-white/80 px-2.5 py-2" key={check.key}>
              {check.ready
                ? <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />
                : <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />}
              <div className="min-w-0">
                <p className="text-xs font-semibold text-slate-800">{check.label}</p>
                {!check.ready && check.message ? <p className="mt-0.5 text-xs leading-4 text-slate-500">{check.message}</p> : null}
              </div>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}

export function resolveWorkItemPrimarySurface(item: WorkItem): WorkItemPrimarySurfaceDefinition {
  const hasWorkflowForm = hasWorkflowFormSummary(item);
  if (hasWorkflowForm) {
    return {
      kind: 'workflow_form',
      title: workflowFormChipText(item),
      description: 'Việc chính là hoàn tất hồ sơ nghiệp vụ của bước trước khi gửi rà soát/duyệt.',
    };
  }

  const configuredSurface = configuredPrimarySurface(item);
  if (configuredSurface) return configuredSurface;

  const workType = normalizeSearch(item.work_type);
  const searchText = normalizeSearch([
    item.work_type,
    item.title,
    item.description,
    item.run_node_name,
    item.run_step_name,
    item.workflow_form_type,
    item.workflow_form_template_code,
    item.workflow_form_template_name,
  ].filter(Boolean).join(' '));
  const isIncomingAcceptance = includesAny(searchText, [
    'nghiệm thu đầu vào',
    'nghiem thu dau vao',
    'incoming acceptance',
    'nghiệm thu thầu phụ',
    'nghiem thu thau phu',
    'nghiệm thu vật tư',
    'nghiem thu vat tu',
  ]);
  const isOutgoingAcceptance = includesAny(searchText, [
    'nghiệm thu đầu ra',
    'nghiem thu dau ra',
    'outgoing acceptance',
    'nghiệm thu & ký duyệt công trình',
    'nghiem thu & ky duyet cong trinh',
  ]);
  const isSubcontractorQuantityDossier = includesAny(searchText, [
    'hồ sơ kiểm tra khối lượng thầu phụ',
    'ho so kiem tra khoi luong thau phu',
    'kiểm tra khối lượng thầu phụ',
    'kiem tra khoi luong thau phu',
    'đối chiếu thầu phụ',
    'doi chieu thau phu',
    'subcontractor reconciliation',
  ]);

  if (['photo_upload', 'evidence_upload'].includes(workType)) {
    return {
      kind: 'evidence',
      title: 'Nộp minh chứng',
      description: 'Việc chính là tải đúng ảnh/tài liệu và kiểm tra trạng thái minh chứng.',
    };
  }

  if (workType === 'approval') {
    return {
      kind: 'approval',
      title: 'Phê duyệt / xác nhận',
      description: 'Việc chính là xem nội dung cần duyệt, ghi nhận quyết định và giữ lại nhật ký kiểm tra.',
    };
  }

  if (isSubcontractorQuantityDossier) {
    return {
      kind: 'subcontractor_reconciliation',
      title: 'Đối chiếu khối lượng thầu phụ',
      description: 'Việc chính là đối chiếu khối lượng, đơn giá áp dụng, chứng từ và công nợ với thầu phụ.',
    };
  }

  if (workType === 'acceptance_check' && isIncomingAcceptance) {
    return {
      kind: 'incoming_acceptance',
      title: 'Nghiệm thu đầu vào',
      description: 'Việc chính là kiểm nhận vật tư/thầu phụ, số lượng đạt/từ chối và minh chứng giao nhận.',
    };
  }

  if (workType === 'acceptance_check' || isOutgoingAcceptance) {
    return {
      kind: 'outgoing_acceptance',
      title: 'Nghiệm thu đầu ra',
      description: 'Việc chính là lập biên bản nghiệm thu, ghi nhận đạt/không đạt và danh sách cần xử lý.',
    };
  }

  if (workType === 'material_check') {
    if (includesAny(searchText, ['nghiệm thu đầu vào', 'nghiem thu dau vao', 'incoming acceptance', 'nhận vật tư', 'nhan vat tu'])) {
      return {
        kind: 'incoming_acceptance',
        title: 'Nghiệm thu đầu vào',
        description: 'Việc chính là kiểm nhận vật tư, số lượng nhận/từ chối và minh chứng giao nhận.',
      };
    }
    if (includesAny(searchText, ['vật tư dư', 'vat tu du', 'surplus', 'lưu kho', 'luu kho'])) {
      return {
        kind: 'surplus_handling',
        title: 'Xử lý vật tư dư',
        description: 'Việc chính là quyết định lưu kho, bàn giao hoặc xử lý phần vật tư còn dư.',
      };
    }
    return {
      kind: 'material_snapshot',
      title: 'Ghi nhận vật tư',
      description: 'Việc chính là ghi nhận vật tư trước/sau thi công và ảnh hưởng tồn công trình.',
    };
  }

  if (workType === 'handover_check' || includesAny(searchText, ['đối chiếu chủ đầu tư', 'doi chieu chu dau tu', 'owner client'])) {
    return {
      kind: 'owner_client_reconciliation',
      title: 'Đối chiếu chủ đầu tư / bàn giao',
      description: 'Việc chính là đối chiếu hồ sơ, công nợ và trạng thái bàn giao với chủ đầu tư.',
    };
  }

  if (workType === 'document_upload') {
    return {
      kind: 'evidence',
      title: 'Nộp tài liệu',
      description: 'Việc chính là tải đúng tài liệu và kiểm tra trạng thái minh chứng.',
    };
  }

  return {
    kind: 'generic',
    title: workTypeLabel(item.work_type),
    description: 'Việc chính là xử lý trạng thái, hoạt động và minh chứng liên quan đến đầu việc.',
  };
}

function configuredPrimarySurface(item: WorkItem): WorkItemPrimarySurfaceDefinition | null {
  const definitions: Partial<Record<WorkItemPrimarySurfaceKind, WorkItemPrimarySurfaceDefinition>> = {
    workflow_form: {
      kind: 'workflow_form',
      title: workflowFormChipText(item),
      description: 'Hoàn tất đúng hồ sơ nghiệp vụ đã được cấu hình cho revision quy trình này.',
    },
    evidence: {
      kind: 'evidence',
      title: 'Nộp minh chứng',
      description: 'Tải đúng ảnh hoặc tài liệu bắt buộc của bước trước khi hoàn tất.',
    },
    approval: {
      kind: 'approval',
      title: 'Phê duyệt / xác nhận',
      description: 'Kiểm tra hồ sơ, ghi nhận quyết định và giữ đầy đủ dấu vết phê duyệt.',
    },
    incoming_acceptance: {
      kind: 'incoming_acceptance',
      title: 'Nghiệm thu đầu vào',
      description: 'Kiểm nhận lô vật tư, số lượng đạt/từ chối và chứng từ giao nhận.',
    },
    material_snapshot: {
      kind: 'material_snapshot',
      title: 'Ghi nhận vật tư',
      description: 'Ghi nhận số lượng vật tư trước/sau và ảnh hưởng tồn tại công trình.',
    },
    surplus_handling: {
      kind: 'surplus_handling',
      title: 'Xử lý vật tư dư',
      description: 'Lưu kho, bàn giao hoặc xử lý vật tư còn dư với đầy đủ truy vết.',
    },
    outgoing_acceptance: {
      kind: 'outgoing_acceptance',
      title: 'Nghiệm thu đầu ra',
      description: 'Lập biên bản nghiệm thu, kết quả và danh sách tồn tại cần xử lý.',
    },
    owner_client_reconciliation: {
      kind: 'owner_client_reconciliation',
      title: 'Đối chiếu chủ đầu tư',
      description: 'Đối chiếu checklist, hồ sơ nghiệm thu, khối lượng và trạng thái với chủ đầu tư.',
    },
    subcontractor_reconciliation: {
      kind: 'subcontractor_reconciliation',
      title: 'Đối chiếu khối lượng thầu phụ',
      description: 'Đối chiếu checklist, khối lượng xác nhận, BOQ và chứng từ thầu phụ.',
    },
    contract: {
      kind: 'contract',
      title: 'Hợp đồng & nghĩa vụ thương mại',
      description: 'Tạo/review/ký version hợp đồng, milestone, phụ lục và khoản giữ lại trong workspace công trình.',
    },
    quality: {
      kind: 'quality',
      title: 'QA/QC · ITP/RFI/NCR',
      description: 'Quản lý kế hoạch kiểm tra, inspection, không phù hợp/rework và occurrence QA/QC lặp.',
    },
    finance: {
      kind: 'finance',
      title: 'Tài chính công trình',
      description: 'Ghi nhận chi phí mua vật tư, lập và theo dõi hồ sơ đề nghị thu/chi nội bộ trước khi bàn giao Kế toán xử lý trên hệ thống chuyên môn.',
    },
    handover: {
      kind: 'handover',
      title: 'Đóng tài chính & bàn giao',
      description: 'Hoàn thiện checklist chứng từ, kiểm tra readiness và khóa hồ sơ bàn giao công trình.',
    },
    generic: {
      kind: 'generic',
      title: workTypeLabel(item.work_type),
      description: 'Xử lý trạng thái, hoạt động và minh chứng của đầu việc.',
    },
  };
  const kind = String(item.surface_kind ?? '') as WorkItemPrimarySurfaceKind;
  return definitions[kind] ?? null;
}

export function primarySurfaceUsesWorkflowForm(kind: WorkItemPrimarySurfaceKind) {
  return kind === 'workflow_form' || kind === 'approval';
}

export function primarySurfaceUsesEvidence(kind: WorkItemPrimarySurfaceKind) {
  return kind === 'evidence';
}

function renderPrimaryPanel(
  kind: WorkItemPrimarySurfaceKind,
  item: WorkItem,
  evidence: EvidenceRecord[],
  readOnly: boolean,
  navigationRequest?: WorkItemRequirementNavigationRequest | null,
  workflowFormActionRef?: Ref<WorkflowFormActionHandle>,
  onWorkflowFormActionStateChange?: (state: WorkflowFormActionState | null) => void,
) {
  switch (kind) {
    case 'workflow_form':
      return (
        <WorkflowFormPanel
          workItemId={item.id}
          projectId={item.project_id}
          evidence={evidence}
          workItemStatus={item.status}
          readOnly={readOnly}
          navigationRequest={navigationRequest}
          externalActions
          actionRef={workflowFormActionRef}
          onActionStateChange={onWorkflowFormActionStateChange}
        />
      );
    case 'evidence':
      return <EvidencePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} navigationRequest={navigationRequest} variant="primary" />;
    case 'approval':
      return <ApprovalPrimaryPanel item={item} evidence={evidence} navigationRequest={navigationRequest} />;
    case 'incoming_acceptance':
      return <IncomingAcceptancePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'material_snapshot':
      return <MaterialSnapshotPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'surplus_handling':
      return <SurplusHandlingPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'outgoing_acceptance':
      return <OutgoingAcceptancePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} workItem={item} />;
    case 'owner_client_reconciliation':
      return <OwnerClientReconciliationPanel workItemId={item.id} projectId={item.project_id} />;
    case 'subcontractor_reconciliation':
      return <SubcontractorReconciliationPanel workItemId={item.id} projectId={item.project_id} />;
    case 'contract':
      return <ContractPrimaryPanel item={item} />;
    case 'quality':
      return <QualityPrimaryPanel item={item} />;
    case 'finance':
      return <FinancePrimaryPanel item={item} />;
    case 'handover':
      return <HandoverPrimaryPanel item={item} />;
    default:
      return <GenericPrimaryPanel item={item} />;
  }
}

function ContractPrimaryPanel({ item }: { item: WorkItem }) {
  return <section className="rounded-md border border-indigo-200 bg-indigo-50 p-4">
    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div>
        <h3 className="text-sm font-semibold text-ink">Workspace hợp đồng của công trình</h3>
        <p className="mt-1 text-sm leading-6 text-slate-700">Xử lý hợp đồng trong domain workspace để giữ version, phê duyệt, nghĩa vụ phải trả/phải thu theo mốc, phụ lục và khoản giữ lại có truy vết. Hồ sơ biểu mẫu/PDF của bước vẫn nằm ở tab hỗ trợ bên dưới.</p>
      </div>
      <Link className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-indigo-600 px-3 py-2 text-sm font-semibold text-white hover:bg-indigo-700" to={`/projects/${item.project_id}/contracts`}><ScrollText className="h-4 w-4" /> Mở hợp đồng</Link>
    </div>
  </section>;
}

function QualityPrimaryPanel({ item }: { item: WorkItem }) {
  return <section className="rounded-md border border-cyan-200 bg-cyan-50 p-4">
    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div>
        <h3 className="text-sm font-semibold text-ink">Quality Workspace của công trình</h3>
        <p className="mt-1 text-sm leading-6 text-slate-700">Lập/activate ITP, tạo RFI theo hold point, xử lý NCR–rework–tái kiểm tra và theo dõi lịch QA/QC lặp. Hồ sơ form của bước vẫn được giữ trong tab hỗ trợ.</p>
      </div>
      <Link className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-cyan-700 px-3 py-2 text-sm font-semibold text-white hover:bg-cyan-800" to={`/projects/${item.project_id}/quality`}><ShieldCheck className="h-4 w-4" /> Mở QA/QC</Link>
    </div>
  </section>;
}

function FinancePrimaryPanel({ item }: { item: WorkItem }) {
  return <section className="rounded-md border border-emerald-200 bg-emerald-50 p-4">
    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div><h3 className="text-sm font-semibold text-ink">Hồ sơ tài chính quản trị công trình</h3><p className="mt-1 text-sm leading-6 text-slate-700">Ghi nhận chi phí mua vật tư và theo dõi đề nghị thu/chi đến khi bàn giao Kế toán. Không ghi sổ, kê khai thuế hoặc thay thế MISA.</p></div>
      <Link className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-emerald-700 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-800" to={`/projects/${item.project_id}/accounting?tab=tasks`}><FileText className="h-4 w-4" /> Mở tài chính công trình</Link>
    </div>
  </section>;
}

function HandoverPrimaryPanel({ item }: { item: WorkItem }) {
  return <section className="rounded-md border border-teal-200 bg-teal-50 p-4">
    <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
      <div><h3 className="text-sm font-semibold text-ink">Hồ sơ đóng công trình và bàn giao</h3><p className="mt-1 text-sm leading-6 text-slate-700">Khóa nghiệm thu cuối, xác minh checklist/minh chứng, kiểm tra chất lượng, kho, chi phí vật tư và đề nghị thu/chi nội bộ trước khi hoàn tất bàn giao.</p></div>
      <Link className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-teal-700 px-3 py-2 text-sm font-semibold text-white hover:bg-teal-800" to={`/projects/${item.project_id}/handover`}><PackageCheck className="h-4 w-4" /> Mở bàn giao</Link>
    </div>
  </section>;
}

function PrimarySurfaceHeader({ surface, item }: { surface: WorkItemPrimarySurfaceDefinition; item: WorkItem }) {
  return (
    <div className="rounded-md border border-blue-100 bg-blue-50 px-3 py-2">
      <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Nội dung chính</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">{surface.title}</h3>
          <p className="mt-1 text-xs leading-5 text-slate-600">{surface.description}</p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center gap-2 text-xs font-semibold text-slate-600">
          <SurfacePill icon={surfaceIcon(surface.kind)} label={workTypeLabel(item.work_type)} />
          {item.run_node_name ? <SurfacePill icon={<ClipboardCheck className="h-3.5 w-3.5" />} label={item.run_node_name} /> : null}
        </div>
      </div>
    </div>
  );
}

function ApprovalPrimaryPanel({
  item,
  evidence,
  navigationRequest,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  navigationRequest?: WorkItemRequirementNavigationRequest | null;
}) {
  return (
    <section className="space-y-3">
      <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-ink">Quyết định cần ghi nhận</h3>
            <p className="mt-1 text-sm leading-6 text-slate-700">
              Kiểm tra nội dung, minh chứng và hồ sơ liên quan. Các nút duyệt, từ chối hoặc yêu cầu sửa nằm ở cuối màn hình để giữ một điểm ra quyết định duy nhất.
            </p>
          </div>
          <div className="shrink-0">
            <StatusBadge status={item.status} />
          </div>
        </div>
        <div className="mt-3 grid gap-2 text-xs sm:grid-cols-3">
          <ApprovalFact label="Bộ phận" value={departmentLabel(item.department)} />
          <ApprovalFact label="Hạn" value={dateText(item.due_at)} />
          <ApprovalFact label="Phê duyệt" value={item.approval_request_id ? 'Đã liên kết' : 'Chưa liên kết'} />
        </div>
      </div>
      {hasWorkflowFormSummary(item) ? (
        <WorkflowFormPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} workItemStatus={item.status} navigationRequest={navigationRequest} />
      ) : (
        <EmptyState title="Không có hồ sơ nghiệp vụ gắn với phê duyệt này." />
      )}
    </section>
  );
}

function GenericPrimaryPanel({ item }: { item: WorkItem }) {
  return (
    <section className="rounded-md border border-line bg-white p-4">
      <div className="flex items-start gap-3">
        <span className="rounded-md bg-slate-100 p-2 text-slate-600">
          <FileText className="h-4 w-4" />
        </span>
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-ink">{item.title}</h3>
          <p className="mt-1 text-sm leading-6 text-slate-600">
            {item.description || 'Đầu việc này chưa có hồ sơ hoặc panel nghiệp vụ riêng. Dùng minh chứng, hoạt động và thao tác cuối màn hình để xử lý.'}
          </p>
          <div className="mt-3 grid gap-2 text-xs sm:grid-cols-4">
            <ApprovalFact label="Loại" value={workTypeLabel(item.work_type)} />
            <ApprovalFact label="Node" value={item.run_node_name ?? '-'} />
            <ApprovalFact label="Bước" value={item.run_step_name ?? '-'} />
            <ApprovalFact label="Hạn" value={dateText(item.due_at)} />
          </div>
        </div>
      </div>
    </section>
  );
}

function SurfacePill({ icon, label }: { icon: ReactNode; label: string }) {
  return (
    <span className="inline-flex max-w-52 items-center gap-1.5 truncate rounded-md bg-white px-2 py-1 text-blue-700 ring-1 ring-inset ring-blue-100">
      <span className="shrink-0">{icon}</span>
      <span className="truncate">{label}</span>
    </span>
  );
}

function ApprovalFact({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md bg-white px-2.5 py-2 ring-1 ring-inset ring-white">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <div className="mt-0.5 font-semibold text-slate-700">{value}</div>
    </div>
  );
}

function surfaceIcon(kind: WorkItemPrimarySurfaceKind) {
  switch (kind) {
    case 'evidence':
      return <Upload className="h-3.5 w-3.5" />;
    case 'approval':
      return <CheckCircle2 className="h-3.5 w-3.5" />;
    case 'incoming_acceptance':
    case 'outgoing_acceptance':
      return <ClipboardCheck className="h-3.5 w-3.5" />;
    case 'material_snapshot':
    case 'surplus_handling':
      return <PackageCheck className="h-3.5 w-3.5" />;
    case 'owner_client_reconciliation':
    case 'subcontractor_reconciliation':
      return <FileCheck2 className="h-3.5 w-3.5" />;
    case 'contract':
      return <ScrollText className="h-3.5 w-3.5" />;
    case 'quality':
      return <ShieldCheck className="h-3.5 w-3.5" />;
    case 'generic':
      return <AlertTriangle className="h-3.5 w-3.5" />;
    default:
      return <FileText className="h-3.5 w-3.5" />;
  }
}

function includesAny(value: string, tokens: string[]) {
  return tokens.some((token) => value.includes(normalizeSearch(token)));
}

function normalizeSearch(value?: string) {
  return String(value ?? '')
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .trim();
}
