import type { AnyRecord, WorkflowFormField } from '../../lib/types/domain';

type RuleEvaluator = (actual: unknown, actualExists: boolean, expected: unknown) => boolean;

const ruleEvaluators: Record<string, RuleEvaluator> = {
  equals: (actual, actualExists, expected) => actualExists && valuesEqual(actual, expected),
  not_equals: (actual, actualExists, expected) => !actualExists || !valuesEqual(actual, expected),
  in: (actual, actualExists, expected) => actualExists && Array.isArray(expected) && expected.some((candidate) => valuesEqual(actual, candidate)),
  exists: (_actual, actualExists) => actualExists,
  not_exists: (_actual, actualExists) => !actualExists,
};

export function workflowFormFieldVisible(field: WorkflowFormField, answers: AnyRecord) {
  return evaluateRule(field.visibility_condition_json, answers, true);
}

export function workflowFormFieldRequired(field: WorkflowFormField, answers: AnyRecord) {
  if (field.required) return true;
  return evaluateRule(field.validation_json?.required_when, answers, false);
}

export function effectiveWorkflowFormFields(fields: WorkflowFormField[], answers: AnyRecord) {
  return fields
    .filter((field) => workflowFormFieldVisible(field, answers))
    .map((field) => ({ ...field, required: workflowFormFieldRequired(field, answers) }));
}

function evaluateRule(rawRule: unknown, answers: AnyRecord, emptyResult: boolean) {
  if (!isRecord(rawRule) || Object.keys(rawRule).length === 0) return emptyResult;
  const evaluator = ruleEvaluators[String(rawRule.operator ?? '')];
  if (!evaluator) return false;
  const fieldKey = String(rawRule.field_key ?? '').trim();
  if (!fieldKey) return false;
  const actual = answers[fieldKey];
  const actualExists = Object.prototype.hasOwnProperty.call(answers, fieldKey) && !valueEmpty(actual);
  return evaluator(actual, actualExists, rawRule.value);
}

function valuesEqual(actual: unknown, expected: unknown): boolean {
  if (Array.isArray(actual)) return actual.some((value) => valuesEqual(value, expected));
  if (typeof actual === 'number' && typeof expected === 'string' && expected.trim() !== '') return actual === Number(expected);
  if (typeof expected === 'number' && typeof actual === 'string' && actual.trim() !== '') return Number(actual) === expected;
  return actual === expected;
}

function valueEmpty(value: unknown) {
  return value === null || value === undefined || (typeof value === 'string' && value.trim() === '');
}

function isRecord(value: unknown): value is AnyRecord {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value);
}
