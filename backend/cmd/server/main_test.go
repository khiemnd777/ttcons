package main

import (
	"testing"

	"consflow/backend/internal/shared/config"
)

func TestServerConfigKeepsMultipartUploadsInBoundedMemory(t *testing.T) {
	cfg := serverConfig(config.Config{MaxUploadBytes: 10 * 1024 * 1024})
	if !cfg.DisablePreParseMultipartForm {
		t.Fatal("multipart pre-parsing can spill uploaded files into the process temporary directory")
	}
	if cfg.BodyLimit != 11*1024*1024 {
		t.Fatalf("body limit=%d, want upload limit plus bounded multipart overhead", cfg.BodyLimit)
	}
}
