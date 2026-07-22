package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

type MigrationRecord struct {
	Family         string
	ID             string
	Provider       string
	Key            string
	Size           int64
	ContentType    string
	ChecksumSHA256 string
}

type MigrationRepository interface {
	InventoryLocal(context.Context) ([]MigrationRecord, error)
	MoveToS3(context.Context, MigrationRecord) (bool, error)
}

type SQLMigrationRepository struct {
	db *sql.DB
}

func NewSQLMigrationRepository(db *sql.DB) (*SQLMigrationRepository, error) {
	if db == nil {
		return nil, errors.New("storage migration database is nil")
	}
	return &SQLMigrationRepository{db: db}, nil
}

func (r *SQLMigrationRepository) InventoryLocal(ctx context.Context) ([]MigrationRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT family, id, provider, storage_key, file_size, content_type, checksum_sha256
		FROM (
			SELECT 'evidence_records' AS family, id::text AS id, storage_provider AS provider,
				COALESCE(storage_key, '') AS storage_key, COALESCE(file_size, 0)::bigint AS file_size,
				COALESCE(mime_type, 'application/octet-stream') AS content_type,
				COALESCE(checksum_sha256, '') AS checksum_sha256
			FROM evidence_records
			WHERE storage_provider = 'local'
			UNION ALL
			SELECT 'workflow_form_signatures', id::text, visual_storage_provider,
				COALESCE(visual_storage_key, ''), COALESCE(visual_file_size, 0)::bigint,
				COALESCE(visual_mime_type, 'application/octet-stream'),
				COALESCE(visual_checksum_sha256, '')
			FROM workflow_form_signatures
			WHERE visual_storage_provider = 'local'
			UNION ALL
			SELECT 'workflow_form_pdf_exports', id::text, storage_provider,
				storage_key, file_size::bigint, COALESCE(mime_type, 'application/pdf'), checksum_sha256
			FROM workflow_form_pdf_exports
			WHERE storage_provider = 'local'
			UNION ALL
			SELECT 'input_invoices', id::text, storage_provider,
				storage_key, file_size::bigint, COALESCE(mime_type, 'application/pdf'), checksum_sha256
			FROM input_invoices
			WHERE storage_provider = 'local'
			UNION ALL
			SELECT 'output_invoices', id::text, storage_provider,
				storage_key, file_size::bigint, COALESCE(mime_type, 'application/pdf'), checksum_sha256
			FROM output_invoices
			WHERE storage_provider = 'local'
		) inventory
		ORDER BY family, id
	`)
	if err != nil {
		return nil, fmt.Errorf("inventory local storage metadata: %w", err)
	}
	defer rows.Close()

	records := make([]MigrationRecord, 0)
	for rows.Next() {
		var record MigrationRecord
		if err := rows.Scan(&record.Family, &record.ID, &record.Provider, &record.Key, &record.Size, &record.ContentType, &record.ChecksumSHA256); err != nil {
			return nil, fmt.Errorf("scan local storage metadata: %w", err)
		}
		record.Provider = strings.TrimSpace(record.Provider)
		record.ContentType = normalizedContentType(record.ContentType)
		record.ChecksumSHA256 = strings.ToLower(strings.TrimSpace(record.ChecksumSHA256))
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate local storage metadata: %w", err)
	}
	return records, nil
}

func (r *SQLMigrationRepository) MoveToS3(ctx context.Context, record MigrationRecord) (bool, error) {
	var query string
	switch record.Family {
	case "evidence_records":
		query = `UPDATE evidence_records SET storage_provider='s3', storage_key=$1 WHERE id=$2::uuid AND storage_provider='local' AND storage_key=$3`
	case "workflow_form_signatures":
		query = `UPDATE workflow_form_signatures SET visual_storage_provider='s3', visual_storage_key=$1 WHERE id=$2::uuid AND visual_storage_provider='local' AND visual_storage_key=$3`
	case "workflow_form_pdf_exports":
		query = `UPDATE workflow_form_pdf_exports SET storage_provider='s3', storage_key=$1 WHERE id=$2::uuid AND storage_provider='local' AND storage_key=$3`
	case "input_invoices":
		query = `UPDATE input_invoices SET storage_provider='s3', storage_key=$1 WHERE id=$2::uuid AND storage_provider='local' AND storage_key=$3`
	case "output_invoices":
		query = `UPDATE output_invoices SET storage_provider='s3', storage_key=$1 WHERE id=$2::uuid AND storage_provider='local' AND storage_key=$3`
	default:
		return false, fmt.Errorf("unsupported storage metadata family %q", record.Family)
	}
	result, err := r.db.ExecContext(ctx, query, record.Key, record.ID, record.Key)
	if err != nil {
		return false, fmt.Errorf("move %s storage metadata to S3: %w", record.Family, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

type MigrationOptions struct {
	Apply bool
}

type MigrationFailure struct {
	Family string `json:"family"`
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type MigrationSummary struct {
	DryRun          bool               `json:"dry_run"`
	Inventory       int                `json:"inventory"`
	SourceVerified  int                `json:"source_verified"`
	Copied          int                `json:"copied"`
	Reused          int                `json:"reused"`
	DestinationRead int                `json:"destination_verified"`
	Updated         int                `json:"updated"`
	Conflicts       int                `json:"update_conflicts"`
	Failed          int                `json:"failed"`
	Failures        []MigrationFailure `json:"failures,omitempty"`
}

func MigrateLocalToS3(ctx context.Context, repository MigrationRepository, source, destination Store, options MigrationOptions) (MigrationSummary, error) {
	summary := MigrationSummary{DryRun: !options.Apply}
	if repository == nil {
		return summary, errors.New("storage migration repository is nil")
	}
	if source == nil || source.Provider() != LocalProvider {
		return summary, errors.New("storage migration source must be local")
	}
	if destination == nil || destination.Provider() != S3Provider {
		return summary, errors.New("storage migration destination must be s3")
	}
	records, err := repository.InventoryLocal(ctx)
	if err != nil {
		return summary, err
	}
	summary.Inventory = len(records)
	for _, record := range records {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		if err := validateMigrationRecord(record); err != nil {
			summary.addFailure(record, err)
			continue
		}
		if !options.Apply {
			if _, _, err := hashStoredObject(ctx, source, record.Key, record.Size, record.ChecksumSHA256); err != nil {
				summary.addFailure(record, err)
				continue
			}
			summary.SourceVerified++
			continue
		}

		created, sourceHash, err := copyObject(ctx, source, destination, record)
		if err != nil {
			summary.addFailure(record, err)
			continue
		}
		summary.SourceVerified++
		if created {
			summary.Copied++
		} else {
			summary.Reused++
		}
		_, destinationHash, verifyErr := hashStoredObject(ctx, destination, record.Key, record.Size, sourceHash)
		if verifyErr != nil {
			if created {
				_ = destination.Delete(ctx, record.Key)
			}
			summary.addFailure(record, fmt.Errorf("verify S3 copy: %w", verifyErr))
			continue
		}
		if destinationHash != sourceHash {
			if created {
				_ = destination.Delete(ctx, record.Key)
			}
			summary.addFailure(record, errors.New("verify S3 copy: checksum mismatch"))
			continue
		}
		summary.DestinationRead++
		updated, updateErr := repository.MoveToS3(ctx, record)
		if updateErr != nil {
			summary.addFailure(record, updateErr)
			continue
		}
		if !updated {
			summary.Conflicts++
			summary.addFailure(record, errors.New("storage metadata changed before provider update"))
			continue
		}
		summary.Updated++
	}
	if summary.Failed > 0 {
		return summary, fmt.Errorf("storage migration completed with %d failed records", summary.Failed)
	}
	return summary, nil
}

func validateMigrationRecord(record MigrationRecord) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.Family) == "" {
		return errors.New("storage metadata identity is incomplete")
	}
	if strings.TrimSpace(record.Provider) != LocalProvider {
		return fmt.Errorf("source provider is %q, want local", record.Provider)
	}
	if record.Key != strings.TrimSpace(record.Key) {
		return errors.New("storage key is not canonical")
	}
	if _, err := storageKeySegments(record.Key); err != nil {
		return err
	}
	if record.Size < 0 {
		return errors.New("stored file size is negative")
	}
	if record.ChecksumSHA256 != "" && !validSHA256(record.ChecksumSHA256) {
		return errors.New("stored checksum is not a lowercase SHA-256 hex digest")
	}
	return nil
}

func copyObject(ctx context.Context, source, destination Store, record MigrationRecord) (bool, string, error) {
	object, err := source.Open(ctx, record.Key)
	if err != nil {
		return false, "", fmt.Errorf("open local source: %w", err)
	}
	hasher := sha256.New()
	counter := &countingReader{reader: io.TeeReader(object.Reader, hasher)}
	info, putErr := destination.Put(ctx, record.Key, counter, PutOptions{ContentType: record.ContentType, ExpectedSize: record.Size})
	closeErr := object.Reader.Close()
	if putErr != nil && !errors.Is(putErr, ErrObjectExists) {
		return false, "", fmt.Errorf("copy object to S3: %w", putErr)
	}
	if closeErr != nil {
		return false, "", fmt.Errorf("close local source: %w", closeErr)
	}
	if putErr == nil {
		if counter.count != record.Size || info.Size != record.Size {
			_ = destination.Delete(ctx, record.Key)
			return false, "", fmt.Errorf("copy object to S3: size mismatch local=%d destination=%d expected=%d", counter.count, info.Size, record.Size)
		}
		sourceHash := hex.EncodeToString(hasher.Sum(nil))
		if record.ChecksumSHA256 != "" && sourceHash != record.ChecksumSHA256 {
			_ = destination.Delete(ctx, record.Key)
			return false, "", errors.New("local source checksum does not match database metadata")
		}
		if info.ChecksumSHA256 != "" && info.ChecksumSHA256 != sourceHash {
			_ = destination.Delete(ctx, record.Key)
			return false, "", errors.New("S3 put checksum does not match streamed local source")
		}
		return true, sourceHash, nil
	}

	_, sourceHash, hashErr := hashStoredObject(ctx, source, record.Key, record.Size, record.ChecksumSHA256)
	if hashErr != nil {
		return false, "", fmt.Errorf("verify local source after existing S3 object: %w", hashErr)
	}
	return false, sourceHash, nil
}

func hashStoredObject(ctx context.Context, store Store, key string, expectedSize int64, expectedChecksum string) (int64, string, error) {
	object, err := store.Open(ctx, key)
	if err != nil {
		return 0, "", err
	}
	hasher := sha256.New()
	count, readErr := io.Copy(hasher, &contextReader{ctx: ctx, reader: object.Reader})
	closeErr := object.Reader.Close()
	if readErr != nil {
		return count, "", readErr
	}
	if closeErr != nil {
		return count, "", closeErr
	}
	if count != expectedSize {
		return count, "", fmt.Errorf("size mismatch: read %d, expected %d", count, expectedSize)
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))
	if expectedChecksum != "" && checksum != expectedChecksum {
		return count, checksum, errors.New("checksum mismatch")
	}
	return count, checksum, nil
}

func (s *MigrationSummary) addFailure(record MigrationRecord, err error) {
	s.Failed++
	if len(s.Failures) < 100 {
		s.Failures = append(s.Failures, MigrationFailure{Family: record.Family, ID: record.ID, Reason: err.Error()})
	}
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && value == strings.ToLower(value)
}
