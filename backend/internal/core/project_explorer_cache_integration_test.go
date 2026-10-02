package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestProjectExplorerRedisCacheInvalidatesByDatabaseRevision(t *testing.T) {
	redisURL := os.Getenv("CONSFLOW_TEST_REDIS_CACHE_URL")
	if redisURL == "" {
		t.Skip("set CONSFLOW_TEST_REDIS_CACHE_URL to run Redis-backed project explorer tests")
	}
	database := openIsolatedAccountingTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}

	userID := newID()
	staffID := newID()
	projectID := newID()
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users(id, email, password_hash, full_name) VALUES ($1, $2, 'demo:test', 'Cache Integration User')`, []any{userID, userID + "@example.com"}},
		{`INSERT INTO staff(id, user_id, full_name, email) VALUES ($1, $2, 'Cache Integration Staff', $3)`, []any{staffID, userID, staffID + "@example.com"}},
		{`INSERT INTO projects(id, name, code, client_name, pm_staff_id) VALUES ($1, 'Cache Integration Project', $2, 'Cache Integration Client', $3)`, []any{projectID, "CACHE-" + projectID[:8], staffID}},
		{`INSERT INTO project_members(project_id, staff_id) VALUES ($1, $2)`, []any{projectID, staffID}},
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}

	service := NewService(NewRepository(database), "test-secret")
	service.ConfigureProjectExplorerCache(client, ProjectExplorerCacheConfig{
		Mode:             "serve",
		OperationTimeout: time.Second,
		IndexTTL:         time.Minute,
		RowTTL:           time.Minute,
		LockTTL:          time.Second,
		MaxValueBytes:    256 * 1024,
	}, nil)
	request := ProjectExplorerRequest{Page: 1, PageSize: 25}

	first, err := service.ProjectExplorer(ctx, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 1 || len(first.Items) != 1 || number(first.Items[0]["open_issues"]) != 0 {
		t.Fatalf("first page=%+v", first)
	}
	searched, err := service.ProjectExplorer(ctx, userID, ProjectExplorerRequest{
		Page: 1, PageSize: 25, Search: "Cache Integration",
	})
	if err != nil {
		t.Fatal(err)
	}
	if searched.Total != 1 || len(searched.Items) != 1 {
		t.Fatalf("server-side search page=%+v", searched)
	}
	firstKeys, err := client.Keys(ctx, "consflow:project-explorer:v1:row:"+projectID+":*").Result()
	if err != nil || len(firstKeys) != 1 {
		t.Fatalf("first row keys=%v error=%v", firstKeys, err)
	}

	if _, err := database.ExecContext(ctx, `INSERT INTO issues(project_id, title) VALUES ($1, 'Revision test issue')`, projectID); err != nil {
		t.Fatal(err)
	}
	second, err := service.ProjectExplorer(ctx, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || number(second.Items[0]["open_issues"]) != 1 {
		t.Fatalf("row revision did not invalidate derived count: %+v", second)
	}
	secondKeys, err := client.Keys(ctx, "consflow:project-explorer:v1:row:"+projectID+":*").Result()
	if err != nil || len(secondKeys) < 2 {
		t.Fatalf("versioned row keys=%v error=%v", secondKeys, err)
	}

	if _, err := database.ExecContext(ctx, `UPDATE project_members SET active=false WHERE project_id=$1 AND staff_id=$2`, projectID, staffID); err != nil {
		t.Fatal(err)
	}
	third, err := service.ProjectExplorer(ctx, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	if third.Total != 0 || len(third.Items) != 0 {
		t.Fatalf("actor revision did not remove inaccessible project: %+v", third)
	}

	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE project_members SET active=true WHERE project_id=$1 AND staff_id=$2`, projectID, staffID); err != nil {
		t.Fatal(err)
	}
	fallback, err := service.ProjectExplorer(ctx, userID, request)
	if err != nil {
		t.Fatalf("closed Redis must fall back to PostgreSQL: %v", err)
	}
	if fallback.Total != 1 || len(fallback.Items) != 1 {
		t.Fatalf("fallback page=%+v", fallback)
	}
}

func TestDeputyExplorerCacheRevokesAndRestoresCompanyScope(t *testing.T) {
	redisURL := os.Getenv("CONSFLOW_TEST_REDIS_CACHE_URL")
	if redisURL == "" {
		t.Skip("set CONSFLOW_TEST_REDIS_CACHE_URL for warm-cache supervision test")
	}
	database := openIsolatedAccountingTestDB(t)
	ctx := context.Background()
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	client := redis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	actor, project := newID(), newID()
	mustExec(t, database, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:test','Deputy Cache','active')`, actor, actor+"@test.local")
	mustExec(t, database, `INSERT INTO projects(id,name,code,client_name) VALUES($1,'Deputy Cache Project',$2,'Client')`, project, "DEPC-"+project[:8])
	mustExec(t, database, `INSERT INTO user_roles(user_id,role_id) SELECT $1,id FROM roles WHERE name='deputy_manager'`, actor)
	service := NewService(NewRepository(database), "test-secret")
	service.ConfigureProjectExplorerCache(client, ProjectExplorerCacheConfig{Mode: "serve", OperationTimeout: time.Second, IndexTTL: time.Minute, RowTTL: time.Minute, LockTTL: time.Second, MaxValueBytes: 256 * 1024}, nil)
	request := ProjectExplorerRequest{Page: 1, PageSize: 25}
	for _, phase := range []struct {
		marker bool
		want   int
	}{{true, 1}, {true, 1}, {false, 0}, {false, 0}, {true, 1}, {true, 1}} {
		if phase.marker {
			mustExec(t, database, `INSERT INTO role_permissions(role_id,permission_id) SELECT role.id,permission.id FROM roles role JOIN permissions permission ON permission.key='project.supervision.view' WHERE role.name='deputy_manager' ON CONFLICT DO NOTHING`)
		} else {
			mustExec(t, database, `DELETE FROM role_permissions WHERE role_id IN(SELECT id FROM roles WHERE name='deputy_manager') AND permission_id IN(SELECT id FROM permissions WHERE key='project.supervision.view')`)
		}
		page, err := service.ProjectExplorer(ctx, actor, request)
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != phase.want || len(page.Items) != phase.want {
			t.Fatalf("marker=%v warm index total=%d rows=%d want=%d", phase.marker, page.Total, len(page.Items), phase.want)
		}
	}
	for _, status := range []string{"inactive", "active"} {
		mustExec(t, database, `UPDATE users SET status=$2 WHERE id=$1`, actor, status)
		page, err := service.ProjectExplorer(ctx, actor, request)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if status == "active" {
			want = 1
		}
		if page.Total != want || len(page.Items) != want {
			t.Fatalf("status=%s cached scope %#v", status, page)
		}
	}
	for _, sample := range []bool{true, false} {
		mustExec(t, database, `UPDATE projects SET is_sample=$2 WHERE id=$1`, project, sample)
		page, err := service.ProjectExplorer(ctx, actor, request)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if sample {
			want = 0
		}
		if page.Total != want || len(page.Items) != want {
			t.Fatalf("sample=%v cached scope %#v", sample, page)
		}
	}
}
