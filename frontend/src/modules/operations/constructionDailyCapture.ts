export const supervisionCriteria = [
  ['progress', 'Tiến độ'], ['workforce', 'Nhân công'], ['deliveries', 'Vật tư nhập trường'],
  ['quality', 'Chất lượng'], ['completed_quantity', 'Khối lượng hoàn thành'], ['variation', 'Phát sinh'],
  ['mep', 'Hệ thống MEP'], ['safety', 'An toàn'], ['acceptance', 'Nghiệm thu'],
  ['risk', 'Rủi ro'], ['images', 'Ảnh hiện trường'], ['instructions', 'Chỉ dẫn'],
] as const;
export type SupervisionCode = typeof supervisionCriteria[number][0];
export type SupervisionAssessment = { code: SupervisionCode; assessment: 'pass' | 'fail' | 'not_applicable' | 'not_assessed'; note?: string };
export type MaterialArrivalDraft = { row_id: string; material_name: string; specification: string; origin: string; quantity: string; unit: string; note: string };
export type NextPlanDraft = { row_id: string; title: string; instruction_code: string; location: string; subcontractor: string; receiver: string; contact: string; issued_date: string; due_date: string; progress_percent: string; state: string };
export type BlockerCaptureDraft = { row_id: string; content: string; risk_code: string; detected_at: string; cause: string; impact: string; severity: string; proposed_action: string; owner: string; target_date: string; status: string };
export function emptyNextPlan(title = ''): NextPlanDraft { return { row_id: crypto.randomUUID(), title, instruction_code: '', location: '', subcontractor: '', receiver: '', contact: '', issued_date: '', due_date: '', progress_percent: '', state: '' }; }
export function emptyBlockerCapture(content = ''): BlockerCaptureDraft { return { row_id: crypto.randomUUID(), content, risk_code: '', detected_at: '', cause: '', impact: '', severity: '', proposed_action: '', owner: '', target_date: '', status: '' }; }
function hasCaptureRow(row: Record<string, string>) { return Object.entries(row).some(([key, value]) => key !== 'row_id' && key !== 'id' && value.trim() !== ''); }
export function loadDailyWorkRows(structured: Record<string, any>, nextTitles: string[], blockerTitles: string[]) {
  function load<Row extends NextPlanDraft | BlockerCaptureDraft>(saved: unknown, titles: string[], key: 'title' | 'content', create: (title: string) => Row): Row[] {
    const rows = Array.isArray(saved) ? saved : [];
    const sources = [...rows, ...titles.filter(title => !rows.some(row => String(row[key] ?? '').trim() === title.trim())).map(title => ({ [key]: title }))];
    return sources.map(source => {
      const draft = create(String(source[key] ?? ''));
      for (const field of Object.keys(draft)) if (source?.[field] != null) (draft as Record<string, string>)[field] = String(source[field]);
      return draft;
    });
  }
  return { nextPlans: load(structured.next_plan_items, nextTitles, 'title', emptyNextPlan), blockers: load(structured.blockers, blockerTitles, 'content', emptyBlockerCapture) };
}
export function serializeDailyWorkRows(nextPlans: NextPlanDraft[], blockers: BlockerCaptureDraft[]) {
  const next_items = nextPlans.filter(hasCaptureRow).map(row => ({ ...row, title: row.title.trim(), progress_percent: row.progress_percent.trim() === '' ? null : Number(row.progress_percent) }));
  const blockerItems = blockers.filter(hasCaptureRow).map(({ row_id, content, risk_code, detected_at, cause, impact, severity, proposed_action, owner, target_date, status }) => ({ row_id, content: content.trim(), risk_code, detected_at, cause, impact, severity, proposed_action, owner, target_date, status }));
  return { next_items, blockers: blockerItems };
}
export function validateDailyWorkRows(nextPlans: NextPlanDraft[], blockers: BlockerCaptureDraft[]): Record<string, string> {
  const errors: Record<string, string> = {};
  if (nextPlans.filter(hasCaptureRow).length > 120) errors.next_items = 'Tối đa 120 dòng kế hoạch';
  if (blockers.filter(hasCaptureRow).length > 120) errors.blockers = 'Tối đa 120 dòng vấn đề';
  const validDate = (value: string) => { if (!value) return true; const date = new Date(`${value}T00:00:00Z`); return /^\d{4}-\d{2}-\d{2}$/.test(value) && Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value; };
  for (const row of nextPlans.filter(hasCaptureRow)) {
    for (const [field, max] of Object.entries({ title: 180, instruction_code: 80, location: 200, subcontractor: 160, receiver: 120, contact: 100, state: 80 })) if (Array.from((row as Record<string, string>)[field]).length > max) errors[`next_${row.row_id}_${field}`] = `Tối đa ${max} ký tự`;
    if (!row.title.trim()) errors[`next_${row.row_id}_title`] = 'Nội dung kế hoạch';
    if (row.progress_percent.trim() !== '' && (!Number.isFinite(Number(row.progress_percent)) || Number(row.progress_percent) < 0 || Number(row.progress_percent) > 100)) errors[`next_${row.row_id}_progress_percent`] = 'Tiến độ kế hoạch phải từ 0 đến 100';
    for (const field of ['issued_date', 'due_date'] as const) if (!validDate(row[field])) errors[`next_${row.row_id}_${field}`] = 'Ngày kế hoạch không hợp lệ';
  }
  for (const row of blockers.filter(hasCaptureRow)) {
    for (const [field, max] of Object.entries({ content: 1000, risk_code: 80, cause: 1000, impact: 1000, proposed_action: 1000, owner: 160, status: 80 })) if (Array.from((row as Record<string, string>)[field]).length > max) errors[`blocker_${row.row_id}_${field}`] = `Tối đa ${max} ký tự`;
    if (!row.content.trim()) errors[`blocker_${row.row_id}_content`] = 'Nội dung vấn đề';
    if (row.severity && !['low', 'medium', 'high', 'critical'].includes(row.severity)) errors[`blocker_${row.row_id}_severity`] = 'Mức độ rủi ro không hợp lệ';
    for (const field of ['detected_at', 'target_date'] as const) if (!validDate(row[field])) errors[`blocker_${row.row_id}_${field}`] = 'Ngày vấn đề không hợp lệ';
  }
  return errors;
}
export function emptyMaterialArrival(): MaterialArrivalDraft {
  return { row_id: crypto.randomUUID(), material_name: '', specification: '', origin: '', quantity: '', unit: '', note: '' };
}
export function hasMaterialArrival(row: MaterialArrivalDraft) {
  return Object.entries(row).some(([key, value]) => key !== 'row_id' && value.trim() !== '');
}
export function loadDailyCapture(structured: Record<string, any>) {
  const saved = Array.isArray(structured.supervision_assessments) ? structured.supervision_assessments : [];
  const assessments: SupervisionAssessment[] = supervisionCriteria.map(([code]) => {
    const row = saved.find((item: any) => item.code === code);
    return { code, assessment: row?.assessment ?? 'not_assessed', note: String(row?.note ?? '') };
  });
  const arrivals: MaterialArrivalDraft[] = (Array.isArray(structured.material_arrival_items) ? structured.material_arrival_items : []).map((row: any) => ({
    row_id: String(row.row_id || crypto.randomUUID()), material_name: String(row.material_name ?? ''),
    specification: String(row.specification ?? ''), origin: String(row.origin ?? ''),
    quantity: row.quantity == null ? '' : String(row.quantity), unit: String(row.unit ?? ''), note: String(row.note ?? ''),
  }));
  return { assessments, arrivals };
}
export function serializeDailyCapture(assessments: SupervisionAssessment[], arrivals: MaterialArrivalDraft[]) {
  return {
    supervision_assessments: assessments.filter((row) => row.assessment !== 'not_assessed' || row.note?.trim()).map((row) => ({ ...row, note: row.note?.trim() || undefined })),
    material_arrival_items: arrivals.filter(hasMaterialArrival).map((row) => ({ ...row, material_name: row.material_name.trim(), specification: row.specification.trim(), origin: row.origin.trim(), quantity: row.quantity.trim() === '' ? null : Number(row.quantity), unit: row.unit.trim(), note: row.note.trim() })),
  };
}
export function validateDailyCapture(arrivals: MaterialArrivalDraft[]) {
  const rows = arrivals.filter(hasMaterialArrival);
  if (rows.length > 200) return 'Tối đa 200 dòng vật tư nhập trường';
  if (rows.some((row) => !row.material_name.trim())) return 'Tên vật tư nhập trường';
  if (rows.some((row) => row.quantity.trim() !== '' && (!Number.isFinite(Number(row.quantity)) || Number(row.quantity) < 0))) return 'Số lượng nhập trường phải từ 0 trở lên';
  if (rows.some((row) => row.quantity.trim() !== '' && !row.unit.trim())) return 'Đơn vị vật tư nhập trường khi có số lượng';
  return '';
}
