-- Roll back restored pre-seed pipeline template 75151291-ae2e-449e-83f7-57999a43baf0
BEGIN;
DELETE FROM pipeline_templates WHERE id='75151291-ae2e-449e-83f7-57999a43baf0';
COMMIT;
