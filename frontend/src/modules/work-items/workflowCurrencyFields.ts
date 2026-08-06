import type { AnyRecord, WorkflowFormField } from '../../lib/types/domain';

const legacyTopLevelCurrencyFields: Record<string, ReadonlySet<string>> = {
  client_procurement_contract_advance_package: new Set(['advance_amount']),
  client_owner_contract_package: new Set(['contract_value']),
  client_subcontractor_payment_dossier: new Set(['payment_amount']),
  client_subcontractor_payment_execution: new Set(['paid_amount']),
  client_output_settlement_package: new Set(['settlement_amount']),
  client_output_invoice_issue: new Set(['invoice_amount']),
  client_output_payment_collection: new Set(['received_amount']),
  client_input_invoice_closure: new Set(['invoice_amount']),
};

const legacyStructuredCurrencyFields: Record<string, Record<string, ReadonlySet<string>>> = {
  client_boq_supplier_price_collection: { quote_items: new Set(['amount']) },
  client_procurement_sourcing: { sourcing_items: new Set(['quoted_amount']) },
  client_price_negotiation_log: { negotiation_items: new Set(['before_amount', 'after_amount']) },
  client_subcontractor_quantity_dossier: { quantity_items: new Set(['unit_price']) },
};

const legacyCurrencyDefaults = {
  currency_code: 'VND',
  maximum_fraction_digits: 0,
  allow_negative: false,
};

export function effectiveWorkflowCurrencyFields(fields: WorkflowFormField[], templateCode?: string): WorkflowFormField[] {
  if (!templateCode) return fields;
  const topLevelKeys = legacyTopLevelCurrencyFields[templateCode];
  const structuredKeys = legacyStructuredCurrencyFields[templateCode];
  if (!topLevelKeys && !structuredKeys) return fields;

  return fields.map((field) => {
    if (field.field_type === 'number' && topLevelKeys?.has(field.field_key)) {
      return {
        ...field,
        field_type: 'currency',
        validation_json: { ...legacyCurrencyDefaults, ...(field.validation_json ?? {}) },
      };
    }
    const nestedKeys = structuredKeys?.[field.field_key];
    if (field.field_type !== 'structured_list' || !nestedKeys || !Array.isArray(field.validation_json?.item_fields)) return field;
    return {
      ...field,
      validation_json: {
        ...(field.validation_json ?? {}),
        item_fields: field.validation_json.item_fields.map((rawSpec: unknown) => {
          if (!rawSpec || typeof rawSpec !== 'object' || Array.isArray(rawSpec)) return rawSpec;
          const spec = rawSpec as AnyRecord;
          return spec.type === 'number' && nestedKeys.has(String(spec.key)) ? { ...legacyCurrencyDefaults, ...spec, type: 'currency' } : spec;
        }),
      },
    };
  });
}
