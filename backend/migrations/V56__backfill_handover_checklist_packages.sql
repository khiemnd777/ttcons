INSERT INTO handover_checklist_items (
  handover_id,project_id,item_code,category,title,required,status,source_type,source_id
)
SELECT
  h.id,h.project_id,item.item_code,item.category,item.title,true,'pending',item.source_type,
  CASE WHEN item.item_code='final_acceptance' THEN h.final_acceptance_id
       WHEN item.source_type='system_check' THEN h.id
       ELSE NULL END
FROM handover_records h
CROSS JOIN (VALUES
  ('final_acceptance','quality','Biên bản nghiệm thu cuối đạt','acceptance'),
  ('warehouse_closure','operation','Kho công trình và vật tư dư đã đóng','system_check'),
  ('ap_reconciliation','finance','Công nợ đầu vào đã đối soát/tất toán','system_check'),
  ('ar_reconciliation','finance','Công nợ đầu ra đã đối soát/tất toán','system_check'),
  ('as_built_documents','as_built','Bản vẽ và hồ sơ hoàn công',NULL),
  ('operation_manuals','operation','Hướng dẫn vận hành/bảo trì',NULL),
  ('warranty_documents','warranty','Cam kết và hồ sơ bảo hành',NULL),
  ('signed_handover_minutes','legal','Biên bản bàn giao có xác nhận hai bên',NULL)
) AS item(item_code,category,title,source_type)
WHERE h.status <> 'deleted'
ON CONFLICT (handover_id,item_code) DO NOTHING;
