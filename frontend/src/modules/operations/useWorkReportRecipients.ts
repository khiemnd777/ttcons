import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';

export type WorkReportRecipient = {
  id: string;
  full_name: string;
  position?: string;
  is_recommended?: boolean;
  recommendation_reason?: 'project_pm' | 'only_eligible_recipient' | '';
};

export function useWorkReportRecipients(projectId: string, enabled: boolean) {
  const query = useQuery({
    queryKey: ['work-report-recipients', projectId],
    queryFn: () => get<WorkReportRecipient[]>(`/work-reports/recipients?project_id=${projectId}`),
    enabled: enabled && Boolean(projectId),
    retry: false,
  });
  const recommendedRecipient = useMemo(
    () => query.data?.find((recipient) => recipient.is_recommended) ?? (query.data?.length === 1 ? query.data[0] : undefined),
    [query.data],
  );

  return { ...query, recommendedRecipient };
}
