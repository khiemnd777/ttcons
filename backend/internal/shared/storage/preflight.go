package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	preflightPayloadBytes   = 32
	preflightKeyAttempts    = 3
	preflightCleanupTimeout = 5 * time.Second
)

func VerifyWriteAccess(ctx context.Context, store Store) error {
	if store == nil {
		return errors.New("storage preflight store is nil")
	}
	var key string
	var payload []byte
	created := false
	for attempt := 0; attempt < preflightKeyAttempts; attempt++ {
		random := make([]byte, preflightPayloadBytes)
		if _, err := rand.Read(random); err != nil {
			return fmt.Errorf("generate storage preflight data: %w", err)
		}
		key = "_consflow/probes/" + hex.EncodeToString(random[:16])
		payload = random[16:]
		if _, err := store.Put(ctx, key, bytes.NewReader(payload), PutOptions{
			ContentType:  "application/octet-stream",
			ExpectedSize: int64(len(payload)),
		}); err != nil {
			if errors.Is(err, ErrObjectExists) {
				continue
			}
			return fmt.Errorf("storage preflight put: %w", err)
		}
		created = true
		break
	}
	if !created {
		return fmt.Errorf("storage preflight put: %w after %d random keys", ErrObjectExists, preflightKeyAttempts)
	}

	object, err := store.Open(ctx, key)
	if err != nil {
		return cleanupPreflightObject(ctx, store, key, fmt.Errorf("storage preflight open: %w", err))
	}
	read, readErr := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, reader: object.Reader}, int64(len(payload))+1))
	closeErr := object.Reader.Close()
	if readErr != nil {
		return cleanupPreflightObject(ctx, store, key, fmt.Errorf("storage preflight read: %w", readErr))
	}
	if closeErr != nil {
		return cleanupPreflightObject(ctx, store, key, fmt.Errorf("storage preflight close: %w", closeErr))
	}
	if !bytes.Equal(read, payload) {
		return cleanupPreflightObject(ctx, store, key, errors.New("storage preflight read integrity mismatch"))
	}
	if err := deletePreflightObject(ctx, store, key); err != nil {
		return fmt.Errorf("storage preflight delete: %w", err)
	}
	return nil
}

func cleanupPreflightObject(ctx context.Context, store Store, key string, primary error) error {
	if cleanupErr := deletePreflightObject(ctx, store, key); cleanupErr != nil {
		return errors.Join(primary, fmt.Errorf("storage preflight cleanup: %w", cleanupErr))
	}
	return primary
}

func deletePreflightObject(ctx context.Context, store Store, key string) error {
	cleanupContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), preflightCleanupTimeout)
	defer cancel()
	return store.Delete(cleanupContext, key)
}
