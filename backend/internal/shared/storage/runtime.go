package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type RuntimeConfig struct {
	WriteProvider string
	LocalRoot     string
	S3            S3Config
}

type Runtime struct {
	Registry      *Registry
	WriteProvider string
	local         *Local
}

func NewRuntime(ctx context.Context, cfg RuntimeConfig) (*Runtime, error) {
	writeProvider := strings.ToLower(strings.TrimSpace(cfg.WriteProvider))
	if writeProvider == "" {
		writeProvider = LocalProvider
	}
	if writeProvider != LocalProvider && writeProvider != S3Provider {
		return nil, fmt.Errorf("unsupported storage write provider %q", writeProvider)
	}

	localStore, err := NewLocal(cfg.LocalRoot)
	if err != nil {
		return nil, fmt.Errorf("configure local storage: %w", err)
	}
	stores := []Store{localStore}
	s3Configured := strings.TrimSpace(cfg.S3.Bucket) != "" || strings.TrimSpace(cfg.S3.Region) != "" || strings.TrimSpace(cfg.S3.Endpoint) != "" || cfg.S3.UsePathStyle
	if writeProvider == S3Provider || s3Configured {
		s3Store, s3Err := NewS3(ctx, cfg.S3)
		if s3Err != nil {
			_ = localStore.Close()
			return nil, fmt.Errorf("configure S3 storage: %w", s3Err)
		}
		stores = append(stores, s3Store)
	}
	registry, err := NewRegistry(stores...)
	if err != nil {
		_ = localStore.Close()
		return nil, err
	}
	if _, err := registry.Store(writeProvider); err != nil {
		_ = localStore.Close()
		return nil, err
	}
	return &Runtime{Registry: registry, WriteProvider: writeProvider, local: localStore}, nil
}

func (r *Runtime) Close() error {
	if r == nil || r.local == nil {
		return nil
	}
	err := r.local.Close()
	r.local = nil
	return err
}

func (r *Runtime) Validate() error {
	if r == nil || r.Registry == nil {
		return errors.New("storage runtime is not configured")
	}
	_, err := r.Registry.Store(r.WriteProvider)
	return err
}
