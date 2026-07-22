package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Migration struct {
	Version string
	Name    string
	Path    string
	SQL     string
	SHA256  string
}

type Result struct {
	Applied []string
	Skipped []string
}

type CheckResult struct {
	Count      int
	Migrations []string
}

type Options struct {
	LockTimeout time.Duration
}

const (
	defaultLockTimeout = 30 * time.Second
	// "Consflow" encoded as a positive int64. The key is intentionally stable so
	// every application version coordinates on the same PostgreSQL session lock.
	migrationAdvisoryLockKey int64 = 4859223964874731383
)

func Run(ctx context.Context, db *sql.DB, dir string) (Result, error) {
	return RunWithOptions(ctx, db, dir, Options{LockTimeout: defaultLockTimeout})
}

func RunWithOptions(ctx context.Context, db *sql.DB, dir string, options Options) (result Result, runErr error) {
	if strings.TrimSpace(dir) == "" {
		return Result{}, errors.New("migrations directory is required")
	}
	if db == nil {
		return Result{}, errors.New("migration database is required")
	}
	if options.LockTimeout <= 0 {
		return Result{}, errors.New("migration lock timeout must be positive")
	}
	migrations, err := load(dir)
	if err != nil {
		return Result{}, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("pin migration database connection: %w", err)
	}
	defer conn.Close()

	if err := acquireAdvisoryLock(ctx, conn, options.LockTimeout); err != nil {
		return Result{}, err
	}
	defer func() {
		unlockContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := releaseAdvisoryLock(unlockContext, conn); err != nil {
			runErr = errors.Join(runErr, err)
		}
	}()

	if err := ensureTable(ctx, conn); err != nil {
		return Result{}, err
	}
	if err := validateAppliedManifest(ctx, conn, migrations); err != nil {
		return Result{}, err
	}

	result = Result{}
	for _, migration := range migrations {
		applied, err := applyOne(ctx, conn, migration)
		if err != nil {
			return result, err
		}
		label := fmt.Sprintf("%s (%s)", migration.Version, migration.Name)
		if applied {
			result.Applied = append(result.Applied, label)
		} else {
			result.Skipped = append(result.Skipped, label)
		}
	}
	return result, nil
}

func validateAppliedManifest(ctx context.Context, conn *sql.Conn, manifest []Migration) error {
	knownVersions := make(map[string]struct{}, len(manifest))
	for _, migration := range manifest {
		knownVersions[migration.Version] = struct{}{}
	}
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return fmt.Errorf("read applied migration manifest: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return fmt.Errorf("scan applied migration manifest: %w", err)
		}
		if _, exists := knownVersions[version]; !exists {
			return fmt.Errorf("migration history contains version %s absent from the image migration manifest", version)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read applied migration manifest: %w", err)
	}
	return nil
}

func acquireAdvisoryLock(ctx context.Context, conn *sql.Conn, timeout time.Duration) error {
	lockContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var acquired bool
		if err := conn.QueryRowContext(lockContext, `SELECT pg_try_advisory_lock($1)`, migrationAdvisoryLockKey).Scan(&acquired); err != nil {
			if errors.Is(lockContext.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
				return fmt.Errorf("migration advisory lock timeout after %s", timeout)
			}
			return fmt.Errorf("acquire migration advisory lock: %w", err)
		}
		if acquired {
			return nil
		}
		select {
		case <-lockContext.Done():
			if errors.Is(lockContext.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
				return fmt.Errorf("migration advisory lock timeout after %s", timeout)
			}
			return fmt.Errorf("acquire migration advisory lock: %w", lockContext.Err())
		case <-ticker.C:
		}
	}
}

func releaseAdvisoryLock(ctx context.Context, conn *sql.Conn) error {
	var released bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_advisory_unlock($1)`, migrationAdvisoryLockKey).Scan(&released); err != nil {
		return fmt.Errorf("release migration advisory lock: %w", err)
	}
	if !released {
		return errors.New("release migration advisory lock: lock was not held by the pinned session")
	}
	return nil
}

func CheckDir(dir string) (CheckResult, error) {
	if strings.TrimSpace(dir) == "" {
		return CheckResult{}, errors.New("migrations directory is required")
	}
	migrations, err := load(dir)
	if err != nil {
		return CheckResult{}, err
	}
	result := CheckResult{
		Count:      len(migrations),
		Migrations: make([]string, 0, len(migrations)),
	}
	for _, migration := range migrations {
		result.Migrations = append(result.Migrations, fmt.Sprintf("%s (%s) %s", migration.Version, migration.Name, migration.SHA256))
	}
	return result, nil
}

func load(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	out := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name := parseName(entry.Name())
		if version == "" {
			return nil, fmt.Errorf("invalid migration filename %q: expected V1__name.sql or 001_name.sql", entry.Name())
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		sqlText := strings.TrimSpace(string(raw))
		if sqlText == "" {
			return nil, fmt.Errorf("migration %s is empty", entry.Name())
		}
		sum := sha256.Sum256(raw)
		out = append(out, Migration{
			Version: version,
			Name:    name,
			Path:    path,
			SQL:     sqlText,
			SHA256:  hex.EncodeToString(sum[:]),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return compareVersion(out[i].Version, out[j].Version) < 0
	})
	for i := 1; i < len(out); i++ {
		if out[i-1].Version == out[i].Version {
			return nil, fmt.Errorf("duplicate migration version %q in %s and %s", out[i].Version, out[i-1].Path, out[i].Path)
		}
	}
	return out, nil
}

func parseName(filename string) (string, string) {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if strings.HasPrefix(base, "V") && strings.Contains(base, "__") {
		parts := strings.SplitN(strings.TrimPrefix(base, "V"), "__", 2)
		return strings.TrimSpace(parts[0]), normalizeName(parts[1])
	}
	if strings.Contains(base, "__") {
		parts := strings.SplitN(base, "__", 2)
		return strings.TrimSpace(parts[0]), normalizeName(parts[1])
	}
	if strings.Contains(base, "_") {
		parts := strings.SplitN(base, "_", 2)
		return strings.TrimSpace(strings.TrimPrefix(parts[0], "V")), normalizeName(parts[1])
	}
	return "", ""
}

func normalizeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "_", " ")
	if value == "" {
		return "unnamed"
	}
	return value
}

func compareVersion(a, b string) int {
	ai, aErr := strconv.Atoi(a)
	bi, bErr := strconv.Atoi(b)
	if aErr == nil && bErr == nil {
		if ai < bi {
			return -1
		}
		if ai > bi {
			return 1
		}
		return 0
	}
	return strings.Compare(a, b)
}

func ensureTable(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			checksum_sha256 TEXT NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			execution_ms INTEGER NOT NULL DEFAULT 0
		)
	`)
	return err
}

func applyOne(ctx context.Context, conn *sql.Conn, migration Migration) (bool, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `LOCK TABLE schema_migrations IN EXCLUSIVE MODE`); err != nil {
		return false, fmt.Errorf("lock schema_migrations: %w", err)
	}

	var existingChecksum string
	err = tx.QueryRowContext(ctx, `SELECT checksum_sha256 FROM schema_migrations WHERE version=$1`, migration.Version).Scan(&existingChecksum)
	if err == nil {
		if existingChecksum != migration.SHA256 {
			return false, fmt.Errorf("migration %s checksum mismatch: database=%s file=%s", migration.Version, existingChecksum, migration.SHA256)
		}
		return false, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("read migration history %s: %w", migration.Version, err)
	}

	started := time.Now()
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return false, fmt.Errorf("apply migration %s (%s): %w", migration.Version, migration.Name, err)
	}
	executionMS := int(time.Since(started).Milliseconds())
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name, checksum_sha256, execution_ms)
		VALUES ($1, $2, $3, $4)
	`, migration.Version, migration.Name, migration.SHA256, executionMS); err != nil {
		return false, fmt.Errorf("record migration %s: %w", migration.Version, err)
	}
	return true, tx.Commit()
}
