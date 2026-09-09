import type { WorkflowFormDetail } from '../../lib/types/domain';

export type WorkflowFormArtifactAccess = {
  canAddConfirmation: boolean;
  canGeneratePDF: boolean;
  canDeletePDF: boolean;
};

export function workflowFormArtifactAccess(form?: WorkflowFormDetail): WorkflowFormArtifactAccess {
  const submitted = form?.instance?.status === 'submitted';
  return {
    canAddConfirmation: submitted && form?.capabilities?.can_add_confirmation === true,
    canGeneratePDF: submitted && form?.capabilities?.can_generate_pdf === true,
    canDeletePDF: submitted && form?.capabilities?.can_delete_pdf === true,
  };
}

export function workflowFormNextAction({
  submitted,
  validationIssueCount,
  confirmationCount,
  pdfCount,
  access,
}: {
  submitted: boolean;
  validationIssueCount: number;
  confirmationCount: number;
  pdfCount: number;
  access: WorkflowFormArtifactAccess;
}) {
  if (!submitted && validationIssueCount > 0) return 'Hoàn tất checklist/hồ sơ';
  if (!submitted) return 'Chốt hồ sơ nghiệp vụ';
  if (confirmationCount === 0 && access.canAddConfirmation) return 'Thêm xác nhận hồ sơ';
  if (pdfCount === 0 && access.canGeneratePDF) return 'Tạo PDF';
  if (confirmationCount === 0) return 'Hồ sơ đã chốt · Chỉ xem';
  if (pdfCount === 0) return 'Xem xác nhận đã lưu';
  return 'Mở tài liệu hoặc hoàn tất đầu việc';
}
