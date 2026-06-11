import { useEffect, useMemo, useRef, useState, type PointerEvent } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import {
  Archive,
  CheckCircle2,
  Copy,
  GitBranch,
  LayoutGrid,
  Link2,
  MousePointer2,
  PanelRightOpen,
  Plus,
  Save,
  Send,
  Trash2,
  Wand2,
  X,
} from 'lucide-react';
import { del, get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { EmptyState } from '../../components/EmptyState';
import { useToast } from '../../components/ToastProvider';
import type { PipelineNode, PipelineTemplate, PipelineTemplateEdge } from '../../lib/types/domain';
import { EnumChip } from '../../components/EnumChip';
import { enumOptions } from '../../lib/utils/status';

const roleOptions = ['PM', 'QS', 'Giám sát', 'Kế toán', 'Thủ kho', 'QA/QC', 'Giám đốc', 'Chủ đầu tư', 'Mua hàng', 'Đội thi công'];
const nodeTypes = ['survey', 'design', 'boq', 'budget_approval', 'procurement', 'warehouse_issue', 'construction', 'acceptance', 'rework', 'replanning', 'handover', 'financial_closure', 'custom'];
const approvalRules = ['PM duyệt', 'PM + QS duyệt', 'PM + Kế toán duyệt', 'PM + QS + Kế toán duyệt', 'PM + Kế toán + Giám đốc duyệt', 'Chủ đầu tư xác nhận', 'QA/QC + Chủ đầu tư xác nhận'];
const failPathOptions = ['none', 'previous', 'rework', 'replanning'];

const NODE_WIDTH = 236;
const NODE_HEIGHT = 136;
const CANVAS_WIDTH = 1320;
const CANVAS_HEIGHT = 720;

type NodePosition = { x: number; y: number };
type DragState = {
  nodeId: string;
  pointerId: number;
  offsetX: number;
  offsetY: number;
  startClientX: number;
  startClientY: number;
  lastX: number;
  lastY: number;
  moved: boolean;
};

type NodePreset = {
  type: string;
  name: string;
  description: string;
  role: string;
  approval: string;
};

type CanvasEdge = PipelineTemplateEdge & {
  fromName: string;
  toName: string;
  inferred: boolean;
  missingFrom: boolean;
  missingTo: boolean;
};

type SelectOption = string | { label: string; value: string };

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
  const canvasRef = useRef<HTMLDivElement | null>(null);
  const dragRef = useRef<DragState | null>(null);
  const [selectedTemplateId, setSelectedTemplateId] = useState(id ?? '');
  const [selectedNodeId, setSelectedNodeId] = useState('');
  const [connectSourceId, setConnectSourceId] = useState('');
  const [newStep, setNewStep] = useState('');
  const [nodeForm, setNodeForm] = useState<Record<string, any>>({});
  const [nodePositions, setNodePositions] = useState<Record<string, NodePosition>>({});

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
      setConnectSourceId('');
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
  const nodeIndexById = useMemo(() => new Map(nodes.map((node, index) => [node.id, index])), [nodes]);
  const selectedNode = selectedNodeId ? nodeById.get(selectedNodeId) : undefined;

  useEffect(() => {
    setNodePositions((current) => {
      const next: Record<string, NodePosition> = {};
      nodes.forEach((node, index) => {
        next[node.id] = current[node.id] ?? initialNodePosition(node, index);
      });
      return next;
    });
  }, [nodes]);

  useEffect(() => {
    if (selectedNodeId && !nodeById.has(selectedNodeId)) setSelectedNodeId('');
    if (connectSourceId && !nodeById.has(connectSourceId)) setConnectSourceId('');
  }, [connectSourceId, nodeById, selectedNodeId]);

  useEffect(() => {
    if (!selectedNode) {
      setNodeForm({});
      setNewStep('');
      return;
    }

    setNodeForm({
      name: selectedNode.name,
      description: selectedNode.description ?? '',
      type: selectedNode.type ?? 'custom',
      responsible_role_ids: Array.isArray(selectedNode.responsible_role_ids) ? selectedNode.responsible_role_ids[0] : 'PM',
      approval_rule: selectedNode.approval_rule ?? 'PM duyệt',
      fail_path: selectedNode.fail_path ?? 'previous',
      required_permission: selectedNode.required_permission ?? '',
      exit_conditions_text: formatExitConditions(selectedNode.exit_conditions),
      rework_enabled: Boolean(selectedNode.rework_enabled),
      replanning_enabled: Boolean(selectedNode.replanning_enabled),
    });
    setNewStep('');
  }, [selectedNode?.id]);

  const edgeRows = useMemo<CanvasEdge[]>(() => {
    if (rawEdges.length) {
      return [...rawEdges]
        .sort((left, right) => edgeNodeOrder(left, nodeIndexById, 'from_node_id') - edgeNodeOrder(right, nodeIndexById, 'from_node_id'))
        .map((edge, index) => {
          const fromNodeId = String(edge.from_node_id ?? '');
          const toNodeId = String(edge.to_node_id ?? '');
          return {
            ...edge,
            id: edge.id ?? `edge-${index}`,
            from_node_id: fromNodeId,
            to_node_id: toNodeId,
            fromName: nodeById.get(fromNodeId)?.name ?? 'Node đã xóa',
            toName: nodeById.get(toNodeId)?.name ?? 'Node đã xóa',
            condition_type: edge.condition_type ?? 'always',
            label: edge.label ?? 'Hoàn tất',
            inferred: false,
            missingFrom: !nodeById.has(fromNodeId),
            missingTo: !nodeById.has(toNodeId),
          };
        });
    }

    return nodes.slice(0, -1).map((node, index) => {
      const nextNode = nodes[index + 1];
      return {
        id: `order-${node.id}-${nextNode.id}`,
        from_node_id: node.id,
        to_node_id: nextNode.id,
        fromName: node.name,
        toName: nextNode.name,
        condition_type: 'order_index',
        label: 'Hoàn tất',
        inferred: true,
        missingFrom: false,
        missingTo: false,
      };
    });
  }, [nodeById, nodeIndexById, nodes, rawEdges]);

  const canvasSize = useMemo(() => {
    const maxX = Math.max(CANVAS_WIDTH, ...nodes.map((node, index) => nodePosition(node, index, nodePositions).x + NODE_WIDTH + 120));
    const maxY = Math.max(CANVAS_HEIGHT, ...nodes.map((node, index) => nodePosition(node, index, nodePositions).y + NODE_HEIGHT + 120));
    return { width: maxX, height: maxY };
  }, [nodePositions, nodes]);

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
      setConnectSourceId('');
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
      setConnectSourceId('');
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
      setConnectSourceId('');
      setNodePositions((current) => ({
        ...current,
        [created.id]: {
          x: Number(created.position_x ?? nextNodePosition(nodes.length).x),
          y: Number(created.position_y ?? nextNodePosition(nodes.length).y),
        },
      }));
      invalidate();
      toast.success('Đã thêm node');
    },
    onError: (error) => toast.error('Không thêm được node', error),
  });

  const saveNode = useMutation({
    mutationFn: () => {
      if (!selectedNodeId) throw new Error('Chưa chọn node');
      return put(`/pipeline-nodes/${selectedNodeId}`, {
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

  const addStep = useMutation({
    mutationFn: () => {
      if (!selectedNodeId) throw new Error('Chưa chọn node');
      return post(`/pipeline-nodes/${selectedNodeId}/steps`, {
        name: newStep || 'Checklist mới',
        description: 'Checklist mẫu trong node',
        step_type: 'task',
        order_index: (selectedNode?.steps?.length ?? 0) + 1,
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

  const createEdge = useMutation({
    mutationFn: ({ fromNodeId, toNodeId }: { fromNodeId: string; toNodeId: string }) =>
      post<PipelineTemplateEdge>(`/pipeline-templates/${selectedTemplateId}/edges`, {
        from_node_id: fromNodeId,
        to_node_id: toNodeId,
        condition_type: 'always',
        label: 'Hoàn tất',
      }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã nối node');
    },
    onError: (error) => toast.error('Không nối được node', error),
  });

  const deleteEdge = useMutation({
    mutationFn: (edgeId: string) => del(`/pipeline-edges/${edgeId}`),
    onSuccess: () => {
      invalidate();
      toast.success('Đã xóa cạnh');
    },
    onError: (error) => toast.error('Không xóa được cạnh', error),
  });

  const deleteNode = useMutation({
    mutationFn: (nodeId: string) => del(`/pipeline-nodes/${nodeId}`),
    onSuccess: (_, nodeId) => {
      if (selectedNodeId === nodeId) setSelectedNodeId('');
      if (connectSourceId === nodeId) setConnectSourceId('');
      setNodePositions((current) => {
        const next = { ...current };
        delete next[nodeId];
        return next;
      });
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
      setNodePositions((current) => {
        const next = { ...current };
        updates.forEach((update) => {
          next[update.nodeId] = update.position;
        });
        return next;
      });
      invalidate();
      toast.success('Đã tự sắp xếp node');
    },
    onError: (error) => toast.error('Không tự sắp xếp được node', error),
  });

  const selectTemplate = (item: PipelineTemplate) => {
    setSelectedTemplateId(item.id);
    setSelectedNodeId('');
    setConnectSourceId('');
    navigate(`/pipeline-builder/${item.id}`);
  };

  const startConnect = () => {
    if (!selectedNodeId) {
      toast.error('Chọn node nguồn trước');
      return;
    }
    setConnectSourceId(selectedNodeId);
    toast.info({ title: 'Đang nối node', description: 'Chọn node đích trên canvas để tạo mũi tên.' });
  };

  const activateNode = (nodeId: string) => {
    if (!connectSourceId) {
      setSelectedNodeId(nodeId);
      return;
    }

    if (connectSourceId === nodeId) {
      setConnectSourceId('');
      toast.info('Đã hủy nối node');
      return;
    }

    const duplicateEdge = rawEdges.some((edge) => String(edge.from_node_id) === connectSourceId && String(edge.to_node_id) === nodeId);
    if (duplicateEdge) {
      setSelectedNodeId(nodeId);
      setConnectSourceId('');
      toast.error('Cạnh này đã tồn tại');
      return;
    }

    createEdge.mutate({ fromNodeId: connectSourceId, toNodeId: nodeId });
    setConnectSourceId('');
  };

  const handleNodePointerDown = (event: PointerEvent<HTMLDivElement>, node: PipelineNode, index: number) => {
    if (event.button !== 0) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    event.preventDefault();
    event.stopPropagation();
    const rect = canvas.getBoundingClientRect();
    const position = nodePosition(node, index, nodePositions);
    event.currentTarget.setPointerCapture(event.pointerId);
    dragRef.current = {
      nodeId: node.id,
      pointerId: event.pointerId,
      offsetX: event.clientX - rect.left - position.x,
      offsetY: event.clientY - rect.top - position.y,
      startClientX: event.clientX,
      startClientY: event.clientY,
      lastX: position.x,
      lastY: position.y,
      moved: false,
    };
  };

  const handleNodePointerMove = (event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    const canvas = canvasRef.current;
    if (!drag || drag.pointerId !== event.pointerId || !canvas) return;
    const rect = canvas.getBoundingClientRect();
    const x = clamp(event.clientX - rect.left - drag.offsetX, 12, canvasSize.width - NODE_WIDTH - 12);
    const y = clamp(event.clientY - rect.top - drag.offsetY, 12, canvasSize.height - NODE_HEIGHT - 12);
    const moved = drag.moved || Math.hypot(event.clientX - drag.startClientX, event.clientY - drag.startClientY) > 4;
    dragRef.current = { ...drag, lastX: x, lastY: y, moved };
    if (moved) {
      setNodePositions((current) => ({ ...current, [drag.nodeId]: { x, y } }));
    }
  };

  const finishNodePointer = (event: PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) return;
    event.preventDefault();
    event.stopPropagation();
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    dragRef.current = null;
    if (drag.moved) {
      saveNodePosition.mutate({ nodeId: drag.nodeId, position: { x: drag.lastX, y: drag.lastY } });
      return;
    }
    if (connectSourceId) activateNode(drag.nodeId);
  };

  const clearSelection = (event: PointerEvent<HTMLDivElement>) => {
    if (event.target !== event.currentTarget) return;
    setSelectedNodeId('');
    setConnectSourceId('');
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

  if (templates.isLoading) return <LoadingSpinner />;

  return (
    <>
      <PageHeader
        title="Pipeline Builder"
        description="Thiết kế template workflow động gồm node, step, role phụ trách, approval rule, điều kiện chuyển bước, rework và replanning path."
        actions={
          <>
            <button title="Tạo template" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => createTemplate.mutate()}>
              <Plus className="h-4 w-4" />
              Template
            </button>
            <button title="Clone template" disabled={!selectedTemplateId} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={() => clone.mutate()}>
              <Copy className="h-4 w-4" />
              Clone
            </button>
            <button title="Publish template" disabled={!selectedTemplateId} className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:opacity-50" onClick={() => publish.mutate()}>
              <Send className="h-4 w-4" />
              Publish
            </button>
          </>
        }
      />

      <div className={clsx('grid gap-4', selectedNode ? 'xl:grid-cols-[300px_minmax(0,1fr)_380px]' : 'xl:grid-cols-[300px_minmax(0,1fr)]')}>
        <div className="space-y-4">
          <section className="rounded-lg border border-line bg-white p-3 shadow-panel">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-ink">Templates</h2>
              <Archive className="h-4 w-4 text-slate-400" />
            </div>
            <div className="space-y-2">
              {(templates.data ?? []).map((item) => (
                <button
                  key={item.id}
                  className={clsx(
                    'w-full rounded-md border px-3 py-3 text-left hover:border-blue-300 hover:bg-blue-50',
                    selectedTemplateId === item.id ? 'border-blue-300 bg-blue-50' : 'border-line',
                  )}
                  onClick={() => selectTemplate(item)}
                >
                  <p className="text-sm font-semibold text-ink">{item.name}</p>
                  <div className="mt-2 flex items-center justify-between gap-2">
                    <span className="text-xs text-slate-500">v{item.version} · {item.node_count ?? 0} node</span>
                    <StatusBadge status={item.status} />
                  </div>
                </button>
              ))}
            </div>
          </section>

          <section className="rounded-lg border border-line bg-white p-3 shadow-panel">
            <div className="mb-3 flex items-center justify-between">
              <h2 className="text-sm font-semibold text-ink">Thư viện node</h2>
              <LayoutGrid className="h-4 w-4 text-slate-400" />
            </div>
            <div className="space-y-2">
              {nodeLibrary.map((preset) => (
                <button
                  key={preset.type}
                  className="w-full rounded-md border border-line px-3 py-3 text-left hover:border-blue-300 hover:bg-blue-50 disabled:opacity-50"
                  disabled={!selectedTemplateId || addNode.isPending}
                  onClick={() => addNode.mutate(preset)}
                >
                  <p className="text-sm font-semibold text-ink">{preset.name}</p>
                  <p className="mt-1 line-clamp-2 text-xs leading-5 text-slate-500">{preset.description}</p>
                  <div className="mt-2 flex flex-wrap gap-1.5">
                    <EnumChip group="node_type" value={preset.type} />
                    <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{preset.role}</span>
                  </div>
                </button>
              ))}
            </div>
          </section>
        </div>

        <section className="min-w-0 overflow-hidden rounded-lg border border-line bg-white shadow-panel">
          <div className="border-b border-line p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h2 className="text-base font-semibold text-ink">{template.data?.name ?? 'Chưa chọn template'}</h2>
                  {template.data?.status ? <StatusBadge status={template.data.status} /> : null}
                  {connectSourceId ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">Đang nối từ {nodeById.get(connectSourceId)?.name}</span> : null}
                </div>
                <p className="mt-1 text-sm text-slate-500">{template.data?.description ?? 'Chọn template hoặc tạo template mới để bắt đầu.'}</p>
              </div>
              <div className="flex flex-wrap gap-2">
                <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={() => addNode.mutate(undefined)} disabled={!selectedTemplateId || addNode.isPending}>
                  <Plus className="h-4 w-4" />
                  Node
                </button>
                <button className={clsx('inline-flex items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold disabled:opacity-50', connectSourceId ? 'border-blue-300 bg-blue-50 text-blue-700' : 'border-line text-slate-700 hover:bg-slate-100')} onClick={connectSourceId ? () => setConnectSourceId('') : startConnect} disabled={!selectedNodeId}>
                  {connectSourceId ? <X className="h-4 w-4" /> : <Link2 className="h-4 w-4" />}
                  {connectSourceId ? 'Hủy nối' : 'Nối node'}
                </button>
                <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={() => autoLayout.mutate()} disabled={!nodes.length || autoLayout.isPending}>
                  <Wand2 className="h-4 w-4" />
                  Tự sắp xếp
                </button>
                <button className={clsx('inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold', validationIssues.length ? 'border border-amber-200 bg-amber-50 text-amber-800 hover:bg-amber-100' : 'border border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100')} onClick={validatePipeline} disabled={!selectedTemplateId}>
                  <CheckCircle2 className="h-4 w-4" />
                  Kiểm tra
                </button>
                <button className="inline-flex items-center gap-2 rounded-md border border-rose-200 px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:opacity-50" onClick={() => selectedNodeId && deleteNode.mutate(selectedNodeId)} disabled={!selectedNodeId || deleteNode.isPending}>
                  <Trash2 className="h-4 w-4" />
                  Xóa node
                </button>
              </div>
            </div>
          </div>

          {!selectedTemplateId ? (
            <div className="p-6">
              <EmptyState title="Chưa có template. Tạo template mới để bắt đầu." />
            </div>
          ) : template.isLoading ? (
            <div className="p-8">
              <LoadingSpinner />
            </div>
          ) : (
            <>
              <div className="relative min-h-[640px] overflow-auto bg-slate-50">
                <div
                  ref={canvasRef}
                  className="relative"
                  style={{
                    width: canvasSize.width,
                    height: canvasSize.height,
                    backgroundImage: 'radial-gradient(circle at 1px 1px, #cbd5e1 1px, transparent 0)',
                    backgroundSize: '24px 24px',
                  }}
                  onPointerDown={clearSelection}
                >
                  <svg className="pointer-events-none absolute inset-0 h-full w-full" width={canvasSize.width} height={canvasSize.height}>
                    <defs>
                      <marker id="pipeline-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
                        <path d="M 0 0 L 10 5 L 0 10 z" fill="#64748b" />
                      </marker>
                    </defs>
                    {edgeRows.map((edge) => {
                      const path = edgePath(edge, nodeById, nodeIndexById, nodePositions);
                      if (!path) return null;
                      return (
                        <path
                          key={edge.id}
                          d={path}
                          fill="none"
                          stroke={edge.inferred ? '#94a3b8' : '#64748b'}
                          strokeDasharray={edge.inferred ? '8 8' : undefined}
                          strokeWidth={edge.inferred ? 2 : 2.5}
                          markerEnd="url(#pipeline-arrow)"
                        />
                      );
                    })}
                  </svg>

                  {!nodes.length ? (
                    <div className="absolute left-1/2 top-28 w-[360px] -translate-x-1/2">
                      <EmptyState
                        title="Template chưa có node"
                        action={
                          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => addNode.mutate(undefined)}>
                            <Plus className="h-4 w-4" />
                            Thêm node đầu tiên
                          </button>
                        }
                      />
                    </div>
                  ) : null}

                  {nodes.map((node, index) => {
                    const position = nodePosition(node, index, nodePositions);
                    const roles = Array.isArray(node.responsible_role_ids) ? node.responsible_role_ids.join(', ') : 'PM';
                    const isSelected = selectedNodeId === node.id;
                    const isConnectSource = connectSourceId === node.id;
                    const isConnectTarget = Boolean(connectSourceId && connectSourceId !== node.id);

                    return (
                      <div
                        key={node.id}
                        role="button"
                        tabIndex={0}
                        className={clsx(
                          'absolute z-10 select-none rounded-lg border bg-white p-3 text-left shadow-sm transition hover:-translate-y-0.5 hover:shadow-panel',
                          'cursor-grab active:cursor-grabbing',
                          isSelected && 'border-blue-400 bg-blue-50 shadow-panel ring-4 ring-blue-100',
                          isConnectSource && 'border-blue-500 ring-4 ring-blue-100',
                          isConnectTarget && 'hover:border-emerald-400 hover:ring-4 hover:ring-emerald-100',
                          !isSelected && !isConnectSource && 'border-line',
                        )}
                        style={{ left: position.x, top: position.y, width: NODE_WIDTH, minHeight: NODE_HEIGHT }}
                        onPointerDown={(event) => handleNodePointerDown(event, node, index)}
                        onPointerMove={handleNodePointerMove}
                        onPointerUp={finishNodePointer}
                        onPointerCancel={finishNodePointer}
                        onKeyDown={(event) => {
                          if (connectSourceId && (event.key === 'Enter' || event.key === ' ')) {
                            event.preventDefault();
                            activateNode(node.id);
                          }
                        }}
                      >
                        <div className="mb-3 flex items-start justify-between gap-3">
                          <div className="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-slate-100 text-xs font-black text-slate-700">{index + 1}</div>
                          <div className="flex items-center gap-1">
                            <MousePointer2 className="h-4 w-4 shrink-0 text-slate-300" />
                            <button
                              title="Chi tiết node"
                              aria-label={`Chi tiết node ${node.name}`}
                              className="rounded-md p-1 text-slate-400 hover:bg-white hover:text-blue-600"
                              onPointerDown={(event) => event.stopPropagation()}
                              onClick={(event) => {
                                event.stopPropagation();
                                setConnectSourceId('');
                                setSelectedNodeId((current) => (current === node.id ? '' : node.id));
                              }}
                            >
                              <PanelRightOpen className="h-4 w-4" />
                            </button>
                          </div>
                        </div>
                        <h3 className="line-clamp-2 text-sm font-semibold text-ink">{node.name}</h3>
                        <p className="mt-2 line-clamp-3 text-xs leading-5 text-slate-600">{node.description}</p>
                        <div className="mt-3 flex flex-wrap gap-1">
                          <span className="max-w-full truncate rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{roles}</span>
                          <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{node.steps?.length ?? 0} step</span>
                          <EnumChip group="node_type" value={node.type} />
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>

              <div className="border-t border-line bg-white px-4 py-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <GitBranch className="h-4 w-4 text-slate-400" />
                    <h3 className="text-sm font-semibold text-ink">Luồng chuyển node</h3>
                    <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{edgeRows.length} cạnh</span>
                  </div>
                  <p className="text-xs text-slate-500">{rawEdges.length ? 'Đường nối đã lưu trong template.' : 'Đang hiển thị thứ tự node mặc định.'}</p>
                </div>
                <div className="mt-3 flex gap-2 overflow-x-auto pb-1">
                  {edgeRows.map((edge) => (
                    <div key={edge.id} className="flex min-w-[260px] items-center gap-2 rounded-md border border-line bg-slate-50 px-3 py-2 text-xs">
                      <span className={clsx('min-w-0 flex-1 truncate font-semibold', edge.missingFrom ? 'text-amber-700' : 'text-ink')} title={edge.fromName}>{edge.fromName}</span>
                      <span className="text-slate-400">→</span>
                      <span className={clsx('min-w-0 flex-1 truncate font-semibold', edge.missingTo ? 'text-amber-700' : 'text-ink')} title={edge.toName}>{edge.toName}</span>
                      {edge.inferred ? (
                        <span className="rounded-md bg-white px-2 py-1 font-semibold text-slate-500">mặc định</span>
                      ) : (
                        <EnumChip group="condition_type" value={edge.condition_type} />
                      )}
                      {!edge.inferred ? (
                        <button title="Xóa cạnh" className="rounded-md p-1 text-slate-400 hover:bg-rose-50 hover:text-rose-600" onClick={() => deleteEdge.mutate(edge.id)} disabled={deleteEdge.isPending}>
                          <Trash2 className="h-4 w-4" />
                        </button>
                      ) : null}
                    </div>
                  ))}
                  {!edgeRows.length ? <p className="text-sm text-slate-500">Cần ít nhất 2 node để có luồng chuyển.</p> : null}
                </div>
              </div>
            </>
          )}
        </section>

        {selectedNode ? (
          <aside className="rounded-lg border border-line bg-white p-4 shadow-panel">
            <div className="mb-4 flex items-center justify-between gap-3">
              <div>
                <h2 className="text-base font-semibold text-ink">Cấu hình Node</h2>
                <p className="mt-1 text-xs text-slate-500">{selectedNode.name}</p>
              </div>
              <div className="flex gap-2">
                <button title="Lưu node" className="rounded-md bg-blue-600 p-2 text-white hover:bg-blue-700 disabled:opacity-50" disabled={!selectedNodeId || saveNode.isPending} onClick={() => saveNode.mutate()}>
                  <Save className="h-4 w-4" />
                </button>
                <button title="Đóng cấu hình" className="rounded-md border border-line p-2 text-slate-500 hover:bg-slate-100" onClick={() => setSelectedNodeId('')}>
                  <X className="h-4 w-4" />
                </button>
              </div>
            </div>
            <div className="space-y-3">
              <Field label="Tên node" value={nodeForm.name} onChange={(value) => setNodeForm((current) => ({ ...current, name: value }))} />
              <Field label="Mô tả" value={nodeForm.description} onChange={(value) => setNodeForm((current) => ({ ...current, description: value }))} textarea />
              <Select label="Node type" value={nodeForm.type} options={enumOptions('node_type', nodeTypes)} onChange={(value) => setNodeForm((current) => ({ ...current, type: value }))} />
              <Select label="Role phụ trách" value={nodeForm.responsible_role_ids} options={roleOptions} onChange={(value) => setNodeForm((current) => ({ ...current, responsible_role_ids: value }))} />
              <Select label="Approval rule" value={nodeForm.approval_rule} options={approvalRules} onChange={(value) => setNodeForm((current) => ({ ...current, approval_rule: value }))} />
              <Select label="Fail path" value={nodeForm.fail_path} options={enumOptions('fail_path', failPathOptions)} onChange={(value) => setNodeForm((current) => ({ ...current, fail_path: value }))} />
              <Field label="Required permission" value={nodeForm.required_permission} onChange={(value) => setNodeForm((current) => ({ ...current, required_permission: value }))} />
              <Field label="Điều kiện hoàn tất" value={nodeForm.exit_conditions_text} onChange={(value) => setNodeForm((current) => ({ ...current, exit_conditions_text: value }))} textarea />
              <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
                <Checkbox label="Rework" checked={Boolean(nodeForm.rework_enabled)} onChange={(value) => setNodeForm((current) => ({ ...current, rework_enabled: value }))} />
                <Checkbox label="Replanning" checked={Boolean(nodeForm.replanning_enabled)} onChange={(value) => setNodeForm((current) => ({ ...current, replanning_enabled: value }))} />
              </div>

              <div className="border-t border-line pt-4">
                <h3 className="text-sm font-semibold text-ink">Checklist / Step</h3>
                <div className="mt-3 space-y-2">
                  {(selectedNode.steps ?? []).map((step: Record<string, any>, index: number) => (
                    <div key={step.id ?? `${selectedNode.id}-${index}`} className="flex items-start justify-between gap-2 rounded-md border border-line px-3 py-2">
                      <div className="min-w-0">
                        <p className="text-sm font-medium text-ink">{index + 1}. {step.name}</p>
                        <div className="mt-1"><EnumChip group="step_type" value={step.step_type} /></div>
                      </div>
                      {step.id ? (
                        <button title="Xóa checklist" className="rounded-md p-1 text-slate-300 hover:bg-rose-50 hover:text-rose-600" onClick={() => deleteStep.mutate(String(step.id))} disabled={deleteStep.isPending}>
                          <Trash2 className="h-4 w-4" />
                        </button>
                      ) : null}
                    </div>
                  ))}
                </div>
                <div className="mt-3 flex gap-2">
                  <input className="min-w-0 flex-1 rounded-md border border-line px-3 py-2 text-sm focus-ring" value={newStep} placeholder="Thêm checklist mẫu" onChange={(event) => setNewStep(event.target.value)} />
                  <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:opacity-50" onClick={() => addStep.mutate()} disabled={addStep.isPending}>
                    Thêm
                  </button>
                </div>
              </div>
            </div>
          </aside>
        ) : null}
      </div>
    </>
  );
}

function Field({ label, value, onChange, textarea }: { label: string; value?: string; onChange: (value: string) => void; textarea?: boolean }) {
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      {textarea ? (
        <textarea className="mt-1 min-h-20 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" value={value ?? ''} onChange={(event) => onChange(event.target.value)} />
      ) : (
        <input className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" value={value ?? ''} onChange={(event) => onChange(event.target.value)} />
      )}
    </label>
  );
}

function Select({ label, value, options, onChange }: { label: string; value?: string; options: SelectOption[]; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" value={value ?? ''} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => {
          const normalized = typeof option === 'string' ? { label: option, value: option } : option;
          return <option key={normalized.value} value={normalized.value}>{normalized.label}</option>;
        })}
      </select>
    </label>
  );
}

function Checkbox({ label, checked, onChange }: { label: string; checked: boolean; onChange: (value: boolean) => void }) {
  return (
    <label className="flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700">
      <input type="checkbox" className="h-4 w-4 rounded border-line text-blue-600 focus:ring-blue-500" checked={checked} onChange={(event) => onChange(event.target.checked)} />
      {label}
    </label>
  );
}

function initialNodePosition(node: PipelineNode, index: number): NodePosition {
  const x = Number(node.position_x);
  const y = Number(node.position_y);
  if (Number.isFinite(x) && Number.isFinite(y)) return { x, y };
  return autoLayoutPosition(index);
}

function nodePosition(node: PipelineNode, index: number, positions: Record<string, NodePosition>): NodePosition {
  return positions[node.id] ?? initialNodePosition(node, index);
}

function autoLayoutPosition(index: number): NodePosition {
  const column = index % 4;
  const row = Math.floor(index / 4);
  return { x: 72 + column * 292, y: 80 + row * 190 };
}

function nextNodePosition(index: number): NodePosition {
  const column = index % 4;
  const row = Math.floor(index / 4);
  return { x: 96 + column * 280, y: 112 + row * 180 };
}

function edgeNodeOrder(edge: PipelineTemplateEdge, nodeIndexById: Map<string, number>, key: 'from_node_id' | 'to_node_id') {
  return nodeIndexById.get(String(edge[key] ?? '')) ?? Number.MAX_SAFE_INTEGER;
}

function edgePath(edge: CanvasEdge, nodeById: Map<string, PipelineNode>, nodeIndexById: Map<string, number>, positions: Record<string, NodePosition>) {
  const from = nodeById.get(edge.from_node_id);
  const to = nodeById.get(edge.to_node_id);
  if (!from || !to) return '';
  const fromIndex = nodeIndexById.get(from.id) ?? 0;
  const toIndex = nodeIndexById.get(to.id) ?? 0;
  const fromPosition = nodePosition(from, fromIndex, positions);
  const toPosition = nodePosition(to, toIndex, positions);
  const x1 = fromPosition.x + NODE_WIDTH;
  const y1 = fromPosition.y + 62;
  const x2 = toPosition.x;
  const y2 = toPosition.y + 62;
  const curve = Math.max(80, Math.abs(x2 - x1) * 0.45);
  return `M ${x1} ${y1} C ${x1 + curve} ${y1}, ${x2 - curve} ${y2}, ${x2} ${y2}`;
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
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
