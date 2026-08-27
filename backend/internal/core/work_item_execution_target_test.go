package core

import (
	"context"
	"testing"
)

func TestWorkItemExecutionTargetUsesCanonicalBusinessWorkspace(t *testing.T) {
	tests := []struct {
		name       string
		item       Record
		source     Record
		kind       string
		workspace  string
		route      string
		completion string
		action     string
	}{
		{
			name: "mixed boq work opens boq before form confirmation",
			item: Record{
				"id": "work-1", "project_id": "project-1", "domain_action_key": "source.boq_items_ready",
				"workflow_form_instance_id": "form-1", "run_node_business_code": "boq_without_design",
			},
			source:     Record{"route": "/projects/project-1/boq", "ready": true, "status": "succeeded"},
			kind:       "mixed",
			workspace:  executionWorkspaceBOQ,
			route:      "/projects/project-1/boq?task=work-1",
			completion: "domain_then_form",
			action:     "Cập nhật BOQ",
		},
		{
			name: "survey capture opens preparation workspace",
			item: Record{
				"id": "work-2", "project_id": "project-1", "workflow_form_instance_id": "form-2",
				"run_node_business_code": "survey",
			},
			kind:       "capture",
			workspace:  executionWorkspacePreparation,
			route:      "/projects/project-1/preparation?task=work-2",
			completion: "workflow_form",
			action:     "Mở hồ sơ chuẩn bị",
		},
		{
			name: "construction form stays in operations workspace",
			item: Record{
				"id": "work-3", "project_id": "project-1", "workflow_form_instance_id": "form-3",
				"run_node_business_code": "construction_execution", "surface_kind": "quality",
			},
			kind:       "capture",
			workspace:  executionWorkspaceOperations,
			route:      "/projects/project-1/operations?task=work-3",
			completion: "workflow_form",
			action:     "Mở thi công",
		},
		{
			name: "finance source retains its domain query",
			item: Record{
				"id": "work-4", "project_id": "project-1", "domain_action_key": "source.receivable_ready",
			},
			source:     Record{"route": "/projects/project-1/accounting?tab=tasks", "ready": false, "status": "blocked"},
			kind:       "domain",
			workspace:  executionWorkspaceProjectFinance,
			route:      "/projects/project-1/accounting?tab=tasks&task=work-4",
			completion: "domain_gate",
			action:     "Mở tài chính công trình",
		},
		{
			name:       "unknown legacy work fails closed to detail",
			item:       Record{"id": "work-5", "project_id": "project-1", "work_type": "legacy_custom"},
			kind:       "work_item",
			workspace:  executionWorkspaceWorkItem,
			route:      "/projects/project-1/work-items/work-5",
			completion: "work_item",
			action:     "Mở hồ sơ công việc",
		},
		{
			name: "unsafe source route falls back to canonical project workspace",
			item: Record{
				"id": "work-6", "project_id": "project-1", "domain_action_key": "source.boq_items_ready",
			},
			source:     Record{"route": "https://example.com/projects/project-1/boq"},
			kind:       "domain",
			workspace:  executionWorkspaceBOQ,
			route:      "/projects/project-1/boq?task=work-6",
			completion: "domain_gate",
			action:     "Cập nhật BOQ",
		},
		{
			name: "legacy material capture opens warehouse workspace",
			item: Record{
				"id": "work-7", "project_id": "project-1", "surface_kind": "material_snapshot", "work_type": "material_check",
			},
			kind:       "domain",
			workspace:  executionWorkspaceWarehouse,
			route:      "/projects/project-1/warehouse?task=work-7",
			completion: "domain_gate",
			action:     "Mở kho vật tư",
		},
		{
			name: "approval keeps its decision action inside the boq context",
			item: Record{
				"id": "work-8", "project_id": "project-1", "work_type": "approval", "domain_action_key": "source.boq_approved",
			},
			kind:       "approval",
			workspace:  executionWorkspaceBOQ,
			route:      "/projects/project-1/boq?task=work-8",
			completion: "approval",
			action:     "Mở phê duyệt",
		},
		{
			name: "photo evidence opens the preparation capture surface",
			item: Record{
				"id": "work-9", "project_id": "project-1", "work_type": "photo_upload", "run_node_business_code": "survey",
			},
			kind:       "evidence",
			workspace:  executionWorkspacePreparation,
			route:      "/projects/project-1/preparation?task=work-9",
			completion: "evidence",
			action:     "Nộp minh chứng",
		},
		{
			name: "Rev8 input contract package opens contracts",
			item: Record{
				"id": "work-10", "project_id": "project-1", "domain_action_key": "source.input_contract_package_ready",
			},
			source:     Record{"route": "/projects/project-1/contracts", "ready": false, "status": "blocked"},
			kind:       "domain",
			workspace:  executionWorkspaceContracts,
			route:      "/projects/project-1/contracts?task=work-10",
			completion: "domain_gate",
			action:     "Mở hợp đồng",
		},
		{
			name: "Rev8 subcontractor package opens acceptance",
			item: Record{
				"id": "work-11", "project_id": "project-1", "domain_action_key": "source.subcontractor_acceptance_package_ready",
				"workflow_form_instance_id": "form-11",
			},
			source:     Record{"route": "/projects/project-1/acceptance", "ready": false, "status": "blocked"},
			kind:       "mixed",
			workspace:  executionWorkspaceAcceptance,
			route:      "/projects/project-1/acceptance?task=work-11",
			completion: "domain_then_form",
			action:     "Mở nghiệm thu",
		},
		{
			name: "Rev8 finance close opens project finance",
			item: Record{
				"id": "work-12", "project_id": "project-1", "domain_action_key": "source.project_finance_close_ready",
			},
			source:     Record{"route": "/projects/project-1/accounting?tab=tasks", "ready": false, "status": "blocked"},
			kind:       "domain",
			workspace:  executionWorkspaceProjectFinance,
			route:      "/projects/project-1/accounting?tab=tasks&task=work-12",
			completion: "domain_gate",
			action:     "Mở tài chính công trình",
		},
		{
			name: "Rev9 accounting handoff opens lean workspace",
			item: Record{
				"id": "work-13", "project_id": "project-1", "domain_action_key": "source.accounting_handoff_close_ready",
			},
			source:     Record{"route": "/projects/project-1/accounting", "ready": false, "status": "blocked"},
			kind:       "domain",
			workspace:  executionWorkspaceProjectFinance,
			route:      "/projects/project-1/accounting?task=work-13",
			completion: "domain_gate",
			action:     "Mở bàn giao kế toán",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := workItemExecutionTarget(tt.item, tt.source)
			if got := cleanID(target["kind"]); got != tt.kind {
				t.Fatalf("kind=%q want %q", got, tt.kind)
			}
			if got := cleanID(target["workspace"]); got != tt.workspace {
				t.Fatalf("workspace=%q want %q", got, tt.workspace)
			}
			if got := cleanID(target["route"]); got != tt.route {
				t.Fatalf("route=%q want %q", got, tt.route)
			}
			if got := cleanID(target["completion_mode"]); got != tt.completion {
				t.Fatalf("completion_mode=%q want %q", got, tt.completion)
			}
			if got := cleanID(target["action_label"]); got != tt.action {
				t.Fatalf("action_label=%q want %q", got, tt.action)
			}
		})
	}
}

func TestRev8EveryWorkItemHasOneExecutionTarget(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID := newID()
	staffID := newID()
	projectID := newID()
	adminRoleID := ensureRole(t, db, "admin", "Admin")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Rev8 Target Admin','active')`, actorID, "rev8-target-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,status) VALUES ($1,$2,'Rev8 Target Admin',$3,'management','active')`, staffID, actorID, "rev8-target-staff-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, actorID, adminRoleID)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,pipeline_template_id) VALUES ($1,'Rev8 Execution Target',$2,'Client','planning','40000000-0000-0000-0000-000000000027')`, projectID, "R8T-"+projectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, staffID)
	if _, err := repo.StartPipeline(ctx, projectID, "40000000-0000-0000-0000-000000000027", actorID, "Rev8 Target Admin"); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ListProjectOperationsWorkItems(ctx, projectID, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 21 {
		t.Fatalf("Rev8 work item count=%d want 21", len(items))
	}
	attachExecutionTargets(items)
	workspaceCounts := map[string]int{}
	for _, item := range items {
		target := recordFromAny(item["execution_target"])
		if cleanID(target["kind"]) == "" || cleanID(target["workspace"]) == "" || cleanID(target["route"]) == "" || cleanID(target["action_label"]) == "" {
			t.Fatalf("work item %s has incomplete execution target: %#v", cleanID(item["id"]), target)
		}
		if cleanID(target["workspace"]) == executionWorkspaceWorkItem {
			t.Fatalf("official Rev8 work item %s fell back to generic detail: %#v", cleanID(item["id"]), target)
		}
		workspaceCounts[cleanID(target["workspace"])]++
	}
	wantWorkspaceCounts := map[string]int{
		executionWorkspacePreparation:    6,
		executionWorkspacePlanning:       2,
		executionWorkspaceBOQ:            1,
		executionWorkspaceContracts:      2,
		executionWorkspaceProcurement:    4,
		executionWorkspaceOperations:     1,
		executionWorkspaceAcceptance:     3,
		executionWorkspaceHandover:       1,
		executionWorkspaceProjectFinance: 1,
	}
	for workspace, want := range wantWorkspaceCounts {
		if got := workspaceCounts[workspace]; got != want {
			t.Fatalf("Rev8 workspace %s count=%d want %d; all=%#v", workspace, got, want, workspaceCounts)
		}
	}
}

func TestExecutionRouteWithTaskPreservesExistingQueryAndReplacesTask(t *testing.T) {
	route := executionRouteWithTask("/projects/project-1/accounting?task=old&tab=tasks", "new")
	if route != "/projects/project-1/accounting?tab=tasks&task=new" {
		t.Fatalf("route=%q", route)
	}
}

func TestRev7EveryWorkItemHasOneExecutionTarget(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID := newID()
	staffID := newID()
	projectID := newID()
	adminRoleID := ensureRole(t, db, "admin", "Admin")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Execution Target Admin','active')`, actorID, "execution-target-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,status) VALUES ($1,$2,'Execution Target Admin',$3,'management','active')`, staffID, actorID, "execution-target-staff-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, actorID, adminRoleID)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,pipeline_template_id) VALUES ($1,'Rev7 Execution Target',$2,'Client','planning','40000000-0000-0000-0000-000000000026')`, projectID, "R7T-"+projectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, staffID)
	if _, err := repo.StartPipeline(ctx, projectID, "40000000-0000-0000-0000-000000000026", actorID, "Execution Target Admin"); err != nil {
		t.Fatal(err)
	}

	items, err := repo.ListProjectOperationsWorkItems(ctx, projectID, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 50 {
		t.Fatalf("Rev7 work item count=%d want 50", len(items))
	}
	attachExecutionTargets(items)
	workspaceCounts := map[string]int{}
	for _, item := range items {
		target := recordFromAny(item["execution_target"])
		if cleanID(target["kind"]) == "" || cleanID(target["workspace"]) == "" || cleanID(target["route"]) == "" || cleanID(target["action_label"]) == "" {
			t.Fatalf("work item %s has incomplete execution target: %#v", cleanID(item["id"]), target)
		}
		if cleanID(target["workspace"]) == executionWorkspaceWorkItem && cleanID(target["kind"]) != "approval" {
			t.Fatalf("official Rev7 non-approval work item %s fell back to generic detail: %#v", cleanID(item["id"]), target)
		}
		workspaceCounts[cleanID(target["workspace"])]++
	}
	wantWorkspaceCounts := map[string]int{
		executionWorkspacePreparation:    16,
		executionWorkspacePlanning:       3,
		executionWorkspaceBOQ:            4,
		executionWorkspaceContracts:      6,
		executionWorkspaceProcurement:    5,
		executionWorkspaceOperations:     1,
		executionWorkspaceAcceptance:     6,
		executionWorkspaceHandover:       1,
		executionWorkspaceProjectFinance: 8,
	}
	for workspace, want := range wantWorkspaceCounts {
		if got := workspaceCounts[workspace]; got != want {
			t.Fatalf("Rev7 workspace %s count=%d want %d; all=%#v", workspace, got, want, workspaceCounts)
		}
	}
	if workspaceCounts[executionWorkspaceWarehouse] != 0 || workspaceCounts[executionWorkspaceWorkItem] != 0 {
		t.Fatalf("Rev7 unexpectedly used warehouse/generic fallback: %#v", workspaceCounts)
	}
}
