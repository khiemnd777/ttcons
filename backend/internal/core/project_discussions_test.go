package core

import "testing"

func TestNormalizeProjectDiscussionFilterAndCursor(t *testing.T) {
	projectID := newID()
	filter, offset, err := normalizeProjectDiscussionFilter(ProjectDiscussionFilter{
		ProjectID: "  " + projectID + "  ",
		Search:    "  công trình A  ",
		Cursor:    encodeProjectDiscussionCursor(40),
		Limit:     25,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.ProjectID != projectID || filter.Search != "công trình A" || filter.Limit != 25 || offset != 40 {
		t.Fatalf("normalized filter = %#v, offset = %d", filter, offset)
	}
	defaultFilter, defaultOffset, err := normalizeProjectDiscussionFilter(ProjectDiscussionFilter{})
	if err != nil || defaultFilter.Limit != 20 || defaultOffset != 0 {
		t.Fatalf("default filter = %#v, offset = %d, error = %v", defaultFilter, defaultOffset, err)
	}
	if _, _, err := normalizeProjectDiscussionFilter(ProjectDiscussionFilter{ProjectID: "invalid"}); err == nil {
		t.Fatal("expected invalid project id error")
	}
	if _, _, err := normalizeProjectDiscussionFilter(ProjectDiscussionFilter{Cursor: "invalid"}); err == nil {
		t.Fatal("expected invalid cursor error")
	}
	if _, _, err := normalizeProjectDiscussionFilter(ProjectDiscussionFilter{Limit: 51}); err == nil {
		t.Fatal("expected invalid limit error")
	}
}
