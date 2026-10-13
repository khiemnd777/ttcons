import { downloadBlob } from '../../lib/api/client';

export function nativeReportExcelPath(source: { dailyReportId: string } | { workReportId: string; revisionNo: number }): string {
  if ('dailyReportId' in source) {
    if (!source.dailyReportId) throw new Error('Chưa có báo cáo đã lưu.');
    return `/daily-reports/${encodeURIComponent(source.dailyReportId)}/export.xlsx`;
  }
  if (!source.workReportId || !Number.isSafeInteger(source.revisionNo) || source.revisionNo < 1) throw new Error('Chưa có lần gửi hợp lệ.');
  return `/work-reports/${encodeURIComponent(source.workReportId)}/revisions/${source.revisionNo}/export.xlsx`;
}

export function nativeReportListExcelPath(base: '/work-reports' | `/projects/${string}/daily-reports`, appliedQuery: string): string {
  const params = new URLSearchParams(appliedQuery);
  const allowed = base === '/work-reports' ? ['scope', 'project_id', 'status', 'date_from', 'date_to', 'search', 'supervisor_id', 'read_status'] : ['date_from', 'date_to', 'status', 'search'];
  for (const key of [...params.keys()]) if (!allowed.includes(key)) params.delete(key);
  return `${base}/export.xlsx${params.size ? `?${params}` : ''}`;
}

export async function downloadNativeReportExcel(path: string, filename = 'bao-cao-BM-GS-02.xlsx'): Promise<void> {
  const blob = await downloadBlob(path);
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  try { link.href = url; link.download = filename; link.click(); }
  finally { URL.revokeObjectURL(url); }
}

export function nativeActionListExcelPath(projectId: string, filters: { queue: 'open' | 'waiting_confirmation' | 'done' | 'all'; search: string; assigneeId: string; priority: string; impactScope: string }): string {
  const params = new URLSearchParams({ queue: filters.queue });
  if (filters.search) params.set('search', filters.search);
  if (filters.assigneeId) params.set('assignee_id', filters.assigneeId);
  if (filters.priority) params.set('priority', filters.priority);
  if (filters.impactScope) params.set('impact_scope', filters.impactScope);
  return `/projects/${encodeURIComponent(projectId)}/construction-actions/export.xlsx?${params}`;
}
