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
