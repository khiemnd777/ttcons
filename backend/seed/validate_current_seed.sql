DO $$
DECLARE
  missing integer;
BEGIN
  IF to_regclass('accounting_books') IS NOT NULL OR to_regclass('journal_entries') IS NOT NULL THEN
    RAISE EXCEPTION 'Retired internal accounting tables must not exist';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pipeline_templates
    WHERE id='40000000-0000-0000-0000-000000000028' AND status='published') THEN
    RAISE EXCEPTION 'Current workflow Rev9 is not published';
  END IF;
  IF EXISTS (SELECT 1 FROM pipeline_templates WHERE status='published'
    AND id IN ('40000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000010',
      '40000000-0000-0000-0000-000000000020','40000000-0000-0000-0000-000000000021',
      '40000000-0000-0000-0000-000000000022','40000000-0000-0000-0000-000000000023',
      '40000000-0000-0000-0000-000000000024','40000000-0000-0000-0000-000000000025',
      '40000000-0000-0000-0000-000000000026','40000000-0000-0000-0000-000000000027')) THEN
    RAISE EXCEPTION 'Retired official workflow revisions must not be available';
  END IF;
  SELECT count(*) INTO missing FROM (VALUES
    ('admin@consflow.local','admin'),('developer@consflow.local','developer'),
    ('pm@consflow.local','pm'),('accounting@consflow.local','accounting'),
    ('procurement@consflow.local','procurement'),('qs@consflow.local','qs')
  ) expected(email,role_name)
  WHERE NOT EXISTS (SELECT 1 FROM users account JOIN staff profile ON profile.user_id=account.id
    JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
    JOIN roles role ON role.id=assignment.role_id
    WHERE account.email=expected.email AND role.name=expected.role_name);
  IF missing<>0 THEN RAISE EXCEPTION 'Seed is missing % canonical demo identities',missing; END IF;
  IF NOT EXISTS (SELECT 1 FROM materials WHERE code='MAT-CADIVI')
    OR NOT EXISTS (SELECT 1 FROM partners WHERE partner_code='PT-ANPHAT') THEN
    RAISE EXCEPTION 'Seed master catalogs are incomplete';
  END IF;
END $$;
