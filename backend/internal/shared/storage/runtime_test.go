package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeDefaultUsesOnlyExplicitExternalFilesystemPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "external-storage")
	runtime, err := NewRuntime(context.Background(), RuntimeConfig{WriteProvider: "default", LocalRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if runtime.WriteProvider != LocalProvider {
		t.Fatalf("write provider=%q, want %q", runtime.WriteProvider, LocalProvider)
	}
	if _, err := runtime.Registry.Store(S3Provider); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("unexpected S3 registration error=%v", err)
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		t.Fatalf("external storage path stat=%v error=%v", info, err)
	}
}

func TestRuntimeS3DoesNotCreateOrRegisterLocalStorage(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret-key")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	localRoot := filepath.Join(t.TempDir(), "must-not-exist")
	runtime, err := NewRuntime(context.Background(), RuntimeConfig{
		WriteProvider: S3Provider,
		LocalRoot:     localRoot,
		S3: S3Config{
			Bucket: "artifacts",
			Region: "ap-southeast-1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if _, err := runtime.Registry.Store(LocalProvider); !errors.Is(err, ErrUnknownProvider) {
		t.Fatalf("local provider registration error=%v", err)
	}
	if _, err := os.Stat(localRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("S3 runtime touched local path: %v", err)
	}
}
