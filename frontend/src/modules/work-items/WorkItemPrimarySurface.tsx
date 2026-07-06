import { ReactNode } from 'react';
import { AlertTriangle, CheckCircle2, ClipboardCheck, FileCheck2, FileText, PackageCheck, Upload } from 'lucide-react';
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
import { WorkflowFormPanel } from './WorkflowFormPanel';
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
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  surface?: WorkItemPrimarySurfaceDefinition;
}) {
  return (
    <section className="space-y-3">
      <PrimarySurfaceHeader surface={surface} item={item} />
      {renderPrimaryPanel(surface.kind, item, evidence)}
    </section>
  );
}

export function resolveWorkItemPrimarySurface(item: WorkItem): WorkItemPrimarySurfaceDefinition {
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

  if (['photo_upload', 'document_upload', 'evidence_upload'].includes(workType)) {
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

  if (workType === 'acceptance_check' || includesAny(searchText, ['nghiệm thu đầu ra', 'nghiem thu dau ra', 'outgoing acceptance'])) {
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

  if (includesAny(searchText, ['đối chiếu thầu phụ', 'doi chieu thau phu', 'subcontractor'])) {
    return {
      kind: 'subcontractor_reconciliation',
      title: 'Đối chiếu thầu phụ',
      description: 'Việc chính là đối chiếu khối lượng, chứng từ và công nợ với thầu phụ.',
    };
  }

  if (workType === 'handover_check' || includesAny(searchText, ['đối chiếu chủ đầu tư', 'doi chieu chu dau tu', 'owner client'])) {
    return {
      kind: 'owner_client_reconciliation',
      title: 'Đối chiếu chủ đầu tư / bàn giao',
      description: 'Việc chính là đối chiếu hồ sơ, công nợ và trạng thái bàn giao với chủ đầu tư.',
    };
  }

  if (hasWorkflowFormSummary(item)) {
    return {
      kind: 'workflow_form',
      title: workflowFormChipText(item),
      description: 'Việc chính là hoàn tất biểu mẫu nghiệp vụ của bước trước khi gửi rà soát/duyệt.',
    };
  }

  return {
    kind: 'generic',
    title: workTypeLabel(item.work_type),
    description: 'Việc chính là xử lý trạng thái, hoạt động và minh chứng liên quan đến đầu việc.',
  };
}

export function primarySurfaceUsesWorkflowForm(kind: WorkItemPrimarySurfaceKind) {
  return kind === 'workflow_form' || kind === 'approval';
}

export function primarySurfaceUsesEvidence(kind: WorkItemPrimarySurfaceKind) {
  return kind === 'evidence';
}

function renderPrimaryPanel(kind: WorkItemPrimarySurfaceKind, item: WorkItem, evidence: EvidenceRecord[]) {
  switch (kind) {
    case 'workflow_form':
      return <WorkflowFormPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'evidence':
      return <EvidencePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'approval':
      return <ApprovalPrimaryPanel item={item} evidence={evidence} />;
    case 'incoming_acceptance':
      return <IncomingAcceptancePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'material_snapshot':
      return <MaterialSnapshotPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'surplus_handling':
      return <SurplusHandlingPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'outgoing_acceptance':
      return <OutgoingAcceptancePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />;
    case 'owner_client_reconciliation':
      return <OwnerClientReconciliationPanel workItemId={item.id} projectId={item.project_id} />;
    case 'subcontractor_reconciliation':
      return <SubcontractorReconciliationPanel workItemId={item.id} projectId={item.project_id} />;
    default:
      return <GenericPrimaryPanel item={item} />;
  }
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

function ApprovalPrimaryPanel({ item, evidence }: { item: WorkItem; evidence: EvidenceRecord[] }) {
  return (
    <section className="space-y-3">
      <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-ink">Quyết định cần ghi nhận</h3>
            <p className="mt-1 text-sm leading-6 text-slate-700">
              Kiểm tra nội dung, minh chứng và biểu mẫu liên quan. Các nút duyệt, từ chối hoặc yêu cầu sửa nằm ở cuối màn hình để giữ một điểm ra quyết định duy nhất.
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
        <WorkflowFormPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} />
      ) : (
        <EmptyState title="Không có biểu mẫu nghiệp vụ gắn với phê duyệt này." />
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
            {item.description || 'Đầu việc này chưa có biểu mẫu hoặc panel nghiệp vụ riêng. Dùng minh chứng, hoạt động và thao tác cuối màn hình để xử lý.'}
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
