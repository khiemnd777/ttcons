package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
)

type concurrentStore struct{ provider string }

func (s *concurrentStore) Provider() string { return s.provider }
func (s *concurrentStore) Put(context.Context, string, io.Reader, PutOptions) (ObjectInfo, error) {
	return ObjectInfo{}, nil
}
func (s *concurrentStore) Open(context.Context, string) (Object, error) { return Object{}, nil }
func (s *concurrentStore) Delete(context.Context, string) error         { return nil }

func TestRegistryConcurrentRegisterAndStore(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatal(err)
	}
	const count = 64
	var wait sync.WaitGroup
	for index := 0; index < count; index++ {
		provider := fmt.Sprintf("provider-%d", index)
		wait.Add(2)
		go func() {
			defer wait.Done()
			if err := registry.Register(&concurrentStore{provider: provider}); err != nil {
				t.Errorf("Register(%s): %v", provider, err)
			}
		}()
		go func() {
			defer wait.Done()
			for attempt := 0; attempt < 100; attempt++ {
				_, _ = registry.Store(provider)
			}
		}()
	}
	wait.Wait()
	for index := 0; index < count; index++ {
		provider := fmt.Sprintf("provider-%d", index)
		if _, err := registry.Store(provider); err != nil {
			t.Fatalf("Store(%s): %v", provider, err)
		}
	}
}
