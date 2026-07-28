-- PM is an explicit actor on official planning/workflow steps whose runtime
-- permission gate is planning.edit. Keep the grant project-scoped through the
-- existing user-role and project-membership authorization path.
INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r
JOIN permissions p ON p.key IN ('planning.view','planning.edit')
WHERE r.name='pm'
ON CONFLICT DO NOTHING;
