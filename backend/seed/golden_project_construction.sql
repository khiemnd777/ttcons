\set ON_ERROR_STOP on

BEGIN;

DELETE FROM construction_action_items WHERE project_id='30000000-0000-0000-0000-000000000020';
DELETE FROM construction_checks WHERE project_id='30000000-0000-0000-0000-000000000020';
DELETE FROM construction_check_templates WHERE project_id='30000000-0000-0000-0000-000000000020';

CREATE TEMP TABLE golden_construction_context AS
WITH selected_work_item AS (
  SELECT w.id
  FROM work_items w
  JOIN pipeline_run_steps rs ON rs.id=w.run_step_id
  WHERE w.project_id='30000000-0000-0000-0000-000000000020' AND rs.surface_kind='quality'
  ORDER BY w.id
  LIMIT 1
)
SELECT
  sw.id AS work_item_id,
  evidence.id AS request_evidence_id,
  evidence.id AS inspection_evidence_id,
  evidence.id AS corrective_evidence_id
FROM selected_work_item sw
CROSS JOIN LATERAL (
  SELECT e.id
  FROM evidence_records e
  WHERE e.project_id='30000000-0000-0000-0000-000000000020'
    AND e.work_item_id=sw.id AND e.status='accepted'
  ORDER BY e.created_at,e.id
  LIMIT 1
) evidence;

INSERT INTO construction_check_templates (
  id,project_id,name,description,criteria_jsonb,status,technical_itp_id,created_by,created_at,updated_at
) VALUES (
  '86600000-0000-0000-0000-000000000201',
  '30000000-0000-0000-0000-000000000020',
  'Kiểm tra hoàn thiện trần',
  'Mẫu ngắn dùng trực tiếp tại hiện trường.',
  '[
    {"title":"Cao độ và độ phẳng","criteria":"Sai số không quá 3mm","evidence_required":true},
    {"title":"Bề mặt hoàn thiện","criteria":"Không cong vênh, nứt hoặc hở mối nối","evidence_required":true},
    {"title":"Vệ sinh khu vực","criteria":"Không còn vật tư thừa và rác thi công","evidence_required":false}
  ]'::jsonb,
  'active',
  '86000000-0000-0000-0000-000000000202',
  '10000000-0000-0000-0000-000000000006',
  '2026-03-04 10:00+07',
  '2026-03-04 10:00+07'
);

INSERT INTO construction_checks (
  id,project_id,work_item_id,template_id,title,location,criteria_jsonb,scheduled_at,status,result,note,
  evidence_id,requested_by,checked_by,checked_at,technical_rfi_id,created_at,updated_at
) SELECT
  x.id,'30000000-0000-0000-0000-000000000020',ctx.work_item_id,
  '86600000-0000-0000-0000-000000000201',x.title,x.location,x.criteria_jsonb,x.scheduled_at,
  x.status,x.result,x.note,x.evidence_id,x.requested_by,x.checked_by,x.checked_at,x.technical_rfi_id,x.created_at,x.updated_at
FROM golden_construction_context ctx
CROSS JOIN LATERAL (VALUES
  (
    '86700000-0000-0000-0000-000000000201'::uuid,
    'Kiểm tra MEP trước đóng trần','Khu A',
    '[{"title":"MEP trước đóng trần","criteria":"Đúng bản vẽ và đủ test continuity","evidence_required":true}]'::jsonb,
    '2026-03-20 09:00+07'::timestamptz,'completed','pass','Đạt theo bản vẽ và test continuity.',
    ctx.inspection_evidence_id,'10000000-0000-0000-0000-000000000006'::uuid,'10000000-0000-0000-0000-000000000002'::uuid,
    '2026-03-20 10:00+07'::timestamptz,'86300000-0000-0000-0000-000000000201'::uuid,
    '2026-03-19 16:00+07'::timestamptz,'2026-03-20 10:00+07'::timestamptz
  ),
  (
    '86700000-0000-0000-0000-000000000202'::uuid,
    'Kiểm tra cao độ trần','Khu B',
    '[{"title":"Cao độ và độ phẳng","criteria":"Sai số không quá 3mm","evidence_required":true}]'::jsonb,
    '2026-04-10 09:00+07'::timestamptz,'completed','fail','Không đạt: lệch cao độ 8mm.',
    ctx.inspection_evidence_id,'10000000-0000-0000-0000-000000000006'::uuid,'10000000-0000-0000-0000-000000000002'::uuid,
    '2026-04-10 10:00+07'::timestamptz,'86300000-0000-0000-0000-000000000202'::uuid,
    '2026-04-09 16:00+07'::timestamptz,'2026-04-10 10:00+07'::timestamptz
  ),
  (
    '86700000-0000-0000-0000-000000000203'::uuid,
    'Tái kiểm cao độ trần sau khắc phục','Khu B',
    '[{"title":"Cao độ và độ phẳng","criteria":"Sai số không quá 3mm","evidence_required":true}]'::jsonb,
    '2026-04-16 09:00+07'::timestamptz,'completed','pass','Đạt: sai số còn 2mm, trong dung sai.',
    ctx.corrective_evidence_id,'10000000-0000-0000-0000-000000000006'::uuid,'10000000-0000-0000-0000-000000000002'::uuid,
    '2026-04-16 10:00+07'::timestamptz,'86300000-0000-0000-0000-000000000203'::uuid,
    '2026-04-15 16:00+07'::timestamptz,'2026-04-16 10:00+07'::timestamptz
  ),
  (
    '86700000-0000-0000-0000-000000000204'::uuid,
    'Kiểm tra hoàn thiện trần khu Pantry','Pantry tầng 12',
    '[
      {"title":"Cao độ và độ phẳng","criteria":"Sai số không quá 3mm","evidence_required":true},
      {"title":"Bề mặt hoàn thiện","criteria":"Không cong vênh, nứt hoặc hở mối nối","evidence_required":true}
    ]'::jsonb,
    '2026-06-12 09:00+07'::timestamptz,'pending',NULL,NULL,
    NULL,'10000000-0000-0000-0000-000000000006'::uuid,NULL,NULL,NULL,
    '2026-06-11 16:00+07'::timestamptz,'2026-06-11 16:00+07'::timestamptz
  )
) AS x(id,title,location,criteria_jsonb,scheduled_at,status,result,note,evidence_id,requested_by,checked_by,checked_at,technical_rfi_id,created_at,updated_at);

INSERT INTO construction_check_evidence (
  check_id,evidence_id,evidence_role,sort_order,created_by,created_at
) SELECT
  c.id,c.evidence_id,'result',0,COALESCE(c.checked_by,c.requested_by),COALESCE(c.checked_at,c.created_at)
FROM construction_checks c
WHERE c.project_id='30000000-0000-0000-0000-000000000020' AND c.evidence_id IS NOT NULL;

INSERT INTO construction_check_evidence (
  check_id,evidence_id,evidence_role,sort_order,created_by,created_at
) SELECT
  '86700000-0000-0000-0000-000000000204',ctx.request_evidence_id,'context',0,
  '10000000-0000-0000-0000-000000000006','2026-06-11 16:00+07'
FROM golden_construction_context ctx;

INSERT INTO construction_action_items (
  id,project_id,work_item_id,source_check_id,title,description,priority,assigned_user_id,due_at,status,
  impact_scope,before_evidence_id,resolution_evidence_id,resolution_note,submitted_by,submitted_at,
  verified_by,verified_at,retry_count,technical_issue_id,technical_ncr_id,technical_rework_id,created_by,created_at,updated_at
) SELECT
  '86800000-0000-0000-0000-000000000201',
  '30000000-0000-0000-0000-000000000020',ctx.work_item_id,
  '86700000-0000-0000-0000-000000000202',
  'Điều chỉnh cao độ trần khu B',
  'Cân chỉnh ty treo và đo lại toàn bộ lưới trần.',
  'urgent','10000000-0000-0000-0000-000000000006','2026-04-15 17:00+07','done',
  'none',ctx.inspection_evidence_id,ctx.corrective_evidence_id,
  'Đã cân chỉnh khung; sai số sau xử lý còn 2mm.',
  '10000000-0000-0000-0000-000000000006','2026-04-15 16:00+07',
  '10000000-0000-0000-0000-000000000002','2026-04-16 10:00+07',0,
  '70000000-0000-0000-0000-000000000201','86400000-0000-0000-0000-000000000201','72000000-0000-0000-0000-000000000201',
  '10000000-0000-0000-0000-000000000006','2026-04-10 10:00+07','2026-04-16 10:00+07'
FROM golden_construction_context ctx;

INSERT INTO construction_action_events (
  id,action_id,event_type,comment,evidence_id,actor_id,created_at
) SELECT events.*
FROM golden_construction_context ctx
CROSS JOIN LATERAL (VALUES
  ('86900000-0000-0000-0000-000000000201'::uuid,'86800000-0000-0000-0000-000000000201'::uuid,'created','Cao độ trần lệch 8mm.',ctx.inspection_evidence_id,'10000000-0000-0000-0000-000000000006'::uuid,'2026-04-10 10:00+07'::timestamptz),
  ('86900000-0000-0000-0000-000000000202'::uuid,'86800000-0000-0000-0000-000000000201'::uuid,'resolution_submitted','Đã cân chỉnh khung và đo lại.',ctx.corrective_evidence_id,'10000000-0000-0000-0000-000000000006'::uuid,'2026-04-15 16:00+07'::timestamptz),
  ('86900000-0000-0000-0000-000000000203'::uuid,'86800000-0000-0000-0000-000000000201'::uuid,'verified_pass','Đạt; sai số còn 2mm.',ctx.corrective_evidence_id,'10000000-0000-0000-0000-000000000002'::uuid,'2026-04-16 10:00+07'::timestamptz)
) events(id,action_id,event_type,comment,evidence_id,actor_id,created_at);

COMMIT;
