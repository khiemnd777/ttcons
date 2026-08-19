package mailer

import (
	"context"
	"errors"
	"testing"
)

func TestPreviewSenderAcceptsMessageWithoutDelivery(t *testing.T) {
	sender := NewPreview()
	if err := sender.Send(context.Background(), Message{To: "recovery@example.com", Subject: "Subject", Text: "Body"}); err != nil {
		t.Fatalf("preview send error=%v", err)
	}
}

func TestPreviewSenderRespectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := NewPreview().Send(ctx, Message{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("preview cancelled error=%v, want context.Canceled", err)
	}
}
