import { loadDailyCapture, loadDailyWorkRows, type BlockerCaptureDraft, type NextPlanDraft, type MaterialArrivalDraft, type SupervisionAssessment } from './constructionDailyCapture';

function capturedProgressValue(value: unknown): number | undefined {
  for (let depth = 0; depth < 8 && value && typeof value === 'object' && !Array.isArray(value) && 'value' in value; depth++) value = (value as Record<string, unknown>).value;
  const numeric = typeof value === 'number' || (typeof value === 'string' && /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(value.trim()));
  const number = Number(value);
  return numeric && Number.isFinite(number) && number >= 0 && number <= 100 ? number : undefined;
}
export type ConstructionReportViewModel = {
  reportDate: string;
  weather: string;
  progressPercent: number;
  hasProgress: boolean;
  done: string[];
  current: string[];
  next: string[];
  blockers: string[];
  fieldNotes: string[];
  timelineProblems: string[];
  crew: string[];
  materials: string[];
  nextDetails: string[];
  blockerDetails: string[];
  supervision: string[];
  materialArrivals: string[];
  evidenceIds: string[];
  workItemId: string;
  workflowFormInstanceId: string;
  sourceKind: 'workflow' | 'structured' | 'summary';
};

export type ConstructionReportEditModel = {
  reportDate: string;
  weather: string;
  progressPercent: string;
  workItemId: string;
  done: string[];
  current: string[];
  next: string[];
  blockers: string[];
  fieldNotes: string;
  crewItems: Array<{ row_id: string; crew_name: string; worker_count: string; work_hours: string; note: string; representative: string; contact: string }>;
  supervisionAssessments: SupervisionAssessment[];
  materialArrivalItems: MaterialArrivalDraft[];
  nextPlanItems: NextPlanDraft[];
  blockerItems: BlockerCaptureDraft[];
  materialItems: Array<{ material_name: string; quantity: string; unit: string; note: string }>;
  evidenceIds: string[];
  primaryEvidenceId: string;
  manpowerSummary: string;
  materialSummary: string;
};

export function constructionReportViewModel(row: Record<string, any>): ConstructionReportViewModel {
  const structured = asRecord(row.structured_json);
  const completed = asRecordList(structured.completed_items);
  const done = completed.filter((item) => item.result === 'done').map(itemTitle).filter(Boolean);
  const current = completed.filter((item) => item.result === 'partial').map(itemTitle).filter(Boolean);
  const blockedItems = completed.filter((item) => item.result === 'blocked');
  const next = asRecordList(structured.next_plan_items).map(itemTitle).filter(Boolean);
  if (!done.length && !current.length && !next.length) {
    const legacy = parseLegacyWorkSummary(cleanText(row.work_summary));
    done.push(...legacy.done);
    current.push(...legacy.current);
    next.push(...legacy.next);
  }

  const structuredBlockers = asRecordList(structured.blockers);
  const blockers = unique([
    ...blockedItems.map(itemTitle),
    ...structuredBlockers.map((item) => cleanText(item.content) || cleanText(item.title)),
  ].filter(Boolean));
  const issueSummary = cleanText(row.issues_summary);
  const fieldNotes = issueSummary && !isEmptyNote(issueSummary) ? splitLines(issueSummary) : [];
  const crew = structuredResourceList(structured.crew_items, (item) => {
    const name = cleanText(item.crew_name) || cleanText(item.trade) || 'Tổ đội';
    const workers = numberText(item.worker_count, 'người');
    const hours = numberText(item.work_hours, 'giờ');
    return [name, workers, hours, cleanText(item.note), item.representative ? `Đại diện: ${cleanText(item.representative)}` : '', item.contact ? `Liên hệ: ${cleanText(item.contact)}` : ''].filter(Boolean).join(' · ');
  }, row.manpower_summary);
  const materials = structuredResourceList(structured.material_usage_items, (item) => {
    const name = cleanText(item.material_name) || cleanText(item.material_code) || 'Vật tư';
    const quantity = [cleanText(item.quantity), cleanText(item.unit)].filter(Boolean).join(' ');
    return [name, quantity, cleanText(item.note)].filter(Boolean).join(' · ');
  }, row.material_usage_summary);
  const completedEvidenceIds = completed.map((item) => cleanText(item.evidence_id));
  const photos = Array.isArray(row.photos) ? row.photos.map(cleanText) : asStringList(row.photos);
  const evidenceIds = unique([
    cleanText(row.primary_evidence_id),
    cleanText(structured.site_photo_evidence_id),
    ...completedEvidenceIds,
    ...photos,
  ]);
  const workflowFormInstanceId = cleanText(row.workflow_form_instance_id);
  const capturedProgress = capturedProgressValue(structured.progress_percent);
  const manualProgress = row.progress_percent;
  const hasProgress = workflowFormInstanceId ? capturedProgress !== undefined : (typeof manualProgress === 'number' || typeof manualProgress === 'string') && String(manualProgress).trim() !== '' && Number.isFinite(Number(manualProgress));
  const progressPercent = workflowFormInstanceId ? capturedProgress ?? 0 : hasProgress ? Math.min(100, Math.max(0, Number(manualProgress))) : 0;
  const detailRows = (rows: Record<string, any>[], titleKey: string, labels: Record<string, string>) => rows.map(item => [cleanText(item[titleKey]), ...Object.entries(labels).filter(([key]) => key === 'progress_percent' ? capturedProgressValue(item[key]) !== undefined : item[key] != null && cleanText(item[key]) !== '').map(([key, label]) => `${label}: ${key === 'progress_percent' ? `${capturedProgressValue(item[key])}%` : key === 'severity' ? ({ low: 'Thấp', medium: 'Trung bình', high: 'Cao', critical: 'Nghiêm trọng' }[cleanText(item[key])] ?? cleanText(item[key])) : cleanText(item[key])}`)].filter(Boolean).join(' · ')).filter(Boolean);
  const nextDetails = detailRows(asRecordList(structured.next_plan_items), 'title', { instruction_code: 'Mã chỉ dẫn', location: 'Vị trí', subcontractor: 'Thầu phụ / tổ đội', receiver: 'Người nhận', contact: 'Liên hệ', issued_date: 'Ngày giao', due_date: 'Hạn thực hiện', progress_percent: 'Tiến độ', state: 'Tình trạng' });
  const blockerDetails = detailRows(structuredBlockers, 'content', { risk_code: 'Mã rủi ro', detected_at: 'Ngày phát hiện', cause: 'Nguyên nhân', impact: 'Ảnh hưởng', severity: 'Mức độ', proposed_action: 'Biện pháp', owner: 'Người phụ trách', target_date: 'Hạn xử lý', status: 'Tình trạng' });
  const assessmentLabels: Record<string, string> = { pass: 'Đạt', fail: 'Không đạt', not_applicable: 'Không áp dụng', not_assessed: 'Chưa đánh giá' };
  const criterionLabels: Record<string, string> = { progress: 'Tiến độ', workforce: 'Nhân công', deliveries: 'Vật tư nhập trường', quality: 'Chất lượng', completed_quantity: 'Khối lượng hoàn thành', variation: 'Phát sinh', mep: 'MEP', safety: 'An toàn', acceptance: 'Nghiệm thu', risk: 'Rủi ro', images: 'Ảnh hiện trường', instructions: 'Chỉ dẫn' };
  const supervision = asRecordList(structured.supervision_assessments).map(item => [criterionLabels[item.code] ?? cleanText(item.code), assessmentLabels[item.assessment] ?? cleanText(item.assessment), cleanText(item.note)].filter(Boolean).join(' · '));
  const materialArrivals = detailRows(asRecordList(structured.material_arrival_items), 'material_name', { specification: 'Quy cách', origin: 'Xuất xứ', quantity: 'Số lượng', unit: 'Đơn vị', note: 'Ghi chú' });
  const hasCanonicalStructure = Boolean(
    completed.length
    || asRecordList(structured.next_plan_items).length
    || structuredBlockers.length
    || asRecordList(structured.crew_items).length
    || asRecordList(structured.material_usage_items).length
    || cleanText(structured.site_photo_evidence_id),
  );

  return {
    reportDate: cleanText(row.report_date),
    weather: cleanText(row.weather),
    progressPercent,
    hasProgress,
    done: unique(done),
    current: unique(current),
    next: unique(next),
    blockers,
    fieldNotes,
    timelineProblems: hasCanonicalStructure ? blockers : unique([...blockers, ...fieldNotes]),
    crew,
    materials,
    nextDetails: nextDetails.length ? nextDetails : unique(next),
    blockerDetails: blockerDetails.length ? [...blockerDetails, ...blockers.filter(title => !structuredBlockers.some(item => cleanText(item.content) === title))] : blockers,
    supervision,
    materialArrivals,
    evidenceIds,
    workItemId: cleanText(row.work_item_id),
    workflowFormInstanceId,
    sourceKind: workflowFormInstanceId ? 'workflow' : hasCanonicalStructure ? 'structured' : 'summary',
  };
}

export function constructionReportEditModel(row: Record<string, any>): ConstructionReportEditModel {
  const view = constructionReportViewModel(row);
  const structured = asRecord(row.structured_json);
  const capture = loadDailyCapture(structured);
  const workRows = loadDailyWorkRows(structured, view.next, view.blockers);
  const crewItems = asRecordList(structured.crew_items).map((item) => ({
    row_id: cleanText(item.row_id) || crypto.randomUUID(),
    crew_name: cleanText(item.crew_name) || cleanText(item.trade),
    worker_count: cleanText(item.worker_count),
    work_hours: cleanText(item.work_hours),
    note: cleanText(item.note),
    representative: cleanText(item.representative),
    contact: cleanText(item.contact),
  }));
  const materialItems = asRecordList(structured.material_usage_items).map((item) => ({
    material_name: cleanText(item.material_name) || cleanText(item.material_code),
    quantity: cleanText(item.quantity),
    unit: cleanText(item.unit),
    note: cleanText(item.note),
  }));
  return {
    reportDate: view.reportDate.slice(0, 10),
    weather: view.weather || 'Nắng',
    progressPercent: String(view.progressPercent),
    workItemId: view.workItemId,
    done: view.done.length ? view.done : [''],
    current: view.current.length ? view.current : [''],
    next: view.next.length ? view.next : [''],
    blockers: view.blockers.length ? view.blockers : [''],
    fieldNotes: cleanText(row.issues_summary),
    crewItems,
    supervisionAssessments: capture.assessments,
    materialArrivalItems: capture.arrivals,
    nextPlanItems: workRows.nextPlans,
    blockerItems: workRows.blockers,
    materialItems,
    evidenceIds: view.evidenceIds,
    primaryEvidenceId: cleanText(row.primary_evidence_id) || view.evidenceIds[0] || '',
    manpowerSummary: crewItems.length ? '' : cleanText(row.manpower_summary),
    materialSummary: materialItems.length ? '' : cleanText(row.material_usage_summary),
  };
}

function structuredResourceList(value: unknown, formatter: (item: Record<string, any>) => string, fallback: unknown) {
  const items = asRecordList(value).map(formatter).filter(Boolean);
  if (items.length) return unique(items);
  const fallbackText = cleanText(fallback);
  return fallbackText ? splitLines(fallbackText) : [];
}

function parseLegacyWorkSummary(value: string) {
  const result = { done: [] as string[], current: [] as string[], next: [] as string[] };
  if (!value) return result;
  const markers = /(?:^|\n)(Đã làm|Đang làm|Sẽ làm):\s*/g;
  const matches = [...value.matchAll(markers)];
  if (!matches.length) {
    result.done = splitLines(value);
    return result;
  }
  matches.forEach((match, index) => {
    const contentStart = (match.index ?? 0) + match[0].length;
    const contentEnd = matches[index + 1]?.index ?? value.length;
    const items = splitLines(value.slice(contentStart, contentEnd));
    if (match[1] === 'Đã làm') result.done.push(...items);
    if (match[1] === 'Đang làm') result.current.push(...items);
    if (match[1] === 'Sẽ làm') result.next.push(...items);
  });
  return result;
}

function asRecord(value: unknown): Record<string, any> {
  if (value && typeof value === 'object' && !Array.isArray(value)) return value as Record<string, any>;
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value);
      return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
    } catch {
      return {};
    }
  }
  return {};
}

function asRecordList(value: unknown): Record<string, any>[] {
  return Array.isArray(value)
    ? value.filter((item): item is Record<string, any> => Boolean(item) && typeof item === 'object' && !Array.isArray(item))
    : [];
}

function asStringList(value: unknown) {
  if (typeof value !== 'string') return [];
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed.map(cleanText).filter(Boolean) : [];
  } catch {
    return [];
  }
}

function itemTitle(item: Record<string, any>) {
  return cleanText(item.title) || cleanText(item.content) || cleanText(item.activity_name);
}

function numberText(value: unknown, suffix: string) {
  const clean = cleanText(value);
  return clean && clean !== '0' ? `${clean} ${suffix}` : '';
}

function isEmptyNote(value: string) {
  return ['không có', 'khong co', '-', 'none'].includes(value.toLocaleLowerCase('vi'));
}

function splitLines(value: string) {
  return value.split(/\n|;/).map((item) => item.replace(/^[-•\s]+/, '').trim()).filter(Boolean);
}

function cleanText(value: unknown) {
  return typeof value === 'string' ? value.trim() : value == null ? '' : String(value).trim();
}

function unique(items: string[]) {
  return [...new Set(items.map((item) => item.trim()).filter(Boolean))];
}
