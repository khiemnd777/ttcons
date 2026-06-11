import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { del, get, post, put } from '../../lib/api/client';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ToastProvider';
import type { PipelineNode, PipelineTemplate, PipelineTemplateEdge } from '../../lib/types/domain';
import { PipelineBuilderShell } from './components/PipelineBuilderShell';
import { TemplateSidebar } from './components/TemplateSidebar';
import { NodeLibraryPanel } from './components/NodeLibraryPanel';
import { NodeInspectorPanel } from './components/NodeInspectorPanel';
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
  role: string;
  approval: string;
};

const nodeLibrary: NodePreset[] = [
  { type: 'survey', name: 'Khảo sát', description: 'Ghi nhận hiện trạng, kích thước và rủi ro ban đầu.', role: 'Giám sát', approval: 'PM duyệt' },
  { type: 'design', name: 'Thiết kế', description: 'Chuẩn bị bản vẽ, phương án thiết kế và xác nhận với khách hàng.', role: 'PM', approval: 'Chủ đầu tư xác nhận' },
  { type: 'boq', name: 'BOQ / Dự toán', description: 'QS bóc tách khối lượng và kiểm tra ngân sách.', role: 'QS', approval: 'PM + QS + Kế toán duyệt' },
  { type: 'budget_approval', name: 'Duyệt ngân sách', description: 'Kiểm tra ngân sách và duyệt các khoản vượt hạn mức.', role: 'Kế toán', approval: 'PM + Kế toán + Giám đốc duyệt' },
  { type: 'procurement', name: 'Mua vật tư', description: 'Tạo yêu cầu mua, duyệt mua và đặt hàng.', role: 'Mua hàng', approval: 'PM + Kế toán duyệt' },
  { type: 'warehouse_issue', name: 'Xuất kho', description: 'Kiểm tra tồn kho và xuất vật tư ra công trình.', role: 'Thủ kho', approval: 'PM duyệt' },
  { type: 'construction', name: 'Thi công', description: 'Daily report, issue, vật tư sử dụng và tiến độ.', role: 'Giám sát', approval: 'PM duyệt' },
  { type: 'acceptance', name: 'Nghiệm thu', description: 'Checklist nghiệm thu, pass/fail và punch list.', role: 'QA/QC', approval: 'QA/QC + Chủ đầu tư xác nhận' },
  { type: 'rework', name: 'Rework', description: 'Sửa lỗi nghiệm thu, kiểm tra lại và đóng punch list.', role: 'Đội thi công', approval: 'PM duyệt' },
  { type: 'handover', name: 'Bàn giao', description: 'Closure checker, biên bản bàn giao và chứng từ.', role: 'PM', approval: 'Chủ đầu tư xác nhận' },
];

const presetNodes = nodeLibrary.slice(0, 8).map((node) => [node.name, node.type, node.role, node.approval, defaultStepsForType(node.type)] as const);

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
  const [newStep, setNewStep] = useState('');
  const [nodeForm, setNodeForm] = useState<Record<string, any>>({});

  const templates = useQuery({ queryKey: ['pipeline-templates'], queryFn: () => get<PipelineTemplate[]>('/pipeline-templates') });
  const template = useQuery({
    queryKey: ['pipeline-template', selectedTemplateId],
    queryFn: () => get<PipelineTemplate>(`/pipeline-templates/${selectedTemplateId}`),
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
  const detailsNode = detailsNodeId ? nodeById.get(detailsNodeId) : undefined;

  useEffect(() => {
    if (selectedNodeId && !nodeById.has(selectedNodeId)) setSelectedNodeId('');
    if (detailsNodeId && !nodeById.has(detailsNodeId)) setDetailsNodeId('');
  }, [detailsNodeId, nodeById, selectedNodeId]);

  useEffect(() => {
    if (selectedConnectorId && !rawEdges.some((edge) => String(edge.id) === selectedConnectorId)) setSelectedConnectorId('');
  }, [rawEdges, selectedConnectorId]);

  useEffect(() => {
    if (!detailsNode) {
      setNodeForm({});
      setNewStep('');
      return;
    }

    setNodeForm({
      name: detailsNode.name,
      description: detailsNode.description ?? '',
      type: detailsNode.type ?? 'custom',
      responsible_role_ids: Array.isArray(detailsNode.responsible_role_ids) ? detailsNode.responsible_role_ids[0] : 'PM',
      approval_rule: detailsNode.approval_rule ?? 'PM duyệt',
      fail_path: detailsNode.fail_path ?? 'previous',
      required_permission: detailsNode.required_permission ?? '',
      exit_conditions_text: formatExitConditions(detailsNode.exit_conditions),
      rework_enabled: Boolean(detailsNode.rework_enabled),
      replanning_enabled: Boolean(detailsNode.replanning_enabled),
    });
    setNewStep('');
  }, [detailsNode?.id]);

  const validationIssues = useMemo(() => buildValidationIssues(nodes, rawEdges), [nodes, rawEdges]);

  const invalidate = (templateId = selectedTemplateId) => {
    queryClient.invalidateQueries({ queryKey: ['pipeline-templates'] });
    if (templateId) queryClient.invalidateQueries({ queryKey: ['pipeline-template', templateId] });
  };

  const createTemplate = useMutation({
    mutationFn: () =>
      post<PipelineTemplate>('/pipeline-templates', {
        name: 'Template hoàn thiện nội thất mới',
        description: 'Template tạo từ Pipeline Builder',
        project_type: 'Hoàn thiện nội thất',
        status: 'draft',
        nodes: presetNodes.map(([name, type, role, approval, steps], index) => ({
          name,
          description: `Node ${name} trong workflow công trình.`,
          type,
          order_index: index + 1,
          position_x: autoLayoutPosition(index).x,
          position_y: autoLayoutPosition(index).y,
          responsible_role_ids: [role],
          approval_rule: approval,
          fail_path: type === 'acceptance' ? 'rework' : 'previous',
          rework_enabled: type === 'acceptance' || type === 'construction',
          replanning_enabled: ['design', 'boq', 'construction'].includes(String(type)),
          exit_conditions: { text: `Hoàn tất ${name}` },
          steps: steps as string[],
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
        responsible_role_ids: [preset?.role ?? 'PM'],
        approval_rule: preset?.approval ?? 'PM duyệt',
        fail_path: type === 'acceptance' ? 'rework' : 'previous',
        rework_enabled: type === 'acceptance' || type === 'construction' || type === 'rework',
        replanning_enabled: ['design', 'boq', 'construction', 'replanning'].includes(type),
        exit_conditions: { text: 'Hoàn tất checklist và được duyệt' },
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
        responsible_role_ids: [nodeForm.responsible_role_ids].filter(Boolean),
        approval_rule: nodeForm.approval_rule,
        fail_path: nodeForm.fail_path,
        required_permission: nodeForm.required_permission,
        exit_conditions: parseExitConditions(nodeForm.exit_conditions_text),
        rework_enabled: Boolean(nodeForm.rework_enabled),
        replanning_enabled: Boolean(nodeForm.replanning_enabled),
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
      return post(`/pipeline-nodes/${detailsNodeId}/steps`, {
        name: newStep || 'Checklist mới',
        description: 'Checklist mẫu trong node',
        step_type: 'task',
        order_index: (detailsNode?.steps?.length ?? 0) + 1,
      });
    },
    onSuccess: () => {
      setNewStep('');
      invalidate();
      toast.success('Đã thêm checklist');
    },
    onError: (error) => toast.error('Không thêm được checklist', error),
  });

  const deleteStep = useMutation({
    mutationFn: (stepId: string) => del(`/pipeline-steps/${stepId}`),
    onSuccess: () => {
      invalidate();
      toast.success('Đã xóa checklist');
    },
    onError: (error) => toast.error('Không xóa được checklist', error),
  });

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

  const connectorMutationPending = createEdge.isPending || updateEdge.isPending || deleteEdge.isPending;
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
    <PipelineBuilderShell
      sidebar={
        <TemplateSidebar
          templates={templates.data ?? []}
          selectedTemplateId={selectedTemplateId}
          expanded={templatesExpanded}
          onBack={() => navigate('/dashboard')}
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
      onCreateTemplate={() => createTemplate.mutate()}
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
            newStep={newStep}
            savePending={saveNode.isPending}
            addStepPending={addStep.isPending}
            deleteStepPending={deleteStep.isPending}
            onChangeField={(field, value) => setNodeForm((current) => ({ ...current, [field]: value }))}
            onChangeNewStep={setNewStep}
            onSave={() => saveNode.mutate()}
            onClose={() => setDetailsNodeId('')}
            onAddStep={() => addStep.mutate()}
            onDeleteStep={(stepId) => deleteStep.mutate(stepId)}
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
          onSelectedConnectorChange={setSelectedConnectorId}
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
  );
}

function formatExitConditions(value: unknown) {
  if (!value || typeof value !== 'object') return '';
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record);
  if (keys.length === 1 && typeof record.text === 'string') return record.text;
  if (!keys.length) return '';
  return JSON.stringify(record, null, 2);
}

function parseExitConditions(value?: string) {
  const trimmed = value?.trim();
  if (!trimmed) return {};
  if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
    try {
      return JSON.parse(trimmed);
    } catch {
      return { text: trimmed };
    }
  }
  return { text: trimmed };
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

function buildValidationIssues(nodes: PipelineNode[], edges: PipelineTemplateEdge[]) {
  const issues: string[] = [];
  if (!nodes.length) issues.push('Template chưa có node');
  nodes.forEach((node) => {
    if (!node.name?.trim()) issues.push('Có node thiếu tên');
    if (!Array.isArray(node.responsible_role_ids) || !node.responsible_role_ids.length) issues.push(`${node.name || 'Node'} thiếu role phụ trách`);
    if (!node.approval_rule) issues.push(`${node.name || 'Node'} thiếu approval rule`);
  });
  if (nodes.length > 1 && !edges.length) issues.push('Chưa có cạnh được lưu; hiện chỉ có thứ tự node mặc định');

  const nodeIds = new Set(nodes.map((node) => node.id));
  edges.forEach((edge) => {
    if (!nodeIds.has(String(edge.from_node_id))) issues.push('Có cạnh trỏ từ node đã xóa');
    if (!nodeIds.has(String(edge.to_node_id))) issues.push('Có cạnh trỏ tới node đã xóa');
  });
  return issues;
}
