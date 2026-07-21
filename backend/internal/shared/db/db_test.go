package db

import (
	"testing"
	"time"
)

func TestConnectionConfigAppliesRuntimeTimeouts(t *testing.T) {
	config, err := connectionConfig("postgres://user:password@127.0.0.1:5432/database?sslmode=disable", Options{
		StatementTimeout: 1500 * time.Millisecond,
		LockTimeout:      250 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("connectionConfig: %v", err)
	}
	if got := config.RuntimeParams["statement_timeout"]; got != "1500" {
		t.Fatalf("statement_timeout=%q, want 1500", got)
	}
	if got := config.RuntimeParams["lock_timeout"]; got != "250" {
		t.Fatalf("lock_timeout=%q, want 250", got)
	}
}

func TestConnectionConfigWithoutRuntimeTimeoutsSupportsMigrationPool(t *testing.T) {
	config, err := connectionConfig("postgres://user:password@127.0.0.1:5432/database?sslmode=disable", Options{})
	if err != nil {
		t.Fatalf("connectionConfig: %v", err)
	}
	if _, ok := config.RuntimeParams["statement_timeout"]; ok {
		t.Fatal("migration-style connection unexpectedly has statement_timeout")
	}
	if _, ok := config.RuntimeParams["lock_timeout"]; ok {
		t.Fatal("migration-style connection unexpectedly has lock_timeout")
	}
}
