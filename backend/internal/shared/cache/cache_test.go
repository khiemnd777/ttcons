package cache

import (
	"context"
	"testing"
)

func TestConnectRejectsMalformedRedisURLWithoutLocalhostFallback(t *testing.T) {
	client, err := Connect(context.Background(), "not-a-url")
	if err == nil {
		if client != nil {
			_ = client.Close()
		}
		t.Fatal("malformed Redis URL was accepted")
	}
	if client != nil {
		_ = client.Close()
		t.Fatal("malformed Redis URL returned a fallback client")
	}
}
