-- Resolve stale action-required notifications whose recipients cannot decide the
-- current pipeline-node approval stage. Keep the rows for notification history.
WITH pipeline_node_notifications AS (
  SELECT
    notification.id AS notification_id,
    notification.recipient_user_id,
    request.id AS approval_request_id,
    request.project_id,
    request.requested_by,
    request.status AS request_status,
    request.policy_json,
    request.required_roles,
    request.current_stage,
    project.status AS project_status,
    project.created_by,
    account.status AS account_status,
    account.deleted_at AS account_deleted_at,
    CASE
      WHEN request.policy_json->>'mode' = 'ordered'
       AND jsonb_typeof(request.policy_json->'stages'->request.current_stage->'roles') = 'array'
        THEN request.policy_json->'stages'->request.current_stage->'roles'
      WHEN request.policy_json->>'mode' IN ('any', 'all', 'quorum')
       AND jsonb_typeof(request.policy_json->'roles') = 'array'
        THEN request.policy_json->'roles'
      WHEN jsonb_typeof(request.required_roles) = 'array'
        THEN request.required_roles
      ELSE '[]'::jsonb
    END AS stage_roles,
    CASE
      WHEN request.policy_json->>'mode' IN ('any', 'all', 'quorum', 'ordered')
        THEN COALESCE((request.policy_json->>'requester_cannot_approve')::boolean, false)
      ELSE true
    END AS requester_cannot_approve
  FROM user_notifications notification
  JOIN approval_requests request
    ON notification.thread_key = 'pipeline_node_approval:' || request.id::text
   AND request.target_type = 'pipeline_run_node'
  LEFT JOIN projects project ON project.id = request.project_id
  LEFT JOIN users account ON account.id = notification.recipient_user_id
  WHERE notification.event_key = 'workflow.node_approval_requested'
    AND notification.intent = 'action_required'
    AND notification.resolved_at IS NULL
), invalid_notifications AS (
  SELECT projected.notification_id, projected.recipient_user_id
  FROM pipeline_node_notifications projected
  WHERE projected.request_status <> 'pending'
     OR projected.project_status IS NULL
     OR projected.project_status = 'cancelled'
     OR projected.account_status <> 'active'
     OR projected.account_deleted_at IS NOT NULL
     OR (
       projected.requester_cannot_approve
       AND projected.requested_by = projected.recipient_user_id
     )
     OR EXISTS (
       SELECT 1
       FROM approval_actions prior_action
       WHERE prior_action.approval_request_id = projected.approval_request_id
         AND prior_action.user_id = projected.recipient_user_id
         AND prior_action.action = 'approve'
     )
     OR NOT (
       EXISTS (
         SELECT 1
         FROM user_roles privileged_assignment
         JOIN roles privileged_role ON privileged_role.id = privileged_assignment.role_id
         WHERE privileged_assignment.user_id = projected.recipient_user_id
           AND privileged_assignment.project_id IS NULL
           AND privileged_role.name IN ('admin', 'director')
       )
       OR projected.created_by = projected.recipient_user_id
       OR EXISTS (
         SELECT 1
         FROM staff person
         JOIN project_members member ON member.staff_id = person.id AND member.active
         WHERE person.user_id = projected.recipient_user_id
           AND person.status = 'active'
           AND member.project_id = projected.project_id
       )
       OR EXISTS (
         SELECT 1
         FROM user_roles project_assignment
         WHERE project_assignment.user_id = projected.recipient_user_id
           AND project_assignment.project_id = projected.project_id
       )
     )
     OR jsonb_typeof(projected.stage_roles) <> 'array'
     OR jsonb_array_length(projected.stage_roles) = 0
     OR NOT EXISTS (
       SELECT 1
       FROM jsonb_array_elements_text(projected.stage_roles) required_role(role_label)
       WHERE lower(btrim(required_role.role_label)) IN (
         SELECT lower(btrim(actor_role.role_label))
         FROM (
           SELECT role.name AS role_label
           FROM user_roles assignment
           JOIN roles role ON role.id = assignment.role_id
           WHERE assignment.user_id = projected.recipient_user_id
             AND assignment.project_id IS NULL
           UNION
           SELECT role.display_name AS role_label
           FROM user_roles assignment
           JOIN roles role ON role.id = assignment.role_id
           WHERE assignment.user_id = projected.recipient_user_id
             AND assignment.project_id IS NULL
           UNION
           SELECT role.id::text AS role_label
           FROM user_roles assignment
           JOIN roles role ON role.id = assignment.role_id
           WHERE assignment.user_id = projected.recipient_user_id
             AND assignment.project_id IS NULL
           UNION
           SELECT jsonb_array_elements_text(member.role_ids) AS role_label
           FROM staff person
           JOIN project_members member ON member.staff_id = person.id AND member.active
           WHERE person.user_id = projected.recipient_user_id
             AND person.status = 'active'
             AND member.project_id = projected.project_id
           UNION
           SELECT role.name AS role_label
           FROM user_roles assignment
           JOIN roles role ON role.id = assignment.role_id
           WHERE assignment.user_id = projected.recipient_user_id
             AND assignment.project_id = projected.project_id
           UNION
           SELECT role.display_name AS role_label
           FROM user_roles assignment
           JOIN roles role ON role.id = assignment.role_id
           WHERE assignment.user_id = projected.recipient_user_id
             AND assignment.project_id = projected.project_id
           UNION
           SELECT role.id::text AS role_label
           FROM user_roles assignment
           JOIN roles role ON role.id = assignment.role_id
           WHERE assignment.user_id = projected.recipient_user_id
             AND assignment.project_id = projected.project_id
         ) actor_role
         WHERE NULLIF(btrim(actor_role.role_label), '') IS NOT NULL
       )
     )
), resolved_notifications AS (
  UPDATE user_notifications notification
  SET resolved_at = now()
  FROM invalid_notifications invalid
  WHERE notification.id = invalid.notification_id
    AND notification.resolved_at IS NULL
  RETURNING notification.recipient_user_id
), affected_recipients AS (
  SELECT DISTINCT recipient_user_id
  FROM resolved_notifications
)
INSERT INTO notification_actor_revisions (user_id, revision, updated_at)
SELECT recipient_user_id, 1, now()
FROM affected_recipients
ON CONFLICT (user_id) DO UPDATE
SET revision = notification_actor_revisions.revision + 1,
    updated_at = now();
