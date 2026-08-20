package bootstrapadmin

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

func TestReadPasswordFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "admin_password")
	if err := os.WriteFile(path, []byte("mật-khẩu-production-123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	password, err := readPasswordFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if password != "mật-khẩu-production-123" {
		t.Fatalf("password=%q, want one trailing line ending removed", password)
	}

	if err := os.WriteFile(path, []byte("bad\x00secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readPasswordFile(path); err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("NUL password error=%v", err)
	}
}

func TestRunCreatesOneAuditedAdminAndThenNoOps(t *testing.T) {
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("CONSFLOW_TEST_DATABASE_URL is required in CI; Admin bootstrap integration must not skip")
		}
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run Admin bootstrap integration")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	schema := fmt.Sprintf("bootstrap_admin_test_%d", time.Now().UnixNano())
	adminDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer adminDB.Close()
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+quoteIdentifier(schema)); err != nil {
		t.Fatal(err)
	}
	defer adminDB.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+quoteIdentifier(schema)+` CASCADE`)

	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	defer database.Close()
	if _, err := database.ExecContext(ctx, `SET search_path TO `+quoteIdentifier(schema)+`, public`); err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.Run(ctx, database, filepath.Join("..", "..", "..", "migrations")); err != nil {
		t.Fatalf("apply migrations for Admin bootstrap: %v", err)
	}

	secretPath := filepath.Join(t.TempDir(), "bootstrap_admin_password")
	plainPassword := "production-admin-123"
	if err := os.WriteFile(secretPath, []byte(plainPassword+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		Email:        "admin@tt-dnh.com",
		FullName:     "Quản trị hệ thống",
		Department:   "Công nghệ",
		Position:     "Admin",
		PasswordFile: secretPath,
	}
	result, err := Run(ctx, database, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.UserID == "" {
		t.Fatalf("result=%+v, want newly created Admin", result)
	}

	var passwordHash, userStatus, staffStatus, roleName string
	if err := database.QueryRowContext(ctx, `
		SELECT account.password_hash,account.status,person.status,role.name
		FROM users account
		JOIN staff person ON person.user_id=account.id
		JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
		JOIN roles role ON role.id=assignment.role_id
		WHERE account.id=$1
	`, result.UserID).Scan(&passwordHash, &userStatus, &staffStatus, &roleName); err != nil {
		t.Fatal(err)
	}
	if userStatus != "active" || staffStatus != "active" || roleName != "admin" {
		t.Fatalf("unexpected bootstrap identity state user=%s staff=%s role=%s", userStatus, staffStatus, roleName)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(plainPassword)); err != nil {
		t.Fatal("bootstrap password was not stored as bcrypt")
	}

	var auditCount int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE event_type='admin_bootstrap_created'
		  AND target_id=$1
		  AND actor_id IS NULL
		  AND actor_name='system:bootstrap-admin'
		  AND metadata_json::text NOT ILIKE '%tt-dnh%'
		  AND metadata_json::text NOT ILIKE '%password%'
	`, result.UserID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("bootstrap audit count=%d, want 1", auditCount)
	}

	if err := os.Remove(secretPath); err != nil {
		t.Fatal(err)
	}
	second, err := Run(ctx, database, cfg)
	if err != nil {
		t.Fatalf("idempotent bootstrap must not reread the removed password secret: %v", err)
	}
	if second.Created || second.UserID != result.UserID {
		t.Fatalf("second result=%+v, want no-op for %s", second, result.UserID)
	}

	other := cfg
	other.Email = "other-admin@tt-dnh.com"
	if _, err := Run(ctx, database, other); err == nil || !strings.Contains(err.Error(), "active Admin already exists") {
		t.Fatalf("second privileged account error=%v", err)
	}
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
