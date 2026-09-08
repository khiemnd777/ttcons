package core

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeProjectExplorerRequest(t *testing.T) {
	request, err := normalizeProjectExplorerRequest(ProjectExplorerRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if request.Page != 1 || request.PageSize != 25 || request.Sort != "created_at" || request.Direction != "desc" {
		t.Fatalf("defaults=%+v", request)
	}
	for _, invalid := range []ProjectExplorerRequest{
		{Page: -1, PageSize: 25},
		{Page: 1, PageSize: 20},
		{Page: 1, PageSize: 25, Sort: "budget"},
		{Page: 1, PageSize: 25, Direction: "sideways"},
		{Page: 1, PageSize: 25, Search: strings.Repeat("x", 121)},
	} {
		if _, err := normalizeProjectExplorerRequest(invalid); err == nil {
			t.Fatalf("invalid request accepted: %+v", invalid)
		}
	}
	request, err = normalizeProjectExplorerRequest(ProjectExplorerRequest{Sort: "updated_at", Direction: "asc", Page: 1, PageSize: 25})
	if err != nil || request.Sort != "updated_at" || request.Direction != "asc" {
		t.Fatalf("updated_at sort was not accepted: request=%+v err=%v", request, err)
	}
}

func TestProjectExplorerOrderSQLUsesUpdatedAt(t *testing.T) {
	request := ProjectExplorerRequest{Sort: "updated_at", Direction: "desc"}
	if order := projectExplorerOrderSQL(request); !strings.Contains(order, "p.updated_at DESC") {
		t.Fatalf("project explorer order=%q", order)
	}
	if order := projectExplorerOrderSQLForAlias(request, "project", "staff"); !strings.Contains(order, "project.updated_at DESC") {
		t.Fatalf("project export order=%q", order)
	}
}

func TestProjectExplorerCacheHashDoesNotExposeSearch(t *testing.T) {
	request, err := normalizeProjectExplorerRequest(ProjectExplorerRequest{Search: "Chủ đầu tư nhạy cảm", Page: 2, PageSize: 50})
	if err != nil {
		t.Fatal(err)
	}
	hash := request.cacheHash()
	if len(hash) != 64 || strings.Contains(hash, "nhạy") || hash == request.Search {
		t.Fatalf("unsafe cache hash=%q", hash)
	}
	request.Page = 3
	if hash == request.cacheHash() {
		t.Fatal("page change did not change cache hash")
	}
}

func TestAssembleProjectExplorerPagePreservesIndexOrder(t *testing.T) {
	page := assembleProjectExplorerPage(projectExplorerIndex{
		IDs:      []string{"project-b", "project-a", "missing"},
		Total:    51,
		Statuses: []string{"draft"},
	}, map[string]Record{
		"project-a": {"id": "project-a"},
		"project-b": {"id": "project-b"},
	}, ProjectExplorerRequest{Page: 2, PageSize: 25})
	if len(page.Items) != 2 || page.Items[0]["id"] != "project-b" || page.Items[1]["id"] != "project-a" {
		t.Fatalf("items=%v", page.Items)
	}
	if page.TotalPages != 3 {
		t.Fatalf("total_pages=%d", page.TotalPages)
	}
}

func TestProjectExplorerStreamEventScopesInvalidation(t *testing.T) {
	visible := map[string]struct{}{"project-1": {}}
	tests := []struct {
		name  string
		event ActivityStreamEvent
		ok    bool
	}{
		{name: "catalog", event: ActivityStreamEvent{Type: "project_explorer_changed", ScopeType: "catalog"}, ok: true},
		{name: "own actor", event: ActivityStreamEvent{Type: "project_explorer_changed", ScopeType: "actor", ActorID: "user-1"}, ok: true},
		{name: "other actor", event: ActivityStreamEvent{Type: "project_explorer_changed", ScopeType: "actor", ActorID: "user-2"}, ok: false},
		{name: "visible project", event: ActivityStreamEvent{Type: "project_explorer_changed", ScopeType: "project", ProjectID: "project-1"}, ok: true},
		{name: "hidden project", event: ActivityStreamEvent{Type: "project_explorer_changed", ScopeType: "project", ProjectID: "project-2"}, ok: false},
		{name: "unrelated", event: ActivityStreamEvent{Type: "work_item_changed", ScopeType: "project", ProjectID: "project-1"}, ok: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, output, ok := projectExplorerStreamEvent(test.event, "user-1", visible)
			if ok != test.ok {
				t.Fatalf("ok=%v want=%v output=%+v", ok, test.ok, output)
			}
			if ok && (output.ActorID != "" || output.ScopeType != "") {
				t.Fatalf("internal scope leaked: %+v", output)
			}
		})
	}
}

func TestProjectExplorerCacheJitterIsBounded(t *testing.T) {
	ttl := 10 * time.Minute
	got := jitteredTTL("key", ttl)
	if got < ttl || got >= ttl+ttl/5 {
		t.Fatalf("jittered TTL=%v", got)
	}
}

func TestProjectExplorerVersionsEqualDetectsMovedRevision(t *testing.T) {
	baseline := map[string]int64{"project-a": 2, "project-b": 7}
	if !projectExplorerVersionsEqual(baseline, map[string]int64{"project-a": 2, "project-b": 7}) {
		t.Fatal("equal revision snapshots were rejected")
	}
	if projectExplorerVersionsEqual(baseline, map[string]int64{"project-a": 3, "project-b": 7}) {
		t.Fatal("changed project revision was accepted")
	}
	if projectExplorerVersionsEqual(baseline, map[string]int64{"project-a": 2}) {
		t.Fatal("missing scoped project revision was accepted")
	}
	if reason := cacheFallbackReason(errProjectExplorerCacheRevisionMoved); reason != "revision_changed" {
		t.Fatalf("fallback reason=%q", reason)
	}
}
