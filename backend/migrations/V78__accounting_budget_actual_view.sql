CREATE OR REPLACE VIEW accounting_budget_balances AS
SELECT
  b.id,
  b.project_id,
  b.budget_code,
  b.name,
  b.planned_amount,
  b.approved_amount,
  COALESCE(SUM(m.commitment_delta),0)::numeric(18,2) AS committed_amount,
  COALESCE(SUM(m.actual_delta),0)::numeric(18,2) AS actual_amount,
  COALESCE(SUM(m.paid_delta),0)::numeric(18,2) AS paid_amount,
  (
    b.approved_amount-COALESCE(SUM(m.commitment_delta),0)-COALESCE(SUM(m.actual_delta),0)
  )::numeric(18,2) AS available_amount,
  COALESCE(SUM(m.actual_delta),0)::numeric(18,2) AS used_amount,
  (
    b.approved_amount-COALESCE(SUM(m.commitment_delta),0)-COALESCE(SUM(m.actual_delta),0)
  )::numeric(18,2) AS remaining_amount,
  b.status,
  b.approved_by,
  b.created_at,
  b.updated_at,
  b.accounting_book_id,
  (
    b.approved_amount-COALESCE(SUM(m.actual_delta),0)
  )::numeric(18,2) AS actual_variance_amount,
  CASE
    WHEN b.approved_amount=0 THEN 0
    ELSE round(COALESCE(SUM(m.actual_delta),0)*100/b.approved_amount,2)
  END::numeric(9,2) AS actual_utilization_percent
FROM accounting_budgets b
LEFT JOIN budget_movements m ON m.budget_id=b.id
GROUP BY b.id;
