package eventlog

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSanitizeMessageRemovesSecretsAndUserData(t *testing.T) {
	raw := "password=secret-value Authorization:Bearer abc.def token=qwerty postgres://user:pass@db/app email=user@example.com https://example.com/path?token=visible"
	clean := SanitizeMessage(raw)
	for _, forbidden := range []string{"secret-value", "abc.def", "qwerty", "user:pass", "user@example.com", "token=visible"} {
		if strings.Contains(clean, forbidden) {
			t.Fatalf("sanitized message contains %q: %s", forbidden, clean)
		}
	}
	for _, expected := range []string{"[REDACTED]", "[REDACTED_EMAIL]"} {
		if !strings.Contains(clean, expected) {
			t.Fatalf("sanitized message missing %q: %s", expected, clean)
		}
	}
}

func TestPostgresDiagnosticUsesAllowlistedFieldsOnly(t *testing.T) {
	err := &pgconn.PgError{
		Code: "23505", Message: "duplicate key value violates unique constraint", Detail: "Key (email)=(private@example.com) already exists",
		TableName: "users", ConstraintName: "users_email_key",
	}
	errorType, message := Diagnostic(err)
	if errorType != "postgres:23505" {
		t.Fatalf("error type=%q", errorType)
	}
	if strings.Contains(message, "private@example.com") || strings.Contains(message, "already exists") {
		t.Fatalf("postgres detail leaked: %s", message)
	}
	if !strings.Contains(message, "constraint=users_email_key") {
		t.Fatalf("safe constraint missing: %s", message)
	}
}

func TestFingerprintIsStableAndExcludesRawSecret(t *testing.T) {
	entry := Entry{Category: CategoryUnexpected5xx, Method: "POST", RouteTemplate: "/api/payments", ErrorType: "error", InternalMessage: "token=first-secret"}
	first := Fingerprint(entry)
	entry.InternalMessage = "token=second-secret"
	second := Fingerprint(entry)
	if first != second {
		t.Fatalf("redacted-equivalent errors must have the same fingerprint: %s != %s", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("fingerprint length=%d", len(first))
	}
}
