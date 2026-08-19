package mailer

import "context"

// PreviewSender accepts messages without delivering them. It is intended only
// for development and test environments where the account-recovery UI needs to
// remain exercisable before a real mail provider is configured.
type PreviewSender struct{}

func NewPreview() *PreviewSender {
	return &PreviewSender{}
}

func (s *PreviewSender) Send(ctx context.Context, _ Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
