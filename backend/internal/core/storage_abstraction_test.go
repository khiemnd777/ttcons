package core

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	sharedstorage "consflow/backend/internal/shared/storage"

	"github.com/gofiber/fiber/v2"
)

type storageFakeStore struct {
	provider    string
	openCount   int
	deleteCount int
	deleteErr   error
	object      sharedstorage.Object
}

func (s *storageFakeStore) Provider() string {
	if s.provider == "" {
		return sharedstorage.LocalProvider
	}
	return s.provider
}

func (s *storageFakeStore) Put(context.Context, string, io.Reader, sharedstorage.PutOptions) (sharedstorage.ObjectInfo, error) {
	return sharedstorage.ObjectInfo{}, errors.New("unexpected Put")
}

func (s *storageFakeStore) Open(context.Context, string) (sharedstorage.Object, error) {
	s.openCount++
	return s.object, nil
}

func (s *storageFakeStore) Delete(context.Context, string) error {
	s.deleteCount++
	return s.deleteErr
}

type closeSpyReader struct {
	*bytes.Reader
	closed bool
}

func (r *closeSpyReader) Close() error {
	r.closed = true
	return nil
}

func TestSendProtectedFileStreamsHeadersBytesAndClosesReader(t *testing.T) {
	content := []byte("protected bytes")
	reader := &closeSpyReader{Reader: bytes.NewReader(content)}
	app := fiber.New()
	app.Get("/file", func(c *fiber.Ctx) error {
		return sendProtectedFile(c, EvidenceFileResult{
			Reader: reader, Size: int64(len(content)), MIMEType: "application/pdf", Filename: "hóa-đơn.pdf",
		}, "fallback.pdf")
	})

	response, err := app.Test(newTestRequest(http.MethodGet, "/file"))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("response bytes = %q, want %q", got, content)
	}
	if response.Header.Get("Content-Type") != "application/pdf" || response.Header.Get("Content-Length") != "15" {
		t.Fatalf("headers = %#v", response.Header)
	}
	if response.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(response.Header.Get("Content-Disposition"), "filename*=UTF-8''") {
		t.Fatalf("protected headers = %#v", response.Header)
	}
	if !reader.closed {
		t.Fatal("stream reader was not closed by Fiber/fasthttp response lifecycle")
	}
}

func TestCleanupStoredObjectPreservesOriginalErrorAndDeletes(t *testing.T) {
	fake := &storageFakeStore{}
	registry, err := sharedstorage.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, "test-secret")
	service.storageRegistry = registry
	originalErr := errors.New("database insert failed")
	mutationErr := &storedMutationError{err: originalErr, cleanupSafe: true}
	got := service.cleanupStoredObjectAfterError(storedObjectRef{Provider: "local", Key: "evidence/object"}, mutationErr)
	if !errors.Is(got, originalErr) {
		t.Fatalf("cleanup error = %v, want original DB error", got)
	}
	if fake.deleteCount != 1 {
		t.Fatalf("Delete count = %d, want 1", fake.deleteCount)
	}
}

func TestCleanupStoredObjectLogsOrphanRiskWithoutReplacingOriginalError(t *testing.T) {
	fake := &storageFakeStore{deleteErr: errors.New("delete unavailable")}
	registry, err := sharedstorage.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, "test-secret")
	service.storageRegistry = registry

	originalWriter := log.Writer()
	originalFlags := log.Flags()
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	})

	originalErr := errors.New("database transaction failed")
	mutationErr := &storedMutationError{err: originalErr, cleanupSafe: true}
	got := service.cleanupStoredObjectAfterError(storedObjectRef{Provider: "local", Key: "invoice/object.pdf"}, mutationErr)
	if !errors.Is(got, originalErr) {
		t.Fatalf("cleanup error = %v, want original DB error", got)
	}
	logLine := output.String()
	for _, field := range []string{"component=storage", "operation=cleanup", "orphan_risk=true", `provider="local"`, `key="invoice/object.pdf"`} {
		if !strings.Contains(logLine, field) {
			t.Fatalf("cleanup log %q missing %q", logLine, field)
		}
	}
}

func TestCleanupStoredObjectSkipsDeleteWhenCommitOutcomeIsUnknown(t *testing.T) {
	fake := &storageFakeStore{}
	registry, err := sharedstorage.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, "test-secret")
	service.storageRegistry = registry

	originalWriter := log.Writer()
	originalFlags := log.Flags()
	var output bytes.Buffer
	log.SetOutput(&output)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	})

	originalErr := errors.New("commit connection lost")
	mutationErr := &storedMutationError{err: originalErr, cleanupSafe: false}
	got := service.cleanupStoredObjectAfterError(storedObjectRef{Provider: "local", Key: "invoice/uncertain.pdf"}, mutationErr)
	if !errors.Is(got, originalErr) || fake.deleteCount != 0 {
		t.Fatalf("cleanup result=%v deleteCount=%d, want original error and no delete", got, fake.deleteCount)
	}
	for _, field := range []string{"operation=cleanup_skipped", "commit_state=unknown", "orphan_risk=true"} {
		if !strings.Contains(output.String(), field) {
			t.Fatalf("cleanup log %q missing %q", output.String(), field)
		}
	}
}

func TestStoredMutationTrackerClassifiesPreCommitAndCommitAttemptErrors(t *testing.T) {
	detailErr := errors.New("detail read failed")
	tracker := storedMutationTracker{}
	commitCalls := 0
	_, preCommit := finalizeStoredMutation(&tracker, func() (Record, error) {
		return nil, detailErr
	}, func() error {
		commitCalls++
		return nil
	})
	tracker.wrap(&preCommit)
	if !errors.Is(preCommit, detailErr) || !storedMutationCleanupSafe(preCommit) || commitCalls != 0 {
		t.Fatal("pre-commit detail failure must be cleanup-safe")
	}

	commitErr := errors.New("commit outcome unknown")
	tracker = storedMutationTracker{}
	_, commitAttempt := finalizeStoredMutation(&tracker, func() (Record, error) {
		return Record{"id": "committed-candidate"}, nil
	}, func() error { return commitErr })
	tracker.wrap(&commitAttempt)
	if !errors.Is(commitAttempt, commitErr) || storedMutationCleanupSafe(commitAttempt) {
		t.Fatal("commit-attempt failure must not trigger object deletion")
	}
}

func TestEvidenceAuthorizationRunsBeforeStorageOpen(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupWorkItemFlowFixture(t, db, false)
	outside := setupProjectScopeFixture(t, db, []string{"operation.view"})
	evidenceID := newID()
	mustExec(t, db, `
		INSERT INTO evidence_records
			(id, work_item_id, project_id, target_type, target_id, evidence_type, name, mime_type, file_size, storage_provider, storage_key, visibility, status, uploaded_by)
		VALUES
			($1,$2,$3,'work_item',$2,'document','Protected','application/pdf',3,'local','evidence/protected.pdf','project','submitted',$4)
	`, evidenceID, fixture.itemID, fixture.projectID, fixture.workerID)

	fake := &storageFakeStore{object: sharedstorage.Object{
		Reader: io.NopCloser(bytes.NewReader([]byte("pdf"))),
		Info:   sharedstorage.ObjectInfo{Provider: "local", Key: "evidence/protected.pdf", Size: 3, ContentType: "application/pdf"},
	}}
	registry, err := sharedstorage.NewRegistry(fake)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(NewRepository(db), "test-secret")
	service.storageRegistry = registry

	_, err = service.EvidenceFile(ctx, evidenceID, outside.userID)
	if err == nil {
		t.Fatal("expected authorization error")
	}
	var forbidden StatusError
	if !errors.As(err, &forbidden) || forbidden.Status != fiber.StatusForbidden {
		t.Fatalf("error = %T %v, want forbidden", err, err)
	}
	if fake.openCount != 0 {
		t.Fatalf("Open count = %d, want 0 before authorization", fake.openCount)
	}
}

func TestActiveS3WriterKeepsLegacyLocalObjectsReadable(t *testing.T) {
	localReader := &closeSpyReader{Reader: bytes.NewReader([]byte("legacy"))}
	local := &storageFakeStore{object: sharedstorage.Object{
		Reader: localReader,
		Info:   sharedstorage.ObjectInfo{Provider: sharedstorage.LocalProvider, Key: "evidence/legacy.pdf", Size: 6, ContentType: "application/pdf"},
	}}
	s3Store := &storageFakeStore{provider: sharedstorage.S3Provider}
	registry, err := sharedstorage.NewRegistry(local, s3Store)
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(nil, "test-secret")
	if err := service.ConfigureStorage(registry, sharedstorage.S3Provider, 1024); err != nil {
		t.Fatal(err)
	}
	writerRegistry, provider, err := service.storageWriter()
	if err != nil || writerRegistry != registry || provider != sharedstorage.S3Provider {
		t.Fatalf("writer registry=%p provider=%q error=%v", writerRegistry, provider, err)
	}
	result, err := service.storedFileResult(context.Background(), Record{
		"storage_provider": sharedstorage.LocalProvider,
		"storage_key":      "evidence/legacy.pdf",
		"mime_type":        "application/pdf",
	}, "storage_provider", "storage_key", "mime_type", "legacy.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer result.Reader.Close()
	if local.openCount != 1 || s3Store.openCount != 0 || result.Size != 6 {
		t.Fatalf("local opens=%d s3 opens=%d result=%#v", local.openCount, s3Store.openCount, result)
	}
}

func TestPublicEvidenceMetadataCannotInjectS3StorageKey(t *testing.T) {
	_, err := validateWorkItemEvidenceRequest(WorkItemEvidenceRequest{
		EvidenceType:    "document",
		Name:            "Injected",
		StorageProvider: sharedstorage.S3Provider,
		StorageKey:      "evidence/injected.pdf",
	})
	if err == nil {
		t.Fatal("public evidence metadata accepted an S3 storage key")
	}
	if _, err := validateWorkItemEvidenceRequestForStorage(WorkItemEvidenceRequest{
		EvidenceType:    "document",
		Name:            "Internal upload",
		StorageProvider: sharedstorage.S3Provider,
		StorageKey:      "evidence/internal.pdf",
	}, true); err != nil {
		t.Fatalf("trusted internal upload rejected S3 storage: %v", err)
	}
}

func newTestRequest(method, target string) *http.Request {
	request, _ := http.NewRequest(method, target, nil)
	return request
}
