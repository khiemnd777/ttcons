import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';

export function SettingsPage() {
  const settings = useQuery({ queryKey: ['settings'], queryFn: () => get<Record<string, any>[]>('/settings') });
  return (
    <>
      <PageHeader title="Cấu hình" description="Thiết lập hệ thống cho điều kiện pipeline, payment gate và xử lý vật tư dư." />
      <DataTable rows={settings.data ?? []} columns={[
        { key: 'key', header: 'Key', render: (row) => <code className="rounded bg-slate-100 px-2 py-1 text-xs">{row.key}</code> },
        { key: 'description', header: 'Mô tả' },
        { key: 'value_json', header: 'Giá trị', render: (row) => <pre className="max-w-lg whitespace-pre-wrap text-xs">{JSON.stringify(row.value_json, null, 2)}</pre> },
      ]} />
    </>
  );
}
