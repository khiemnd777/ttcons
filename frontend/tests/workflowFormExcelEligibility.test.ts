import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { workflowFormExcelCode, workflowFormRiskExcelEligible } from '../src/modules/work-items/workflowFormExcelEligibility.ts';

test('BM01 accepts only native dimensional survey forms and excludes survey minutes', () => {
  assert.equal(workflowFormExcelCode('client_survey_work_package'), 'BM-GS-01');
  assert.equal(workflowFormExcelCode('client_site_measurement_record'), 'BM-GS-01');
  assert.equal(workflowFormExcelCode('client_site_survey_minutes'), '');
  assert.equal(workflowFormExcelCode('client_survey_requirement_intake'), '');
  assert.equal(workflowFormExcelCode(undefined), '');
});
test('other native form exports keep distinct proven source meaning', () => {
  assert.equal(workflowFormExcelCode('builtin_subcontractor_checklist'), 'BM-GS-03');
  assert.equal(workflowFormExcelCode('client_subcontractor_acceptance_package'), 'BM-GS-03');
  assert.equal(workflowFormExcelCode('client_site_risk_assessment'), 'BM-GS-05');
  assert.equal(workflowFormExcelCode('builtin_acceptance'), '');
  assert.equal(workflowFormExcelCode('builtin_construction_daily_report'), 'BM-GS-02');
});
test('contextual risk export uses only recognized source schema fields', () => {
  assert.equal(workflowFormRiskExcelEligible('builtin_construction_daily_report', ['blockers']), true);
  assert.equal(workflowFormRiskExcelEligible('builtin_construction_daily_report', ['next_plan_items']), false);
  assert.equal(workflowFormRiskExcelEligible('client_survey_work_package', ['risk_items']), true);
  for (const code of ['client_site_survey_minutes','client_survey_requirement_intake',undefined]) assert.equal(workflowFormRiskExcelEligible(code, ['risk_items','blockers']), false);
});
test('panel derives the Excel action from eligibility without changing native PDF availability', async () => {
  const panel = await readFile(new URL('../src/modules/work-items/WorkflowFormPanel.tsx', import.meta.url), 'utf8');
  assert.match(panel, /const exportFormCode = workflowFormExcelCode\(templateCode\)/);
  assert.match(panel, /exportFormCode \? <NativeExcelExportButton/);
  assert.match(panel, /artifactAccess\.canGeneratePDF/);
  assert.doesNotMatch(panel, /client_site_survey_minutes/);
});
