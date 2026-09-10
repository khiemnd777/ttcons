import type { WorkItem, WorkItemCompletionRequirement } from '../../lib/types/domain';

export type WorkItemRequirementNavigationTarget =
  | { kind: 'workflow_form'; focus: 'first_issue' | 'checklist' | 'source' }
  | { kind: 'evidence'; evidenceType?: 'photo' | 'document'; openUpload: boolean }
  | { kind: 'business_source'; route: string }
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

export type RequiredAutomationPresentation = CompletionRequirementPresentation & {
  state: 'done' | 'blocked' | 'neutral';
};

function cleanPresentationText(value: unknown) {
  return typeof value === 'string' ? value.trim().replace(/[.!?]+$/u, '') : '';
}

export function workItemActionErrorReason(error: unknown) {
  if (!error || typeof error !== 'object') return undefined;
  const details = (error as { details?: unknown }).details;
  if (!details || typeof details !== 'object' || Array.isArray(details)) return undefined;
  const reason = (details as Record<string, unknown>).reason;
  return typeof reason === 'string' && reason.trim() ? reason.trim() : undefined;
}

export function businessSourceNavigationPath(route: string, currentSearch: string) {
  if (!route) return route;
  const returnTo = new URLSearchParams(currentSearch).get('returnTo');
  if (!returnTo) return route;

  const hashIndex = route.indexOf('#');
  const hash = hashIndex >= 0 ? route.slice(hashIndex) : '';
  const withoutHash = hashIndex >= 0 ? route.slice(0, hashIndex) : route;
  const queryIndex = withoutHash.indexOf('?');
  const path = queryIndex >= 0 ? withoutHash.slice(0, queryIndex) : withoutHash;
  const params = new URLSearchParams(queryIndex >= 0 ? withoutHash.slice(queryIndex + 1) : '');
  if (!params.has('returnTo')) params.set('returnTo', returnTo);
  const query = params.toString();
  return `${path}${query ? `?${query}` : ''}${hash}`;
}

export function requiredAutomationPresentation(item: WorkItem, actions: readonly string[]): RequiredAutomationPresentation {
  const latestAutomation = (item.automation_events ?? []).find((event) => event.action_key === item.domain_action_key);
  const automationStatus = item.business_source?.status || latestAutomation?.status;
  const reason = item.business_source?.message || latestAutomation?.blocker_reason || latestAutomation?.error_message;
  const sourceBlocked = item.domain_action_key === 'procurement.require_approved'
    && automationStatus === 'blocked'
    && Boolean(item.execution_target?.route);
  const sourceTarget: WorkItemRequirementNavigationTarget | undefined = sourceBlocked
    ? { kind: 'business_source', route: item.execution_target!.route }
    : undefined;
  const retryAtActionPanel = !sourceBlocked
    && automationStatus !== 'succeeded'
    && automationStatus !== 'skipped'
    && actions.includes('complete');

  return {
    description: automationStatus === 'succeeded'
      ? reason || `${item.domain_action_key} đã hoàn tất thành công.`
      : automationStatus === 'blocked' || automationStatus === 'failed'
        ? reason || `${item.domain_action_key} đang bị chặn.`
        : `${item.domain_action_key} sẽ được kiểm tra trước khi đầu việc hoàn tất.`,
    guidance: automationStatus === 'succeeded' || automationStatus === 'skipped'
      ? 'Không còn thao tác bổ sung cho điều kiện này.'
      : sourceBlocked
        ? item.business_source?.target_type === 'procurement_request'
          ? 'Mở yêu cầu mua đang chặn và thực hiện bước duyệt; trạng thái đầu việc sẽ được tải lại sau khi hồ sơ nguồn thay đổi.'
          : 'Mở Mua sắm để bổ sung yêu cầu mua bắt buộc; trạng thái đầu việc sẽ được tải lại sau khi hồ sơ nguồn thay đổi.'
        : retryAtActionPanel
          ? 'Khắc phục lý do chặn nếu có, sau đó chọn Hoàn tất để hệ thống chạy lại kiểm tra.'
          : automationStatus === 'failed'
            ? 'Liên hệ người quản lý workflow và cung cấp thông báo lỗi để kiểm tra tự động hóa.'
            : 'Tự động hóa sẽ được hệ thống kiểm tra khi đầu việc có thể Hoàn tất; liên hệ người quản lý workflow nếu trạng thái không thay đổi.',
    actionLabel: sourceTarget
      ? item.business_source?.target_type === 'procurement_request' ? 'Mở yêu cầu cần duyệt' : item.execution_target?.action_label || 'Mở hồ sơ nguồn'
      : retryAtActionPanel ? 'Đến thao tác Hoàn tất' : undefined,
    target: sourceTarget ?? (retryAtActionPanel ? { kind: 'action_panel' } : undefined),
    state: automationStatus === 'succeeded' || automationStatus === 'skipped'
      ? 'done'
      : automationStatus === 'blocked' || automationStatus === 'failed'
        ? 'blocked'
        : 'neutral',
  };
}

export function permissionRequirementPresentation(item: WorkItem): PermissionRequirementPresentation {
  const permissionName = cleanPresentationText(item.required_permission_name);
  const permissionDescription = cleanPresentationText(item.required_permission_description);

  if (!permissionName) {
    return {
      description: 'Cần quyền thao tác đầu việc.',
      guidance: 'Liên hệ Quản lý dự án hoặc Admin để kiểm tra quyền cần cấp.',
    };
  }

  return {
    description: `Cần quyền: ${permissionName}.`,
    guidance: permissionDescription
      ? `Liên hệ Quản lý dự án hoặc Admin để kiểm tra quyền. Phạm vi quyền: ${permissionDescription}.`
      : 'Liên hệ Quản lý dự án hoặc Admin để kiểm tra quyền.',
  };
}

export function missingCompletionRequirementLabels(item: WorkItem) {
  return (item.completion_requirements ?? [])
    .filter((requirement) => requirement.required && requirement.status === 'missing')
    .map((requirement) => requirement.label);
}

export function completionRequirementNavigationFromError(
  item: WorkItem,
  error: unknown,
): WorkItemRequirementNavigationTarget | undefined {
  if (!error || typeof error !== 'object') return undefined;
  const details = (error as { details?: unknown }).details;
  if (!details || typeof details !== 'object' || Array.isArray(details)) return undefined;
  const rawMissing = (details as Record<string, unknown>).missing_evidence;
  if (!Array.isArray(rawMissing)) return undefined;

  const missingKeys = rawMissing
    .filter((value): value is string => typeof value === 'string')
    .map((value) => value.trim())
    .filter(Boolean);

  for (const key of missingKeys) {
    const current = item.completion_requirements?.find((requirement) => requirement.key === key);
    const requirement: WorkItemCompletionRequirement = {
      key,
      label: current?.label ?? completionRequirementFallbackLabel(key),
      required: true,
      status: 'missing',
      satisfied_by: current?.satisfied_by,
    };
    const target = completionRequirementPresentation(item, requirement).target;
    if (target) return target;
  }

  return undefined;
}

function completionRequirementFallbackLabel(key: string) {
  if (key === 'workflow_form') return 'Hồ sơ nghiệp vụ bắt buộc';
  if (key === 'checklist') return 'Checklist của bước';
  if (key === 'document') return 'Tài liệu';
  if (key === 'photo') return 'Hình ảnh';
  return key;
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
