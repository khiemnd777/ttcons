package eventlog

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	CategoryUnexpected5xx = "unexpected_5xx"
	CategoryPanic         = "panic"
	CategoryControlled503 = "controlled_503"

	RetentionDays       = 90
	recordTimeout       = 300 * time.Millisecond
	retentionInterval   = 24 * time.Hour
	retentionBatchSize  = 1000
	retentionMaxBatches = 100
	retentionLockKey    = int64(4859223964874731391)
	maxMessageRunes     = 4096
	maxStackRunes       = 16384
)

var (
	bearerPattern     = regexp.MustCompile(`(?i)\bbearer\s+[a-z0-9._~+/=-]+`)
	secretPattern     = regexp.MustCompile(`(?i)\b(password|passwd|token|secret|api[_-]?key|authorization)\s*[:=]\s*([^\s,;]+)`)
	dsnPattern        = regexp.MustCompile(`(?i)\b(postgres(?:ql)?|redis|smtp)://[^\s]+`)
	emailPattern      = regexp.MustCompile(`(?i)\b[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+\b`)
	urlQueryPattern   = regexp.MustCompile(`(?i)(https?://[^\s?]+)\?[^\s]+`)
	controlPattern    = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)
	whitespacePattern = regexp.MustCompile(`[\t ]+`)
)

type Entry struct {
	Category        string
	HTTPStatus      int
	ErrorCode       string
	Method          string
	RouteTemplate   string
	RequestClass    string
	RequestID       string
	ActorID         string
	ActorEmail      string
	ProjectID       string
	DurationMS      int64
	ErrorType       string
	InternalMessage string
	StackTrace      string
}

type Recorder interface {
	Record(context.Context, Entry) error
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Record(parent context.Context, entry Entry) error {
	if s == nil || s.db == nil {
		return errors.New("event log database is unavailable")
	}
	entry = normalizeEntry(entry)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), recordTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO event_logs (
			category,http_status,error_code,method,route_template,request_class,request_id,
			actor_id,actor_email,project_id,duration_ms,error_type,error_fingerprint,
			internal_message,stack_trace
		)
		VALUES (
			$1,$2,$3,$4,$5,$6,$7,
			NULLIF($8,'')::uuid,NULLIF($9,''),NULLIF($10,'')::uuid,$11,NULLIF($12,''),$13,
			NULLIF($14,''),NULLIF($15,'')
		)
	`, entry.Category, entry.HTTPStatus, entry.ErrorCode, entry.Method, entry.RouteTemplate,
		entry.RequestClass, entry.RequestID, entry.ActorID, entry.ActorEmail, entry.ProjectID,
		entry.DurationMS, entry.ErrorType, Fingerprint(entry), entry.InternalMessage, entry.StackTrace)
	if err != nil {
		return fmt.Errorf("insert event log: %w", err)
	}
	return nil
}

func Diagnostic(err error) (errorType, message string) {
	if err == nil {
		return "response", ""
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		parts := []string{"PostgreSQL " + safeIdentifier(postgresError.Code), SanitizeMessage(postgresError.Message)}
		for _, attribute := range []struct {
			label string
			value string
		}{
			{label: "table", value: postgresError.TableName},
			{label: "column", value: postgresError.ColumnName},
			{label: "constraint", value: postgresError.ConstraintName},
		} {
			if clean := safeIdentifier(attribute.value); clean != "" {
				parts = append(parts, attribute.label+"="+clean)
			}
		}
		return "postgres:" + safeIdentifier(postgresError.Code), strings.Join(parts, " ")
	}
	errorType = "error"
	if kind := reflect.TypeOf(err); kind != nil {
		errorType = kind.String()
	}
	return truncateRunes(errorType, 200), SanitizeMessage(err.Error())
}

func SanitizeMessage(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = controlPattern.ReplaceAllString(value, " ")
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = secretPattern.ReplaceAllString(value, "$1=[REDACTED]")
	value = dsnPattern.ReplaceAllString(value, "$1://[REDACTED]")
	value = urlQueryPattern.ReplaceAllString(value, "$1?[REDACTED]")
	value = emailPattern.ReplaceAllString(value, "[REDACTED_EMAIL]")
	value = whitespacePattern.ReplaceAllString(value, " ")
	return truncateRunes(strings.TrimSpace(value), maxMessageRunes)
}

func SanitizeStack(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = controlPattern.ReplaceAllString(value, "")
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = secretPattern.ReplaceAllString(value, "$1=[REDACTED]")
	value = dsnPattern.ReplaceAllString(value, "$1://[REDACTED]")
	value = urlQueryPattern.ReplaceAllString(value, "$1?[REDACTED]")
	value = emailPattern.ReplaceAllString(value, "[REDACTED_EMAIL]")
	return truncateRunes(strings.TrimSpace(value), maxStackRunes)
}

func Fingerprint(entry Entry) string {
	value := strings.Join([]string{
		entry.Category,
		strings.ToUpper(entry.Method),
		entry.RouteTemplate,
		entry.ErrorType,
		SanitizeMessage(entry.InternalMessage),
	}, "\x00")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (s *Store) StartRetention(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runRetention(ctx)
		ticker := time.NewTicker(retentionInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runRetention(ctx)
			}
		}
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Store) runRetention(parent context.Context) {
	if s == nil || s.db == nil || parent.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	deleted, err := s.DeleteExpired(ctx)
	if err != nil && parent.Err() == nil {
		log.Printf("event=event_log_retention_failed error=%q", SanitizeMessage(err.Error()))
		return
	}
	if deleted > 0 {
		log.Printf("event=event_log_retention_completed deleted=%d retention_days=%d", deleted, RetentionDays)
	}
}

func (s *Store) DeleteExpired(ctx context.Context) (deleted int64, runErr error) {
	if s == nil || s.db == nil {
		return 0, errors.New("event log database is unavailable")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return 0, fmt.Errorf("pin event log retention connection: %w", err)
	}
	defer conn.Close()
	var acquired bool
	if err := conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock($1)`, retentionLockKey).Scan(&acquired); err != nil {
		return 0, fmt.Errorf("acquire event log retention lock: %w", err)
	}
	if !acquired {
		return 0, nil
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		var released bool
		if err := conn.QueryRowContext(unlockCtx, `SELECT pg_advisory_unlock($1)`, retentionLockKey).Scan(&released); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("release event log retention lock: %w", err))
		} else if !released {
			runErr = errors.Join(runErr, errors.New("release event log retention lock: lock was not held"))
		}
	}()

	for batch := 0; batch < retentionMaxBatches; batch++ {
		result, err := conn.ExecContext(ctx, `
			DELETE FROM event_logs
			WHERE id IN (
				SELECT id
				FROM event_logs
				WHERE created_at < now() - INTERVAL '90 days'
				ORDER BY created_at, id
				LIMIT $1
			)
		`, retentionBatchSize)
		if err != nil {
			return deleted, fmt.Errorf("delete expired event logs: %w", err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return deleted, fmt.Errorf("count deleted event logs: %w", err)
		}
		deleted += rows
		if rows < retentionBatchSize {
			break
		}
	}
	return deleted, nil
}

func normalizeEntry(entry Entry) Entry {
	if !categoryAllowed(entry.Category) {
		entry.Category = CategoryUnexpected5xx
	}
	if entry.HTTPStatus < 500 || entry.HTTPStatus > 599 {
		entry.HTTPStatus = 500
	}
	entry.ErrorCode = truncateRunes(strings.TrimSpace(entry.ErrorCode), 120)
	if entry.ErrorCode == "" {
		entry.ErrorCode = "INTERNAL_ERROR"
	}
	entry.Method = truncateRunes(strings.ToUpper(strings.TrimSpace(entry.Method)), 16)
	if entry.Method == "" {
		entry.Method = "OTHER"
	}
	entry.RouteTemplate = truncateRunes(strings.TrimSpace(entry.RouteTemplate), 500)
	if entry.RouteTemplate == "" {
		entry.RouteTemplate = "__unmatched__"
	}
	entry.RequestClass = truncateRunes(strings.TrimSpace(entry.RequestClass), 40)
	if entry.RequestClass == "" {
		entry.RequestClass = "general"
	}
	entry.RequestID = truncateRunes(strings.TrimSpace(entry.RequestID), 128)
	if entry.RequestID == "" {
		entry.RequestID = "missing"
	}
	if _, err := uuid.Parse(entry.ActorID); err != nil {
		entry.ActorID = ""
	}
	if _, err := uuid.Parse(entry.ProjectID); err != nil {
		entry.ProjectID = ""
	}
	entry.ActorEmail = truncateRunes(strings.TrimSpace(entry.ActorEmail), 320)
	if entry.DurationMS < 0 {
		entry.DurationMS = 0
	}
	entry.ErrorType = truncateRunes(strings.TrimSpace(entry.ErrorType), 200)
	entry.InternalMessage = SanitizeMessage(entry.InternalMessage)
	entry.StackTrace = SanitizeStack(entry.StackTrace)
	return entry
}

func categoryAllowed(value string) bool {
	return value == CategoryUnexpected5xx || value == CategoryPanic || value == CategoryControlled503
}

func safeIdentifier(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(character rune) rune {
		switch {
		case character >= 'a' && character <= 'z':
			return character
		case character >= 'A' && character <= 'Z':
			return character
		case character >= '0' && character <= '9':
			return character
		case character == '_' || character == '.' || character == '-':
			return character
		default:
			return -1
		}
	}, value)
	return truncateRunes(value, 128)
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(value) <= limit {
		return value
	}
	runes := []rune(value)
	return string(runes[:limit])
}
