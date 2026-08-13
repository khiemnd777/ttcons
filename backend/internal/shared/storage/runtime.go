package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type RuntimeConfig struct {
	WriteProvider         string
	LocalRoot             string
	EnableLegacyLocalRead bool
	S3                    S3Config
}

type Runtime struct {
	Registry      *Registry
	WriteProvider string
	local         *Local
}

func NewRuntime(ctx context.Context, cfg RuntimeConfig) (*Runtime, error) {
	writeProvider := strings.ToLower(strings.TrimSpace(cfg.WriteProvider))
	if writeProvider == "" || writeProvider == "default" {
		writeProvider = LocalProvider
	}
	if writeProvider != LocalProvider && writeProvider != S3Provider {
		return nil, fmt.Errorf("unsupported storage write provider %q", writeProvider)
	}

	var localStore *Local
	stores := make([]Store, 0, 2)
	if writeProvider == LocalProvider || cfg.EnableLegacyLocalRead {
		configuredLocal, err := NewLocal(cfg.LocalRoot)
		if err != nil {
			return nil, fmt.Errorf("configure local storage: %w", err)
		}
		localStore = configuredLocal
		stores = append(stores, localStore)
	}
	if writeProvider == S3Provider {
		s3Store, s3Err := NewS3(ctx, cfg.S3)
		if s3Err != nil {
			if localStore != nil {
				_ = localStore.Close()
			}
			return nil, fmt.Errorf("configure S3 storage: %w", s3Err)
		}
		stores = append(stores, s3Store)
	}
	registry, err := NewRegistry(stores...)
	if err != nil {
		if localStore != nil {
			_ = localStore.Close()
		}
		return nil, err
	}
	if _, err := registry.Store(writeProvider); err != nil {
		if localStore != nil {
			_ = localStore.Close()
		}
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
