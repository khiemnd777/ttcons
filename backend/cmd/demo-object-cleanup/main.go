package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"consflow/backend/internal/shared/storage"
)

type composeConfig struct {
	Services map[string]struct {
		Environment map[string]string `json:"environment"`
	} `json:"services"`
	Secrets map[string]struct {
		File string `json:"file"`
	} `json:"secrets"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "demo S3 object cleanup failed:", err)
		os.Exit(1)
	}
}

func run() error {
	checkOnly := len(os.Args) == 3 && os.Args[1] == "--check"
	if len(os.Args) != 2 && !checkOnly {
		return errors.New("expected one manifest path, optionally preceded by --check")
	}
	manifest, err := os.Open(os.Args[len(os.Args)-1])
	if err != nil {
		return errors.New("cannot open object manifest")
	}
	defer manifest.Close()
	keys, err := readManifest(manifest)
	if err != nil {
		return err
	}
	var compose composeConfig
	if err := json.NewDecoder(io.LimitReader(os.Stdin, 2<<20)).Decode(&compose); err != nil {
		return errors.New("cannot decode production Compose configuration")
	}
	env := compose.Services["backend"].Environment
	credentials := compose.Secrets["aws_credentials"].File
	if env["STORAGE_PROVIDER"] != "s3" || env["STORAGE_S3_BUCKET"] == "" ||
		env["STORAGE_S3_REGION"] == "" || credentials == "" {
		return errors.New("production S3 configuration is incomplete")
	}
	if endpoint := env["STORAGE_S3_ENDPOINT"]; endpoint != "" && !strings.HasPrefix(endpoint, "https://") {
		return errors.New("production S3 endpoint is not HTTPS")
	}
	if _, err := os.Stat(credentials); err != nil {
		return errors.New("production AWS credentials file is not readable")
	}
	if err := os.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentials); err != nil {
		return err
	}
	if err := os.Setenv("AWS_PROFILE", env["AWS_PROFILE"]); err != nil {
		return err
	}
	if checkOnly {
		fmt.Printf("demo_s3_manifest_validated=%d\n", len(keys))
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	store, err := storage.NewS3(ctx, storage.S3Config{
		Bucket: env["STORAGE_S3_BUCKET"], Region: env["STORAGE_S3_REGION"],
		Endpoint: env["STORAGE_S3_ENDPOINT"], UsePathStyle: env["STORAGE_S3_USE_PATH_STYLE"] == "true",
		OperationTimeout: 30 * time.Second,
	})
	if err != nil {
		return errors.New("cannot initialize production S3 client")
	}
	for index, key := range keys {
		if err := store.Delete(ctx, key); err != nil {
			return fmt.Errorf("object %d of %d: deletion failed", index+1, len(keys))
		}
		object, err := store.Open(ctx, key)
		if err == nil {
			_ = object.Reader.Close()
			return fmt.Errorf("object %d of %d: still readable after deletion", index+1, len(keys))
		}
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("object %d of %d: absence could not be verified", index+1, len(keys))
		}
	}
	fmt.Printf("demo_s3_objects_deleted_and_unreadable=%d\n", len(keys))
	return nil
}

func readManifest(reader io.Reader) ([]string, error) {
	counts := map[string]int{}
	seen := map[string]bool{}
	keys := make([]string, 0, 23)
	scanner := bufio.NewScanner(io.LimitReader(reader, 32<<10))
	for scanner.Scan() {
		key := scanner.Text()
		if key == "" || strings.TrimSpace(key) != key || strings.ContainsRune(key, '\x00') ||
			strings.HasPrefix(key, "/") || path.Clean(key) != key || seen[key] {
			return nil, errors.New("object manifest contains an invalid or duplicate key")
		}
		prefix := strings.SplitN(key, "/", 2)[0]
		if prefix != "evidence" && prefix != "workflow-form-pdfs" && prefix != "workflow-signatures" {
			return nil, errors.New("object manifest contains an unexpected key family")
		}
		seen[key] = true
		counts[prefix]++
		keys = append(keys, key)
	}
	if err := scanner.Err(); err != nil {
		return nil, errors.New("cannot read object manifest")
	}
	if len(keys) != 23 || counts["evidence"] != 13 || counts["workflow-form-pdfs"] != 9 || counts["workflow-signatures"] != 1 {
		return nil, errors.New("object manifest does not match the restored V145 archive inventory")
	}
	return keys, nil
}
