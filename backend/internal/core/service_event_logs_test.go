package core

import "testing"

func TestNormalizeEventLogFilter(t *testing.T) {
	filter, err := normalizeEventLogFilter(EventLogFilter{Category: " PANIC ", HTTPStatus: 500, Method: "post", PageSize: 200})
	if err != nil {
		t.Fatalf("normalize filter: %v", err)
	}
	if filter.Category != "panic" || filter.Method != "POST" || filter.Page != 1 || filter.PageSize != 25 {
		t.Fatalf("unexpected normalized filter: %#v", filter)
	}
	for _, invalid := range []EventLogFilter{
		{Category: "business_error"},
		{HTTPStatus: 429},
		{Method: "TRACE"},
		{ProjectID: "not-a-uuid"},
		{DateFrom: "2026-08-20", DateTo: "2026-08-19"},
	} {
		if _, err := normalizeEventLogFilter(invalid); err == nil {
			t.Fatalf("expected invalid filter to fail: %#v", invalid)
		}
	}
}
