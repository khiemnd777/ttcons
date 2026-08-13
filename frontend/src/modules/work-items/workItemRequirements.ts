import type { WorkItem, WorkItemCompletionRequirement } from '../../lib/types/domain';

export type WorkItemRequirementNavigationTarget =
  | { kind: 'workflow_form'; focus: 'first_issue' | 'checklist' | 'source' }
  | { kind: 'evidence'; evidenceType?: 'photo' | 'document'; openUpload: boolean }
  | { kind: 'action_panel' };

export type WorkItemRequirementNavigationRequest = {
  requestId: number;
  target: WorkItemRequirementNavigationTarget;
};

export type CompletionRequirementPresentation = {
  description: string;
  guidance: string;
  actionLabel?: string;
  target?: WorkItemRequirementNavigationTarget;
};

export type PermissionRequirementPresentation = {
  description: string;
  guidance: string;
};

function cleanPresentationText(value: unknown) {
  return typeof value === 'string' ? value.trim().replace(/[.!?]+$/u, '') : '';
}

function lowerFirst(value: string) {
  return value ? `${value.charAt(0).toLocaleLowerCase('vi-VN')}${value.slice(1)}` : value;
}

export function permissionRequirementPresentation(item: WorkItem): PermissionRequirementPresentation {
  const permissionName = cleanPresentationText(item.required_permission_name);
  const permissionDescription = cleanPresentationText(item.required_permission_description);

  if (!permissionName) {
    return {
      description: 'Yêu cầu: Quyền thao tác đầu việc.',
      guidance: 'Liên hệ Quản lý dự án hoặc Admin để kiểm tra quyền cần cấp.',
    };
  }

  return {
    description: `Yêu cầu: ${permissionName}.`,
    guidance: permissionDescription
      ? `Quyền này cho phép ${lowerFirst(permissionDescription)}. Nếu chưa được cấp, liên hệ Quản lý dự án hoặc Admin.`
      : 'Nếu chưa được cấp, liên hệ Quản lý dự án hoặc Admin.',
  };
}

export function missingCompletionRequirementLabels(item: WorkItem) {
  return (item.completion_requirements ?? [])
    .filter((requirement) => requirement.required && requirement.status === 'missing')
    .map((requirement) => requirement.label);
}

export function completionRequirementPresentation(
  item: WorkItem,
  requirement: WorkItemCompletionRequirement,
  readOnly = false,
): CompletionRequirementPresentation {
  const satisfied = requirement.status === 'satisfied';
  const hasWorkflowForm = Boolean(
    item.surface_kind === 'workflow_form'
    || item.workflow_form_instance_id
    || item.workflow_form_version_id
    || item.workflow_form_template_code
    || item.workflow_form_template_name,
  );

  if (requirement.key === 'workflow_form') {
    if (!hasWorkflowForm) {
      return {
        description: satisfied ? 'Hồ sơ nghiệp vụ đã được ghi nhận.' : 'Chưa có hồ sơ nghiệp vụ khả dụng trên đầu việc này.',
        guidance: satisfied
          ? 'Điều kiện đã được backend xác nhận.'
          : 'Liên hệ người quản lý workflow để kiểm tra form/version được gắn cho bước này.',
      };
    }
    return {
      description: satisfied ? 'Hồ sơ nghiệp vụ đúng phiên bản đã được chốt.' : 'Hồ sơ nghiệp vụ bắt buộc chưa được chốt.',
      guidance: satisfied
        ? 'Mở hồ sơ để xem phiên bản đã đáp ứng điều kiện.'
        : readOnly
          ? 'Có thể xem trước hồ sơ; thao tác cập nhật sẽ mở khi workflow chuyển đến bước này.'
          : 'Hoàn tất các trường bắt buộc, sau đó chọn Chốt hồ sơ.',
      actionLabel: readOnly ? 'Xem hồ sơ' : satisfied ? 'Xem hồ sơ' : 'Hoàn tất hồ sơ',
      target: { kind: 'workflow_form', focus: readOnly || satisfied ? 'source' : 'first_issue' },
    };
  }

  if (requirement.key === 'photo' || requirement.key === 'document') {
    const isPhoto = requirement.key === 'photo';
    const evidenceLabel = isPhoto ? 'ảnh' : 'tài liệu PDF';
    return {
      description: satisfied ? `Đã có ${evidenceLabel} minh chứng đang hiệu lực.` : `Chưa có ${evidenceLabel} đáp ứng điều kiện hoàn tất.`,
      guidance: satisfied
        ? 'Mở Sổ minh chứng để xem nguồn đã đáp ứng.'
        : readOnly
          ? `Có thể xem trước Sổ minh chứng; tải ${evidenceLabel} sẽ mở khi workflow chuyển đến bước này.`
          : `Tải ít nhất một ${evidenceLabel} đang hiệu lực cho đầu việc này.`,
      actionLabel: readOnly || satisfied ? 'Xem minh chứng' : isPhoto ? 'Tải ảnh' : 'Tải tài liệu',
      target: {
        kind: 'evidence',
        evidenceType: isPhoto ? 'photo' : 'document',
        openUpload: !readOnly && !satisfied,
      },
    };
  }

  if (requirement.key === 'checklist') {
    if (satisfied && requirement.satisfied_by === 'evidence') {
      return {
        description: 'Checklist đã được đáp ứng qua minh chứng đang hiệu lực.',
        guidance: 'Mở Sổ minh chứng để xem nguồn đã đáp ứng.',
        actionLabel: 'Xem minh chứng',
        target: { kind: 'evidence', openUpload: false },
      };
    }
    if (satisfied && requirement.satisfied_by === 'completion_payload') {
      return {
        description: 'Checklist đã được ghi nhận trong dữ liệu hoàn tất của đầu việc.',
        guidance: 'Không có field riêng cần thao tác trên màn hình hiện tại.',
      };
    }
    if (hasWorkflowForm) {
      return {
        description: satisfied
          ? 'Checklist đã được đáp ứng qua hồ sơ nghiệp vụ đã chốt.'
          : 'Checklist của bước chưa đáp ứng điều kiện hoàn tất.',
        guidance: satisfied
          ? 'Mở hồ sơ để xem nguồn đã đáp ứng; nếu form không có field checklist riêng, hồ sơ đã chốt chính là nguồn xác nhận.'
          : readOnly
            ? 'Có thể xem trước checklist; thao tác cập nhật sẽ mở khi workflow chuyển đến bước này.'
            : 'Hoàn tất field checklist trong hồ sơ. Nếu form không có field checklist riêng, hãy hoàn tất và chốt đúng hồ sơ của bước.',
        actionLabel: readOnly || satisfied ? 'Xem nguồn đáp ứng' : 'Mở checklist',
        target: { kind: 'workflow_form', focus: readOnly || satisfied ? 'source' : 'checklist' },
      };
    }
    return {
      description: satisfied ? 'Checklist đã được backend xác nhận.' : 'Checklist của bước chưa đáp ứng điều kiện hoàn tất.',
      guidance: satisfied
        ? 'Không có field hoặc section riêng cần thao tác trên màn hình hiện tại.'
        : 'Màn hình này không có field checklist trực tiếp. Thực hiện checklist theo hướng dẫn của bước rồi thử Hoàn tất; nếu vẫn bị chặn, liên hệ người quản lý workflow.',
    };
  }

  return {
    description: satisfied ? 'Điều kiện đã được backend xác nhận.' : 'Điều kiện chưa đáp ứng.',
    guidance: satisfied
      ? 'Không có field hoặc section riêng cần thao tác trên màn hình hiện tại.'
      : 'Chưa có đích thao tác được cấu hình cho điều kiện này. Thực hiện theo mô tả của bước hoặc liên hệ người quản lý workflow.',
  };
}
