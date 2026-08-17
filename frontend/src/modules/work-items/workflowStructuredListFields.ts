import type { AnyRecord, WorkflowFormField } from '../../lib/types/domain';

const clientSiteRiskAssessmentFormCode = 'client_site_risk_assessment';
const clientSiteRiskItemsFieldKey = 'risk_items';
const clientSiteRiskCategoryFieldKey = 'category';

export function effectiveWorkflowStructuredListFields(fields: WorkflowFormField[], templateCode?: string): WorkflowFormField[] {
  if (templateCode !== clientSiteRiskAssessmentFormCode) return fields;

  return fields.map((field) => {
    if (field.field_type !== 'structured_list' || field.field_key !== clientSiteRiskItemsFieldKey || !Array.isArray(field.validation_json?.item_fields)) {
      return field;
    }
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
  });
}
