import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { get, streamSSE, type StreamStatus } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { NotificationSummary } from '../../lib/types/domain';

const ActorRealtimeStatusContext = createContext<StreamStatus>('offline');

export function ActorRealtimeProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const actorID = user?.id ?? '';
  const [status, setStatus] = useState<StreamStatus>('offline');

  useEffect(() => {
    if (!actorID || user?.must_change_password) {
      setStatus('offline');
      return undefined;
    }
    const reconcileNotifications = async () => {
      const previous = queryClient.getQueryData<NotificationSummary>(['notification-summary', actorID]);
      const current = await queryClient.fetchQuery({
        queryKey: ['notification-summary', actorID],
        queryFn: () => get<NotificationSummary>('/me/notifications/summary'),
        staleTime: 0,
      }).catch(() => undefined);
      if (!current || previous?.actor_revision === current.actor_revision) return;
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['notifications', actorID] }),
        queryClient.invalidateQueries({ queryKey: ['notification-projects', actorID] }),
      ]);
    };
    return streamSSE('/me/activity-attention/stream', {
      onStatus: (status) => {
        setStatus(status);
        if (status !== 'connected') return;
        void reconcileNotifications();
      },
      onEvent: (message) => {
        if (message.event === 'notification_changed') {
          void queryClient.invalidateQueries({ queryKey: ['notification-summary', actorID] });
          void queryClient.invalidateQueries({ queryKey: ['notifications', actorID] });
          void queryClient.invalidateQueries({ queryKey: ['notification-projects', actorID] });
        }
        if (message.event === 'work_report_changed') {
          void queryClient.invalidateQueries({ queryKey: ['work-report-summary'] });
          void queryClient.invalidateQueries({ queryKey: ['work-reports'] });
          void queryClient.invalidateQueries({ queryKey: ['work-report-detail'] });
          void queryClient.invalidateQueries({ queryKey: ['my-work-navigation-summary', actorID] });
        }
        if (message.event === 'work_queue_changed' || message.event === 'work_item_changed') {
          void queryClient.invalidateQueries({ queryKey: ['my-work-navigation-summary', actorID] });
          void queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
          void queryClient.invalidateQueries({ queryKey: ['my-work-scope-summary'] });
          void queryClient.invalidateQueries({ queryKey: ['my-workflow-actions'] });
          void queryClient.invalidateQueries({ queryKey: ['my-project-startup-actions'] });
          void queryClient.invalidateQueries({ queryKey: ['project-startup-readiness'] });
        }
        if (message.event === 'attention_changed') {
          void queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
        }
      },
    });
  }, [actorID, queryClient, user?.must_change_password]);

  return <ActorRealtimeStatusContext.Provider value={status}>{children}</ActorRealtimeStatusContext.Provider>;
}

export function useActorRealtimeStatus() {
  return useContext(ActorRealtimeStatusContext);
}
