import assert from 'node:assert/strict';
import test from 'node:test';
import { buildValidationIssues, defaultWorkflowFormVersionForStep, workflowFormOptions, workflowFormsForTemplate } from '../src/modules/pipeline-builder/pipelineBuilderDomain.ts';

const catalogItem = (versionId: string, values: any = {}) => ({template_id: 'template', template_code: 'builtin_construction_daily_report', template_name: 'Report', template_status: 'active', form_type: 'daily_report', version_id: versionId, version_no: 3, version_status: 'published', ...values});

test('a valid pinned form hidden from new choices passes Pipeline validation and remains visible only on its own step', () => {
  const catalog = [catalogItem('selectable')];
  const pinned = catalogItem('hidden-pin', {fields: [{field_key: 'decision', label: 'Decision'}]});
  const forms = workflowFormsForTemplate(catalog, [pinned]);
  assert.deepEqual(forms.map((form) => form.version_id), ['selectable', 'hidden-pin']);
  assert.deepEqual(workflowFormOptions(forms, '').map((form) => form.version_id), ['selectable']);
  assert.deepEqual(workflowFormOptions(forms, 'hidden-pin').map((form) => form.version_id), ['selectable', 'hidden-pin']);
  const nodes = [{id: 'node-1', name: 'Stage', steps: [{id: 'step-1', name: 'Pinned form', form_required: true, form_version_id: 'hidden-pin'}]}];
  const issues = buildValidationIssues(nodes as any, [], [], new Set(forms.map((form) => form.version_id)), true);
  assert.equal(issues.some((issue) => issue.includes('workflow form')), false, issues.join('; '));
  const invalid = buildValidationIssues(nodes as any, [], [], new Set(catalog.map((form) => form.version_id)), true);
  assert.equal(invalid.some((issue) => issue.includes('workflow form chưa published')), true);
});

test('default form chooses compatible preferred version while native version remains explicitly available', () => {
  const native = catalogItem('native-v3', {default_eligible: false});
  const compatible = catalogItem('compatible-v2', {version_no: 2, default_eligible: true});
  const catalog = [native, compatible];
  assert.equal(defaultWorkflowFormVersionForStep('daily_report', catalog, 'construction'), 'compatible-v2');
  assert.equal(catalog[0].version_id, 'native-v3'); assert.equal(catalog.length, 2);
  assert.equal(catalog.find(form=>form.version_id==='native-v3'), native, 'explicit selection retains new version');
});

test('default eligibility applies to form-type, generic and final fallback, not only preferred template', () => {
  const excluded = catalogItem('native', {default_eligible:false, template_code:'new-native'});
  const report = catalogItem('report', {template_code:'other-report',default_eligible:true});
  const generic = catalogItem('generic', {template_code:'generic',form_type:'generic_submission'});
  const other = catalogItem('other', {form_type:'checklist',template_code:'other'});
  assert.equal(defaultWorkflowFormVersionForStep('daily_report',[excluded,report,generic]),'report');
  assert.equal(defaultWorkflowFormVersionForStep('daily_report',[excluded,generic]),'generic');
  assert.equal(defaultWorkflowFormVersionForStep('daily_report',[excluded,other]),'other');
  assert.equal(defaultWorkflowFormVersionForStep('task',[catalogItem('excluded-generic',{default_eligible:false,form_type:'generic_submission'}),other]),'other');
});

test('default resolver returns empty when only explicit-only catalog matches exist', () => {
  const excluded = catalogItem('native-v3', {default_eligible:false});
  for (const kind of ['daily_report','checklist','meeting_minutes','task']) assert.equal(defaultWorkflowFormVersionForStep(kind,[excluded],'construction'),'');
  assert.equal(defaultWorkflowFormVersionForStep('daily_report',[],'construction'),'');
});

test('catalog without eligibility field remains backward compatible and is not mutated', () => {
  const legacy = Object.freeze(catalogItem('legacy-v2',{version_no:2}));
  const catalog = Object.freeze([legacy]);
  assert.equal(defaultWorkflowFormVersionForStep('daily_report',[...catalog],'construction'),'legacy-v2');
  assert.equal('default_eligible' in legacy,false);
});

test('complete BM03 assessment is the newest automatic default while incomplete native revisions remain explicit', () => {
  // The catalog contract orders each template by version_no DESC.
  const incomplete = catalogItem('incomplete-assessment', {template_code:'builtin_subcontractor_checklist',form_type:'checklist',version_no:6,default_eligible:false});
  const assessment = catalogItem('complete-assessment', {template_code:'builtin_subcontractor_checklist',form_type:'checklist',version_no:5,default_eligible:true});
  const olderAssessment = catalogItem('older-assessment', {template_code:'builtin_subcontractor_checklist',form_type:'checklist',version_no:4,default_eligible:true});
  const legacy = catalogItem('legacy', {template_code:'builtin_subcontractor_checklist',form_type:'checklist',version_no:2,default_eligible:true});
  const catalog = Object.freeze([incomplete,assessment,olderAssessment,legacy]);
  assert.equal(defaultWorkflowFormVersionForStep('checklist',[...catalog],'construction','Checklist thầu phụ'),'complete-assessment');
  assert.equal(defaultWorkflowFormVersionForStep('checklist',[...catalog],'acceptance','Subcontractor'),'complete-assessment');
  assert.equal(catalog.find(form=>form.version_id===legacy.version_id),legacy,'existing explicit pins remain selectable');
  assert.equal(catalog.find(form=>form.version_id===incomplete.version_id),incomplete,'incomplete native profile remains explicitly selectable');
});

test('BM03 package fallback follows eligibility while daily, survey and risk native forms remain excluded', () => {
  const packageForm = catalogItem('assessment-package', {template_code:'client_subcontractor_acceptance_package',form_type:'checklist',default_eligible:true});
  const catalog = ['builtin_construction_daily_report','client_site_measurement_record','client_survey_work_package','client_site_risk_assessment'].map(code=>catalogItem(code,{template_code:code,default_eligible:false}));
  assert.equal(defaultWorkflowFormVersionForStep('checklist',[...catalog,packageForm]),'assessment-package');
  for (const code of catalog) assert.equal(defaultWorkflowFormVersionForStep('task',[code]),'');
});
