import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { Timeline } from '../../components/Timeline';
import { DataTable } from '../../components/DataTable';
import { EnumChip } from '../../components/EnumChip';
import { ProjectBackLink } from '../../components/ProjectBackLink';

export function AuditPage({ scoped = false }: { scoped?: boolean }) {
  const { id } = useParams();
  const path = scoped && id ? `/projects/${id}/audit-logs` : '/audit-logs';
  const logs = useQuery({ queryKey: ['audit', path], queryFn: () => get<Record<string, any>[]>(path) });
  return (
    <>
      <PageHeader
        title="Audit Log / Workflow Playback"
        description="Timeline trả lời ai tạo request, ai duyệt, reject vì sao, node nào bị kẹt, vật tư/công nợ phát sinh khi nào."
        actions={scoped && id ? <ProjectBackLink projectId={id} /> : undefined}
      />
      <div className="grid gap-4 xl:grid-cols-[420px_1fr]">
        <Timeline events={logs.data?.slice(0, 20) ?? []} />
        <DataTable rows={logs.data ?? []} columns={[
          { key: 'created_at', header: 'Thời gian' },
          { key: 'event_type', header: 'Event', render: (row) => <EnumChip group="event_type" value={row.event_type} /> },
          { key: 'actor_name', header: 'Actor' },
          { key: 'target_type', header: 'Target', render: (row) => <EnumChip group="target_type" value={row.target_type} /> },
          { key: 'description', header: 'Mô tả' },
        ]} />
      </div>
    </>
  );
}
