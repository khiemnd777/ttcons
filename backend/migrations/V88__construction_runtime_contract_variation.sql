-- Close the construction runtime loop without mutating published workflow
-- definitions or historical pipeline runs.

ALTER TABLE contract_variations
  ADD COLUMN IF NOT EXISTS source_construction_action_id UUID
  REFERENCES construction_action_items(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_contract_variations_source_construction_action
  ON contract_variations(source_construction_action_id,created_at DESC)
  WHERE source_construction_action_id IS NOT NULL;

INSERT INTO permissions (id,key,name,description,module)
VALUES (
  gen_random_uuid(),
  'contract.variation.propose',
  'Đề xuất phát sinh thi công',
  'Đề xuất phát sinh ngoài kế hoạch từ không gian thi công để QS rà soát',
  'contract'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON permission.key='contract.variation.propose'
WHERE role.name IN ('admin','pm','supervisor')
ON CONFLICT DO NOTHING;
