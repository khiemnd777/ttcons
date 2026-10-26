import { useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { Dialog } from '../../components/Dialog';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useToast } from '../../components/ToastProvider';
import { get, put } from '../../lib/api/client';
import type { NotificationCatalogItem, NotificationPreference } from '../../lib/types/domain';

type CatalogQuery = Pick<UseQueryResult<NotificationCatalogItem[]>, 'data' | 'isPending' | 'isError' | 'isSuccess' | 'isFetching' | 'refetch'>;

export function NotificationPreferencesDialog({ open, catalog, onClose }: { open: boolean; catalog: CatalogQuery; onClose: () => void }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const preferences = useQuery({ queryKey: ['notification-preferences'], queryFn: () => get<NotificationPreference[]>('/me/notifications/preferences'), enabled: open, retry: false });
  const categories = useMemo(() => {
    const groups = new Map<string, { key: string; label: string; required: boolean }>();
    for (const event of catalog.data ?? []) {
      if (!event.enabled) continue;
      groups.set(event.category_key, { key: event.category_key, label: event.category_label, required: Boolean(groups.get(event.category_key)?.required || event.required_delivery) });
    }
    return Array.from(groups.values());
  }, [catalog.data]);
  const baseline = useMemo(() => Object.fromEntries(categories.map(category => {
    const preference = preferences.data?.find(item => item.category_key === category.key && item.event_key === '*');
    return [category.key, category.required || preference?.in_app_enabled !== false];
  })), [categories, preferences.data]);
  // Keep explicit user choices across a partial-save reconciliation. The refreshed
  // server baseline removes already-persisted choices from the next write set.
  const [choices, setChoices] = useState<Record<string, boolean>>({});
  const submitting = useRef(false);
  const loading = catalog.isPending || preferences.isPending;
  const failed = catalog.isError || preferences.isError;
  const fetching = catalog.isFetching || preferences.isFetching;
  const ready = catalog.isSuccess && preferences.isSuccess && !fetching;
  const changed = categories.filter(category => !category.required && choices[category.key] !== undefined && choices[category.key] !== baseline[category.key]);
  const canSave = ready && changed.length > 0;
  const empty = ready && categories.length === 0;
  const save = useMutation({
    mutationFn: async () => {
      if (!canSave) throw new Error('Tùy chọn chưa sẵn sàng hoặc chưa có thay đổi để lưu.');
      for (const category of changed) {
        await put('/me/notifications/preferences', {
          category_key: category.key, event_key: '*', in_app_enabled: choices[category.key],
          email_enabled: false, push_enabled: false,
          digest_mode: choices[category.key] ? 'immediate' : 'off', quiet_hours_json: {},
        });
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['notification-preferences'] });
      toast.success('Đã lưu tùy chọn thông báo.');
      onClose();
    },
    onError: async (error) => {
      toast.error('Chưa lưu được đầy đủ tùy chọn. Một số thay đổi có thể đã được lưu.', error);
      await preferences.refetch();
    },
    onSettled: () => { submitting.current = false; },
  });
  useEffect(() => {
    if (!open) {
      setChoices({});
      save.reset();
    }
  }, [open, save.reset]);
  const close = () => { if (!submitting.current) onClose(); };
  const retry = () => {
    if (catalog.isError) void catalog.refetch();
    if (preferences.isError) void preferences.refetch();
  };
  const errorText = catalog.isError && preferences.isError ? 'Không tải được dữ liệu tùy chọn thông báo.'
    : catalog.isError ? 'Không tải được danh mục thông báo.' : 'Không tải được tùy chọn đã lưu.';

  return (
    <Dialog open={open} title="Tùy chọn thông báo" description="Thông báo cần hành động hoặc bảo mật luôn được giữ trên App." onClose={close}
      footer={<div className="flex justify-end gap-2">
        <button type="button" disabled={save.isPending} className="min-h-11 rounded-md border border-line px-3 py-2 text-sm font-semibold disabled:opacity-50" onClick={close}>{empty ? 'Đóng' : 'Hủy'}</button>
        {!empty ? <button type="button" disabled={!canSave || save.isPending} className="min-h-11 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => {
          if (!canSave || submitting.current) return;
          submitting.current = true;
          save.mutate();
        }}>{save.isPending ? 'Đang lưu…' : 'Lưu tùy chọn'}</button> : null}
      </div>}>
      {loading ? <LoadingSpinner label="Đang tải tùy chọn thông báo…" /> : failed ? <div role="alert" className="space-y-2 text-sm text-rose-700">
        <p>{errorText}</p>
        <button type="button" disabled={fetching} className="min-h-11 rounded-md border border-rose-200 px-3 py-2 font-semibold disabled:opacity-50" onClick={retry}>{fetching ? 'Đang tải lại…' : 'Thử lại'}</button>
      </div> : empty ? <p className="text-sm text-slate-500">Chưa có nhóm thông báo để cấu hình.</p> : <div className="space-y-3">
        {save.isError ? <p role="alert" className="text-sm text-rose-700">Chưa lưu được đầy đủ tùy chọn. Một số thay đổi có thể đã được lưu.</p> : null}
        {categories.map(category => <label key={category.key} className="flex min-h-11 items-center justify-between gap-4 rounded-lg border border-line p-4">
          <span className="min-w-0 break-words"><strong className="block text-sm text-ink">{category.label}</strong>{category.required ? <span className="text-xs text-slate-500">Luôn bật</span> : null}</span>
          <input type="checkbox" className="h-4 w-4 shrink-0" checked={category.required || (choices[category.key] ?? baseline[category.key])} disabled={category.required || !ready || save.isPending} onChange={event => setChoices(current => ({ ...current, [category.key]: event.target.checked }))} />
        </label>)}
      </div>}
    </Dialog>
  );
}
