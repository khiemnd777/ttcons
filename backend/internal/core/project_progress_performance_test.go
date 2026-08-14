package core

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"
)

func TestProjectProgressBoardPerformanceBudget(t *testing.T) {
	if os.Getenv("CONSFLOW_RUN_PROJECT_PROGRESS_PERF") != "1" {
		t.Skip("set CONSFLOW_RUN_PROJECT_PROGRESS_PERF=1 to run the 10k project progress benchmark")
	}
	database := openIsolatedAccountingTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	userID := newID()
	staffID := newID()
	roleID := ensureRole(t, database, "admin", "Admin")
	mustAccountingExec(t, ctx, database,
		`INSERT INTO users(id,email,password_hash,full_name) VALUES ($1,$2,'demo:test','Progress Performance Admin')`,
		userID, userID+"@example.com",
	)
	mustAccountingExec(t, ctx, database,
		`INSERT INTO staff(id,user_id,full_name,email,department,status) VALUES ($1,$2,'Progress Performance PM',$3,'Dự án','active')`,
		staffID, userID, staffID+"@example.com",
	)
	mustAccountingExec(t, ctx, database, `INSERT INTO user_roles(user_id,role_id) VALUES ($1,$2)`, userID, roleID)

	service := NewService(NewRepository(database), "test-secret")
	inserted := 0
	for _, target := range []int{1_000, 5_000, 10_000} {
		batchSize := target - inserted
		mustAccountingExec(t, ctx, database, `
			INSERT INTO projects(name,code,client_name,status,pm_staff_id,created_at)
			SELECT
				'Progress Performance ' || sequence_no,
				'PROGRESS-PERF-' || $1 || '-' || sequence_no,
				'Performance Client',
				CASE WHEN sequence_no % 5=0 THEN 'completed' ELSE 'planning' END,
				$2,
				TIMESTAMPTZ '2026-08-14 00:00:00+00' - (sequence_no * INTERVAL '1 second')
			FROM generate_series($3::integer, $4::integer) sequence_no
		`, userID[:8], staffID, inserted+1, target)
		inserted += batchSize

		summaryDurations := make([]time.Duration, 0, 10)
		stageDurations := make([]time.Duration, 0, 10)
		var summary ProjectProgressSummaryResponse
		var page ProjectProgressStagePage
		for run := 0; run < 10; run++ {
			startedAt := time.Now()
			var err error
			summary, err = service.ProjectProgressBoardSummary(ctx, userID, ProjectProgressFilter{})
			if err != nil {
				t.Fatal(err)
			}
			summaryDurations = append(summaryDurations, time.Since(startedAt))

			startedAt = time.Now()
			page, err = service.ProjectProgressBoardStage(ctx, userID, ProjectProgressStageRequest{
				StageKey: "not_started",
				Limit:    25,
			})
			if err != nil {
				t.Fatal(err)
			}
			stageDurations = append(stageDurations, time.Since(startedAt))
		}

		if summary.FilteredTotal != target || summary.StageCounts["not_started"] != target-(target/5) {
			t.Fatalf("target=%d summary=%+v", target, summary)
		}
		if page.Total != target-(target/5) || len(page.Items) != 25 || !page.HasMore || page.NextCursor == "" {
			t.Fatalf("target=%d page total=%d items=%d has_more=%v", target, page.Total, len(page.Items), page.HasMore)
		}
		summaryPayload, err := json.Marshal(summary)
		if err != nil {
			t.Fatal(err)
		}
		stagePayload, err := json.Marshal(page)
		if err != nil {
			t.Fatal(err)
		}
		summaryP95 := durationP95(summaryDurations)
		stageP95 := durationP95(stageDurations)
		t.Logf(
			"projects=%d summary_p95=%s stage_p95=%s summary_bytes=%d stage_bytes=%d",
			target, summaryP95, stageP95, len(summaryPayload), len(stagePayload),
		)
		if summaryP95 > 300*time.Millisecond {
			t.Fatalf("target=%d summary p95=%s exceeds 300ms", target, summaryP95)
		}
		if stageP95 > 200*time.Millisecond {
			t.Fatalf("target=%d stage p95=%s exceeds 200ms", target, stageP95)
		}
		if len(summaryPayload) >= 50*1024 {
			t.Fatalf("target=%d summary payload=%d exceeds 50 KiB", target, len(summaryPayload))
		}
		if len(stagePayload) >= 100*1024 {
			t.Fatalf("target=%d stage payload=%d exceeds 100 KiB", target, len(stagePayload))
		}
	}
}

func durationP95(values []time.Duration) time.Duration {
	ordered := append([]time.Duration(nil), values...)
	sort.Slice(ordered, func(left, right int) bool { return ordered[left] < ordered[right] })
	if len(ordered) == 0 {
		return 0
	}
	index := (len(ordered)*95 + 99) / 100
	return ordered[index-1]
}
