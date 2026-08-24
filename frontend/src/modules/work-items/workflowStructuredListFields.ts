import type { AnyRecord, WorkflowFormField } from '../../lib/types/domain';

const clientSiteRiskAssessmentFormCode = 'client_site_risk_assessment';
const clientSiteRiskItemsFieldKey = 'risk_items';
const clientSiteRiskCategoryFieldKey = 'category';
const clientSurveyWorkPackageFormCode = 'client_survey_work_package';
const clientSurveyDimensionItemsFieldKey = 'dimension_items';
const clientSurveyDimensionAreaFieldKey = 'area';
const clientConstructionHandoffPlanFormCode = 'client_construction_handoff_plan';
const clientConstructionInspectionItemsFieldKey = 'inspection_items';
const clientConstructionEvidenceRequiredFieldKey = 'evidence_required';
const constructionHandoffEvidenceHelpText = 'Mỗi dòng sẽ sinh đúng một đầu việc và một kiểm tra khi kích hoạt bàn giao. Mọi hạng mục bắt buộc ít nhất 1 ảnh hoặc tài liệu khi ghi kết quả.';

export function effectiveWorkflowStructuredListFields(fields: WorkflowFormField[], templateCode?: string): WorkflowFormField[] {
  if (
    templateCode !== clientSiteRiskAssessmentFormCode
    && templateCode !== clientSurveyWorkPackageFormCode
    && templateCode !== clientConstructionHandoffPlanFormCode
  ) return fields;

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
    if (templateCode === clientConstructionHandoffPlanFormCode && field.field_key === clientConstructionInspectionItemsFieldKey) {
      return {
        ...field,
        help_text: constructionHandoffEvidenceHelpText,
        validation_json: {
          ...(field.validation_json ?? {}),
          item_fields: field.validation_json.item_fields.filter((rawSpec: unknown) => {
            if (!rawSpec || typeof rawSpec !== 'object' || Array.isArray(rawSpec)) return true;
            return String((rawSpec as AnyRecord).key ?? '') !== clientConstructionEvidenceRequiredFieldKey;
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
