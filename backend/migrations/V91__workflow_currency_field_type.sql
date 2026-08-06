ALTER TABLE workflow_form_fields
  DROP CONSTRAINT IF EXISTS workflow_form_fields_type_check;

ALTER TABLE workflow_form_fields
  ADD CONSTRAINT workflow_form_fields_type_check CHECK (field_type IN (
    'text',
    'number',
    'currency',
    'date',
    'textarea',
    'select',
    'multi_select',
    'checkbox',
    'evidence_reference',
    'structured_list'
  ));
