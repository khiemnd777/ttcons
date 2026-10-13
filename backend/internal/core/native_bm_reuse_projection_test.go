package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func nativeReuseWorkbookText(t *testing.T, code string, sheet BMGSSheet) string {
	t.Helper()
	raw, err := generateBMGSWorkbook(BMGSWorkbookData{TemplateCode: code, Sheets: []BMGSSheet{sheet}})
	if err != nil {
		t.Fatal(err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows(book.GetSheetList()[0])
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprint(rows)
}

func TestNativeBMReuseSubcontractorNotes(t *testing.T) {
	for _, c := range []struct {
		name, comment, quality, measurement string
		count                               int
	}{
		{"generic", "Source observation", "", "", 1},
		{"equal quality", "Source observation", "Source observation", "", 1},
		{"equal measurement", "Source observation", "", "Source observation", 1},
		{"distinct", "Source observation", "Explicit quality note", "Explicit measurement note", 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			row := Record{"title": "Native item", "comment": c.comment, "quality_note": c.quality, "measurement_note": c.measurement, "status": "done", "owner_response": "accepted"}
			sheet, err := nativeBMGSProjectSheet("BM-GS-03", BMGSSourceRef{Kind: "subcontractor_checklist", ID: "source"}, Record{"answers_snapshot_json": Record{"checklist_items": []Record{row}}})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(cleanText(sheet.Rows[0]["result"]), "Chưa đánh giá") {
				t.Fatal("generic facts became quality result", sheet.Rows)
			}
			text := nativeReuseWorkbookText(t, "BM-GS-03", sheet)
			if strings.Count(text, c.comment) != c.count {
				t.Fatal("source note lost or duplicated", text)
			}
			if c.quality == "" && c.measurement == "" && !strings.Contains(text, "Ghi chú nguồn: Source observation") {
				t.Fatal("generic note not labeled", text)
			}
			for _, note := range []string{c.quality, c.measurement} {
				if note != "" && !strings.Contains(text, note) {
					t.Fatal("explicit note lost", note)
				}
			}
		})
	}
}

func TestNativeBMReuseHandoverItemNotes(t *testing.T) {
	sheet, err := nativeBMGSProjectSheet("BM-GS-08", BMGSSourceRef{Kind: "handover", ID: "source"}, Record{"checklist_items": []Record{{"title": "Native item", "status": "verified", "notes": "Joint observed", "physical_result": nil, "defect": nil}}})
	if err != nil {
		t.Fatal(err)
	}
	if sheet.Rows[0]["result"] != nil || sheet.Rows[0]["defect"] != nil {
		t.Fatal("generic note became physical assessment", sheet.Rows)
	}
	if text := nativeReuseWorkbookText(t, "BM-GS-08", sheet); !strings.Contains(text, "Native item · Ghi chú nguồn: Joint observed") {
		t.Fatal("item note lost or unlabeled", text)
	}
}

func TestNativeBMReuseActionLinkedCheckLocation(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_reuse_", nil)
	owner := setupProjectScopeFixture(t, db, []string{"operation.view", "construction.action.edit"})
	other := setupProjectScopeFixture(t, db, []string{"operation.view"})
	s := NewService(NewRepository(db), "test-secret")
	ctx := context.Background()
	checkID, actionID := newID(), newID()
	mustExec(t, db, `INSERT INTO construction_checks(id,project_id,title,location) VALUES($1,$2,'Native check','Check location')`, checkID, owner.projectID)
	mustExec(t, db, `INSERT INTO construction_action_items(id,project_id,title,source_check_id,assigned_user_id,due_at,created_by) VALUES($1,$2,'Native action',$3,$4,'2026-10-10T08:00:00+07:00',$4)`, actionID, owner.projectID, checkID, owner.userID)
	assertLocation := func(want string) {
		t.Helper()
		row, err := s.nativeConstructionActionSource(ctx, actionID, owner.userID)
		if err != nil {
			t.Fatal(err)
		}
		sheet, err := nativeBMGSProjectSheet("BM-GS-04", BMGSSourceRef{Kind: "action", ID: actionID}, row)
		if err != nil {
			t.Fatal(err)
		}
		if got := cleanText(sheet.Rows[0]["location"]); got != want {
			t.Fatalf("location=%q want=%q", got, want)
		}
	}
	assertLocation("Check location")
	location := "Action override"
	req := ConstructionActionUpdateRequest{Title: "Native action", AssignedUserID: owner.userID, DueAt: "2026-10-10T08:00:00+07:00", Location: &location}
	if _, err := s.UpdateConstructionAction(ctx, actionID, req, owner.userID, "Owner"); err != nil {
		t.Fatal(err)
	}
	assertLocation(location)
	location = ""
	if _, err := s.UpdateConstructionAction(ctx, actionID, req, owner.userID, "Owner"); err != nil {
		t.Fatal(err)
	}
	assertLocation("")
	// Fresh unmodified actions can inherit only the exact active same-project check.
	mustExec(t, db, `UPDATE construction_action_items SET source_check_id=NULL WHERE id=$1`, actionID)
	actionID = newID()
	mustExec(t, db, `INSERT INTO construction_action_items(id,project_id,title,source_check_id) VALUES($1,$2,'Second action',$3)`, actionID, owner.projectID, checkID)
	mustExec(t, db, `UPDATE construction_checks SET status='cancelled' WHERE id=$1`, checkID)
	assertLocation("")
	mustExec(t, db, `UPDATE construction_checks SET status='pending',project_id=$2 WHERE id=$1`, checkID, other.projectID)
	assertLocation("")
}
