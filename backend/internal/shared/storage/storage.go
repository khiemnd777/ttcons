package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

var (
	ErrInvalidKey      = errors.New("storage key is invalid")
	ErrNotFound        = errors.New("storage object not found")
	ErrObjectExists    = errors.New("storage object already exists")
	ErrUnknownProvider = errors.New("storage provider is not registered")
	ErrProviderExists  = errors.New("storage provider is already registered")
)

type PutOptions struct {
	ContentType  string
	ExpectedSize int64
}

type ObjectInfo struct {
	Provider       string
	Key            string
	Size           int64
	ContentType    string
	ChecksumSHA256 string
}

type Object struct {
	Reader io.ReadCloser
	Info   ObjectInfo
}

type Store interface {
	Provider() string
	Put(ctx context.Context, key string, source io.Reader, options PutOptions) (ObjectInfo, error)
	Open(ctx context.Context, key string) (Object, error)
	Delete(ctx context.Context, key string) error
}

type Registry struct {
	mu     sync.RWMutex
	stores map[string]Store
}

func NewRegistry(stores ...Store) (*Registry, error) {
	registry := &Registry{stores: make(map[string]Store, len(stores))}
	for _, store := range stores {
		if err := registry.Register(store); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

func (r *Registry) Register(store Store) error {
	if r == nil {
		return errors.New("storage registry is nil")
	}
	if store == nil {
		return errors.New("storage store is nil")
	}
	provider := strings.TrimSpace(store.Provider())
	if provider == "" {
		return errors.New("storage provider is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stores == nil {
		r.stores = make(map[string]Store)
	}
	if _, exists := r.stores[provider]; exists {
		return fmt.Errorf("%w: %s", ErrProviderExists, provider)
	}
	r.stores[provider] = store
	return nil
}

func (r *Registry) Store(provider string) (Store, error) {
	provider = strings.TrimSpace(provider)
	if r == nil || provider == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, provider)
	}
	r.mu.RLock()
	store, ok := r.stores[provider]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownProvider, provider)
	}
	return store, nil
}

func (r *Registry) Put(ctx context.Context, provider, key string, source io.Reader, options PutOptions) (ObjectInfo, error) {
	store, err := r.Store(provider)
	if err != nil {
		return ObjectInfo{}, err
	}
	return store.Put(ctx, key, source, options)
}

func (r *Registry) Open(ctx context.Context, provider, key string) (Object, error) {
	store, err := r.Store(provider)
	if err != nil {
		return Object{}, err
	}
	return store.Open(ctx, key)
}

func (r *Registry) Delete(ctx context.Context, provider, key string) error {
	store, err := r.Store(provider)
	if err != nil {
		return err
	}
	return store.Delete(ctx, key)
}
