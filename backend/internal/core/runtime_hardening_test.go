package core

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestActivityStreamCapacityLimitsAndReleases(t *testing.T) {
	capacity := NewActivityStreamCapacity(2, 2, 1)
	release, ok := capacity.Acquire("192.0.2.1", "actor-1")
	if !ok {
		t.Fatal("first stream unexpectedly rejected")
	}
	if _, ok := capacity.Acquire("192.0.2.2", "actor-1"); ok {
		t.Fatal("second stream for same actor unexpectedly accepted")
	}
	release()
	release()
	if _, ok := capacity.Acquire("192.0.2.2", "actor-1"); !ok {
		t.Fatal("stream was not released idempotently")
	}
}

func TestReadyReportsDatabaseState(t *testing.T) {
	handler := &Handler{}
	app := fiber.New()
	app.Get("/api/ready", handler.Ready)

	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/ready", nil))
	if err != nil || response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("unconfigured readiness status=%d err=%v, want 503", response.StatusCode, err)
	}

	handler.ConfigureReadiness(time.Second, func(context.Context) error { return nil })
	response, err = app.Test(httptest.NewRequest(fiber.MethodGet, "/api/ready", nil))
	if err != nil || response.StatusCode != fiber.StatusOK {
		t.Fatalf("ready status=%d err=%v, want 200", response.StatusCode, err)
	}
}

func TestReadyReturns503WhenDatabasePingFails(t *testing.T) {
	handler := &Handler{}
	handler.ConfigureReadiness(time.Second, func(_ context.Context) error { return errors.New("database unavailable") })
	app := fiber.New()
	app.Get("/api/ready", handler.Ready)
	response, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/api/ready", nil))
	if err != nil || response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("failed readiness status=%d err=%v, want 503", response.StatusCode, err)
	}
}

func TestProjectActivityInvalidationRemovesRecordIdentifiers(t *testing.T) {
	output := projectActivityInvalidation(ActivityStreamEvent{
		Type:       "activity_changed",
		ProjectID:  "project-1",
		WorkItemID: "work-item-1",
		ActivityID: "activity-1",
		ScopeType:  "work_item",
		ScopeID:    "scope-1",
		ActorID:    "actor-1",
		At:         "2026-07-20T00:00:00Z",
	})
	if output.Type != "activity_changed" || output.ProjectID != "project-1" || output.At == "" {
		t.Fatalf("minimal invalidation lost required fields: %#v", output)
	}
	if output.WorkItemID != "" || output.ActivityID != "" || output.ScopeType != "" || output.ScopeID != "" || output.ActorID != "" {
		t.Fatalf("minimal invalidation leaked record identifiers: %#v", output)
	}
}

func TestConfigureActivityStreamingSetsMaximumConnectionLifetime(t *testing.T) {
	service := NewService(nil, "test-secret")
	service.ConfigureActivityStreaming(nil, "", time.Second, 10, 5, 2, time.Second, 7*time.Minute)
	if service.streamMaxConnectionLifetime != 7*time.Minute {
		t.Fatalf("stream lifetime=%s, want 7m", service.streamMaxConnectionLifetime)
	}
}

func TestSSEConnectionLifetimeJitterStaysWithinConfiguredMaximum(t *testing.T) {
	const maximum = 5 * time.Minute
	minimum := maximum - maximum/5
	lower := jitteredSSEConnectionLifetime(maximum, func(uint64) (uint64, error) { return 0, nil })
	upper := jitteredSSEConnectionLifetime(maximum, func(maxExclusive uint64) (uint64, error) { return maxExclusive - 1, nil })
	if lower != minimum {
		t.Fatalf("lower jittered lifetime=%s, want %s", lower, minimum)
	}
	if upper != maximum {
		t.Fatalf("upper jittered lifetime=%s, want %s", upper, maximum)
	}
	fallback := jitteredSSEConnectionLifetime(maximum, func(uint64) (uint64, error) { return 0, errors.New("random unavailable") })
	if fallback != maximum {
		t.Fatalf("random failure lifetime=%s, want safe maximum %s", fallback, maximum)
	}
}

func TestActivityAttentionEventUsesConnectionProjectSnapshot(t *testing.T) {
	visibleProjects := map[string]struct{}{"project-1": {}}
	if !activityAttentionEventVisible(ActivityStreamEvent{ProjectID: "project-1"}, visibleProjects) {
		t.Fatal("event in connection visibility snapshot was rejected")
	}
	if activityAttentionEventVisible(ActivityStreamEvent{ProjectID: "project-2"}, visibleProjects) {
		t.Fatal("event outside connection visibility snapshot was accepted")
	}
	if !activityAttentionEventVisible(ActivityStreamEvent{}, visibleProjects) {
		t.Fatal("non-project invalidation was rejected")
	}
}
