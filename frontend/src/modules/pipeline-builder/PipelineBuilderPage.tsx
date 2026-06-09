import { useEffect, useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, Copy, GitCommitHorizontal, Plus, Save, Send, Trash2, ArrowUp, ArrowDown } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { EmptyState } from '../../components/EmptyState';
import type { PipelineNode, PipelineTemplate } from '../../lib/types/domain';

const roleOptions = ['PM', 'QS', 'Giám sát', 'Kế toán', 'Thủ kho', 'QA/QC', 'Giám đốc', 'Chủ đầu tư', 'Mua hàng', 'Đội thi công'];
const nodeTypes = ['survey', 'design', 'boq', 'budget_approval', 'procurement', 'warehouse_issue', 'construction', 'acceptance', 'rework', 'replanning', 'handover', 'financial_closure', 'custom'];
const approvalRules = ['PM duyệt', 'PM + QS duyệt', 'PM + Kế toán duyệt', 'PM + QS + Kế toán duyệt', 'PM + Kế toán + Giám đốc duyệt', 'Chủ đầu tư xác nhận', 'QA/QC + Chủ đầu tư xác nhận'];

const presetNodes = [
  ['Khảo sát', 'survey', 'Giám sát', 'PM duyệt', ['Upload ảnh hiện trạng', 'Ghi nhận kích thước chính', 'Xác định rủi ro mặt bằng']],
  ['Thiết kế', 'design', 'PM', 'Chủ đầu tư xác nhận', ['Upload bản vẽ', 'Review phương án thiết kế', 'Khách hàng xác nhận']],
  ['BOQ / Dự toán', 'boq', 'QS', 'PM + QS + Kế toán duyệt', ['Bóc tách khối lượng', 'Nhập đơn giá dự kiến', 'Kiểm tra ngân sách']],
  ['Mua vật tư', 'procurement', 'Mua hàng', 'PM + Kế toán duyệt', ['Sinh nhu cầu vật tư từ BOQ', 'Kiểm tra tồn kho', 'Tạo đề xuất mua vật tư']],
  ['Thi công', 'construction', 'Giám sát', 'PM duyệt', ['Daily report', 'Ghi nhận vật tư sử dụng', 'Kiểm tra chất lượng']],
  ['Nghiệm thu', 'acceptance', 'QA/QC', 'QA/QC + Chủ đầu tư xác nhận', ['Checklist nghiệm thu', 'Upload ảnh minh chứng', 'Pass / Fail nghiệm thu']],
];

export function PipelineBuilderPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [selectedTemplateId, setSelectedTemplateId] = useState(id ?? '');
  const [selectedNodeId, setSelectedNodeId] = useState('');
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
    if (id) setSelectedTemplateId(id);
  }, [id]);

  const nodes = useMemo(() => (template.data?.nodes ?? []).sort((a, b) => Number(a.order_index ?? 0) - Number(b.order_index ?? 0)), [template.data]);
  const selectedNode = nodes.find((node) => node.id === selectedNodeId) ?? nodes[0];

  useEffect(() => {
    if (selectedNode && selectedNode.id !== selectedNodeId) setSelectedNodeId(selectedNode.id);
  }, [selectedNode, selectedNodeId]);

  useEffect(() => {
    if (selectedNode) {
      setNodeForm({
        name: selectedNode.name,
        description: selectedNode.description ?? '',
        type: selectedNode.type ?? 'custom',
        responsible_role_ids: Array.isArray(selectedNode.responsible_role_ids) ? selectedNode.responsible_role_ids[0] : 'PM',
        approval_rule: selectedNode.approval_rule ?? 'PM duyệt',
        fail_path: selectedNode.fail_path ?? 'previous',
        required_permission: selectedNode.required_permission ?? '',
        exit_conditions_text: selectedNode.exit_conditions?.text ?? selectedNode.exit_conditions?.client_payment_min_percent ? JSON.stringify(selectedNode.exit_conditions) : '',
      });
    }
  }, [selectedNode?.id]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['pipeline-templates'] });
    queryClient.invalidateQueries({ queryKey: ['pipeline-template', selectedTemplateId] });
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
          position_x: 120 + index * 240,
          position_y: 120,
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
      invalidate();
      setSelectedTemplateId(created.id);
      navigate(`/pipeline-builder/${created.id}`);
    },
  });

  const publish = useMutation({
    mutationFn: () => post(`/pipeline-templates/${selectedTemplateId}/publish`, {}),
    onSuccess: invalidate,
  });
  const clone = useMutation({
    mutationFn: () => post<PipelineTemplate>(`/pipeline-templates/${selectedTemplateId}/clone`, {}),
    onSuccess: (created) => {
      invalidate();
      setSelectedTemplateId(created.id);
      navigate(`/pipeline-builder/${created.id}`);
    },
  });
  const addNode = useMutation({
    mutationFn: () =>
      post(`/pipeline-templates/${selectedTemplateId}/nodes`, {
        name: 'Node mới',
        description: 'Mô tả node mới',
        type: 'custom',
        order_index: nodes.length + 1,
        position_x: 120 + nodes.length * 240,
        position_y: 120,
        responsible_role_ids: ['PM'],
        approval_rule: 'PM duyệt',
        fail_path: 'previous',
        exit_conditions: { text: 'Hoàn tất checklist và được duyệt' },
      }),
    onSuccess: invalidate,
  });
  const saveNode = useMutation({
    mutationFn: () =>
      put(`/pipeline-nodes/${selectedNodeId}`, {
        name: nodeForm.name,
        description: nodeForm.description,
        type: nodeForm.type,
        responsible_role_ids: [nodeForm.responsible_role_ids],
        approval_rule: nodeForm.approval_rule,
        fail_path: nodeForm.fail_path,
        required_permission: nodeForm.required_permission,
        exit_conditions: nodeForm.exit_conditions_text ? { text: nodeForm.exit_conditions_text } : {},
      }),
    onSuccess: invalidate,
  });
  const addStep = useMutation({
    mutationFn: () =>
      post(`/pipeline-nodes/${selectedNodeId}/steps`, {
        name: newStep || 'Checklist mới',
        description: 'Checklist mẫu trong node',
        step_type: 'task',
        order_index: (selectedNode?.steps?.length ?? 0) + 1,
      }),
    onSuccess: () => {
      setNewStep('');
      invalidate();
    },
  });
  const moveNode = useMutation({
    mutationFn: ({ node, orderIndex }: { node: PipelineNode; orderIndex: number }) => put(`/pipeline-nodes/${node.id}`, { order_index: orderIndex }),
    onSuccess: invalidate,
  });

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

      <div className="grid gap-4 xl:grid-cols-[280px_1fr_380px]">
        <aside className="rounded-lg border border-line bg-white p-3 shadow-panel">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-ink">Templates</h2>
            <Archive className="h-4 w-4 text-slate-400" />
          </div>
          <div className="space-y-2">
            {(templates.data ?? []).map((item) => (
              <button
                key={item.id}
                className={`w-full rounded-md border px-3 py-3 text-left hover:border-blue-300 hover:bg-blue-50 ${selectedTemplateId === item.id ? 'border-blue-300 bg-blue-50' : 'border-line'}`}
                onClick={() => {
                  setSelectedTemplateId(item.id);
                  navigate(`/pipeline-builder/${item.id}`);
                }}
              >
                <p className="text-sm font-semibold text-ink">{item.name}</p>
                <div className="mt-2 flex items-center justify-between">
                  <span className="text-xs text-slate-500">v{item.version} · {item.node_count ?? 0} node</span>
                  <StatusBadge status={item.status} />
                </div>
              </button>
            ))}
          </div>
        </aside>

        <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
          <div className="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 className="text-base font-semibold text-ink">{template.data?.name ?? 'Chưa chọn template'}</h2>
              <p className="text-sm text-slate-500">{template.data?.description}</p>
            </div>
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => addNode.mutate()} disabled={!selectedTemplateId}>
              <Plus className="h-4 w-4" />
              Node
            </button>
          </div>

          {!selectedTemplateId ? (
            <EmptyState title="Chưa có template. Tạo template mới để bắt đầu." />
          ) : template.isLoading ? (
            <LoadingSpinner />
          ) : (
            <div className="flex flex-wrap items-stretch gap-3">
              {nodes.map((node, index) => (
                <div className="flex items-center gap-3" key={node.id}>
                  <button
                    className={`w-56 rounded-lg border p-4 text-left transition hover:-translate-y-0.5 hover:shadow-panel ${selectedNodeId === node.id ? 'border-blue-400 bg-blue-50' : 'border-line bg-white'}`}
                    onClick={() => setSelectedNodeId(node.id)}
                  >
                    <div className="mb-3 grid h-8 w-8 place-items-center rounded-md bg-slate-100 text-xs font-black text-slate-700">{index + 1}</div>
                    <h3 className="text-sm font-semibold text-ink">{node.name}</h3>
                    <p className="mt-2 line-clamp-3 text-xs leading-5 text-slate-600">{node.description}</p>
                    <div className="mt-3 flex flex-wrap gap-1">
                      <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{Array.isArray(node.responsible_role_ids) ? node.responsible_role_ids.join(', ') : 'PM'}</span>
                      <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{node.steps?.length ?? 0} step</span>
                    </div>
                    <div className="mt-3 flex gap-1">
                      <button title="Đưa lên" className="rounded-md p-1 hover:bg-white" onClick={(event) => { event.stopPropagation(); if (index > 0) moveNode.mutate({ node, orderIndex: Number(nodes[index - 1].order_index) }); }}>
                        <ArrowUp className="h-4 w-4" />
                      </button>
                      <button title="Đưa xuống" className="rounded-md p-1 hover:bg-white" onClick={(event) => { event.stopPropagation(); moveNode.mutate({ node, orderIndex: Number(node.order_index) + 1 }); }}>
                        <ArrowDown className="h-4 w-4" />
                      </button>
                    </div>
                  </button>
                  {index < nodes.length - 1 ? <GitCommitHorizontal className="h-6 w-6 text-slate-300" /> : null}
                </div>
              ))}
            </div>
          )}

          <div className="mt-5 rounded-lg border border-line bg-slate-50 p-3">
            <h3 className="mb-2 text-sm font-semibold text-ink">Edge list</h3>
            <div className="grid gap-2 md:grid-cols-2">
              {(template.data?.edges ?? []).map((edge: Record<string, any>) => (
                <div key={edge.id} className="rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-600">
                  {edge.label ?? 'Hoàn tất'} · {edge.condition_type}
                </div>
              ))}
              {!template.data?.edges?.length ? <p className="text-sm text-slate-500">Các node được nối theo order_index khi start pipeline.</p> : null}
            </div>
          </div>
        </section>

        <aside className="rounded-lg border border-line bg-white p-4 shadow-panel">
          <div className="mb-4 flex items-center justify-between">
            <h2 className="text-base font-semibold text-ink">Node detail</h2>
            <button title="Lưu node" className="rounded-md bg-blue-600 p-2 text-white hover:bg-blue-700 disabled:opacity-50" disabled={!selectedNodeId} onClick={() => saveNode.mutate()}>
              <Save className="h-4 w-4" />
            </button>
          </div>
          {selectedNode ? (
            <div className="space-y-3">
              <Field label="Tên node" value={nodeForm.name} onChange={(value) => setNodeForm((current) => ({ ...current, name: value }))} />
              <Field label="Mô tả" value={nodeForm.description} onChange={(value) => setNodeForm((current) => ({ ...current, description: value }))} textarea />
              <Select label="Node type" value={nodeForm.type} options={nodeTypes} onChange={(value) => setNodeForm((current) => ({ ...current, type: value }))} />
              <Select label="Role phụ trách" value={nodeForm.responsible_role_ids} options={roleOptions} onChange={(value) => setNodeForm((current) => ({ ...current, responsible_role_ids: value }))} />
              <Select label="Approval rule" value={nodeForm.approval_rule} options={approvalRules} onChange={(value) => setNodeForm((current) => ({ ...current, approval_rule: value }))} />
              <Select label="Fail path" value={nodeForm.fail_path} options={['none', 'previous', 'rework', 'replanning']} onChange={(value) => setNodeForm((current) => ({ ...current, fail_path: value }))} />
              <Field label="Required permission" value={nodeForm.required_permission} onChange={(value) => setNodeForm((current) => ({ ...current, required_permission: value }))} />
              <Field label="Điều kiện hoàn tất" value={nodeForm.exit_conditions_text} onChange={(value) => setNodeForm((current) => ({ ...current, exit_conditions_text: value }))} textarea />

              <div className="border-t border-line pt-4">
                <h3 className="text-sm font-semibold text-ink">Checklist / Step</h3>
                <div className="mt-3 space-y-2">
                  {(selectedNode.steps ?? []).map((step: Record<string, any>, index: number) => (
                    <div key={step.id} className="flex items-start justify-between gap-2 rounded-md border border-line px-3 py-2">
                      <div>
                        <p className="text-sm font-medium text-ink">{index + 1}. {step.name}</p>
                        <p className="text-xs text-slate-500">{step.step_type}</p>
                      </div>
                      <Trash2 className="h-4 w-4 text-slate-300" />
                    </div>
                  ))}
                </div>
                <div className="mt-3 flex gap-2">
                  <input className="min-w-0 flex-1 rounded-md border border-line px-3 py-2 text-sm focus-ring" value={newStep} placeholder="Thêm checklist mẫu" onChange={(event) => setNewStep(event.target.value)} />
                  <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => addStep.mutate()}>
                    Thêm
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <EmptyState title="Chọn node để chỉnh cấu hình" />
          )}
        </aside>
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

function Select({ label, value, options, onChange }: { label: string; value?: string; options: string[]; onChange: (value: string) => void }) {
  return (
    <label className="block">
      <span className="text-sm font-semibold text-slate-700">{label}</span>
      <select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" value={value ?? ''} onChange={(event) => onChange(event.target.value)}>
        {options.map((option) => (
          <option key={option} value={option}>{option}</option>
        ))}
      </select>
    </label>
  );
}
