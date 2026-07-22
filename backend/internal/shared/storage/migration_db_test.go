package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func TestSQLMigrationRepositoryInventoriesEveryLocalRowAcrossFiveFamilies(t *testing.T) {
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if strings.TrimSpace(dsn) == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("CONSFLOW_TEST_DATABASE_URL is required in CI; storage migration inventory must not skip")
		}
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed storage migration inventory tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	schema := "storage_migration_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	baseConfig, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*baseConfig)
	defer admin.Close()
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = admin.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	if baseConfig.RuntimeParams == nil {
		baseConfig.RuntimeParams = map[string]string{}
	}
	baseConfig.RuntimeParams["search_path"] = schema
	database := stdlib.OpenDB(*baseConfig)
	defer database.Close()
	if err := database.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE evidence_records (
			id uuid PRIMARY KEY, storage_provider text NOT NULL, storage_key text,
			file_size bigint, mime_type text, checksum_sha256 text
		);
		CREATE TABLE workflow_form_signatures (
			id uuid PRIMARY KEY, visual_storage_provider text NOT NULL, visual_storage_key text,
			visual_file_size bigint, visual_mime_type text, visual_checksum_sha256 text
		);
		CREATE TABLE workflow_form_pdf_exports (
			id uuid PRIMARY KEY, storage_provider text NOT NULL, storage_key text NOT NULL,
			file_size bigint NOT NULL, mime_type text, checksum_sha256 text NOT NULL
		);
		CREATE TABLE input_invoices (
			id uuid PRIMARY KEY, storage_provider text NOT NULL, storage_key text NOT NULL,
			file_size bigint NOT NULL, mime_type text, checksum_sha256 text NOT NULL
		);
		CREATE TABLE output_invoices (
			id uuid PRIMARY KEY, storage_provider text NOT NULL, storage_key text NOT NULL,
			file_size bigint NOT NULL, mime_type text, checksum_sha256 text NOT NULL
		);
	`); err != nil {
		t.Fatal(err)
	}

	type fixture struct {
		family  string
		id      string
		key     string
		content []byte
	}
	fixtures := []fixture{
		{family: "evidence_records", id: uuid.NewString(), key: "evidence/valid.pdf", content: []byte("evidence")},
		{family: "workflow_form_signatures", id: uuid.NewString(), key: "workflow-signatures/valid.svg", content: []byte("signature")},
		{family: "workflow_form_pdf_exports", id: uuid.NewString(), key: "workflow-form-pdfs/valid.pdf", content: []byte("workflow-pdf")},
		{family: "input_invoices", id: uuid.NewString(), key: "input-invoices/valid.pdf", content: []byte("input-invoice")},
		{family: "output_invoices", id: uuid.NewString(), key: "output-invoices/valid.pdf", content: []byte("output-invoice")},
	}
	for _, fixture := range fixtures {
		checksum := sha256.Sum256(fixture.content)
		checksumHex := hex.EncodeToString(checksum[:])
		var statement string
		switch fixture.family {
		case "evidence_records":
			statement = `INSERT INTO evidence_records VALUES ($1,'local',$2,$3,'application/pdf',$4)`
		case "workflow_form_signatures":
			statement = `INSERT INTO workflow_form_signatures VALUES ($1,'local',$2,$3,'image/svg+xml',$4)`
		case "workflow_form_pdf_exports":
			statement = `INSERT INTO workflow_form_pdf_exports VALUES ($1,'local',$2,$3,'application/pdf',$4)`
		case "input_invoices":
			statement = `INSERT INTO input_invoices VALUES ($1,'local',$2,$3,'application/pdf',$4)`
		case "output_invoices":
			statement = `INSERT INTO output_invoices VALUES ($1,'local',$2,$3,'application/pdf',$4)`
		}
		if _, err := database.ExecContext(ctx, statement, fixture.id, fixture.key, len(fixture.content), checksumHex); err != nil {
			t.Fatalf("insert %s: %v", fixture.family, err)
		}
	}
	invalidID := uuid.NewString()
	blankID := uuid.NewString()
	paddedID := uuid.NewString()
	invalidStatements := []struct {
		statement string
		id        string
	}{
		{`INSERT INTO evidence_records (id,storage_provider,storage_key,file_size,mime_type,checksum_sha256) VALUES ($1,'local',NULL,0,'application/pdf',NULL)`, invalidID},
		{`INSERT INTO input_invoices (id,storage_provider,storage_key,file_size,mime_type,checksum_sha256) VALUES ($1,'local','',0,'application/pdf','')`, blankID},
		{`INSERT INTO output_invoices (id,storage_provider,storage_key,file_size,mime_type,checksum_sha256) VALUES ($1,'local',' output-invoices/padded.pdf ',0,'application/pdf','')`, paddedID},
	}
	for _, fixture := range invalidStatements {
		if _, err := database.ExecContext(ctx, fixture.statement, fixture.id); err != nil {
			t.Fatal(err)
		}
	}

	repository, err := NewSQLMigrationRepository(database)
	if err != nil {
		t.Fatal(err)
	}
	records, err := repository.InventoryLocal(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 8 {
		t.Fatalf("inventory size=%d, want five valid plus three invalid local rows", len(records))
	}
	familyCounts := map[string]int{}
	invalidVisible := false
	for _, record := range records {
		familyCounts[record.Family]++
		if record.ID == invalidID {
			invalidVisible = true
			if record.Key != "" {
				t.Fatalf("invalid raw key=%q, want empty without normalization", record.Key)
			}
		}
		if record.ID == paddedID && record.Key != " output-invoices/padded.pdf " {
			t.Fatalf("padded raw key=%q, want byte-for-byte database value", record.Key)
		}
	}
	for _, fixture := range fixtures {
		if familyCounts[fixture.family] == 0 {
			t.Fatalf("family %s missing from inventory: %#v", fixture.family, familyCounts)
		}
	}
	if !invalidVisible {
		t.Fatal("invalid local row was hidden from operator inventory")
	}

	local := newMemoryStore(LocalProvider)
	for _, fixture := range fixtures {
		local.objects[fixture.key] = fixture.content
	}
	destination := newMemoryStore(S3Provider)
	summary, migrationErr := MigrateLocalToS3(ctx, repository, local, destination, MigrationOptions{})
	if migrationErr == nil || summary.Inventory != 8 || summary.SourceVerified != 5 || summary.Failed != 3 {
		t.Fatalf("dry-run error=%v summary=%#v", migrationErr, summary)
	}

	recordByFamily := map[string]MigrationRecord{}
	for _, record := range records {
		if validateMigrationRecord(record) == nil {
			recordByFamily[record.Family] = record
		}
	}
	for _, fixture := range fixtures {
		record := recordByFamily[fixture.family]
		updated, err := repository.MoveToS3(ctx, record)
		if err != nil || !updated {
			t.Fatalf("move %s updated=%v error=%v", fixture.family, updated, err)
		}
		updated, err = repository.MoveToS3(ctx, record)
		if err != nil || updated {
			t.Fatalf("old-value predicate %s updated=%v error=%v", fixture.family, updated, err)
		}
	}
	var moved int
	for _, table := range []string{"evidence_records", "workflow_form_signatures", "workflow_form_pdf_exports", "input_invoices", "output_invoices"} {
		providerColumn := "storage_provider"
		if table == "workflow_form_signatures" {
			providerColumn = "visual_storage_provider"
		}
		if err := database.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s='s3'`, table, providerColumn)).Scan(&moved); err != nil {
			t.Fatal(err)
		}
		if moved != 1 {
			t.Fatalf("%s moved rows=%d, want 1", table, moved)
		}
	}
}
