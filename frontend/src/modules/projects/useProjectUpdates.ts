import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ProjectUpdatesPage } from '../../lib/types/domain';
import { loadProjectUpdates, projectUpdateIDs } from './projectUpdates';

export function useProjectUpdates(ids: string[]) {
  const { user } = useAuth();
  const projectIDs = projectUpdateIDs(ids);
  return useQuery({
    queryKey: ['project-updates', user?.id, projectIDs],
    queryFn: () => loadProjectUpdates(projectIDs ? projectIDs.split(',') : [],
      (batchIDs) => get<ProjectUpdatesPage>(`/projects/updates-summary?${new URLSearchParams({ project_ids: batchIDs })}`)),
    enabled: Boolean(user?.id && projectIDs),
    retry: false,
    staleTime: 0,
    // Also reconciles expiry/time-based visibility when there is no write event.
    refetchInterval: 60_000,
  });
}
