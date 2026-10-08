-- Deliberate local demo bootstrap after the current migration chain.
-- Workflow definitions/forms/RBAC come from migrations; seed never replays
-- applied migrations or creates completed projects, payments or ledger entries.
BEGIN;

DO $$
BEGIN
  IF to_regclass('accounting_books') IS NOT NULL THEN
    RAISE EXCEPTION 'Run current migrations before seeding; legacy ledger schema is still present';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pipeline_templates
    WHERE id='40000000-0000-0000-0000-000000000028' AND status='published') THEN
    RAISE EXCEPTION 'Current published workflow Rev9 is required before seeding';
  END IF;
END $$;

INSERT INTO users (id, email, password_hash, full_name, status) VALUES
('10000000-0000-0000-0000-000000000001','admin@consflow.local','demo:123456','Admin hệ thống','active'),
('10000000-0000-0000-0000-000000000002','pm@consflow.local','demo:123456','Nguyễn Minh PM','active'),
('10000000-0000-0000-0000-000000000003','qs@consflow.local','demo:123456','Trần Hà QS','active'),
('10000000-0000-0000-0000-000000000004','accounting@consflow.local','demo:123456','Ngân Kế Toán','active'),
('10000000-0000-0000-0000-000000000005','warehouse@consflow.local','demo:123456','Phạm Long Thủ kho','active'),
('10000000-0000-0000-0000-000000000006','supervisor@consflow.local','demo:123456','Đặng Khoa Giám sát','active'),
('10000000-0000-0000-0000-000000000007','director@consflow.local','demo:123456','Hoàng Long','active'),
('10000000-0000-0000-0000-000000000008','procurement@consflow.local','demo:123456','Ngô Lan Mua hàng','active'),
('10000000-0000-0000-0000-000000000009','qaqc@consflow.local','demo:123456','Phan Huy QA/QC','active'),
('10000000-0000-0000-0000-000000000010','construction@consflow.local','demo:123456','Đội trưởng Nguyễn Tín','active'),
('10000000-0000-0000-0000-000000000011','subcontractor@consflow.local','demo:123456','Đại diện Thầu phụ An Tâm','active'),
('10000000-0000-0000-0000-000000000012','client@consflow.local','demo:123456','Anh Bình Chủ đầu tư','inactive'),
('10000000-0000-0000-0000-000000000013','design@consflow.local','demo:123456','Nguyễn Hà Thiết kế','active'),
('10000000-0000-0000-0000-000000000014','sales@consflow.local','demo:123456','Trần Vy Sale','active'),
('10000000-0000-0000-0000-000000000015','legal@consflow.local','demo:123456','Lê Quang Pháp lý','active'),
('10000000-0000-0000-0000-000000000016','contract@consflow.local','demo:123456','Vũ An Hợp đồng','active'),
('10000000-0000-0000-0000-000000000017','developer@consflow.local','demo:123456','Nhà phát triển hệ thống','active')
ON CONFLICT (id) DO NOTHING;

INSERT INTO staff (id, user_id, full_name, email, phone, department, position, status) VALUES
('21000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','Admin hệ thống','admin@consflow.local','0900000001','Công nghệ','Admin','active'),
('21000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','Nguyễn Minh PM','pm@consflow.local','0900000002','Dự án','Project Manager','active'),
('21000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000003','Trần Hà QS','qs@consflow.local','0900000003','QS','Quantity Surveyor','active'),
('21000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004','Ngân Kế Toán','accounting@consflow.local','0900000004','Kế toán','Accountant','active'),
('21000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005','Phạm Long Thủ kho','warehouse@consflow.local','0900000005','Kho','Warehouse Lead','active'),
('21000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','Đặng Khoa Giám sát','supervisor@consflow.local','0900000006','Thi công','Site Supervisor','active'),
('21000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','Hoàng Long','director@consflow.local','0900000007','Ban giám đốc','Director','active'),
('21000000-0000-0000-0000-000000000008','10000000-0000-0000-0000-000000000008','Ngô Lan Mua hàng','procurement@consflow.local','0900000008','Mua hàng','Procurement Lead','active'),
('21000000-0000-0000-0000-000000000009','10000000-0000-0000-0000-000000000009','Phan Huy QA/QC','qaqc@consflow.local','0900000009','QA/QC','Quality Engineer','active'),
('21000000-0000-0000-0000-000000000010','10000000-0000-0000-0000-000000000010','Đội trưởng Nguyễn Tín','construction@consflow.local','0900000010','Thi công','Construction Team Lead','active'),
('21000000-0000-0000-0000-000000000011','10000000-0000-0000-0000-000000000011','Đại diện Thầu phụ An Tâm','subcontractor@consflow.local','0900000011','Thầu phụ','Subcontractor Representative','active'),
('21000000-0000-0000-0000-000000000012','10000000-0000-0000-0000-000000000012','Anh Bình Chủ đầu tư','client@consflow.local','0900000012','Chủ đầu tư','Client Representative','inactive'),
('21000000-0000-0000-0000-000000000013','10000000-0000-0000-0000-000000000013','Nguyễn Hà Thiết kế','design@consflow.local','0900000013','Thiết kế','Design Lead','active'),
('21000000-0000-0000-0000-000000000014','10000000-0000-0000-0000-000000000014','Trần Vy Sale','sales@consflow.local','0900000014','Kinh doanh','Sales Lead','active'),
('21000000-0000-0000-0000-000000000015','10000000-0000-0000-0000-000000000015','Lê Quang Pháp lý','legal@consflow.local','0900000015','Pháp lý','Legal Reviewer','active'),
('21000000-0000-0000-0000-000000000016','10000000-0000-0000-0000-000000000016','Vũ An Hợp đồng','contract@consflow.local','0900000016','Hợp đồng','Contract Officer','active'),
('21000000-0000-0000-0000-000000000017','10000000-0000-0000-0000-000000000017','Nhà phát triển hệ thống','developer@consflow.local','0900000017','Công nghệ','Developer','active')
ON CONFLICT (id) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u JOIN roles r ON (
  (u.email='admin@consflow.local' AND r.name='admin') OR
  (u.email='developer@consflow.local' AND r.name='developer') OR
  (u.email='pm@consflow.local' AND r.name='pm') OR
  (u.email='qs@consflow.local' AND r.name='qs') OR
  (u.email='accounting@consflow.local' AND r.name='accounting') OR
  (u.email='warehouse@consflow.local' AND r.name='warehouse') OR
  (u.email='supervisor@consflow.local' AND r.name='supervisor') OR
  (u.email='director@consflow.local' AND r.name='director') OR
  (u.email='procurement@consflow.local' AND r.name='procurement') OR
  (u.email='qaqc@consflow.local' AND r.name='qa_qc') OR
  (u.email='construction@consflow.local' AND r.name='construction_team') OR
  (u.email='subcontractor@consflow.local' AND r.name='subcontractor') OR
  (u.email='client@consflow.local' AND r.name='client') OR
  (u.email='design@consflow.local' AND r.name='design') OR
  (u.email='sales@consflow.local' AND r.name='sales') OR
  (u.email='legal@consflow.local' AND r.name='legal') OR
  (u.email='contract@consflow.local' AND r.name='contract_officer'))
WHERE NOT EXISTS (SELECT 1 FROM user_roles grant_row
  WHERE grant_row.user_id=u.id AND grant_row.role_id=r.id AND grant_row.project_id IS NULL);

INSERT INTO partners (id, partner_code, name, partner_types, tax_code, contact_name, phone, address, payment_terms_days, notes) VALUES
('90000000-0000-0000-0000-000000000001','PT-ANPHAT','Công ty An Phát','["client"]','0312345678','Anh Bình','0909000001','Quận 1, TP.HCM',30,'Chủ đầu tư demo'),
('90000000-0000-0000-0000-000000000002','PT-ANHMINH','Anh Minh','["client"]',NULL,'Anh Minh','0909000002','TP. Thủ Đức, TP.HCM',15,'Khách hàng cá nhân demo'),
('90000000-0000-0000-0000-000000000003','PT-CHIHANH','Chị Hạnh','["client"]',NULL,'Chị Hạnh','0909000003','Masteri An Phú, TP.HCM',15,'Khách hàng cá nhân demo'),
('90000000-0000-0000-0000-000000000101','PT-CADIVI','Cadivi','["supplier"]','0300427407','Sale Cadivi','0909100001','TP.HCM',30,'Nhà cung cấp dây điện'),
('90000000-0000-0000-0000-000000000102','PT-BINHMINH','Bình Minh','["supplier"]','0301464823','Sale Bình Minh','0909100002','TP.HCM',30,'Nhà cung cấp ống nước'),
('90000000-0000-0000-0000-000000000103','PT-DULUX','Dulux','["supplier"]',NULL,'Sale Dulux','0909100003','TP.HCM',30,'Nhà cung cấp sơn'),
('90000000-0000-0000-0000-000000000104','PT-VINHTUONG','Vĩnh Tường','["supplier"]',NULL,'Sale Vĩnh Tường','0909100004','TP.HCM',30,'Nhà cung cấp trần thạch cao'),
('90000000-0000-0000-0000-000000000105','PT-PRIME','Prime','["supplier"]',NULL,'Sale Prime','0909100005','TP.HCM',30,'Nhà cung cấp gạch'),
('90000000-0000-0000-0000-000000000106','PT-RANGDONG','Rạng Đông','["supplier"]',NULL,'Sale Rạng Đông','0909100006','TP.HCM',30,'Nhà cung cấp đèn'),
('90000000-0000-0000-0000-000000000107','PT-TONGHOP','Nhà cung cấp tổng hợp','["supplier","service_provider"]',NULL,'Sale tổng hợp','0909100007','TP.HCM',15,'Nhà cung cấp phụ kiện'),
('90000000-0000-0000-0000-000000000108','PT-APOLLO','Apollo','["supplier"]',NULL,'Sale Apollo','0909100008','TP.HCM',30,'Nhà cung cấp keo'),
('90000000-0000-0000-0000-000000000201','PT-ANTAM','Thầu phụ điện An Tâm','["subcontractor"]',NULL,'Anh Tâm','0909200001','TP.HCM',20,'Thầu phụ điện'),
('90000000-0000-0000-0000-000000000202','PT-MINHLONG','Đội sơn Minh Long','["labor_team","subcontractor"]',NULL,'Anh Long','0909200002','TP.HCM',15,'Đội sơn demo')
ON CONFLICT (id) DO NOTHING;

INSERT INTO materials (id, code, name, unit, category, default_supplier_partner_id, default_supplier, standard_price) VALUES
('50000000-0000-0000-0000-000000000001','MAT-CADIVI','Dây điện Cadivi','m','Điện','90000000-0000-0000-0000-000000000101','Cadivi',12000),
('50000000-0000-0000-0000-000000000002','MAT-PVC-BM','Ống PVC Bình Minh','m','Nước','90000000-0000-0000-0000-000000000102','Bình Minh',28000),
('50000000-0000-0000-0000-000000000003','MAT-DULUX','Sơn Dulux','thùng','Sơn','90000000-0000-0000-0000-000000000103','Dulux',1450000),
('50000000-0000-0000-0000-000000000004','MAT-GYPSUM','Tấm thạch cao','tấm','Trần','90000000-0000-0000-0000-000000000104','Vĩnh Tường',165000),
('50000000-0000-0000-0000-000000000005','MAT-TILE','Gạch lát nền','m2','Sàn','90000000-0000-0000-0000-000000000105','Prime',320000),
('50000000-0000-0000-0000-000000000006','MAT-LED','Đèn LED âm trần','cái','Điện','90000000-0000-0000-0000-000000000106','Rạng Đông',185000),
('50000000-0000-0000-0000-000000000007','MAT-SCREW','Vít, tắc kê','kg','Phụ kiện','90000000-0000-0000-0000-000000000107','Nhà cung cấp tổng hợp',65000),
('50000000-0000-0000-0000-000000000008','MAT-SILICONE','Keo silicone','chai','Phụ kiện','90000000-0000-0000-0000-000000000108','Apollo',78000)
ON CONFLICT (id) DO NOTHING;

INSERT INTO warehouses (id, name, location, type) VALUES
('51000000-0000-0000-0000-000000000001','Kho trung tâm Bình Tân','Bình Tân, TP.HCM','central'),
('51000000-0000-0000-0000-000000000002','Kho site Fit-out Q1','Quận 1, TP.HCM','site')
ON CONFLICT (id) DO NOTHING;

COMMIT;
