import { useEffect, useMemo, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { del, downloadBlob, get, post, put } from '../../lib/api/client';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ToastProvider';
import { Dialog } from '../../components/Dialog';
import { useAuth } from '../../lib/auth/AuthContext';
import type { PipelineEdgeConditionDefinition, PipelineNode, PipelineTemplate, PipelineTemplateEdge, WorkflowFormCatalogItem } from '../../lib/types/domain';
import { PipelineBuilderShell } from './components/PipelineBuilderShell';
import { TemplateSidebar } from './components/TemplateSidebar';
import { NodeLibraryPanel } from './components/NodeLibraryPanel';
import { NodeInspectorPanel } from './components/NodeInspectorPanel';
import { EdgeInspectorPanel } from './components/EdgeInspectorPanel';
import { DiagramCanvas } from './components/diagram/DiagramCanvas';
import { autoLayoutPosition, nextNodePosition } from './components/diagram/diagramConstants';
import type { DiagramEditingMode, NodePosition } from './components/diagram/diagramTypes';
import { WorkflowDefinitionImportDialog } from '../workflow-definitions/WorkflowDefinitionImportDialog';
import { WorkflowDefinitionExportDialog } from '../workflow-definitions/WorkflowDefinitionExportDialog';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import {
  buildCreateEdgePayload,
  buildUpdateEdgeLayoutPayload,
  buildUpdateEdgePayload,
  type ConnectorCreateInput,
  type ConnectorReconnectInput,
} from './components/diagram/edgePersistence';
import {
  boolValue,
  buildValidationIssues,
  cleanString,
  defaultRoleForNode,
  defaultStepTypeForNode,
  defaultStepTypeForStep,
  defaultStepsForType,
  defaultWorkflowFormVersionForStep,
  isRecord,
  nextStepBusinessCode,
  normalizePermissions,
  normalizeRoleIds,
  pipelineTemplatePDFDownloadName,
  sortedStepsForNode,
  validBusinessCode,
  validEdgePriority,
  workflowFormRequiredForStepType,
  workflowFormsForTemplate,
} from './pipelineBuilderDomain';

type NodePreset = {
  type: string;
  name: string;
  description: string;
};

type EdgeForm = {
  label: string;
  condition_type: string;
  priority: string;
  condition_config: Record<string, any>;
};

type StepForm = {
  business_code: string;
  name: string;
  step_type: string;
  required_role_ids: string[];
  form_required: boolean;
  form_version_id: string;
};

const nodeLibrary: NodePreset[] = [
  { type: 'survey', name: 'Khảo sát', description: 'Ghi nhận hiện trạng, kích thước và rủi ro ban đầu.' },
  { type: 'design', name: 'Thiết kế', description: 'Chuẩn bị bản vẽ, phương án thiết kế và xác nhận với khách hàng.' },
  { type: 'boq', name: 'BOQ / Dự toán', description: 'QS bóc tách khối lượng và kiểm tra ngân sách.' },
  { type: 'budget_approval', name: 'Duyệt ngân sách', description: 'Kiểm tra ngân sách và duyệt các khoản vượt hạn mức.' },
  { type: 'procurement', name: 'Mua vật tư', description: 'Tạo yêu cầu mua, duyệt mua và đặt hàng.' },
  { type: 'warehouse_issue', name: 'Xuất kho', description: 'Kiểm tra tồn kho và xuất vật tư ra công trình.' },
  { type: 'construction', name: 'Thi công', description: 'Daily report, issue, vật tư sử dụng và tiến độ.' },
  { type: 'acceptance', name: 'Nghiệm thu', description: 'Checklist nghiệm thu, pass/fail và punch list.' },
  { type: 'rework', name: 'Rework', description: 'Sửa lỗi nghiệm thu, kiểm tra lại và đóng punch list.' },
  { type: 'handover', name: 'Bàn giao', description: 'Closure checker, biên bản bàn giao và chứng từ.' },
];

const presetNodes = nodeLibrary.slice(0, 8).map((node) => [node.name, node.type, defaultStepsForType(node.type)] as const);

export function PipelineBuilderPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasPermission } = useAuth();
  const [selectedTemplateId, setSelectedTemplateId] = useState(id ?? '');
  const [selectedNodeId, setSelectedNodeId] = useState('');
  const [selectedConnectorId, setSelectedConnectorId] = useState('');
  const [detailsNodeId, setDetailsNodeId] = useState('');
  const [nodePositionResetToken, setNodePositionResetToken] = useState(0);
  const [templatesExpanded, setTemplatesExpanded] = useState(true);
  const [renameDialogOpen, setRenameDialogOpen] = useState(false);
  const [templateNameForm, setTemplateNameForm] = useState('');
  const [templateNameSubmitAttempted, setTemplateNameSubmitAttempted] = useState(false);
  const [deleteTemplateDialog, setDeleteTemplateDialog] = useState<PipelineTemplate | null>(null);
  const [stepDialog, setStepDialog] = useState<{ mode: 'add' | 'edit' | 'view'; step?: Record<string, any> } | null>(null);
  const [stepForm, setStepForm] = useState<StepForm>({ business_code: '', name: '', step_type: 'task', required_role_ids: ['PM'], form_required: true, form_version_id: '' });
  const [stepSubmitAttempted, setStepSubmitAttempted] = useState(false);
  const [stepValidationRequestId, setStepValidationRequestId] = useState(0);
  const [nodeForm, setNodeForm] = useState<Record<string, any>>({});
  const [edgeForm, setEdgeForm] = useState<EdgeForm>({ label: '', condition_type: 'always', priority: '100', condition_config: {} });
  const [edgeSubmitAttempted, setEdgeSubmitAttempted] = useState(false);
  const [edgeValidationRequestId, setEdgeValidationRequestId] = useState(0);
  const [definitionImportOpen, setDefinitionImportOpen] = useState(false);
  const [definitionExportOpen, setDefinitionExportOpen] = useState(false);
  const renameDialogContentRef = useRef<HTMLDivElement | null>(null);
  const navigateRenameValidation = useValidationNavigation(renameDialogContentRef);

  const templates = useQuery({ queryKey: ['pipeline-templates'], queryFn: () => get<PipelineTemplate[]>('/pipeline-templates') });
  const template = useQuery({
    queryKey: ['pipeline-template', selectedTemplateId],
    queryFn: () => get<PipelineTemplate>(`/pipeline-templates/${selectedTemplateId}`),
    enabled: Boolean(selectedTemplateId),
  });
  const edgeConditions = useQuery({
    queryKey: ['pipeline-edge-conditions'],
    queryFn: () => get<PipelineEdgeConditionDefinition[]>('/pipeline-edge-conditions'),
    enabled: Boolean(selectedTemplateId),
  });
  const workflowForms = useQuery({
    queryKey: ['workflow-form-catalog'],
    queryFn: () => get<WorkflowFormCatalogItem[]>('/workflow-form-templates'),
    enabled: Boolean(selectedTemplateId),
  });
  const permissions = useQuery({
    queryKey: ['permissions'],
    queryFn: () => get<Record<string, any>[]>('/permissions'),
    enabled: Boolean(selectedTemplateId),
  });

  useEffect(() => {
    if (!selectedTemplateId && templates.data?.[0]) {
      setSelectedTemplateId(templates.data[0].id);
    }
  }, [templates.data, selectedTemplateId]);

  useEffect(() => {
    if (id) {
      setSelectedTemplateId(id);
      setSelectedNodeId('');
      setSelectedConnectorId('');
      setDetailsNodeId('');
    }
  }, [id]);

  const nodes = useMemo(
    () => [...(template.data?.nodes ?? [])].sort((a, b) => Number(a.order_index ?? 0) - Number(b.order_index ?? 0)),
    [template.data?.nodes],
  );
  const rawEdges = useMemo(
    () => (Array.isArray(template.data?.edges) ? [...template.data.edges] : []),
    [template.data?.edges],
  );
  const availableWorkflowForms = useMemo(
    () => workflowFormsForTemplate(workflowForms.data ?? [], template.data?.pinned_workflow_forms ?? []),
    [template.data?.pinned_workflow_forms, workflowForms.data],
  );
  const nodeById = useMemo(() => new Map(nodes.map((node) => [node.id, node])), [nodes]);
  const selectedNode = selectedNodeId ? nodeById.get(selectedNodeId) : undefined;
  const selectedConnector = selectedConnectorId ? rawEdges.find((edge) => String(edge.id) === selectedConnectorId) : undefined;
  const templateStatus = String(template.data?.status ?? '');
  const isPublished = templateStatus === 'published';
  const hasPipelineEditPermission = hasPermission('pipeline.edit');
  const canCreateTemplate = hasPermission('pipeline.create');
  const canPublishTemplate = hasPermission('pipeline.publish') && templateStatus === 'draft';
  const canImportDefinition = hasPermission('workflow_definition.import') && canCreateTemplate;
  const canExportDefinition = hasPermission('workflow_definition.export') && hasPermission('pipeline.view');
  const canEditLayout = Boolean(template.data && hasPipelineEditPermission && (templateStatus === 'draft' || templateStatus === 'published'));
  const canEditStructure = Boolean(template.data && hasPipelineEditPermission && templateStatus === 'draft');
  const isReadOnly = Boolean(template.data && !canEditStructure);
  const diagramEditingMode: DiagramEditingMode = canEditStructure ? 'full' : canEditLayout ? 'layout' : 'view';
  const answerFieldOptions = useMemo(() => {
    if (!selectedConnector) return [];
    const sourceNode = nodeById.get(String(selectedConnector.from_node_id));
    const versionIds = new Set((sourceNode?.steps ?? []).map((step: Record<string, any>) => String(step.form_version_id ?? '')).filter(Boolean));
    const seen = new Set<string>();
    return availableWorkflowForms.flatMap((form) => {
      if (!versionIds.has(String(form.version_id))) return [];
      return (form.fields ?? []).flatMap((field) => {
        if (seen.has(field.field_key)) return [];
        seen.add(field.field_key);
        return [{ label: `${field.label} (${field.field_key})`, value: field.field_key }];
      });
    });
  }, [availableWorkflowForms, nodeById, selectedConnector]);
  const answerValueOptionsByFieldKey = useMemo(() => {
    if (!selectedConnector) return {};
    const sourceNode = nodeById.get(String(selectedConnector.from_node_id));
    const versionIds = new Set((sourceNode?.steps ?? []).map((step: Record<string, any>) => String(step.form_version_id ?? '')).filter(Boolean));
    const optionsByFieldKey: Record<string, Array<{ label: string; value: string }>> = {};
    availableWorkflowForms.forEach((form) => {
      if (!versionIds.has(String(form.version_id))) return;
      (form.fields ?? []).forEach((field) => {
        const options = Array.isArray(field.options_json) ? field.options_json : [];
        optionsByFieldKey[field.field_key] = options.map((option) => {
          if (typeof option === 'string') return { label: option, value: option };
          return { label: option.label ?? option.value, value: option.value };
        }).filter((option) => option.value);
      });
    });
    return optionsByFieldKey;
  }, [availableWorkflowForms, nodeById, selectedConnector]);
  const selectedConnectorSource = selectedConnector ? nodeById.get(String(selectedConnector.from_node_id)) : undefined;
  const selectedConnectorTarget = selectedConnector ? nodeById.get(String(selectedConnector.to_node_id)) : undefined;
  const detailsNode = detailsNodeId ? nodeById.get(detailsNodeId) : undefined;
  const edgeConditionTypes = useMemo(
    () => new Set((edgeConditions.data ?? []).map((condition) => condition.type)),
    [edgeConditions.data],
  );
  const selectedEdgeCondition = useMemo(
    () => (edgeConditions.data ?? []).find((condition) => condition.type === edgeForm.condition_type),
    [edgeConditions.data, edgeForm.condition_type],
  );
  const invalidEdgeConfigKeys = useMemo(
    () => (selectedEdgeCondition?.config_schema ?? [])
      .filter((field) => field.required && String(edgeForm.condition_config[field.key] ?? '').trim() === '')
      .map((field) => field.key),
    [edgeForm.condition_config, selectedEdgeCondition?.config_schema],
  );
  const permissionOptions = useMemo(
    () => (permissions.data ?? []).map((permission) => ({
      label: `${permission.key} - ${permission.name}`,
      value: String(permission.key),
    })),
    [permissions.data],
  );
  const workflowFormVersionIds = useMemo(
    () => new Set(availableWorkflowForms.map((form) => String(form.version_id)).filter(Boolean)),
    [availableWorkflowForms],
  );

  useEffect(() => {
    if (selectedNodeId && !nodeById.has(selectedNodeId)) setSelectedNodeId('');
    if (detailsNodeId && !nodeById.has(detailsNodeId)) setDetailsNodeId('');
  }, [detailsNodeId, nodeById, selectedNodeId]);

  useEffect(() => {
    if (selectedConnectorId && !rawEdges.some((edge) => String(edge.id) === selectedConnectorId)) setSelectedConnectorId('');
  }, [rawEdges, selectedConnectorId]);

  useEffect(() => {
    if (!selectedConnector) {
      setEdgeForm({ label: '', condition_type: 'always', priority: '100', condition_config: {} });
      setEdgeSubmitAttempted(false);
      return;
    }

    setEdgeForm({
      label: selectedConnector.label ?? '',
      condition_type: selectedConnector.condition_type ?? 'always',
      priority: String(selectedConnector.priority ?? 100),
      condition_config: isRecord(selectedConnector.condition_config) ? selectedConnector.condition_config : {},
    });
    setEdgeSubmitAttempted(false);
  }, [selectedConnector?.id]);

  useEffect(() => {
    if (!detailsNode) {
      setNodeForm({});
      closeStepDialog();
      return;
    }

    setNodeForm({
      business_code: detailsNode.business_code ?? '',
      name: detailsNode.name,
      description: detailsNode.description ?? '',
      type: detailsNode.type ?? 'custom',
      permissions: normalizePermissions(detailsNode),
      approval_policy_json: isRecord(detailsNode.approval_policy_json) ? detailsNode.approval_policy_json : { mode: 'none' },
      exit_conditions: isRecord(detailsNode.exit_conditions) ? detailsNode.exit_conditions : {},
    });
    closeStepDialog();
  }, [detailsNode?.id]);

  useEffect(() => {
    if (!stepDialog || stepDialog.mode === 'view' || !stepForm.form_required || stepForm.form_version_id || !workflowForms.data?.length) return;
    setStepForm((current) => {
      if (!current.form_required || current.form_version_id) return current;
      return { ...current, form_version_id: defaultWorkflowFormVersionForStep(current.step_type, workflowForms.data ?? [], detailsNode?.type, current.name) };
    });
  }, [detailsNode?.type, stepDialog, stepForm.form_required, stepForm.form_version_id, workflowForms.data]);

  const validationIssues = useMemo(
    () => buildValidationIssues(nodes, rawEdges, edgeConditions.data ?? [], workflowFormVersionIds, isPublished),
    [nodes, rawEdges, edgeConditions.data, workflowFormVersionIds, isPublished],
  );

  const invalidate = (templateId = selectedTemplateId) => {
    queryClient.invalidateQueries({ queryKey: ['pipeline-templates'] });
    if (templateId) queryClient.invalidateQueries({ queryKey: ['pipeline-template', templateId] });
  };

  const createTemplate = useMutation({
    mutationFn: () =>
      post<PipelineTemplate>('/pipeline-templates', {
        name: 'Template mới',
        description: 'Template tạo từ Pipeline Builder',
        project_type: 'Hoàn thiện nội thất',
        status: 'draft',
        nodes: presetNodes.map(([name, type, steps], index) => ({
          name,
          description: `Node ${name} trong workflow công trình.`,
          type,
          order_index: index + 1,
          position_x: autoLayoutPosition(index).x,
          position_y: autoLayoutPosition(index).y,
          permissions: [],
          steps: steps.map((stepName) => ({
            name: stepName,
            step_type: defaultStepTypeForStep(type, stepName),
            required_role_ids: [defaultRoleForNode(type)],
          })),
        })),
      }),
    onSuccess: (created) => {
      invalidate(created.id);
      setSelectedTemplateId(created.id);
      setSelectedNodeId('');
      setSelectedConnectorId('');
      setDetailsNodeId('');
      navigate(`/pipeline-builder/${created.id}`);
      toast.success('Đã tạo template');
    },
    onError: (error) => toast.error('Không tạo được template', error),
  });

  const publish = useMutation({
    mutationFn: () => post(`/pipeline-templates/${selectedTemplateId}/publish`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã publish template');
    },
    onError: (error) => toast.error('Không publish được template', error),
  });

  const clone = useMutation({
    mutationFn: () => post<PipelineTemplate>(`/pipeline-templates/${selectedTemplateId}/clone`, {}),
    onSuccess: (created) => {
      invalidate(created.id);
      setSelectedTemplateId(created.id);
      setSelectedNodeId('');
      setSelectedConnectorId('');
      setDetailsNodeId('');
      navigate(`/pipeline-builder/${created.id}`);
      toast.success('Đã tạo revision nháp mới');
    },
    onError: (error) => toast.error('Không clone được template', error),
  });

  const exportPDF = useMutation({
    mutationFn: async () => {
      if (!selectedTemplateId || !template.data) throw new Error('Chưa tải xong workflow template');
      const blob = await downloadBlob(`/pipeline-templates/${selectedTemplateId}/pdf`);
      return { blob, filename: pipelineTemplatePDFDownloadName(template.data) };
    },
    onSuccess: ({ blob, filename }) => {
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = filename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã xuất PDF workflow');
    },
    onError: (error) => toast.error('Không xuất được PDF workflow', error),
  });

  const updateTemplateName = useMutation({
    mutationFn: () => {
      if (!selectedTemplateId) throw new Error('Chưa chọn template');
      return put<PipelineTemplate>(`/pipeline-templates/${selectedTemplateId}`, {
        name: templateNameForm.trim(),
      });
    },
    onSuccess: () => {
      closeRenameDialog();
      invalidate();
      toast.success('Đã đổi tên template');
    },
    onError: (error) => toast.error('Không đổi tên được template', error),
  });

  const deleteTemplate = useMutation({
    mutationFn: async (templateToDelete: PipelineTemplate) => {
      await del(`/pipeline-templates/${templateToDelete.id}`);
      return templateToDelete;
    },
    onSuccess: (deletedTemplate) => {
      const remainingTemplates = (templates.data ?? []).filter((item) => item.id !== deletedTemplate.id);
      const nextTemplate = remainingTemplates[0];

      setDeleteTemplateDialog(null);
      queryClient.setQueryData<PipelineTemplate[]>(['pipeline-templates'], remainingTemplates);
      queryClient.removeQueries({ queryKey: ['pipeline-template', deletedTemplate.id] });
      queryClient.invalidateQueries({ queryKey: ['pipeline-templates'] });

      if (selectedTemplateId === deletedTemplate.id) {
        setSelectedNodeId('');
        setSelectedConnectorId('');
        setDetailsNodeId('');
        setSelectedTemplateId(nextTemplate?.id ?? '');
        navigate(nextTemplate ? `/pipeline-builder/${nextTemplate.id}` : '/pipeline-builder');
      }

      toast.success('Đã xóa template');
    },
    onError: (error) => toast.error('Không xóa được template', error),
  });

  const addNode = useMutation({
    mutationFn: (preset?: NodePreset) => {
      const position = nextNodePosition(nodes.length);
      const type = preset?.type ?? 'custom';
      return post<PipelineNode>(`/pipeline-templates/${selectedTemplateId}/nodes`, {
        name: preset?.name ?? 'Node mới',
        description: preset?.description ?? 'Mô tả node mới',
        type,
        order_index: nodes.length + 1,
        position_x: position.x,
        position_y: position.y,
        permissions: [],
      });
    },
    onSuccess: (created) => {
      setSelectedNodeId(created.id);
      setDetailsNodeId('');
      invalidate();
      toast.success('Đã thêm node');
    },
    onError: (error) => toast.error('Không thêm được node', error),
  });

  const saveNode = useMutation({
    mutationFn: () => {
      if (!detailsNodeId) throw new Error('Chưa chọn node');
      return put(`/pipeline-nodes/${detailsNodeId}`, {
        business_code: cleanString(nodeForm.business_code) || null,
        name: nodeForm.name,
        description: nodeForm.description,
        type: nodeForm.type,
        permissions: Array.isArray(nodeForm.permissions) ? nodeForm.permissions : [],
        approval_policy_json: isRecord(nodeForm.approval_policy_json) ? nodeForm.approval_policy_json : { mode: 'none' },
        exit_conditions: isRecord(nodeForm.exit_conditions) ? nodeForm.exit_conditions : {},
      });
    },
    onSuccess: () => {
      invalidate();
      toast.success('Đã lưu node');
    },
    onError: (error) => toast.error('Không lưu được node', error),
  });

  const saveNodePosition = useMutation({
    mutationFn: ({ templateId, nodeId, position }: { templateId: string; nodeId: string; position: NodePosition }) =>
      put<PipelineTemplate>(`/pipeline-templates/${templateId}/layout`, {
        nodes: [{
          id: nodeId,
          position_x: Math.round(position.x),
          position_y: Math.round(position.y),
        }],
      }),
    onSuccess: (updated) => {
      queryClient.setQueryData(['pipeline-template', updated.id], updated);
      queryClient.invalidateQueries({ queryKey: ['pipeline-templates'] });
      toast.success('Đã lưu vị trí node');
    },
    onError: (error, variables) => {
      setNodePositionResetToken((current) => current + 1);
      queryClient.invalidateQueries({ queryKey: ['pipeline-template', variables.templateId] });
      toast.error('Không lưu được vị trí node', error);
    },
  });

  const createEdge = useMutation({
    mutationFn: async (input: ConnectorCreateInput) => {
      const templateId = selectedTemplateId;
      if (!templateId) throw new Error('Chưa chọn template');
      const edge = await post<PipelineTemplateEdge>(`/pipeline-templates/${templateId}/edges`, buildCreateEdgePayload(input));
      return { edge, templateId };
    },
    onSuccess: ({ edge, templateId }) => {
      queryClient.setQueryData<PipelineTemplate>(['pipeline-template', templateId], (current) => {
        if (!current) return current;
        const edges = Array.isArray(current.edges) ? current.edges : [];
        if (edges.some((currentEdge) => currentEdge.id === edge.id)) return current;
        return { ...current, edges: [...edges, edge] };
      });
      invalidate(templateId);
      toast.success('Đã tạo connector');
    },
    onError: (error) => toast.error('Không tạo được connector', error),
  });

  const updateEdge = useMutation({
    mutationFn: async (input: ConnectorReconnectInput) => {
      const templateId = selectedTemplateId;
      if (!templateId) throw new Error('Chưa chọn template');
      const currentEdge = rawEdges.find((edge) => edge.id === input.connectorId);
      const edge = await put<PipelineTemplateEdge>(`/pipeline-edges/${input.connectorId}`, buildUpdateEdgePayload(currentEdge, input));
      return { edge, templateId };
    },
    onSuccess: ({ edge, templateId }) => {
      queryClient.setQueryData<PipelineTemplate>(['pipeline-template', templateId], (current) => {
        if (!current) return current;
        const edges = Array.isArray(current.edges) ? current.edges : [];
        const edgeExists = edges.some((currentEdge) => currentEdge.id === edge.id);
        return {
          ...current,
          edges: edgeExists ? edges.map((currentEdge) => (currentEdge.id === edge.id ? edge : currentEdge)) : [...edges, edge],
        };
      });
      invalidate(templateId);
      toast.success('Đã cập nhật connector');
    },
    onError: (error) => toast.error('Không cập nhật được connector', error),
  });

  const updateEdgeLayout = useMutation({
    mutationFn: ({ templateId, input }: { templateId: string; input: ConnectorReconnectInput }) =>
      put<PipelineTemplate>(`/pipeline-templates/${templateId}/layout`, {
        edges: [buildUpdateEdgeLayoutPayload(input)],
      }),
    onSuccess: (updated) => {
      queryClient.setQueryData(['pipeline-template', updated.id], updated);
      queryClient.invalidateQueries({ queryKey: ['pipeline-templates'] });
      toast.success('Đã lưu điểm nối connector');
    },
    onError: (error, variables) => {
      queryClient.invalidateQueries({ queryKey: ['pipeline-template', variables.templateId] });
      toast.error('Không lưu được điểm nối connector', error);
    },
  });

  const saveEdgeSettings = useMutation({
    mutationFn: async () => {
      const templateId = selectedTemplateId;
      if (!templateId) throw new Error('Chưa chọn template');
      if (!selectedConnector) throw new Error('Chưa chọn connector');
      const edge = await put<PipelineTemplateEdge>(`/pipeline-edges/${selectedConnector.id}`, {
        label: edgeForm.label.trim(),
        condition_type: edgeForm.condition_type,
        condition_config: edgeForm.condition_config,
        priority: Number(edgeForm.priority),
      });
      return { edge, templateId };
    },
    onSuccess: ({ edge, templateId }) => {
      queryClient.setQueryData<PipelineTemplate>(['pipeline-template', templateId], (current) => {
        if (!current) return current;
        const edges = Array.isArray(current.edges) ? current.edges : [];
        return {
          ...current,
          edges: edges.map((currentEdge) => (String(currentEdge.id) === String(edge.id) ? edge : currentEdge)),
        };
      });
      setEdgeSubmitAttempted(false);
      invalidate(templateId);
      toast.success('Đã lưu connector');
    },
    onError: (error) => toast.error('Không lưu được connector', error),
  });

  const deleteEdge = useMutation({
    mutationFn: async (edgeId: string) => {
      const templateId = selectedTemplateId;
      if (!templateId) throw new Error('Chưa chọn template');
      await del(`/pipeline-edges/${edgeId}`);
      return { edgeId, templateId };
    },
    onSuccess: ({ edgeId, templateId }) => {
      setSelectedConnectorId((current) => (current === edgeId ? '' : current));
      queryClient.setQueryData<PipelineTemplate>(['pipeline-template', templateId], (current) => {
        if (!current) return current;
        const edges = Array.isArray(current.edges) ? current.edges : [];
        return { ...current, edges: edges.filter((edge) => String(edge.id) !== edgeId) };
      });
      invalidate(templateId);
      toast.success('Đã xóa connector');
    },
    onError: (error) => toast.error('Không xóa được connector', error),
  });

  const addStep = useMutation({
    mutationFn: () => {
      if (!detailsNodeId) throw new Error('Chưa chọn node');
      const nextOrderIndex = Math.max(0, ...(detailsNode?.steps ?? []).map((step: Record<string, any>) => Number(step.order_index ?? 0))) + 1;
      return post(`/pipeline-nodes/${detailsNodeId}/steps`, {
        business_code: cleanString(stepForm.business_code) || null,
        name: stepForm.name.trim(),
        description: 'Checklist mẫu trong node',
        step_type: stepForm.step_type,
        required_role_ids: normalizeRoleIds(stepForm.required_role_ids),
        form_required: stepForm.form_required,
        form_version_id: stepForm.form_version_id || null,
        order_index: nextOrderIndex,
      });
    },
    onSuccess: () => {
      closeStepDialog();
      invalidate();
      toast.success('Đã thêm checklist');
    },
    onError: (error) => toast.error('Không thêm được checklist', error),
  });

  const updateStep = useMutation({
    mutationFn: () => {
      const stepId = stepDialog?.step?.id;
      if (!stepId) throw new Error('Chưa chọn checklist');
      return put(`/pipeline-steps/${stepId}`, {
        business_code: cleanString(stepForm.business_code) || null,
        name: stepForm.name.trim(),
        step_type: stepForm.step_type,
        required_role_ids: normalizeRoleIds(stepForm.required_role_ids),
        form_required: stepForm.form_required,
        form_version_id: stepForm.form_version_id || null,
      });
    },
    onSuccess: () => {
      closeStepDialog();
      invalidate();
      toast.success('Đã lưu checklist');
    },
    onError: (error) => toast.error('Không lưu được checklist', error),
  });

  const deleteStep = useMutation({
    mutationFn: (stepId: string) => del(`/pipeline-steps/${stepId}`),
    onSuccess: () => {
      invalidate();
      toast.success('Đã xóa checklist');
    },
    onError: (error) => toast.error('Không xóa được checklist', error),
  });

  const reorderStep = useMutation({
    mutationFn: async ({ orderedStepIds }: { orderedStepIds: string[] }) => {
      const templateId = selectedTemplateId;
      if (!templateId) throw new Error('Chưa chọn template');
      const steps = sortedStepsForNode(detailsNode);
      const stepById = new Map(steps.map((step) => [String(step.id), step]));
      const currentStepIds = steps.map((step) => (step.id ? String(step.id) : '')).filter(Boolean);
      const uniqueOrderedIds = orderedStepIds.filter((stepId, index) => orderedStepIds.indexOf(stepId) === index && stepById.has(stepId));
      if (uniqueOrderedIds.length !== currentStepIds.length) return { updates: [], templateId };

      const updates = uniqueOrderedIds
        .map((stepId, index) => ({ stepId, orderIndex: index + 1, previousOrderIndex: Number(stepById.get(stepId)?.order_index ?? 0) }))
        .filter((update) => update.orderIndex !== update.previousOrderIndex);

      await Promise.all(updates.map((update) => put(`/pipeline-steps/${update.stepId}`, { order_index: update.orderIndex })));
      return { updates, templateId };
    },
    onSuccess: ({ updates, templateId }) => {
      if (!updates.length) return;
      const updateById = new Map(updates.map((update) => [update.stepId, update.orderIndex]));
      queryClient.setQueryData<PipelineTemplate>(['pipeline-template', templateId], (current) => {
        if (!current?.nodes) return current;
        return {
          ...current,
          nodes: current.nodes.map((node) => ({
            ...node,
            steps: Array.isArray(node.steps)
              ? node.steps.map((step: Record<string, any>) => {
                  const orderIndex = updateById.get(String(step.id));
                  return orderIndex === undefined ? step : { ...step, order_index: orderIndex };
                })
              : node.steps,
          })),
        };
      });
      invalidate(templateId);
      toast.success('Đã sắp xếp checklist');
    },
    onError: (error) => toast.error('Không sắp xếp được checklist', error),
  });

  const openAddStepDialog = () => {
    const stepType = defaultStepTypeForNode(detailsNode?.type);
    setStepForm({
      business_code: nextStepBusinessCode(detailsNode, detailsNode?.steps ?? []),
      name: '',
      step_type: stepType,
      required_role_ids: [defaultRoleForNode(detailsNode?.type)],
      form_required: true,
      form_version_id: defaultWorkflowFormVersionForStep(stepType, workflowForms.data ?? [], detailsNode?.type),
    });
    setStepSubmitAttempted(false);
    setStepDialog({ mode: 'add' });
  };

  const openEditStepDialog = (step: Record<string, any>) => {
    const stepType = String(step.step_type ?? 'task');
    const formRequired = boolValue(step.form_required, true);
    setStepForm({
      business_code: cleanString(step.business_code),
      name: String(step.name ?? ''),
      step_type: stepType,
      required_role_ids: normalizeRoleIds(step.required_role_ids),
      form_required: formRequired,
      form_version_id: cleanString(step.form_version_id) || (formRequired ? defaultWorkflowFormVersionForStep(stepType, workflowForms.data ?? [], detailsNode?.type, String(step.name ?? '')) : ''),
    });
    setStepSubmitAttempted(false);
    setStepDialog({ mode: 'edit', step });
  };

  const openViewStepDialog = (step: Record<string, any>) => {
    setStepForm({
      business_code: cleanString(step.business_code),
      name: String(step.name ?? ''),
      step_type: String(step.step_type ?? 'task'),
      required_role_ids: normalizeRoleIds(step.required_role_ids),
      form_required: boolValue(step.form_required, true),
      form_version_id: cleanString(step.form_version_id),
    });
    setStepSubmitAttempted(false);
    setStepDialog({ mode: 'view', step });
  };

  const closeStepDialog = () => {
    setStepDialog(null);
    setStepForm({ business_code: '', name: '', step_type: 'task', required_role_ids: ['PM'], form_required: true, form_version_id: '' });
    setStepSubmitAttempted(false);
  };

  const changeStepField = (field: keyof StepForm, value: string | string[] | boolean) => {
    setStepForm((current) => {
      if (field === 'step_type') {
        const stepType = String(value);
        const currentDefault = defaultWorkflowFormVersionForStep(current.step_type, workflowForms.data ?? [], detailsNode?.type, current.name);
        const shouldRefreshForm = !current.form_version_id || current.form_version_id === currentDefault;
        return {
          ...current,
          step_type: stepType,
          form_required: current.form_required || workflowFormRequiredForStepType(stepType),
          form_version_id: shouldRefreshForm ? defaultWorkflowFormVersionForStep(stepType, workflowForms.data ?? [], detailsNode?.type, current.name) : current.form_version_id,
        };
      }
      if (field === 'name') {
        const name = String(value);
        const currentDefault = defaultWorkflowFormVersionForStep(current.step_type, workflowForms.data ?? [], detailsNode?.type, current.name);
        const shouldRefreshForm = !current.form_version_id || current.form_version_id === currentDefault;
        return {
          ...current,
          name,
          form_version_id: shouldRefreshForm ? defaultWorkflowFormVersionForStep(current.step_type, workflowForms.data ?? [], detailsNode?.type, name) : current.form_version_id,
        };
      }
      if (field === 'form_required') {
        const formRequired = Boolean(value);
        return {
          ...current,
          form_required: formRequired,
          form_version_id: formRequired && !current.form_version_id ? defaultWorkflowFormVersionForStep(current.step_type, workflowForms.data ?? [], detailsNode?.type, current.name) : current.form_version_id,
        };
      }
      return { ...current, [field]: value };
    });
  };

  const submitStepDialog = () => {
    if (stepDialog?.mode === 'view') return;
    setStepSubmitAttempted(true);
    const invalidFields: string[] = [];
    if (Number(template.data?.engine_version ?? 1) >= 3 && !validBusinessCode(stepForm.business_code)) invalidFields.push('Business code');
    if (!stepForm.name.trim()) invalidFields.push('Tên checklist');
    if (!normalizeRoleIds(stepForm.required_role_ids).length) invalidFields.push('Role phụ trách');
    if (stepForm.form_required && !stepForm.form_version_id) invalidFields.push('Workflow form');
    if (stepForm.form_version_id && workflowFormVersionIds.size > 0 && !workflowFormVersionIds.has(stepForm.form_version_id)) invalidFields.push('Workflow form published');
    if (invalidFields.length) {
      toast.warn({
        title: 'Thiếu thông tin checklist',
        description: invalidFields.join(', '),
      });
      setStepValidationRequestId((current) => current + 1);
      return;
    }
    if (stepDialog?.mode === 'edit') {
      updateStep.mutate();
      return;
    }
    addStep.mutate();
  };

  const submitEdgeSettings = () => {
    setEdgeSubmitAttempted(true);
    const invalidLabels: string[] = [];
    const priority = Number(edgeForm.priority);
    if (edgeForm.label.trim().length > 80) invalidLabels.push('Nhãn');
    if (!edgeForm.condition_type || !edgeConditionTypes.has(edgeForm.condition_type)) invalidLabels.push('Điều kiện chuyển tiếp');
    if (!Number.isInteger(priority) || priority < 1 || priority > 999) invalidLabels.push('Ưu tiên');
    if (edgeConditions.isError) invalidLabels.push('Danh sách điều kiện');
    invalidLabels.push(...(selectedEdgeCondition?.config_schema ?? [])
      .filter((field) => invalidEdgeConfigKeys.includes(field.key))
      .map((field) => field.label));

    if (invalidLabels.length) {
      toast.warn({
        title: 'Thiếu hoặc sai cấu hình connector',
        description: invalidLabels.join(', '),
      });
      setEdgeValidationRequestId((current) => current + 1);
      return;
    }

    saveEdgeSettings.mutate();
  };

  const deleteNode = useMutation({
    mutationFn: (nodeId: string) => del(`/pipeline-nodes/${nodeId}`),
    onSuccess: (_, nodeId) => {
      if (selectedNodeId === nodeId) setSelectedNodeId('');
      if (detailsNodeId === nodeId) setDetailsNodeId('');
      invalidate();
      toast.success('Đã xóa node');
    },
    onError: (error) => toast.error('Không xóa được node', error),
  });

  const autoLayout = useMutation({
    mutationFn: async () => {
      const templateId = selectedTemplateId;
      if (!templateId) throw new Error('Chưa chọn template');
      const updates = nodes.map((node, index) => ({
        nodeId: node.id,
        position: autoLayoutPosition(index),
      }));
      const updated = await put<PipelineTemplate>(`/pipeline-templates/${templateId}/layout`, {
        nodes: updates.map((update) => ({
          id: update.nodeId,
          position_x: update.position.x,
          position_y: update.position.y,
        })),
      });
      return { updated, templateId };
    },
    onSuccess: ({ updated, templateId }) => {
      queryClient.setQueryData(['pipeline-template', templateId], updated);
      queryClient.invalidateQueries({ queryKey: ['pipeline-templates'] });
      toast.success('Đã tự sắp xếp node');
    },
    onError: (error) => {
      if (selectedTemplateId) queryClient.invalidateQueries({ queryKey: ['pipeline-template', selectedTemplateId] });
      toast.error('Không tự sắp xếp được node', error);
    },
  });

  const selectTemplate = (item: PipelineTemplate) => {
    setSelectedTemplateId(item.id);
    setSelectedNodeId('');
    setSelectedConnectorId('');
    setDetailsNodeId('');
    navigate(`/pipeline-builder/${item.id}`);
  };

  const openRenameDialog = () => {
    setTemplateNameForm(template.data?.name ?? '');
    setTemplateNameSubmitAttempted(false);
    setRenameDialogOpen(true);
  };

  const closeRenameDialog = () => {
    setRenameDialogOpen(false);
    setTemplateNameForm('');
    setTemplateNameSubmitAttempted(false);
  };

  const submitRenameDialog = () => {
    setTemplateNameSubmitAttempted(true);
    if (!templateNameForm.trim()) {
      toast.warn({
        title: 'Thiếu tên template',
        description: 'Vui lòng nhập tên template',
      });
      navigateRenameValidation('template_name');
      return;
    }
    updateTemplateName.mutate();
  };

  const validatePipeline = () => {
    if (!validationIssues.length) {
      toast.success('Pipeline hợp lệ');
      return;
    }
    toast.error({
      title: 'Workflow cần kiểm tra',
      description: validationIssues.slice(0, 4).join('; ') + (validationIssues.length > 4 ? `; +${validationIssues.length - 4} lỗi khác` : ''),
    });
  };

  const goBack = () => {
    navigate('/projects');
  };

  const connectorMutationPending =
    createEdge.isPending ||
    updateEdge.isPending ||
    updateEdgeLayout.isPending ||
    deleteEdge.isPending ||
    saveEdgeSettings.isPending ||
    saveNodePosition.isPending ||
    autoLayout.isPending;
  const canDeleteSelection = canEditStructure && Boolean(selectedConnector || selectedNode);
  const deleteSelectionLabel = selectedConnector ? 'Xóa connector' : 'Xóa node';
  const deleteSelectionPending = selectedConnector ? deleteEdge.isPending : deleteNode.isPending;
  const deleteSelection = () => {
    if (!canEditStructure) return;
    if (selectedConnector) {
      deleteEdge.mutate(String(selectedConnector.id));
      return;
    }
    if (selectedNodeId) deleteNode.mutate(selectedNodeId);
  };

  const reconnectConnector = (input: ConnectorReconnectInput) => {
    if (diagramEditingMode === 'full') {
      updateEdge.mutate(input);
      return;
    }
    if (diagramEditingMode !== 'layout' || !selectedTemplateId) return;

    const edge = rawEdges.find((candidate) => String(candidate.id) === input.connectorId);
    if (!edge || String(edge.from_node_id) !== input.source.nodeId || String(edge.to_node_id) !== input.target.nodeId) {
      toast.warn({
        title: 'Không thể đổi node liên kết',
        description: 'Revision đã phát hành chỉ cho phép đổi cạnh bám của connector trên chính source/target node hiện tại.',
      });
      return;
    }
    updateEdgeLayout.mutate({ templateId: selectedTemplateId, input });
  };

  if (templates.isLoading) {
    return (
      <div className="grid h-screen place-items-center bg-surface">
        <LoadingSpinner />
      </div>
    );
  }

  return (
    <>
      <PipelineBuilderShell
        sidebar={
          <TemplateSidebar
            templates={templates.data ?? []}
            selectedTemplateId={selectedTemplateId}
            expanded={templatesExpanded}
            showBackButton
            createTemplatePending={createTemplate.isPending}
            canCreateTemplate={canCreateTemplate}
            canDeleteTemplate={hasPipelineEditPermission}
            deleteTemplatePendingId={deleteTemplate.isPending ? deleteTemplateDialog?.id : undefined}
            onBack={goBack}
            onCreateTemplate={() => createTemplate.mutate()}
            onDeleteTemplate={setDeleteTemplateDialog}
            onToggleExpanded={() => setTemplatesExpanded((current) => !current)}
            onSelectTemplate={selectTemplate}
          >
            <NodeLibraryPanel
              presets={nodeLibrary}
              canAddNode={Boolean(selectedTemplateId) && canEditStructure}
              addNodePending={addNode.isPending}
              onAddNode={(preset) => addNode.mutate(preset)}
            />
          </TemplateSidebar>
        }
        templateName={template.data?.name}
        templateStatus={template.data?.status}
        templateDescription={template.data?.description}
        selectedTemplateId={selectedTemplateId}
        isReadOnly={isReadOnly}
        isPublished={isPublished}
        canEditLayout={canEditLayout}
        canDeleteSelection={canDeleteSelection}
        deleteSelectionLabel={deleteSelectionLabel}
        nodeCount={nodes.length}
        validationIssueCount={validationIssues.length}
        autoLayoutPending={autoLayout.isPending}
        canExportPDF={Boolean(selectedTemplateId && template.data) && !template.isLoading}
        canImportDefinition={canImportDefinition}
        canExportDefinition={canExportDefinition}
        canCloneTemplate={canCreateTemplate}
        canPublishTemplate={canPublishTemplate}
        exportPDFPending={exportPDF.isPending}
        deleteSelectionPending={deleteSelectionPending}
        onRenameTemplate={openRenameDialog}
        onCloneTemplate={() => clone.mutate()}
        onPublishTemplate={() => publish.mutate()}
        onAutoLayout={() => { if (canEditLayout) autoLayout.mutate(); }}
        onValidatePipeline={validatePipeline}
        onExportPDF={() => exportPDF.mutate()}
        onImportDefinition={() => setDefinitionImportOpen(true)}
        onExportDefinition={() => setDefinitionExportOpen(true)}
        onDeleteSelection={deleteSelection}
        inspector={
          detailsNode ? (
            <NodeInspectorPanel
              node={detailsNode}
              readOnly={isReadOnly}
              readOnlyLabel={isPublished ? 'Đã phát hành · Chỉ xem' : 'Chỉ xem'}
              nodeForm={nodeForm}
              permissionOptions={permissionOptions}
              permissionsLoading={permissions.isLoading}
              stepDialogMode={stepDialog?.mode ?? null}
              stepForm={stepForm}
              stepNameInvalid={stepSubmitAttempted && !stepForm.name.trim()}
              stepCodeInvalid={stepSubmitAttempted && Number(template.data?.engine_version ?? 1) >= 3 && !validBusinessCode(stepForm.business_code)}
              stepRolesInvalid={stepSubmitAttempted && !normalizeRoleIds(stepForm.required_role_ids).length}
              stepFormInvalid={stepSubmitAttempted && (
                (stepForm.form_required && !stepForm.form_version_id)
                || Boolean(stepForm.form_version_id && workflowFormVersionIds.size > 0 && !workflowFormVersionIds.has(stepForm.form_version_id))
              )}
              stepValidationRequestId={stepValidationRequestId}
              savePending={saveNode.isPending}
              addStepPending={addStep.isPending}
              updateStepPending={updateStep.isPending}
              deleteStepPending={deleteStep.isPending}
              reorderStepPending={reorderStep.isPending}
              workflowForms={availableWorkflowForms}
              workflowFormsLoading={workflowForms.isLoading}
              workflowFormsError={workflowForms.isError}
              onChangeField={(field, value) => {
                if (!isReadOnly) setNodeForm((current) => ({ ...current, [field]: value }));
              }}
              onChangeStepField={(field, value) => {
                if (!isReadOnly) changeStepField(field, value);
              }}
              onSave={() => {
                if (!isReadOnly) saveNode.mutate();
              }}
              onClose={() => setDetailsNodeId('')}
              onOpenAddStep={() => {
                if (!isReadOnly) openAddStepDialog();
              }}
              onOpenEditStep={(step) => {
                if (!isReadOnly) openEditStepDialog(step);
              }}
              onOpenViewStep={(step) => {
                if (isReadOnly) openViewStepDialog(step);
              }}
              onCloseStepDialog={closeStepDialog}
              onSubmitStep={() => {
                if (!isReadOnly) submitStepDialog();
              }}
              onDeleteStep={(stepId) => {
                if (!isReadOnly) deleteStep.mutate(stepId);
              }}
              onReorderSteps={(orderedStepIds) => {
                if (!isReadOnly) reorderStep.mutate({ orderedStepIds });
              }}
            />
          ) : selectedConnector ? (
            <EdgeInspectorPanel
              edge={selectedConnector}
              fromNode={selectedConnectorSource}
              toNode={selectedConnectorTarget}
              readOnly={isReadOnly}
              readOnlyLabel={isPublished ? 'Đã phát hành · Chỉ xem' : 'Chỉ xem'}
              conditions={edgeConditions.data ?? []}
              conditionsLoading={edgeConditions.isLoading}
              conditionLoadError={edgeConditions.isError}
              edgeForm={edgeForm}
              conditionInvalid={edgeSubmitAttempted && (!edgeForm.condition_type || !edgeConditionTypes.has(edgeForm.condition_type))}
              labelInvalid={edgeSubmitAttempted && edgeForm.label.trim().length > 80}
              priorityInvalid={edgeSubmitAttempted && !validEdgePriority(edgeForm.priority)}
              configInvalidKeys={edgeSubmitAttempted ? invalidEdgeConfigKeys : []}
              validationRequestId={edgeValidationRequestId}
              answerFieldOptions={answerFieldOptions}
              answerValueOptionsByFieldKey={answerValueOptionsByFieldKey}
              savePending={saveEdgeSettings.isPending}
              onChangeField={(field, value) => {
                if (!isReadOnly) setEdgeForm((current) => field === 'condition_type' ? { ...current, condition_type: value, condition_config: {} } : { ...current, [field]: value });
              }}
              onChangeConditionConfig={(field, value) => {
                if (!isReadOnly) setEdgeForm((current) => ({ ...current, condition_config: { ...current.condition_config, [field]: value } }));
              }}
              onSave={() => {
                if (!isReadOnly) submitEdgeSettings();
              }}
              onClose={() => setSelectedConnectorId('')}
            />
          ) : null
        }
      >
        {!selectedTemplateId ? (
          <div className="grid min-h-0 flex-1 place-items-center bg-slate-50 p-6">
            <EmptyState title="Chưa có template. Tạo template mới để bắt đầu." />
          </div>
        ) : template.isLoading ? (
          <div className="grid min-h-0 flex-1 place-items-center bg-slate-50 p-8">
            <LoadingSpinner />
          </div>
        ) : (
          <DiagramCanvas
            nodes={nodes}
            edges={rawEdges}
            selectedNodeId={selectedNodeId}
            editingMode={diagramEditingMode}
            positionResetToken={nodePositionResetToken}
            connectorMutationPending={connectorMutationPending}
            onSelectNode={(nodeId) => {
              setSelectedNodeId(nodeId);
              setSelectedConnectorId('');
            }}
            onSelectedConnectorChange={(connectorId) => {
              setSelectedConnectorId(connectorId);
              if (connectorId) {
                setSelectedNodeId('');
                setDetailsNodeId('');
              }
            }}
            onToggleNodeDetails={(nodeId) => {
              setSelectedNodeId(nodeId);
              setSelectedConnectorId('');
              setDetailsNodeId((current) => (current === nodeId ? '' : nodeId));
            }}
            onSaveNodePosition={(nodeId, position) => {
              if (diagramEditingMode !== 'view' && selectedTemplateId) {
                saveNodePosition.mutate({ templateId: selectedTemplateId, nodeId, position });
              }
            }}
            onCreateConnector={(input) => { if (diagramEditingMode === 'full') createEdge.mutate(input); }}
            onReconnectConnector={reconnectConnector}
            onReconnectConstraintViolation={() => toast.warn({
              title: 'Không thể đổi node liên kết',
              description: 'Chỉ được đổi cạnh bám của đầu connector trên chính node hiện tại.',
            })}
            onDeleteConnector={(connectorId) => { if (diagramEditingMode === 'full') deleteEdge.mutate(connectorId); }}
            onDeleteNode={(nodeId) => { if (diagramEditingMode === 'full') deleteNode.mutate(nodeId); }}
          />
        )}
      </PipelineBuilderShell>

      <Dialog
        open={renameDialogOpen}
        title="Đổi tên template"
        onClose={closeRenameDialog}
        size="sm"
        footer={
          <div className="flex justify-end gap-2">
            <button
              className="rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-white disabled:opacity-50"
              onClick={closeRenameDialog}
              disabled={updateTemplateName.isPending}
            >
              Hủy
            </button>
            <button
              className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50"
              onClick={submitRenameDialog}
              disabled={updateTemplateName.isPending}
            >
              Lưu
            </button>
          </div>
        }
      >
        <div ref={renameDialogContentRef} data-validation-key="template_name" data-validation-error={templateNameSubmitAttempted && !templateNameForm.trim() ? 'true' : undefined}>
        <label className="block text-sm font-semibold text-ink" htmlFor="template-name">
          Tên template *
        </label>
        <input
          id="template-name"
          name="template_name"
          className={clsx(
            'mt-2 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100',
            templateNameSubmitAttempted && !templateNameForm.trim() ? 'border-rose-300 bg-rose-50' : 'border-line',
          )}
          value={templateNameForm}
          onChange={(event) => setTemplateNameForm(event.target.value)}
          autoFocus
          aria-invalid={templateNameSubmitAttempted && !templateNameForm.trim()}
          aria-describedby={templateNameSubmitAttempted && !templateNameForm.trim() ? 'template-name-error' : undefined}
        />
        {templateNameSubmitAttempted && !templateNameForm.trim() ? (
          <p id="template-name-error" role="alert" className="mt-2 text-sm font-medium text-rose-700">Vui lòng nhập tên template.</p>
        ) : null}
        </div>
      </Dialog>

      <Dialog
        open={Boolean(deleteTemplateDialog)}
        title="Xóa template"
        onClose={() => setDeleteTemplateDialog(null)}
        size="sm"
        footer={
          <div className="flex justify-end gap-2">
            <button
              className="rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-white disabled:opacity-50"
              onClick={() => setDeleteTemplateDialog(null)}
              disabled={deleteTemplate.isPending}
            >
              Hủy
            </button>
            <button
              className="rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:opacity-50"
              onClick={() => {
                if (deleteTemplateDialog) deleteTemplate.mutate(deleteTemplateDialog);
              }}
              disabled={deleteTemplate.isPending}
            >
              Xóa
            </button>
          </div>
        }
      >
        <p className="text-sm leading-6 text-slate-600">Bạn muốn xóa template này khỏi danh sách?</p>
        <p className="mt-3 rounded-md border border-line bg-slate-50 px-3 py-2 text-sm font-semibold text-ink">
          {deleteTemplateDialog?.name}
        </p>
        <p className="mt-3 text-sm leading-6 text-slate-600">Các pipeline run lịch sử vẫn được giữ lại.</p>
      </Dialog>

      <WorkflowDefinitionImportDialog
        open={definitionImportOpen}
        templateScope="pipeline"
        allowedScopes={['pipeline', 'bundle']}
        onClose={() => setDefinitionImportOpen(false)}
        onSuccess={(result) => {
          setDefinitionImportOpen(false);
          invalidate(result.pipelines[0]?.template_id);
          const importedID = result.pipelines[0]?.template_id;
          if (importedID) {
            setSelectedTemplateId(importedID);
            navigate(`/pipeline-builder/${importedID}`);
          }
          toast.success(`Đã import ${result.pipeline_count} Pipeline và ${result.form_count} Form ở trạng thái nháp`);
        }}
        onError={(title, error) => toast.error(title, error)}
      />
      <WorkflowDefinitionExportDialog
        open={definitionExportOpen}
        scope="pipeline"
        selectedID={selectedTemplateId}
        selectedName={template.data?.name}
        onClose={() => setDefinitionExportOpen(false)}
        onSuccess={() => toast.success('Đã xuất định nghĩa Pipeline và dependency')}
        onError={(title, error) => toast.error(title, error)}
      />
    </>
  );
}
