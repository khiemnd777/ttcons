package core

import (
	"context"
	"testing"
)

func TestProjectOperationsFixtureIsolation(t *testing.T) {
	database := openProjectOperationsTestDB(t)
	ctx := context.Background()
	snapshot := func() string {
		t.Helper()
		var fingerprint string
		if err := database.QueryRowContext(ctx, `
			SELECT md5(concat(
				(SELECT COALESCE(string_agg(to_jsonb(r)::text, E'\n' ORDER BY id), '') FROM public.roles r),
				(SELECT COALESCE(string_agg(to_jsonb(r)::text, E'\n' ORDER BY role_id,permission_id), '') FROM public.role_permissions r),
				(SELECT COALESCE(string_agg(to_jsonb(r)::text, E'\n' ORDER BY id), '') FROM public.users r),
				(SELECT COALESCE(string_agg(to_jsonb(r)::text, E'\n' ORDER BY id), '') FROM public.projects r),
				(SELECT COALESCE(string_agg(nspname, E'\n' ORDER BY nspname), '') FROM pg_namespace WHERE nspname LIKE 'project_test_%')
			))
		`).Scan(&fingerprint); err != nil {
			t.Fatal(err)
		}
		return fingerprint
	}
	before := snapshot()
	for _, name := range []string{"first_run", "repeat_run"} {
		if !t.Run(name, TestProjectOperationsAccessPolicy) {
			t.Fatal("isolated access-policy fixture failed")
		}
		if snapshot() != before {
			t.Fatal("access-policy fixture changed public catalog/data or left its isolated schema behind")
		}
	}
}
