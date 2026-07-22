package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLocalPutOpenDeleteAndExclusiveCreate(t *testing.T) {
	store, err := NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	content := []byte("protected local object")
	info, err := store.Put(context.Background(), "evidence/project/item/file.txt", bytes.NewReader(content), PutOptions{
		ContentType:  "text/plain",
		ExpectedSize: int64(len(content)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Provider != LocalProvider || info.Size != int64(len(content)) || len(info.ChecksumSHA256) != 64 {
		t.Fatalf("unexpected metadata: %#v", info)
	}
	if _, err := store.Put(context.Background(), info.Key, bytes.NewReader(content), PutOptions{ExpectedSize: int64(len(content))}); !errors.Is(err, ErrObjectExists) {
		t.Fatalf("second Put error = %v, want ErrObjectExists", err)
	}

	object, err := store.Open(context.Background(), info.Key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(object.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := object.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, content) || object.Info.Size != int64(len(content)) || object.Info.ContentType != "text/plain; charset=utf-8" {
		t.Fatalf("opened object = %#v bytes=%q", object.Info, got)
	}

	if err := store.Delete(context.Background(), info.Key); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), info.Key); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
	if _, err := store.Open(context.Background(), info.Key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open after Delete error = %v, want ErrNotFound", err)
	}
}

func TestLocalRejectsTraversalAndCleansSizeMismatch(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	for _, key := range []string{"", ".", "..", "../outside", "/absolute", "nested/../../outside"} {
		if _, err := store.Put(context.Background(), key, bytes.NewReader([]byte("x")), PutOptions{ExpectedSize: 1}); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Put(%q) error = %v, want ErrInvalidKey", key, err)
		}
	}
	key := "generated/size-mismatch.pdf"
	if _, err := store.Put(context.Background(), key, bytes.NewReader([]byte("short")), PutOptions{ExpectedSize: 99}); err == nil {
		t.Fatal("expected size mismatch")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(key))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial object remains after mismatch: %v", err)
	}
}

func TestLocalRejectsSymlinkParentsAndObjects(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	store, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := os.Symlink(outside, filepath.Join(root, "linked-parent")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "linked-parent/escape.txt", bytes.NewReader([]byte("x")), PutOptions{ExpectedSize: 1}); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("symlink parent Put error = %v, want ErrInvalidKey", err)
	}

	target := filepath.Join(outside, "target.txt")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, "linked-object")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(context.Background(), "linked-object"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("symlink object Open error = %v, want ErrInvalidKey", err)
	}
	if err := store.Delete(context.Background(), "linked-object"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("symlink object Delete error = %v, want ErrInvalidKey", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "outside" {
		t.Fatalf("outside target changed: bytes=%q err=%v", got, err)
	}
}

func TestLocalDescriptorRelativePutDoesNotFollowParentSwap(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "safe"), 0o750); err != nil {
		t.Fatal(err)
	}
	store, err := NewLocal(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	store.beforeFinal = func() {
		store.beforeFinal = nil
		if err := os.Rename(filepath.Join(root, "safe"), filepath.Join(root, "safe-original")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(root, "safe")); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := store.Put(context.Background(), "safe/object.txt", bytes.NewReader([]byte("protected")), PutOptions{ExpectedSize: 9}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, "object.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("descriptor-relative Put escaped storage root: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "safe-original", "object.txt"))
	if err != nil || string(got) != "protected" {
		t.Fatalf("object was not written through pinned parent descriptor: bytes=%q err=%v", got, err)
	}
}
