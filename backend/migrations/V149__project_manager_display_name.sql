-- Rename current catalog presentation; never rewrite grants, assignments or
-- historical template/run/report/form/audit snapshots.
UPDATE roles SET display_name='Quản lý dự án'
WHERE name='pm' AND system_role AND NOT external_only;

-- Old workflow labels and newly selected member labels identify the same PM.
-- Preserve exact comparison behavior for every other role label.
CREATE FUNCTION consflow_role_label_key(label text) RETURNS text
LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE AS $$
  SELECT CASE WHEN lower(btrim(label)) IN ('pm','project manager','quản lý dự án')
    THEN 'pm' ELSE label END
$$;
