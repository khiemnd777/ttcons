import type { AnyRecord, WorkflowFormField } from '../../lib/types/domain';

const clientSiteRiskAssessmentFormCode = 'client_site_risk_assessment';
const clientSiteRiskItemsFieldKey = 'risk_items';
const clientSiteRiskCategoryFieldKey = 'category';
const clientSurveyWorkPackageFormCode = 'client_survey_work_package';
const clientSurveyDimensionItemsFieldKey = 'dimension_items';
const clientSurveyDimensionAreaFieldKey = 'area';

export function effectiveWorkflowStructuredListFields(fields: WorkflowFormField[], templateCode?: string): WorkflowFormField[] {
  if (templateCode !== clientSiteRiskAssessmentFormCode && templateCode !== clientSurveyWorkPackageFormCode) return fields;

  return fields.map((field) => {
    if (field.field_type !== 'structured_list' || !Array.isArray(field.validation_json?.item_fields)) {
      return field;
    }
    if (templateCode === clientSiteRiskAssessmentFormCode && field.field_key === clientSiteRiskItemsFieldKey) {
      return {
        ...field,
        validation_json: {
          ...(field.validation_json ?? {}),
          item_fields: field.validation_json.item_fields.filter((rawSpec: unknown) => {
            if (!rawSpec || typeof rawSpec !== 'object' || Array.isArray(rawSpec)) return true;
            return String((rawSpec as AnyRecord).key ?? '') !== clientSiteRiskCategoryFieldKey;
          }),
        },
      };
    }
    if (templateCode !== clientSurveyWorkPackageFormCode || field.field_key !== clientSurveyDimensionItemsFieldKey) {
      return field;
    }
    return {
      ...field,
      validation_json: {
        ...(field.validation_json ?? {}),
        item_fields: field.validation_json.item_fields.map((rawSpec: unknown) => {
          if (!rawSpec || typeof rawSpec !== 'object' || Array.isArray(rawSpec)) return rawSpec;
          const spec = rawSpec as AnyRecord;
          if (String(spec.key ?? '') !== clientSurveyDimensionAreaFieldKey) return rawSpec;
          const { min_length: _publishedMinLength, ...effectiveSpec } = spec;
          return { ...effectiveSpec, required: false };
        }),
      },
    };
  });
}
