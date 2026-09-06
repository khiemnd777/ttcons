import { useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { ApiError, get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { canOpenRecord, isRecordKind, loadSourceRecord, RecordUnavailable, type RecordKind, type RecordTarget } from './recordNavigation';

/** Apply an exact authorized source once per URL identity, independent of list filters/paging. */
export function useRecordDeepLink({ kind, projectId, onOpen }: { kind: RecordKind; projectId?: string; onOpen: (row: any) => void }) {
  const [params, setParams] = useSearchParams();
  const { user, hasPermission } = useAuth();
  const requested = params.get('record');
  const requestedKind = params.get('record_type') || kind;
  const active = requested !== null && requestedKind === kind;
  const target: RecordTarget = { kind, id: requested ?? '', projectId: projectId || params.get('project_id') || undefined };
  const key = `${user?.id}:${kind}:${target.projectId ?? ''}:${requested}`;
  const applied = useRef('');
  const callback = useRef(onOpen);
  callback.current = onOpen;
  const query = useQuery({
    queryKey: ['record-link', user?.id, kind, target.projectId, requested],
    queryFn: () => {
      if (!canOpenRecord(target, hasPermission)) throw new RecordUnavailable();
      return loadSourceRecord(target, get);
    },
    enabled: active && Boolean(user), staleTime: 0, gcTime: 0, retry: false, refetchOnWindowFocus: false,
  });
  useEffect(() => {
    if (!active) { applied.current = ''; return; }
    if (query.data && !query.isFetching && !query.error && applied.current !== key) {
      applied.current = key;
      callback.current(query.data);
    }
  }, [active, key, query.data, query.isFetching, query.error]);
  const dismiss = () => setParams(current => {
    const next = new URLSearchParams(current); next.delete('record'); next.delete('record_type'); return next;
  }, { replace: true });
  const error = active ? query.error : null;
  const unavailable = error instanceof RecordUnavailable || error instanceof ApiError && [401, 403, 404, 410].includes(error.status);
  const companion: Partial<Record<RecordKind, RecordKind>> = { boq_item: 'supplier_quotation', supplier_quotation: 'boq_item', rework: 'replanning', replanning: 'rework' };
  const unknownKind = requested !== null && (!isRecordKind(requestedKind) || requestedKind !== kind && requestedKind !== companion[kind])
    && !['supplier_quotation', 'replanning'].includes(kind);
  const feedback = <Dialog manageFocus open={unknownKind || active && (query.isPending || query.isFetching || Boolean(error))} title="Mở hồ sơ" onClose={dismiss} size="md">
    {error || unknownKind ? <div role="alert" className="space-y-3 text-sm text-slate-700"><p>{unavailable || unknownKind ? 'Không tìm thấy hồ sơ hoặc anh không có quyền truy cập.' : 'Không tải được hồ sơ.'}</p>{!unavailable && !unknownKind ? <button type="button" className="font-semibold text-blue-700" onClick={() => void query.refetch()}>Thử lại</button> : null}</div> : <p role="status" className="py-5 text-sm text-slate-500">Đang tải hồ sơ…</p>}
  </Dialog>;
  return { active, feedback, dismiss, data: active ? query.data : undefined, error };
}
