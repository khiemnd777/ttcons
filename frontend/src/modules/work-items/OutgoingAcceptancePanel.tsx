import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Loader2, RefreshCw, RotateCcw, Save, Send, X } from 'lucide-react';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import type {
  AcceptanceRecord,
  EvidenceRecord,
  OutgoingAcceptanceResponse,
  WorkflowFormPDFExport,
} from '../../lib/types/domain';
import {
  AcceptanceEditor,
  acceptanceDraftFromRecord,
  acceptanceDraftPayload,
  createAcceptanceDraft,
  uploadAcceptanceDraftEvidence,
  type AcceptanceDraft,
  type AcceptancePayload,
} from '../acceptance/AcceptanceEditor';

export function OutgoingAcceptancePanel({
  workItemId,
  projectId,
  evidence,
}: {
  workItemId: string;
  projectId?: string;
  evidence: EvidenceRecord[];
}) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [draft, setDraft] = useState<AcceptanceDraft>(() => createAcceptanceDraft({ workItemId }));

  const acceptanceQuery = useQuery({
    queryKey: ['outgoing-acceptance', workItemId],
    queryFn: () => get<OutgoingAcceptanceResponse>(`/work-items/${workItemId}/outgoing-acceptance`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  const records = acceptanceQuery.data?.records ?? [];
  const draftRecord = records.find((record) => record.status === 'draft');
  const draftDetailQuery = useQuery({
    queryKey: ['acceptance', draftRecord?.id],
    queryFn: () => get<AcceptanceRecord>(`/acceptances/${draftRecord?.id}`),
    enabled: Boolean(draftRecord?.id),
    retry: false,
  });

  const form = acceptanceQuery.data?.form;
  const formInstance = form?.instance;
  const pdfExports = ((form?.pdf_exports ?? []) as WorkflowFormPDFExport[])
    .filter((record) => record.deletion_status !== 'pending');
  const signatures = form?.signatures ?? [];
  const evidenceOptions = useMemo(() => evidence.filter((item) => item.status !== 'rejected'), [evidence]);
  const submitted = records.filter((record) => record.status === 'submitted' || record.status === 'completed');

  useEffect(() => {
    if (draftDetailQuery.data) {
      setDraft(acceptanceDraftFromRecord(draftDetailQuery.data));
      return;
    }
    if (!draftRecord) {
      setDraft(createAcceptanceDraft({ workItemId, formInstanceId: formInstance?.id, pdfExportId: pdfExports[0]?.id, hasSignature: signatures.length > 0 }));
    }
  }, [draftDetailQuery.data, draftRecord?.id, formInstance?.id, pdfExports[0]?.id, signatures.length, workItemId]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['outgoing-acceptance', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    queryClient.invalidateQueries({ queryKey: ['acceptances', projectId] });
    if (draft.id) {
      queryClient.invalidateQueries({ queryKey: ['acceptance', draft.id] });
    }
    if (projectId) {
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
    }
  };

  function buildPayload(sourceDraft: AcceptanceDraft, requireComplete: boolean): AcceptancePayload | null {
    const { payload, invalid } = acceptanceDraftPayload({ ...sourceDraft, work_item_id: workItemId }, requireComplete);
    if (!projectId) invalid.unshift('Công trình');
    if (invalid.length) {
      toast.warn({ title: 'Dữ liệu nghiệm thu đầu ra chưa hợp lệ', description: invalid.slice(0, 8).join(', ') });
      return null;
    }
    return payload;
  }

  async function saveDraftRecord(requireComplete: boolean) {
    const initialPayload = buildPayload(draft, requireComplete);
    if (!initialPayload || !projectId) return null;
    const readyDraft = await uploadAcceptanceDraftEvidence({ ...draft, work_item_id: workItemId }, setDraft);
    const payload = buildPayload(readyDraft, requireComplete);
    if (!payload) return null;
    if (readyDraft.id) {
      return put<AcceptanceRecord>(`/acceptances/${readyDraft.id}`, payload);
    }
    return post<AcceptanceRecord>(`/projects/${projectId}/acceptances`, payload);
  }

  const saveDraft = useMutation({
    mutationFn: () => saveDraftRecord(false),
    onSuccess: (record) => {
      if (!record) return;
      setDraft(acceptanceDraftFromRecord(record));
      invalidate();
      toast.success('Đã lưu nháp nghiệm thu đầu ra');
    },
    onError: (error) => toast.error('Không lưu được nghiệm thu đầu ra', error),
  });

  const submitDraft = useMutation({
    mutationFn: async () => {
      const record = await saveDraftRecord(true);
      if (!record) return null;
      return post<AcceptanceRecord>(`/acceptances/${record.id}/submit`);
    },
    onSuccess: (record) => {
      if (!record) return;
      setDraft(createAcceptanceDraft({ workItemId, formInstanceId: formInstance?.id, pdfExportId: pdfExports[0]?.id, hasSignature: signatures.length > 0 }));
      invalidate();
      toast.success('Đã gửi nghiệm thu đầu ra');
    },
    onError: (error) => toast.error('Không gửi được nghiệm thu đầu ra', error),
  });

  const actionMutation = useMutation({
    mutationFn: ({ acceptanceId, action }: { acceptanceId: string; action: 'pass' | 'conditional-pass' | 'fail' }) =>
      post<AcceptanceRecord>(`/acceptances/${acceptanceId}/${action}`),
    onSuccess: (_record, variables) => {
      invalidate();
      toast.success(actionToastTitle(variables.action));
    },
    onError: (error) => toast.error('Không chốt được nghiệm thu đầu ra', error),
  });

  const actionPending = saveDraft.isPending || submitDraft.isPending || actionMutation.isPending || acceptanceQuery.isFetching || draftDetailQuery.isFetching;

  if (acceptanceQuery.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Nghiệm thu đầu ra</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (acceptanceQuery.error) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Nghiệm thu đầu ra</h3>
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm font-semibold text-amber-900">Không tải được dữ liệu nghiệm thu đầu ra</p>
            <button
              className="inline-flex items-center gap-2 rounded-md border border-amber-300 px-3 py-2 text-sm font-semibold text-amber-900 hover:bg-amber-100"
              onClick={() => acceptanceQuery.refetch()}
            >
              <RefreshCw className="h-4 w-4" />
              Tải lại
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (!acceptanceQuery.data?.available) return null;

  return (
    <section>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">Nghiệm thu đầu ra</h3>
          <p className="mt-1 text-xs text-slate-500">
            {submitted.length ? `${submitted.length} hồ sơ đã gửi/chốt` : 'Chưa có hồ sơ đã gửi'}
          </p>
        </div>
        <StatusBadge status={draft.id ? 'draft' : 'new'} />
      </div>

      <div className="space-y-4 rounded-md border border-line bg-white p-3">
        <AcceptanceEditor
          draft={draft}
          projectId={projectId}
          evidence={evidenceOptions}
          formInstance={formInstance?.id ? { id: formInstance.id, title: formInstance.title } : undefined}
          pdfExports={pdfExports}
          fixedWorkItem
          disabled={actionPending}
          onChange={setDraft}
        />
        <div className="flex flex-wrap justify-end gap-2">
          <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60" disabled={actionPending} onClick={() => saveDraft.mutate()}>
            {saveDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
            Lưu nháp
          </button>
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={actionPending} onClick={() => submitDraft.mutate()}>
            {submitDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
            Gửi nghiệm thu
          </button>
        </div>

        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink">Hồ sơ đã gửi/chốt</h4>
          {submitted.length ? (
            <div className="space-y-2">
              {submitted.map((record) => (
                <div key={record.id} className="grid gap-2 rounded-md border border-line px-3 py-2 text-sm xl:grid-cols-[1fr_auto_auto]">
                  <div className="min-w-0">
                    <p className="truncate font-semibold text-ink">
                      {record.acceptance_code ?? record.id} · {record.title}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      {record.acceptance_date ? dateText(record.acceptance_date) : record.type}
                      {record.submitted_at ? ` · Gửi ${dateText(record.submitted_at)}` : ''}
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <StatusBadge status={record.result ?? record.status} />
                    <StatusBadge status={record.signature_status ?? 'pending'} />
                    <StatusBadge status={record.document_status ?? 'pending'} />
                  </div>
                  {record.status === 'submitted' ? (
                    <div className="flex flex-wrap justify-end gap-2">
                      <button className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:opacity-60" disabled={actionPending} title="Đạt" onClick={() => actionMutation.mutate({ acceptanceId: String(record.id), action: 'pass' })}><Check className="h-4 w-4" /></button>
                      <button className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:opacity-60" disabled={actionPending} title="Đạt có điều kiện" onClick={() => actionMutation.mutate({ acceptanceId: String(record.id), action: 'conditional-pass' })}><RotateCcw className="h-4 w-4" /></button>
                      <button className="rounded-md border border-line p-1.5 hover:bg-slate-100 disabled:opacity-60" disabled={actionPending} title="Không đạt" onClick={() => actionMutation.mutate({ acceptanceId: String(record.id), action: 'fail' })}><X className="h-4 w-4" /></button>
                    </div>
                  ) : (
                    <span />
                  )}
                </div>
              ))}
            </div>
          ) : (
            <EmptyState title="Chưa có hồ sơ nghiệm thu đầu ra đã gửi" />
          )}
        </div>
      </div>
    </section>
  );
}

function actionToastTitle(action: string) {
  if (action === 'pass') return 'Đã chốt nghiệm thu đầu ra đạt';
  if (action === 'conditional-pass') return 'Đã chốt nghiệm thu đầu ra đạt có điều kiện';
  return 'Đã chốt nghiệm thu đầu ra không đạt';
}
