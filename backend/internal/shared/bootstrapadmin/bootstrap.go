package bootstrapadmin

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	bootstrapLockKey     int64 = 11220260820
	maxPasswordFileBytes       = 1024
)

type Config struct {
	Email        string
	FullName     string
	Department   string
	Position     string
	PasswordFile string
}

type Result struct {
	Created bool
	UserID  string
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Email:        strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))),
		FullName:     strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_FULL_NAME")),
		Department:   strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_DEPARTMENT")),
		Position:     strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_POSITION")),
		PasswordFile: strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_PASSWORD_FILE")),
	}
	if cfg.Department == "" {
		cfg.Department = "Công nghệ"
	}
	if cfg.Position == "" {
		cfg.Position = "Admin"
	}
	if err := validateIdentity(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Run(ctx context.Context, database *sql.DB, cfg Config) (Result, error) {
	if database == nil {
		return Result{}, errors.New("database connection is required")
	}
	cfg.Email = strings.ToLower(strings.TrimSpace(cfg.Email))
	cfg.FullName = strings.TrimSpace(cfg.FullName)
	cfg.Department = strings.TrimSpace(cfg.Department)
	cfg.Position = strings.TrimSpace(cfg.Position)
	if cfg.Department == "" {
		cfg.Department = "Công nghệ"
	}
	if cfg.Position == "" {
		cfg.Position = "Admin"
	}
	if err := validateIdentity(cfg); err != nil {
		return Result{}, err
	}

	tx, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return Result{}, fmt.Errorf("begin bootstrap transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, bootstrapLockKey); err != nil {
		return Result{}, fmt.Errorf("lock Admin bootstrap: %w", err)
	}
	if err := requireRBACCatalog(ctx, tx); err != nil {
		return Result{}, err
	}

	adminRoleID, err := activeAdminRoleID(ctx, tx)
	if err != nil {
		return Result{}, err
	}
	target, err := findTargetUser(ctx, tx, cfg.Email)
	if err != nil {
		return Result{}, err
	}
	if target != nil {
		isAdmin, err := targetIsActiveAdmin(ctx, tx, target.ID, target.Status, adminRoleID)
		if err != nil {
			return Result{}, err
		}
		if isAdmin {
			if err := tx.Commit(); err != nil {
				return Result{}, fmt.Errorf("commit Admin bootstrap no-op: %w", err)
			}
			return Result{Created: false, UserID: target.ID}, nil
		}
	}

	var activeAdminExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users account
			JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
			WHERE assignment.role_id=$1 AND account.status='active'
		)
	`, adminRoleID).Scan(&activeAdminExists); err != nil {
		return Result{}, fmt.Errorf("check existing Admin account: %w", err)
	}
	if activeAdminExists {
		return Result{}, errors.New("an active Admin already exists; bootstrap refuses to create another privileged account")
	}
	if target != nil {
		return Result{}, errors.New("bootstrap email already belongs to a non-Admin or inactive account")
	}

	password, err := readPasswordFile(cfg.PasswordFile)
	if err != nil {
		return Result{}, err
	}
	if count := utf8.RuneCountInString(password); count < 8 || len([]byte(password)) > 72 {
		return Result{}, errors.New("bootstrap Admin password must contain at least 8 characters and no more than 72 bytes")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Result{}, fmt.Errorf("hash bootstrap Admin password: %w", err)
	}
	password = ""

	var userID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email,password_hash,full_name,status)
		VALUES ($1,$2,$3,'active')
		RETURNING id::text
	`, cfg.Email, string(passwordHash), cfg.FullName).Scan(&userID); err != nil {
		return Result{}, fmt.Errorf("create bootstrap Admin account: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO staff (user_id,full_name,email,department,position,status)
		VALUES ($1,$2,$3,$4,$5,'active')
	`, userID, cfg.FullName, cfg.Email, cfg.Department, cfg.Position); err != nil {
		return Result{}, fmt.Errorf("create bootstrap Admin staff profile: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id,role_id,project_id)
		VALUES ($1,$2,NULL)
	`, userID, adminRoleID); err != nil {
		return Result{}, fmt.Errorf("assign bootstrap Admin role: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (
			actor_id,actor_name,event_type,target_type,target_id,description,metadata_json
		) VALUES (
			NULL,'system:bootstrap-admin','admin_bootstrap_created','user_account',$1,
			'Khởi tạo tài khoản Admin production',
			'{"source":"consflow-bootstrap-admin","scope":"global"}'::jsonb
		)
	`, userID); err != nil {
		return Result{}, fmt.Errorf("write bootstrap Admin audit event: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Result{}, fmt.Errorf("commit Admin bootstrap: %w", err)
	}
	return Result{Created: true, UserID: userID}, nil
}

type targetUser struct {
	ID     string
	Status string
}

func validateIdentity(cfg Config) error {
	if cfg.Email == "" {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL is required")
	}
	parsed, err := mail.ParseAddress(cfg.Email)
	if err != nil || parsed.Address != cfg.Email || strings.ContainsAny(cfg.Email, "\r\n") {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL must be a plain valid email address")
	}
	if count := utf8.RuneCountInString(cfg.FullName); count < 2 || count > 160 {
		return errors.New("BOOTSTRAP_ADMIN_FULL_NAME must contain between 2 and 160 characters")
	}
	if utf8.RuneCountInString(cfg.Department) > 160 || utf8.RuneCountInString(cfg.Position) > 160 {
		return errors.New("bootstrap Admin department and position must not exceed 160 characters")
	}
	return nil
}

func requireRBACCatalog(ctx context.Context, tx *sql.Tx) error {
	var migrationApplied bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version='112')
	`).Scan(&migrationApplied); err != nil {
		return fmt.Errorf("verify production RBAC migration: %w", err)
	}
	if !migrationApplied {
		return errors.New("production RBAC migration V112 is not applied; run consflow-migrate before Admin bootstrap")
	}
	return nil
}

func activeAdminRoleID(ctx context.Context, tx *sql.Tx) (string, error) {
	var roleID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text
		FROM roles
		WHERE name='admin' AND system_role=true AND external_only=false
		FOR SHARE
	`).Scan(&roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("production Admin role is missing or invalid after migration V112")
		}
		return "", fmt.Errorf("load production Admin role: %w", err)
	}

	var missingPermissions, forbiddenPermissions int
	if err := tx.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (
				WHERE permission.key <> 'settings.update'
				  AND assignment.permission_id IS NULL
			),
			COUNT(*) FILTER (
				WHERE permission.key = 'settings.update'
				  AND assignment.permission_id IS NOT NULL
			)
		FROM permissions permission
		LEFT JOIN role_permissions assignment
		  ON assignment.permission_id=permission.id
		 AND assignment.role_id=$1
	`, roleID).Scan(&missingPermissions, &forbiddenPermissions); err != nil {
		return "", fmt.Errorf("verify production Admin permissions: %w", err)
	}
	if missingPermissions != 0 || forbiddenPermissions != 0 {
		return "", errors.New("production Admin permission catalog is incomplete or violates reserved Settings permissions")
	}
	return roleID, nil
}

func findTargetUser(ctx context.Context, tx *sql.Tx, email string) (*targetUser, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id::text,status
		FROM users
		WHERE lower(email)=lower($1)
		ORDER BY id
		FOR UPDATE
	`, email)
	if err != nil {
		return nil, fmt.Errorf("look up bootstrap Admin account: %w", err)
	}
	defer rows.Close()

	var matches []targetUser
	for rows.Next() {
		var item targetUser
		if err := rows.Scan(&item.ID, &item.Status); err != nil {
			return nil, fmt.Errorf("read bootstrap Admin account: %w", err)
		}
		matches = append(matches, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan bootstrap Admin account: %w", err)
	}
	if len(matches) > 1 {
		return nil, errors.New("multiple case-insensitive accounts match the bootstrap email")
	}
	if len(matches) == 0 {
		return nil, nil
	}
	return &matches[0], nil
}

func targetIsActiveAdmin(ctx context.Context, tx *sql.Tx, userID, status, roleID string) (bool, error) {
	if status != "active" {
		return false, nil
	}
	var hasRole, hasStaff bool
	if err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (
				SELECT 1 FROM user_roles
				WHERE user_id=$1 AND role_id=$2 AND project_id IS NULL
			),
			EXISTS (
				SELECT 1 FROM staff
				WHERE user_id=$1 AND status='active'
			)
	`, userID, roleID).Scan(&hasRole, &hasStaff); err != nil {
		return false, fmt.Errorf("verify bootstrap Admin account: %w", err)
	}
	if hasRole && !hasStaff {
		return false, errors.New("existing Admin account has no active linked staff profile")
	}
	return hasRole && hasStaff, nil
}

func readPasswordFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("BOOTSTRAP_ADMIN_PASSWORD_FILE is required when no Admin exists")
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open BOOTSTRAP_ADMIN_PASSWORD_FILE: %w", err)
	}
	defer file.Close()

	reader := bufio.NewReader(io.LimitReader(file, maxPasswordFileBytes+1))
	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read BOOTSTRAP_ADMIN_PASSWORD_FILE: %w", err)
	}
	if len(raw) > maxPasswordFileBytes {
		return "", fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD_FILE exceeds %d bytes", maxPasswordFileBytes)
	}
	if strings.ContainsRune(string(raw), '\x00') {
		return "", errors.New("BOOTSTRAP_ADMIN_PASSWORD_FILE contains a NUL byte")
	}
	raw = []byte(strings.TrimSuffix(strings.TrimSuffix(string(raw), "\r\n"), "\n"))
	if len(raw) == 0 {
		return "", errors.New("BOOTSTRAP_ADMIN_PASSWORD_FILE is empty")
	}
	return string(raw), nil
}
