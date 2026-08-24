package core

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestActivityAttentionPerformanceBudget(t *testing.T) {
	if os.Getenv("CONSFLOW_RUN_ACTIVITY_ATTENTION_PERF") != "1" {
		t.Skip("set CONSFLOW_RUN_ACTIVITY_ATTENTION_PERF=1 to run the 10k activity attention benchmark")
	}
	database := openIsolatedAccountingTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	userID := newID()
	staffID := newID()
	projectID := newID()
	roleID := ensureRole(t, database, "admin", "Admin")
	mustAccountingExec(t, ctx, database,
		`INSERT INTO users(id,email,password_hash,full_name) VALUES ($1,$2,'demo:test','Attention Performance Admin')`,
		userID, userID+"@example.com",
	)
	mustAccountingExec(t, ctx, database,
		`INSERT INTO staff(id,user_id,full_name,email,department,status) VALUES ($1,$2,'Attention Performance Admin',$3,'Dự án','active')`,
		staffID, userID, staffID+"@example.com",
	)
	mustAccountingExec(t, ctx, database, `INSERT INTO user_roles(user_id,role_id) VALUES ($1,$2)`, userID, roleID)
	mustAccountingExec(t, ctx, database,
		`INSERT INTO projects(id,name,code,client_name,status,pm_staff_id) VALUES ($1,'Attention Performance','ATTENTION-PERF','Performance Client','planning',$2)`,
		projectID, staffID,
	)

	service := NewService(NewRepository(database), "test-secret")
	inserted := 0
	for _, target := range []int{1_000, 5_000, 10_000} {
		mustAccountingExec(t, ctx, database, `
			INSERT INTO work_item_activities(project_id,scope_type,scope_id,actor_id,activity_type,comment,visibility,metadata_json,created_at)
			SELECT
				$1,
				'project',
				$1,
				$2,
				'question',
				'Attention performance ' || sequence_no,
				'project',
				'{}'::jsonb,
				TIMESTAMPTZ '2026-08-24 00:00:00+00' + (sequence_no * INTERVAL '1 millisecond')
			FROM generate_series($3::integer, $4::integer) sequence_no
		`, projectID, userID, inserted+1, target)
		inserted = target

		summaryDurations := make([]time.Duration, 0, 10)
		detailDurations := make([]time.Duration, 0, 10)
		var summary ActivityAttentionSummary
		var items []Record
		for run := 0; run < 10; run++ {
			startedAt := time.Now()
			var err error
			summary, err = service.MyActivityAttentionSummary(ctx, projectID, userID)
			if err != nil {
				t.Fatal(err)
			}
			summaryDurations = append(summaryDurations, time.Since(startedAt))

			startedAt = time.Now()
			items, err = service.MyActivityAttention(ctx, ActivityAttentionFilter{ProjectID: projectID, Limit: 120}, userID)
			if err != nil {
				t.Fatal(err)
			}
			detailDurations = append(detailDurations, time.Since(startedAt))
		}

		if summary.Count != 120 || !summary.HasMore {
			t.Fatalf("target=%d summary=%+v", target, summary)
		}
		if len(items) != 120 {
			t.Fatalf("target=%d detail items=%d, want 120", target, len(items))
		}
		summaryP95 := durationP95(summaryDurations)
		detailP95 := durationP95(detailDurations)
		t.Logf("activities=%d summary_p95=%s detail_p95=%s", target, summaryP95, detailP95)
		if summaryP95 > 150*time.Millisecond {
			t.Fatalf("target=%d summary p95=%s exceeds 150ms", target, summaryP95)
		}
		if detailP95 > 200*time.Millisecond {
			t.Fatalf("target=%d detail p95=%s exceeds 200ms", target, detailP95)
		}
	}
}
