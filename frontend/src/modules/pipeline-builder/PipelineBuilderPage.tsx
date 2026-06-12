import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import { del, get, post, put } from '../../lib/api/client';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ToastProvider';
import { Dialog } from '../../components/Dialog';
import type { PipelineEdgeConditionDefinition, PipelineNode, PipelineTemplate, PipelineTemplateEdge } from '../../lib/types/domain';
import { PipelineBuilderShell } from './components/PipelineBuilderShell';
import { TemplateSidebar } from './components/TemplateSidebar';
import { NodeLibraryPanel } from './components/NodeLibraryPanel';
import { NodeInspectorPanel } from './components/NodeInspectorPanel';
import { EdgeInspectorPanel } from './components/EdgeInspectorPanel';
import { DiagramCanvas } from './components/diagram/DiagramCanvas';
import { autoLayoutPosition, nextNodePosition } from './components/diagram/diagramConstants';
import type { NodePosition } from './components/diagram/diagramTypes';
import {
  buildCreateEdgePayload,
  buildUpdateEdgePayload,
  type ConnectorCreateInput,
  type ConnectorReconnectInput,
} from './components/diagram/edgePersistence';

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
  const [selectedTemplateId, setSelectedTemplateId] = useState(id ?? '');
  const [selectedNodeId, setSelectedNodeId] = useState('');
  const [selectedConnectorId, setSelectedConnectorId] = useState('');
  const [detailsNodeId, setDetailsNodeId] = useState('');
  const [templatesExpanded, setTemplatesExpanded] = useState(true);
  const [renameDialogOpen, setRenameDialogOpen] = useState(false);
  const [templateNameForm, setTemplateNameForm] = useState('');
  const [templateNameSubmitAttempted, setTemplateNameSubmitAttempted] = useState(false);
  const [deleteTemplateDialog, setDeleteTemplateDialog] = useState<PipelineTemplate | null>(null);
  const [stepDialog, setStepDialog] = useState<{ mode: 'add' | 'edit'; step?: Record<string, any> } | null>(null);
  const [stepForm, setStepForm] = useState({ name: '', step_type: 'task', responsible_role_id: 'PM' });
  const [stepSubmitAttempted, setStepSubmitAttempted] = useState(false);
  const [nodeForm, setNodeForm] = useState<Record<string, any>>({});
  const [edgeForm, setEdgeForm] = useState<EdgeForm>({ label: '', condition_type: 'always', priority: '100', condition_config: {} });
  const [edgeSubmitAttempted, setEdgeSubmitAttempted] = useState(false);

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
  const nodeById = useMemo(() => new Map(nodes.map((node) => [node.id, node])), [nodes]);
  const selectedNode = selectedNodeId ? nodeById.get(selectedNodeId) : undefined;
  const selectedConnector = selectedConnectorId ? rawEdges.find((edge) => String(edge.id) === selectedConnectorId) : undefined;
  const selectedConnectorSource = selectedConnector ? nodeById.get(String(selectedConnector.from_node_id)) : undefined;
  const selectedConnectorTarget = selectedConnector ? nodeById.get(String(selectedConnector.to_node_id)) : undefined;
  const detailsNode = detailsNodeId ? nodeById.get(detailsNodeId) : undefined;
  const edgeConditionTypes = useMemo(
    () => new Set((edgeConditions.data ?? []).map((condition) => condition.type)),
    [edgeConditions.data],
  );
  const permissionOptions = useMemo(
    () => (permissions.data ?? []).map((permission) => ({
      label: `${permission.key} - ${permission.name}`,
      value: String(permission.key),
    })),
    [permissions.data],
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
      name: detailsNode.name,
      description: detailsNode.description ?? '',
      type: detailsNode.type ?? 'custom',
      permissions: normalizePermissions(detailsNode),
    });
    closeStepDialog();
  }, [detailsNode?.id]);

  const validationIssues = useMemo(() => buildValidationIssues(nodes, rawEdges, edgeConditionTypes), [nodes, rawEdges, edgeConditionTypes]);

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
      toast.success('Đã clone template');
    },
    onError: (error) => toast.error('Không clone được template', error),
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
        name: nodeForm.name,
        description: nodeForm.description,
        type: nodeForm.type,
        permissions: Array.isArray(nodeForm.permissions) ? nodeForm.permissions : [],
      });
    },
    onSuccess: () => {
      invalidate();
      toast.success('Đã lưu node');
    },
    onError: (error) => toast.error('Không lưu được node', error),
  });

  const saveNodePosition = useMutation({
    mutationFn: ({ nodeId, position }: { nodeId: string; position: NodePosition }) =>
      put(`/pipeline-nodes/${nodeId}`, {
        position_x: Math.round(position.x),
        position_y: Math.round(position.y),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['pipeline-template', selectedTemplateId] });
    },
    onError: (error) => toast.error('Không lưu được vị trí node', error),
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
        name: stepForm.name.trim(),
        description: 'Checklist mẫu trong node',
        step_type: stepForm.step_type,
        required_role_ids: [stepForm.responsible_role_id].filter(Boolean),
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
        name: stepForm.name.trim(),
        step_type: stepForm.step_type,
        required_role_ids: [stepForm.responsible_role_id].filter(Boolean),
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
    mutationFn: async ({ stepId, direction }: { stepId: string; direction: 'up' | 'down' }) => {
      const steps = sortedStepsForNode(detailsNode);
      const currentIndex = steps.findIndex((step) => String(step.id) === stepId);
      const targetIndex = direction === 'up' ? currentIndex - 1 : currentIndex + 1;
      if (currentIndex < 0 || targetIndex < 0 || targetIndex >= steps.length) return [];

      const reorderedSteps = [...steps];
      const [currentStep] = reorderedSteps.splice(currentIndex, 1);
      reorderedSteps.splice(targetIndex, 0, currentStep);
      const updates = reorderedSteps
        .map((step, index) => ({ stepId: String(step.id), orderIndex: index + 1, previousOrderIndex: Number(step.order_index ?? 0) }))
        .filter((update) => update.orderIndex !== update.previousOrderIndex);

      await Promise.all(updates.map((update) => put(`/pipeline-steps/${update.stepId}`, { order_index: update.orderIndex })));
      return updates;
    },
    onSuccess: (updates) => {
      if (!updates.length) return;
      const updateById = new Map(updates.map((update) => [update.stepId, update.orderIndex]));
      queryClient.setQueryData<PipelineTemplate>(['pipeline-template', selectedTemplateId], (current) => {
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
      invalidate();
      toast.success('Đã sắp xếp checklist');
    },
    onError: (error) => toast.error('Không sắp xếp được checklist', error),
  });

  const openAddStepDialog = () => {
    setStepForm({ name: '', step_type: defaultStepTypeForNode(detailsNode?.type), responsible_role_id: defaultRoleForNode(detailsNode?.type) });
    setStepSubmitAttempted(false);
    setStepDialog({ mode: 'add' });
  };

  const openEditStepDialog = (step: Record<string, any>) => {
    setStepForm({
      name: String(step.name ?? ''),
      step_type: String(step.step_type ?? 'task'),
      responsible_role_id: Array.isArray(step.required_role_ids) && step.required_role_ids[0] ? String(step.required_role_ids[0]) : defaultRoleForNode(detailsNode?.type),
    });
    setStepSubmitAttempted(false);
    setStepDialog({ mode: 'edit', step });
  };

  const closeStepDialog = () => {
    setStepDialog(null);
    setStepForm({ name: '', step_type: 'task', responsible_role_id: 'PM' });
    setStepSubmitAttempted(false);
  };

  const submitStepDialog = () => {
    setStepSubmitAttempted(true);
    if (!stepForm.name.trim()) {
      toast.warn({
        title: 'Thiếu thông tin checklist',
        description: 'Tên checklist là bắt buộc',
      });
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

    if (invalidLabels.length) {
      toast.warn({
        title: 'Thiếu hoặc sai cấu hình connector',
        description: invalidLabels.join(', '),
      });
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
      const updates = nodes.map((node, index) => ({
        nodeId: node.id,
        position: autoLayoutPosition(index),
        orderIndex: index + 1,
      }));
      await Promise.all(
        updates.map((update) =>
          put(`/pipeline-nodes/${update.nodeId}`, {
            position_x: update.position.x,
            position_y: update.position.y,
            order_index: update.orderIndex,
          }),
        ),
      );
      return updates;
    },
    onSuccess: (updates) => {
      const updateById = new Map(updates.map((update) => [update.nodeId, update]));
      queryClient.setQueryData<PipelineTemplate>(['pipeline-template', selectedTemplateId], (current) => {
        if (!current?.nodes) return current;
        return {
          ...current,
          nodes: current.nodes.map((node) => {
            const update = updateById.get(node.id);
            if (!update) return node;
            return {
              ...node,
              position_x: update.position.x,
              position_y: update.position.y,
              order_index: update.orderIndex,
            };
          }),
        };
      });
      invalidate();
      toast.success('Đã tự sắp xếp node');
    },
    onError: (error) => toast.error('Không tự sắp xếp được node', error),
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

  const connectorMutationPending = createEdge.isPending || updateEdge.isPending || deleteEdge.isPending || saveEdgeSettings.isPending;
  const canDeleteSelection = Boolean(selectedConnector || selectedNode);
  const deleteSelectionLabel = selectedConnector ? 'Xóa connector' : 'Xóa node';
  const deleteSelectionPending = selectedConnector ? deleteEdge.isPending : deleteNode.isPending;
  const deleteSelection = () => {
    if (selectedConnector) {
      deleteEdge.mutate(String(selectedConnector.id));
      return;
    }
    if (selectedNodeId) deleteNode.mutate(selectedNodeId);
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
            showBackButton={false}
            createTemplatePending={createTemplate.isPending}
            deleteTemplatePendingId={deleteTemplate.isPending ? deleteTemplateDialog?.id : undefined}
            onBack={() => navigate('/dashboard')}
            onCreateTemplate={() => createTemplate.mutate()}
            onDeleteTemplate={setDeleteTemplateDialog}
            onToggleExpanded={() => setTemplatesExpanded((current) => !current)}
            onSelectTemplate={selectTemplate}
          >
            <NodeLibraryPanel
              presets={nodeLibrary}
              canAddNode={Boolean(selectedTemplateId)}
              addNodePending={addNode.isPending}
              onAddNode={(preset) => addNode.mutate(preset)}
            />
          </TemplateSidebar>
        }
        templateName={template.data?.name}
        templateStatus={template.data?.status}
        templateDescription={template.data?.description}
        selectedTemplateId={selectedTemplateId}
        canDeleteSelection={canDeleteSelection}
        deleteSelectionLabel={deleteSelectionLabel}
        nodeCount={nodes.length}
        validationIssueCount={validationIssues.length}
        autoLayoutPending={autoLayout.isPending}
        deleteSelectionPending={deleteSelectionPending}
        onRenameTemplate={openRenameDialog}
        onCloneTemplate={() => clone.mutate()}
        onPublishTemplate={() => publish.mutate()}
        onAutoLayout={() => autoLayout.mutate()}
        onValidatePipeline={validatePipeline}
        onDeleteSelection={deleteSelection}
        inspector={
          detailsNode ? (
            <NodeInspectorPanel
              node={detailsNode}
              nodeForm={nodeForm}
              permissionOptions={permissionOptions}
              permissionsLoading={permissions.isLoading}
              stepDialogMode={stepDialog?.mode ?? null}
              stepForm={stepForm}
              stepNameInvalid={stepSubmitAttempted && !stepForm.name.trim()}
              savePending={saveNode.isPending}
              addStepPending={addStep.isPending}
              updateStepPending={updateStep.isPending}
              deleteStepPending={deleteStep.isPending}
              reorderStepPending={reorderStep.isPending}
              onChangeField={(field, value) => setNodeForm((current) => ({ ...current, [field]: value }))}
              onChangeStepField={(field, value) => setStepForm((current) => ({ ...current, [field]: value }))}
              onSave={() => saveNode.mutate()}
              onClose={() => setDetailsNodeId('')}
              onOpenAddStep={openAddStepDialog}
              onOpenEditStep={openEditStepDialog}
              onCloseStepDialog={closeStepDialog}
              onSubmitStep={submitStepDialog}
              onDeleteStep={(stepId) => deleteStep.mutate(stepId)}
              onMoveStep={(stepId, direction) => reorderStep.mutate({ stepId, direction })}
            />
          ) : selectedConnector ? (
            <EdgeInspectorPanel
              edge={selectedConnector}
              fromNode={selectedConnectorSource}
              toNode={selectedConnectorTarget}
              conditions={edgeConditions.data ?? []}
              conditionsLoading={edgeConditions.isLoading}
              conditionLoadError={edgeConditions.isError}
              edgeForm={edgeForm}
              conditionInvalid={edgeSubmitAttempted && (!edgeForm.condition_type || !edgeConditionTypes.has(edgeForm.condition_type))}
              priorityInvalid={edgeSubmitAttempted && !validEdgePriority(edgeForm.priority)}
              savePending={saveEdgeSettings.isPending}
              onChangeField={(field, value) => setEdgeForm((current) => ({ ...current, [field]: value }))}
              onSave={submitEdgeSettings}
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
            onSaveNodePosition={(nodeId, position) => saveNodePosition.mutate({ nodeId, position })}
            onCreateConnector={(input) => createEdge.mutate(input)}
            onReconnectConnector={(input) => updateEdge.mutate(input)}
            onDeleteConnector={(connectorId) => deleteEdge.mutate(connectorId)}
            onDeleteNode={(nodeId) => deleteNode.mutate(nodeId)}
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
        <label className="block text-sm font-semibold text-ink" htmlFor="template-name">
          Tên template *
        </label>
        <input
          id="template-name"
          className={clsx(
            'mt-2 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100',
            templateNameSubmitAttempted && !templateNameForm.trim() ? 'border-rose-300 bg-rose-50' : 'border-line',
          )}
          value={templateNameForm}
          onChange={(event) => setTemplateNameForm(event.target.value)}
          autoFocus
        />
        {templateNameSubmitAttempted && !templateNameForm.trim() ? (
          <p className="mt-2 text-sm font-medium text-rose-700">Vui lòng nhập tên template.</p>
        ) : null}
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
    </>
  );
}

function isRecord(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}

function normalizePermissions(node: PipelineNode) {
  if (Array.isArray(node.permissions)) return node.permissions.map(String);
  if (node.required_permission) return [String(node.required_permission)];
  return [];
}

function sortedStepsForNode(node?: PipelineNode) {
  return [...(node?.steps ?? [])].sort((a: Record<string, any>, b: Record<string, any>) => {
    const aOrder = Number(a.order_index ?? 0);
    const bOrder = Number(b.order_index ?? 0);
    if (aOrder !== bOrder) return aOrder - bOrder;
    return String(a.created_at ?? a.id ?? '').localeCompare(String(b.created_at ?? b.id ?? ''), 'vi');
  });
}

function validEdgePriority(value: string) {
  const priority = Number(value);
  return Number.isInteger(priority) && priority >= 1 && priority <= 999;
}

function defaultStepsForType(type: string) {
  const stepsByType: Record<string, string[]> = {
    survey: ['Upload ảnh hiện trạng', 'Ghi nhận kích thước chính', 'Xác định rủi ro mặt bằng'],
    design: ['Upload bản vẽ', 'Review phương án thiết kế', 'Khách hàng xác nhận'],
    boq: ['Bóc tách khối lượng', 'Nhập đơn giá dự kiến', 'Kiểm tra ngân sách'],
    budget_approval: ['Kiểm tra ngân sách', 'Đối chiếu hạn mức', 'Duyệt ngân sách'],
    procurement: ['Sinh nhu cầu vật tư từ BOQ', 'Kiểm tra tồn kho', 'Tạo đề xuất mua vật tư'],
    warehouse_issue: ['Kiểm tra tồn kho', 'Lập phiếu xuất kho', 'Xác nhận vật tư ra công trình'],
    construction: ['Daily report', 'Ghi nhận vật tư sử dụng', 'Kiểm tra chất lượng'],
    acceptance: ['Checklist nghiệm thu', 'Upload ảnh minh chứng', 'Pass / Fail nghiệm thu'],
  };
  return stepsByType[type] ?? ['Checklist', 'Review', 'Approval'];
}

function defaultStepTypeForNode(type?: string) {
  if (type === 'procurement' || type === 'warehouse_issue') return 'material_check';
  if (type === 'acceptance') return 'acceptance_check';
  if (type === 'boq' || type === 'budget_approval') return 'budget_check';
  return 'task';
}

function defaultStepTypeForStep(nodeType: string, stepName: string) {
  const normalizedName = stepName.toLocaleLowerCase('vi-VN');
  if (normalizedName.includes('duyệt') || normalizedName.includes('xác nhận')) return 'approval';
  return defaultStepTypeForNode(nodeType);
}

function defaultRoleForNode(type?: string) {
  if (type === 'survey' || type === 'construction') return 'Giám sát';
  if (type === 'boq') return 'QS';
  if (type === 'budget_approval' || type === 'financial_closure') return 'Kế toán';
  if (type === 'procurement') return 'Mua hàng';
  if (type === 'warehouse_issue') return 'Thủ kho';
  if (type === 'acceptance') return 'QA/QC';
  if (type === 'rework') return 'Đội thi công';
  return 'PM';
}

function buildValidationIssues(nodes: PipelineNode[], edges: PipelineTemplateEdge[], conditionTypes: Set<string>) {
  const issues: string[] = [];
  if (!nodes.length) issues.push('Template chưa có node');
  nodes.forEach((node) => {
    if (!node.name?.trim()) issues.push('Có node thiếu tên');
  });
  if (nodes.length > 1 && !edges.length) issues.push('Chưa có cạnh được lưu; hiện chỉ có thứ tự node mặc định');

  const nodeIds = new Set(nodes.map((node) => node.id));
  edges.forEach((edge) => {
    if (!nodeIds.has(String(edge.from_node_id))) issues.push('Có cạnh trỏ từ node đã xóa');
    if (!nodeIds.has(String(edge.to_node_id))) issues.push('Có cạnh trỏ tới node đã xóa');
    if (String(edge.label ?? '').trim().length > 80) issues.push(`Connector ${edge.id} có nhãn quá dài`);
    if (!String(edge.condition_type ?? '').trim()) issues.push('Có connector thiếu điều kiện chuyển tiếp');
    if (conditionTypes.size && !conditionTypes.has(String(edge.condition_type ?? ''))) issues.push(`Connector ${edge.label || edge.id} dùng điều kiện chuyển tiếp không được hỗ trợ`);
    if (!validEdgePriority(String(edge.priority ?? 100))) issues.push(`Connector ${edge.label || edge.id} có priority không hợp lệ`);
  });
  return issues;
}
