import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { MyWorkNavigationSummary, NotificationSummary, WorkReportSummary } from '../../lib/types/domain';

export function useShellIndicators(enabled = true) {
  const { user, hasPermission } = useAuth();
  const actorID = user?.id ?? '';
  const canViewWorkReports = hasPermission('work_report.view');

  const notifications = useQuery({
    queryKey: ['notification-summary', actorID],
    queryFn: () => get<NotificationSummary>('/me/notifications/summary'),
    enabled: enabled && Boolean(actorID),
    retry: false,
    staleTime: 30_000,
  });

  const workReports = useQuery({
    queryKey: ['work-report-summary', actorID, 'navigation'],
    queryFn: () => get<WorkReportSummary>('/work-reports/summary'),
    enabled: enabled && Boolean(actorID) && canViewWorkReports,
    retry: false,
    staleTime: 30_000,
  });

  const myWork = useQuery({
    queryKey: ['my-work-navigation-summary', actorID],
    queryFn: () => get<MyWorkNavigationSummary>('/me/work-items/navigation-summary'),
    enabled: enabled && Boolean(actorID),
    retry: false,
    staleTime: 30_000,
  });

  return { notifications, workReports, myWork };
}
