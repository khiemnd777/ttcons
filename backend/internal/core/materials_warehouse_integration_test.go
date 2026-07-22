package core

import (
	"context"
	"fmt"
	"testing"
)

func TestBatch7StockMovementGuardsWarehouseAndSiteBalances(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	warehouseID := newID()
	materialID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE target_type='stock_movement' AND target_id IN (SELECT id FROM stock_movements WHERE material_id=$1)`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE project_id=$1 OR material_id=$2`, projectID, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Batch7 Actor','active')`, actorID, "batch7-stock-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch7 Stock',$2,'Client','planning')`, projectID, "B7S-"+projectID[:8])
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Batch7 Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Batch7 Material','m','Test',true)`, materialID, "B7M-"+materialID[:8])

	_, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     10,
		Unit:         "m",
		Reason:       "issue before receive",
	}, actorID, "Batch7 Actor")
	assertValidationError(t, err)

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "receive",
		Quantity:     5,
		Unit:         "m",
		Reason:       "receive stock",
	}, actorID, "Batch7 Actor"); err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     10,
		Unit:         "m",
		Reason:       "over issue",
	}, actorID, "Batch7 Actor")
	assertValidationError(t, err)

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     3,
		Unit:         "m",
		Reason:       "valid issue",
	}, actorID, "Batch7 Actor"); err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "return_to_warehouse",
		Quantity:     4,
		Unit:         "m",
		Reason:       "over return",
	}, actorID, "Batch7 Actor")
	assertValidationError(t, err)

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "return_to_warehouse",
		Quantity:     2,
		Unit:         "m",
		Reason:       "valid return",
	}, actorID, "Batch7 Actor"); err != nil {
		t.Fatal(err)
	}
	var issued, surplus, stock float64
	var surplusStatus string
	if err := db.QueryRowContext(ctx, `SELECT issued_quantity::float8, surplus_quantity::float8, surplus_status FROM site_material_balances WHERE project_id=$1 AND material_id=$2`, projectID, materialID).Scan(&issued, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 1 || surplus != 1 || surplusStatus != "partial" {
		t.Fatalf("site balance = issued %.3f surplus %.3f status %s, want 1/1/partial", issued, surplus, surplusStatus)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE
			WHEN movement_type IN ('receive','return_to_warehouse') THEN quantity
			WHEN movement_type IN ('issue','transfer','loss') THEN -quantity
			ELSE 0
		END),0)::float8
		FROM stock_movements
		WHERE warehouse_id=$1 AND material_id=$2
	`, warehouseID, materialID).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 4 {
		t.Fatalf("warehouse stock = %.3f, want 4", stock)
	}
}

func TestMaterialBeforeAfterSnapshotAppliesSiteBalanceWithoutStockMovement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	warehouseID := newID()
	materialID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1 AND target_type IN ('stock_movement','material_before_after_snapshot')`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM material_before_after_snapshot_items WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM material_before_after_snapshots WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE project_id=$1 OR material_id=$2`, fixture.projectID, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
	})
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Snapshot Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Snapshot Cable','m','Electrical',true)`, materialID, "SNAP-"+materialID[:8])

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "receive",
		Quantity:     20,
		Unit:         "m",
		Reason:       "receive for snapshot test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    fixture.projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     12,
		Unit:         "m",
		Reason:       "issue to site for snapshot test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	beforeEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Before material count",
		FileURL:      "/evidence/before-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	afterEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "After material count",
		FileURL:      "/evidence/after-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	beforeQty := 12.0
	addedQty := 0.0
	afterQty := 7.0
	wasteQty := 1.0
	affects := true
	req := MaterialSnapshotRequest{
		SnapshotDate:     "2026-07-14",
		ConstructionArea: "Tầng 3",
		Comment:          "Nháp kiểm kê vật tư trước/sau",
		Items: []MaterialSnapshotItemRequest{{
			ItemIndex:          0,
			MaterialID:         materialID,
			Source:             "site_balance",
			BeforeQuantity:     &beforeQty,
			AddedQuantity:      &addedQty,
			AfterQuantity:      &afterQty,
			WasteQuantity:      &wasteQty,
			AffectsSiteBalance: &affects,
			BeforeEvidenceID:   fmt.Sprint(beforeEvidence["id"]),
			AfterEvidenceID:    fmt.Sprint(afterEvidence["id"]),
			ConstructionArea:   "Tầng 3 - trục A",
			Condition:          "mixed",
			Comment:            "Đã dùng 4m, hư hao 1m",
		}},
	}

	saved, err := service.SaveMaterialSnapshot(ctx, fixture.itemID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	savedRecord, _ := saved["snapshot"].(Record)
	if savedRecord["status"] != "draft" {
		t.Fatalf("draft snapshot status = %#v, want draft", savedRecord["status"])
	}
	var used, surplus float64
	if err := db.QueryRowContext(ctx, `
		SELECT used_quantity::float8, surplus_quantity::float8
		FROM site_material_balances
		WHERE project_id=$1 AND material_id=$2
	`, fixture.projectID, materialID).Scan(&used, &surplus); err != nil {
		t.Fatal(err)
	}
	if used != 0 || surplus != 12 {
		t.Fatalf("draft changed balance used=%.3f surplus=%.3f, want 0/12", used, surplus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 2, materialID)

	req.Comment = "Chốt snapshot vật tư"
	submitted, err := service.SubmitMaterialSnapshot(ctx, fixture.itemID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	submittedRecord, _ := submitted["snapshot"].(Record)
	if submittedRecord["status"] != "submitted" || submittedRecord["applied_to_site_balance"] != true {
		t.Fatalf("submitted snapshot = %#v, want submitted/applied", submittedRecord)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT used_quantity::float8, surplus_quantity::float8
		FROM site_material_balances
		WHERE project_id=$1 AND material_id=$2
	`, fixture.projectID, materialID).Scan(&used, &surplus); err != nil {
		t.Fatal(err)
	}
	if used != 4 || surplus != 8 {
		t.Fatalf("submitted balance used=%.3f surplus=%.3f, want 4/8", used, surplus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 2, materialID)
	assertCount(t, db, `SELECT COUNT(*) FROM material_before_after_snapshot_items WHERE material_id=$1 AND used_quantity=4 AND before_evidence_id=$2 AND after_evidence_id=$3`, 1, materialID, beforeEvidence["id"], afterEvidence["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted' AND metadata_json ? 'material_snapshot_id'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='material_snapshot_submitted'`, 1, fixture.projectID)

	_, err = service.SaveMaterialSnapshot(ctx, fixture.itemID, req, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestSiteSurplusHandlingCreatesStockMovementAndPartialStatus(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	suffix := newID()
	surplusPermissionID := ensurePermission(t, db, "warehouse.surplus_handle", "Xử lý vật tư dư", "warehouse")
	handlerRoleID := ensureRole(t, db, "surplus_handler_"+suffix[:8], "Surplus Handler")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, handlerRoleID, surplusPermissionID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2)`, fixture.workerID, handlerRoleID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, handlerRoleID)
	})

	warehouseID := newID()
	materialID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1 AND target_type IN ('stock_movement','site_surplus_handling')`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_surplus_handlings WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE project_id=$1 OR material_id=$2`, fixture.projectID, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
	})
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Surplus Warehouse','Yard','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Surplus Cable','m','Electrical',true)`, materialID, "SUR-"+materialID[:8])

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "receive",
		Quantity:     20,
		Unit:         "m",
		Reason:       "receive for surplus test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    fixture.projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     12,
		Unit:         "m",
		Reason:       "issue to site for surplus test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `
		UPDATE site_material_balances
		SET used_quantity=4,
			surplus_quantity=8,
			surplus_status='pending'
		WHERE project_id=$1 AND material_id=$2
	`, fixture.projectID, materialID)

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Surplus material photo",
		FileURL:      "/evidence/surplus-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	var balanceID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM site_material_balances WHERE project_id=$1 AND material_id=$2`, fixture.projectID, materialID).Scan(&balanceID); err != nil {
		t.Fatal(err)
	}

	firstQty := 3.0
	draft, err := service.CreateSurplusHandling(ctx, fixture.itemID, SurplusHandlingRequest{
		SiteBalanceID:   balanceID,
		HandlingType:    "return_to_warehouse",
		Quantity:        &firstQty,
		Unit:            "m",
		WarehouseID:     warehouseID,
		CustodyLocation: "Kệ A3",
		Condition:       "usable",
		EvidenceID:      fmt.Sprint(evidence["id"]),
		Comment:         "Lưu lại vật tư dư",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	var issued, used, surplus float64
	var surplusStatus string
	if err := db.QueryRowContext(ctx, `
		SELECT issued_quantity::float8, used_quantity::float8, surplus_quantity::float8, surplus_status
		FROM site_material_balances
		WHERE id=$1
	`, balanceID).Scan(&issued, &used, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 12 || used != 4 || surplus != 8 || surplusStatus != "pending" {
		t.Fatalf("draft changed balance issued=%.3f used=%.3f surplus=%.3f status=%s", issued, used, surplus, surplusStatus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 2, materialID)

	submitted, err := service.SubmitSurplusHandling(ctx, fmt.Sprint(draft["id"]), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if submitted["status"] != "submitted" || cleanID(submitted["stock_movement_id"]) == "" {
		t.Fatalf("submitted surplus handling = %#v, want submitted with stock movement", submitted)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT issued_quantity::float8, used_quantity::float8, surplus_quantity::float8, surplus_status
		FROM site_material_balances
		WHERE id=$1
	`, balanceID).Scan(&issued, &used, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 9 || used != 4 || surplus != 5 || surplusStatus != "partial" {
		t.Fatalf("partial balance issued=%.3f used=%.3f surplus=%.3f status=%s, want 9/4/5/partial", issued, used, surplus, surplusStatus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1 AND movement_type='return_to_warehouse' AND reference_type='site_surplus_handling'`, 1, materialID)

	secondQty := 5.0
	second, err := service.CreateSurplusHandling(ctx, fixture.itemID, SurplusHandlingRequest{
		SiteBalanceID:   balanceID,
		HandlingType:    "return_to_subcontractor",
		Quantity:        &secondQty,
		Unit:            "m",
		WarehouseID:     warehouseID,
		CustodyLocation: "Bàn giao đội thầu phụ",
		Condition:       "mixed",
		EvidenceID:      fmt.Sprint(evidence["id"]),
		Comment:         "Trả lại phần còn dư",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SubmitSurplusHandling(ctx, fmt.Sprint(second["id"]), fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT issued_quantity::float8, used_quantity::float8, surplus_quantity::float8, surplus_status
		FROM site_material_balances
		WHERE id=$1
	`, balanceID).Scan(&issued, &used, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 4 || used != 4 || surplus != 0 || surplusStatus != "handled" {
		t.Fatalf("handled balance issued=%.3f used=%.3f surplus=%.3f status=%s, want 4/4/0/handled", issued, used, surplus, surplusStatus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1 AND movement_type='return_to_subcontractor' AND reference_type='site_surplus_handling'`, 1, materialID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted' AND metadata_json ? 'surplus_handling_id'`, 2, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='site_surplus_handling_submitted'`, 2, fixture.projectID)

	_, err = service.UpdateSurplusHandling(ctx, fmt.Sprint(submitted["id"]), SurplusHandlingRequest{
		SiteBalanceID: balanceID,
		HandlingType:  "return_to_warehouse",
		Quantity:      &firstQty,
		Unit:          "m",
		WarehouseID:   warehouseID,
		Condition:     "usable",
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestStockDocumentTransferMovesBothWarehouseBalancesAtomically(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID := newID()
	projectID := newID()
	sourceWarehouseID := newID()
	targetWarehouseID := newID()
	materialID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1 AND target_type IN ('stock_document','stock_movement')`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_documents WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id IN ($1,$2)`, sourceWarehouseID, targetWarehouseID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Warehouse Document Actor','active')`, actorID, "warehouse-document-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Warehouse Document Project',$2,'Client','active')`, projectID, "WDP-"+projectID[:8])
	mustExec(t, db, `INSERT INTO warehouses (id,name,location,type,active) VALUES ($1,'Source Warehouse','A','central',true),($2,'Target Warehouse','B','site',true)`, sourceWarehouseID, targetWarehouseID)
	mustExec(t, db, `INSERT INTO materials (id,code,name,unit,category,active) VALUES ($1,$2,'Transfer Material','m','Test',true)`, materialID, "WDM-"+materialID[:8])

	receipt, err := repo.CreateStockDocument(ctx, StockDocumentRequest{
		ProjectID: projectID, MovementType: "receive", TargetWarehouseID: sourceWarehouseID,
		DocumentDate: "2026-07-13", Note: "Initial receipt",
		Items: []StockDocumentItemRequest{{MaterialID: materialID, Quantity: 10, Unit: "m"}},
	}, actorID, "Warehouse Document Actor")
	if err != nil {
		t.Fatal(err)
	}
	if receipt["movement_type"] != "receive" || len(receipt["items"].([]Record)) != 1 {
		t.Fatalf("receipt = %#v, want one-line receive document", receipt)
	}

	transfer, err := repo.CreateStockDocument(ctx, StockDocumentRequest{
		ProjectID: projectID, MovementType: "transfer", SourceWarehouseID: sourceWarehouseID, TargetWarehouseID: targetWarehouseID,
		DocumentDate: "2026-07-13", Note: "Move stock",
		Items: []StockDocumentItemRequest{{MaterialID: materialID, Quantity: 4, Unit: "m"}},
	}, actorID, "Warehouse Document Actor")
	if err != nil {
		t.Fatal(err)
	}
	if transfer["movement_type"] != "transfer" {
		t.Fatalf("transfer movement_type = %v, want transfer", transfer["movement_type"])
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	sourceAvailable, err := warehouseMaterialAvailableTx(ctx, tx, sourceWarehouseID, materialID)
	if err != nil {
		t.Fatal(err)
	}
	targetAvailable, err := warehouseMaterialAvailableTx(ctx, tx, targetWarehouseID, materialID)
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Rollback()
	if sourceAvailable != 6 || targetAvailable != 4 {
		t.Fatalf("warehouse balances = source %.3f target %.3f, want 6/4", sourceAvailable, targetAvailable)
	}

	_, err = repo.CreateStockDocument(ctx, StockDocumentRequest{
		ProjectID: projectID, MovementType: "transfer", SourceWarehouseID: sourceWarehouseID, TargetWarehouseID: targetWarehouseID,
		DocumentDate: "2026-07-13", Items: []StockDocumentItemRequest{{MaterialID: materialID, Quantity: 7, Unit: "m"}},
	}, actorID, "Warehouse Document Actor")
	assertValidationError(t, err)
	assertCount(t, db, `SELECT COUNT(*) FROM stock_documents WHERE project_id=$1 AND status='posted'`, 2, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 2, materialID)
}

func TestIncomingAcceptanceSubmitDoesNotCreateStockMovement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	warehouseID := newID()
	materialID := newID()
	procurementRequestID := newID()
	procurementItemID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1 AND target_type='incoming_acceptance'`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM incoming_acceptance_records WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM procurement_requests WHERE id=$1`, procurementRequestID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
	})
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Incoming Warehouse','Gate A','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Incoming Cable','m','Electrical',true)`, materialID, "IN-"+materialID[:8])
	mustExec(t, db, `
		INSERT INTO procurement_requests (id, project_id, request_code, title, requested_by, status)
		VALUES ($1,$2,$3,'Incoming acceptance procurement',$4,'ordered')
	`, procurementRequestID, fixture.projectID, "IAR-"+procurementRequestID[:8], fixture.workerID)
	mustExec(t, db, `
		INSERT INTO procurement_request_items (id, request_id, material_id, quantity_planned, quantity_requested, unit, estimated_unit_price, delivery_status, approval_status)
		VALUES ($1,$2,$3,10,10,'m',100,'pending','approved')
	`, procurementItemID, procurementRequestID, materialID)

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Incoming material defect",
		FileURL:      "/evidence/incoming-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	delivered := 10.0
	accepted := 8.0
	rejected := 2.0
	draft, err := service.CreateIncomingAcceptance(ctx, fixture.projectID, IncomingAcceptanceRequest{
		WorkItemID:           fixture.itemID,
		ProcurementRequestID: procurementRequestID,
		WarehouseID:          warehouseID,
		SourceType:           "procurement",
		SupplierName:         "Test Supplier",
		DeliveryReference:    "DN-001",
		ReceivedAt:           "2026-06-20",
		Comment:              "Nghiệm thu đầu vào có vật tư lỗi",
		Items: []IncomingAcceptanceItemRequest{{
			ItemIndex:                0,
			MaterialID:               materialID,
			ProcurementRequestItemID: procurementItemID,
			DeliveredQuantity:        &delivered,
			AcceptedQuantity:         &accepted,
			RejectedQuantity:         &rejected,
			Unit:                     "m",
			Condition:                "mixed",
			DefectNote:               "Một phần vật tư móp méo",
			EvidenceID:               fmt.Sprint(evidence["id"]),
			Decision:                 "conditional_acceptance",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if draft["status"] != "draft" {
		t.Fatalf("draft status = %v, want draft", draft["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 0, materialID)

	submitted, err := service.SubmitIncomingAcceptance(ctx, fmt.Sprint(draft["id"]), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if submitted["status"] != "submitted" || submitted["result"] != "conditional_pass" {
		t.Fatalf("submitted incoming acceptance = %#v, want submitted conditional_pass", submitted)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 0, materialID)
	assertCount(t, db, `SELECT COUNT(*) FROM incoming_acceptance_items WHERE incoming_acceptance_id=$1 AND accepted_quantity=8 AND rejected_quantity=2`, 1, draft["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted' AND metadata_json ? 'incoming_acceptance_id'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='incoming_acceptance_submitted'`, 1, fixture.projectID)

	_, err = service.UpdateIncomingAcceptance(ctx, fmt.Sprint(submitted["id"]), IncomingAcceptanceRequest{
		WorkItemID: fixture.itemID,
		SourceType: "supplier",
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}
