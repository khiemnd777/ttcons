import { useQuery } from '@tanstack/react-query';
import { get } from '../../lib/api/client';
import type { Partner, Project } from '../../lib/types/domain';
import type { FieldDef } from '../_shared/SimpleForm';
import { partnerOptions } from '../core-data/partnerUtils';

type PipelineTemplate = Record<string, any>;

export function useProjectFormCatalogs(enabled = true) {
  const templates = useQuery({
    queryKey: ['pipeline-templates'],
    queryFn: () => get<PipelineTemplate[]>('/pipeline-templates'),
    enabled,
  });
  const partners = useQuery({
    queryKey: ['partners', 'client', 'active'],
    queryFn: () => get<Partner[]>('/partners?status=active&type=client'),
    enabled,
  });

  return {
    templates: templates.data ?? [],
    partners: partners.data ?? [],
    isLoading: templates.isLoading || partners.isLoading,
    error: templates.error || partners.error,
    refetch: async () => {
      await Promise.all([templates.refetch(), partners.refetch()]);
    },
  };
}

export function buildProjectFormFields(
  record: Project | undefined,
  partners: Partner[],
  templates: PipelineTemplate[],
): FieldDef[] {
  return [
    { name: 'name', label: 'Tên công trình', defaultValue: record?.name ?? 'Công trình mới', required: true },
    { name: 'code', label: 'Mã công trình', defaultValue: record?.code ?? `PRJ-${Date.now().toString().slice(-5)}`, required: true },
    {
      name: 'partner_id',
      label: 'Chọn chủ đầu tư có sẵn',
      type: 'select',
      defaultValue: record?.partner_id ?? '',
      options: partnerOptions(partners, ['client']),
      helperText: 'Khi chọn, hệ thống lưu tên chủ đầu tư hiện tại vào công trình.',
      fullWidth: true,
    },
    {
      name: 'client_name',
      label: 'Khách hàng / Chủ đầu tư',
      defaultValue: record?.client_name ?? 'Khách hàng mới',
      required: true,
      hiddenWhen: (values) => Boolean(values.partner_id),
      fullWidth: true,
    },
    { name: 'address', label: 'Địa chỉ', defaultValue: record?.address ?? '', fullWidth: true },
    { name: 'planned_start_date', label: 'Ngày bắt đầu dự kiến', type: 'date', defaultValue: record?.planned_start_date?.slice?.(0, 10) ?? '' },
    { name: 'planned_end_date', label: 'Ngày kết thúc dự kiến', type: 'date', defaultValue: record?.planned_end_date?.slice?.(0, 10) ?? '' },
    { name: 'estimated_budget', label: 'Ngân sách dự kiến', type: 'currency', defaultValue: record?.estimated_budget ?? 300000000, min: 0 },
    { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'currency', defaultValue: record?.contract_amount ?? 450000000, min: 0 },
    {
      name: 'pipeline_template_id',
      label: 'Pipeline template',
      type: 'select',
      defaultValue: record?.pipeline_template_id ?? latestPublishedTemplateID(templates),
      options: templates
        .filter((template) => template.status === 'published')
        .map((template) => ({ label: `${template.name} · revision ${template.revision_no ?? template.version}`, value: template.id })),
      fullWidth: true,
    },
  ];
}

function latestPublishedTemplateID(templates: PipelineTemplate[]) {
  return [...templates]
    .filter((template) => template.status === 'published')
    .sort((left, right) => Number(right.revision_no ?? right.version ?? 0) - Number(left.revision_no ?? left.version ?? 0))[0]?.id ?? '';
}
