ALTER TABLE pipeline_template_nodes
  ADD COLUMN IF NOT EXISTS permissions JSONB NOT NULL DEFAULT '[]';

ALTER TABLE pipeline_run_nodes
  ADD COLUMN IF NOT EXISTS permissions JSONB NOT NULL DEFAULT '[]';

UPDATE pipeline_template_nodes
SET permissions = jsonb_build_array(required_permission)
WHERE required_permission IS NOT NULL
  AND btrim(required_permission) <> ''
  AND permissions = '[]'::jsonb;

UPDATE pipeline_run_nodes
SET permissions = jsonb_build_array(required_permission)
WHERE required_permission IS NOT NULL
  AND btrim(required_permission) <> ''
  AND permissions = '[]'::jsonb;
