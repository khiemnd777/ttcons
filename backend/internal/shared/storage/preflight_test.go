package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type preflightStore struct {
	putCount          int
	openCount         int
	deleteCount       int
	key               string
	payload           []byte
	putErr            error
	putErrors         []error
	openErr           error
	deleteErr         error
	corruptRead       bool
	cancelOnOpen      context.CancelFunc
	deleteSawCanceled bool
}

func (s *preflightStore) Provider() string { return S3Provider }

func (s *preflightStore) Put(_ context.Context, key string, source io.Reader, options PutOptions) (ObjectInfo, error) {
	s.putCount++
	s.key = key
	s.payload, _ = io.ReadAll(source)
	if len(s.putErrors) >= s.putCount && s.putErrors[s.putCount-1] != nil {
		return ObjectInfo{}, s.putErrors[s.putCount-1]
	}
	if s.putErr != nil {
		return ObjectInfo{}, s.putErr
	}
	return ObjectInfo{Provider: S3Provider, Key: key, Size: int64(len(s.payload)), ContentType: options.ContentType}, nil
}

func (s *preflightStore) Open(_ context.Context, key string) (Object, error) {
	s.openCount++
	if s.cancelOnOpen != nil {
		s.cancelOnOpen()
	}
	if s.openErr != nil {
		return Object{}, s.openErr
	}
	payload := append([]byte(nil), s.payload...)
	if s.corruptRead && len(payload) > 0 {
		payload[0] ^= 0xff
	}
	return Object{Reader: io.NopCloser(bytes.NewReader(payload)), Info: ObjectInfo{Provider: S3Provider, Key: key, Size: int64(len(payload))}}, nil
}

func (s *preflightStore) Delete(ctx context.Context, _ string) error {
	s.deleteCount++
	s.deleteSawCanceled = ctx.Err() != nil
	return s.deleteErr
}

func TestStoragePreflightProvesPutOpenReadAndDelete(t *testing.T) {
	store := &preflightStore{}
	if err := VerifyWriteAccess(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if store.putCount != 1 || store.openCount != 1 || store.deleteCount != 1 || !strings.HasPrefix(store.key, "_consflow/probes/") || len(store.payload) != 16 {
		t.Fatalf("put=%d open=%d delete=%d key=%q payload=%d", store.putCount, store.openCount, store.deleteCount, store.key, len(store.payload))
	}
}

func TestStoragePreflightCleansUpPutOrOpenFailure(t *testing.T) {
	for _, test := range []struct {
		name    string
		putErr  error
		openErr error
	}{
		{name: "put", putErr: errors.New("put denied")},
		{name: "open", openErr: errors.New("open denied")},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := &preflightStore{putErr: test.putErr, openErr: test.openErr}
			err := VerifyWriteAccess(context.Background(), store)
			wantDeletes := 1
			if test.putErr != nil {
				wantDeletes = 0
			}
			if err == nil || store.deleteCount != wantDeletes {
				t.Fatalf("error=%v delete=%d", err, store.deleteCount)
			}
		})
	}
}

func TestStoragePreflightRetriesRandomKeyCollisionWithoutDeleting(t *testing.T) {
	store := &preflightStore{putErrors: []error{ErrObjectExists, ErrObjectExists, nil}}
	if err := VerifyWriteAccess(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if store.putCount != 3 || store.deleteCount != 1 {
		t.Fatalf("put=%d delete=%d, want three conditional puts and only final probe cleanup", store.putCount, store.deleteCount)
	}
}

func TestStoragePreflightCleanupIgnoresCanceledOperationContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &preflightStore{openErr: errors.New("open failed"), cancelOnOpen: cancel}
	err := VerifyWriteAccess(ctx, store)
	if err == nil || store.deleteCount != 1 || store.deleteSawCanceled {
		t.Fatalf("error=%v delete=%d canceled_cleanup=%v", err, store.deleteCount, store.deleteSawCanceled)
	}
}

func TestStoragePreflightRejectsCorruptReadAndCleansUp(t *testing.T) {
	store := &preflightStore{corruptRead: true}
	err := VerifyWriteAccess(context.Background(), store)
	if err == nil || !strings.Contains(err.Error(), "integrity mismatch") || store.deleteCount != 1 {
		t.Fatalf("error=%v delete=%d", err, store.deleteCount)
	}
}

func TestStoragePreflightReportsDeleteAndCleanupErrors(t *testing.T) {
	deleteErr := errors.New("delete denied")
	store := &preflightStore{openErr: errors.New("open denied"), deleteErr: deleteErr}
	err := VerifyWriteAccess(context.Background(), store)
	if err == nil || !errors.Is(err, deleteErr) || !strings.Contains(err.Error(), "open denied") || !strings.Contains(err.Error(), "cleanup") {
		t.Fatalf("joined cleanup error=%v", err)
	}

	store = &preflightStore{deleteErr: deleteErr}
	err = VerifyWriteAccess(context.Background(), store)
	if err == nil || !errors.Is(err, deleteErr) || !strings.Contains(err.Error(), "preflight delete") {
		t.Fatalf("delete-step error=%v", err)
	}
}
