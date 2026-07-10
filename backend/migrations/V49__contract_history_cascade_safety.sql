-- Signed versions and incorporated/rejected variations are immutable through UPDATE.
-- DELETE remains available only for parent-project cascade cleanup; the API exposes no
-- direct delete route for these history rows.
DROP TRIGGER IF EXISTS trg_contract_version_immutable ON contract_versions;
CREATE TRIGGER trg_contract_version_immutable
BEFORE UPDATE ON contract_versions
FOR EACH ROW EXECUTE FUNCTION prevent_final_contract_version_mutation();

DROP TRIGGER IF EXISTS trg_contract_variation_immutable ON contract_variations;
CREATE TRIGGER trg_contract_variation_immutable
BEFORE UPDATE ON contract_variations
FOR EACH ROW EXECUTE FUNCTION prevent_final_contract_variation_mutation();
