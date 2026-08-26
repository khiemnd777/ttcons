package core

import (
	"testing"
	"time"
)

func TestNotificationFilterAndCursorValidation(t *testing.T) {
	normalized, err := normalizeNotificationFilter(NotificationFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.State != "all" || normalized.Limit != 20 {
		t.Fatalf("notification defaults = %#v", normalized)
	}
	if normalized, err := normalizeNotificationFilter(NotificationFilter{State: "archived"}); err != nil || normalized.State != "archived" {
		t.Fatalf("archived notification state = %#v, %v", normalized, err)
	}
	if _, err := normalizeNotificationFilter(NotificationFilter{EventKey: "unknown"}); err == nil {
		t.Fatal("unsupported notification type must be rejected")
	}

	createdAt := time.Now().UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano)
	cursorValue := encodeNotificationCursor(createdAt, "9af77ea7-38ba-445e-8d1d-e3bbbd428e47")
	cursor, err := decodeNotificationCursor(cursorValue)
	if err != nil {
		t.Fatal(err)
	}
	if cursor.CreatedAt != createdAt || cursor.ID != "9af77ea7-38ba-445e-8d1d-e3bbbd428e47" {
		t.Fatalf("decoded cursor = %#v", cursor)
	}
	if _, err := decodeNotificationCursor("not-a-cursor"); err == nil {
		t.Fatal("malformed notification cursor must be rejected")
	}
}

func TestNotificationTemplateAndRouteRenderingFailClosed(t *testing.T) {
	payload := Record{"actor_name": "Giám sát", "project_id": "30000000-0000-0000-0000-000000000001"}
	text, err := renderNotificationText("{{actor_name}} đã cập nhật", payload, []string{"actor_name"})
	if err != nil || text != "Giám sát đã cập nhật" {
		t.Fatalf("rendered text = %q, %v", text, err)
	}
	if _, err := renderNotificationText("{{missing}}", payload, []string{"missing"}); err == nil {
		t.Fatal("missing template payload must fail")
	}
	route, err := renderNotificationRoute("/projects/{{project_id}}", payload)
	if err != nil || route != "/projects/30000000-0000-0000-0000-000000000001" {
		t.Fatalf("rendered route = %q, %v", route, err)
	}
	if _, err := renderNotificationRoute("https://example.com/{{project_id}}", payload); err == nil {
		t.Fatal("external notification route must fail")
	}
}
