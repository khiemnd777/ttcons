import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkflowFormField } from '../src/lib/types/domain.ts';
import { effectiveWorkflowCurrencyFields } from '../src/modules/work-items/workflowCurrencyFields.ts';

function field(field_key: string, field_type = 'number', validation_json: Record<string, unknown> = {}): WorkflowFormField {
  return { form_version_id: 'version', field_key, label: field_key, field_type, required: false, validation_json };
}

test('adapts exact legacy top-level money fields without mutating published definitions', () => {
  const stored = field('advance_amount');
  const effective = effectiveWorkflowCurrencyFields([stored], 'client_procurement_contract_advance_package');
  assert.equal(stored.field_type, 'number');
  assert.equal(effective[0].field_type, 'currency');
  assert.equal(effective[0].validation_json?.currency_code, 'VND');
  assert.equal(effective[0].validation_json?.maximum_fraction_digits, 0);
});

test('adapts exact nested legacy money fields and leaves quantities as numbers', () => {
  const stored = field('quantity_items', 'structured_list', {
    item_fields: [
      { key: 'claimed_quantity', type: 'number' },
      { key: 'unit_price', type: 'number' },
    ],
  });
  const [effective] = effectiveWorkflowCurrencyFields([stored], 'client_subcontractor_quantity_dossier');
  const itemFields = effective.validation_json?.item_fields as Array<Record<string, unknown>>;
  assert.equal(itemFields[0].type, 'number');
  assert.equal(itemFields[1].type, 'currency');
  assert.equal(itemFields[1].maximum_fraction_digits, 0);
  assert.equal((stored.validation_json?.item_fields as Array<Record<string, unknown>>)[1].type, 'number');
});

test('does not infer currency from ambiguous labels or field names', () => {
  const stored = field('management_rate');
  const [effective] = effectiveWorkflowCurrencyFields([stored], 'client_director_management_rate_review');
  assert.strictEqual(effective, stored);
  assert.equal(effective.field_type, 'number');
});
