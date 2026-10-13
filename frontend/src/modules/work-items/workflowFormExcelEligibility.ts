/** Each Excel document requires a native form with the same business meaning. */
export function workflowFormExcelCode(templateCode?: string): 'BM-GS-01' | 'BM-GS-02' | 'BM-GS-03' | 'BM-GS-05' | '' {
  if (templateCode === 'builtin_construction_daily_report') return 'BM-GS-02';
  if (templateCode === 'client_survey_work_package' || templateCode === 'client_site_measurement_record') return 'BM-GS-01';
  if (templateCode === 'builtin_subcontractor_checklist' || templateCode === 'client_subcontractor_acceptance_package') return 'BM-GS-03';
  if (templateCode === 'client_site_risk_assessment') return 'BM-GS-05';
  return '';
}

export function workflowFormRiskExcelEligible(templateCode: string | undefined, fieldKeys: string[]): boolean {
  return (templateCode === 'client_survey_work_package' && fieldKeys.includes('risk_items'))
    || (templateCode === 'builtin_construction_daily_report' && fieldKeys.includes('blockers'));
}
