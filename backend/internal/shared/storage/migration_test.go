package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sync"
	"testing"
)

type memoryStore struct {
	provider string
	mu       sync.Mutex
	objects  map[string][]byte
}

type memoryReadSeekCloser struct {
	*bytes.Reader
}

func (memoryReadSeekCloser) Close() error { return nil }

func newMemoryStore(provider string) *memoryStore {
	return &memoryStore{provider: provider, objects: map[string][]byte{}}
}

func (s *memoryStore) Provider() string { return s.provider }

func (s *memoryStore) Put(_ context.Context, key string, source io.Reader, options PutOptions) (ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.objects[key]; exists {
		return ObjectInfo{}, ErrObjectExists
	}
	content, err := io.ReadAll(source)
	if err != nil {
		return ObjectInfo{}, err
	}
	if options.ExpectedSize >= 0 && int64(len(content)) != options.ExpectedSize {
		return ObjectInfo{}, errors.New("size mismatch")
	}
	s.objects[key] = append([]byte(nil), content...)
	hash := sha256.Sum256(content)
	return ObjectInfo{Provider: s.provider, Key: key, Size: int64(len(content)), ContentType: options.ContentType, ChecksumSHA256: hex.EncodeToString(hash[:])}, nil
}

func (s *memoryStore) Open(_ context.Context, key string) (Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	content, exists := s.objects[key]
	if !exists {
		return Object{}, ErrNotFound
	}
	return Object{Reader: memoryReadSeekCloser{Reader: bytes.NewReader(append([]byte(nil), content...))}, Info: ObjectInfo{Provider: s.provider, Key: key, Size: int64(len(content))}}, nil
}

func (s *memoryStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.objects, key)
	s.mu.Unlock()
	return nil
}

type fakeMigrationRepository struct {
	records  []MigrationRecord
	conflict bool
	updates  int
}

func (r *fakeMigrationRepository) InventoryLocal(context.Context) ([]MigrationRecord, error) {
	result := make([]MigrationRecord, 0, len(r.records))
	for _, record := range r.records {
		if record.Provider == LocalProvider {
			result = append(result, record)
		}
	}
	return result, nil
}

func (r *fakeMigrationRepository) MoveToS3(_ context.Context, candidate MigrationRecord) (bool, error) {
	if r.conflict {
		return false, nil
	}
	for index := range r.records {
		if r.records[index].Family == candidate.Family && r.records[index].ID == candidate.ID && r.records[index].Provider == LocalProvider && r.records[index].Key == candidate.Key {
			r.records[index].Provider = S3Provider
			r.updates++
			return true, nil
		}
	}
	return false, nil
}

func testMigrationRecord(content []byte) MigrationRecord {
	hash := sha256.Sum256(content)
	return MigrationRecord{
		Family:         "evidence_records",
		ID:             "00000000-0000-0000-0000-000000000001",
		Provider:       LocalProvider,
		Key:            "evidence/project/object.pdf",
		Size:           int64(len(content)),
		ContentType:    "application/pdf",
		ChecksumSHA256: hex.EncodeToString(hash[:]),
	}
}

func TestStorageMigrationDryRunVerifiesWithoutCopyingOrUpdating(t *testing.T) {
	content := []byte("dry-run")
	record := testMigrationRecord(content)
	repository := &fakeMigrationRepository{records: []MigrationRecord{record}}
	local := newMemoryStore(LocalProvider)
	local.objects[record.Key] = content
	destination := newMemoryStore(S3Provider)

	summary, err := MigrateLocalToS3(context.Background(), repository, local, destination, MigrationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !summary.DryRun || summary.Inventory != 1 || summary.SourceVerified != 1 || summary.Copied != 0 || summary.Updated != 0 || repository.updates != 0 {
		t.Fatalf("summary=%#v updates=%d", summary, repository.updates)
	}
	if len(destination.objects) != 0 {
		t.Fatal("dry-run copied an object")
	}
}

func TestStorageMigrationCopiesVerifiesUpdatesAndRerunsIdempotently(t *testing.T) {
	content := []byte("artifact")
	record := testMigrationRecord(content)
	repository := &fakeMigrationRepository{records: []MigrationRecord{record}}
	local := newMemoryStore(LocalProvider)
	local.objects[record.Key] = content
	destination := newMemoryStore(S3Provider)

	summary, err := MigrateLocalToS3(context.Background(), repository, local, destination, MigrationOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Copied != 1 || summary.DestinationRead != 1 || summary.Updated != 1 || repository.updates != 1 {
		t.Fatalf("first summary=%#v updates=%d", summary, repository.updates)
	}
	if string(local.objects[record.Key]) != string(content) {
		t.Fatal("source local object was deleted or changed")
	}

	summary, err = MigrateLocalToS3(context.Background(), repository, local, destination, MigrationOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Inventory != 0 || summary.Updated != 0 || repository.updates != 1 {
		t.Fatalf("rerun summary=%#v updates=%d", summary, repository.updates)
	}
}

func TestStorageMigrationReusesVerifiedDestinationAfterPartialRun(t *testing.T) {
	content := []byte("existing")
	record := testMigrationRecord(content)
	repository := &fakeMigrationRepository{records: []MigrationRecord{record}}
	local := newMemoryStore(LocalProvider)
	local.objects[record.Key] = content
	destination := newMemoryStore(S3Provider)
	destination.objects[record.Key] = content

	summary, err := MigrateLocalToS3(context.Background(), repository, local, destination, MigrationOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Reused != 1 || summary.DestinationRead != 1 || summary.Updated != 1 {
		t.Fatalf("summary=%#v", summary)
	}
}

func TestStorageMigrationRejectsChecksumMismatchWithoutUpdating(t *testing.T) {
	content := []byte("database expected bytes")
	record := testMigrationRecord(content)
	repository := &fakeMigrationRepository{records: []MigrationRecord{record}}
	local := newMemoryStore(LocalProvider)
	local.objects[record.Key] = []byte("different object bytes")
	destination := newMemoryStore(S3Provider)

	summary, err := MigrateLocalToS3(context.Background(), repository, local, destination, MigrationOptions{Apply: true})
	if err == nil || summary.Failed != 1 || repository.updates != 0 {
		t.Fatalf("error=%v summary=%#v updates=%d", err, summary, repository.updates)
	}
	if _, exists := destination.objects[record.Key]; exists {
		t.Fatal("invalid destination copy was not cleaned up")
	}
}

func TestStorageMigrationReportsOldValueUpdateRaceAndPreservesCopies(t *testing.T) {
	content := []byte("race")
	record := testMigrationRecord(content)
	repository := &fakeMigrationRepository{records: []MigrationRecord{record}, conflict: true}
	local := newMemoryStore(LocalProvider)
	local.objects[record.Key] = content
	destination := newMemoryStore(S3Provider)

	summary, err := MigrateLocalToS3(context.Background(), repository, local, destination, MigrationOptions{Apply: true})
	if err == nil || summary.Conflicts != 1 || summary.Failed != 1 || summary.Updated != 0 {
		t.Fatalf("error=%v summary=%#v", err, summary)
	}
	if string(destination.objects[record.Key]) != string(content) || string(local.objects[record.Key]) != string(content) {
		t.Fatal("update race removed a recoverable object copy")
	}
}
