package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/mailer"
)

type recoveryAvailabilitySender struct{}

func (recoveryAvailabilitySender) Send(context.Context, mailer.Message) error {
	return nil
}

func TestNewAccountRecoverySenderUsesPreviewForInvalidDevelopmentConfiguration(t *testing.T) {
	cfg := config.Config{
		AppEnv:                "development",
		AuthRecoveryEnabled:   true,
		AuthRecoveryTokenTTL:  15 * time.Minute,
		AuthRecoveryVerifyTTL: 24 * time.Hour,
		SMTPPort:              587,
		SMTPTLSMode:           "starttls",
		SMTPTimeout:           10 * time.Second,
	}
	sender, err := newAccountRecoverySender(cfg)
	if err != nil {
		t.Fatalf("development preview sender error=%v", err)
	}
	if _, ok := sender.(*mailer.PreviewSender); !ok {
		t.Fatalf("development sender=%T, want *mailer.PreviewSender", sender)
	}

	cfg.AppEnv = "test"
	sender, err = newAccountRecoverySender(cfg)
	if err != nil {
		t.Fatalf("test preview sender error=%v", err)
	}
	if _, ok := sender.(*mailer.PreviewSender); !ok {
		t.Fatalf("test sender=%T, want *mailer.PreviewSender", sender)
	}

	cfg.AuthRecoveryEnabled = false
	sender, err = newAccountRecoverySender(cfg)
	if err != nil || sender != nil {
		t.Fatalf("disabled recovery sender=%T error=%v, want nil/nil", sender, err)
	}
}

func TestNewAccountRecoverySenderRejectsInvalidStagingAndProductionConfiguration(t *testing.T) {
	for _, appEnv := range []string{"staging", "production"} {
		t.Run(appEnv, func(t *testing.T) {
			cfg := config.Config{
				AppEnv:                appEnv,
				AuthRecoveryEnabled:   true,
				AuthRecoveryTokenTTL:  15 * time.Minute,
				AuthRecoveryVerifyTTL: 24 * time.Hour,
				SMTPPort:              587,
				SMTPTLSMode:           "starttls",
				SMTPTimeout:           10 * time.Second,
			}
			sender, err := newAccountRecoverySender(cfg)
			if err == nil || !strings.Contains(err.Error(), "SMTP_HOST") {
				t.Fatalf("invalid recovery sender error=%v, want SMTP_HOST", err)
			}
			if sender != nil {
				t.Fatalf("invalid %s configuration returned sender=%T", appEnv, sender)
			}
		})
	}
}

func TestPreviewAccountRecoveryIsAvailable(t *testing.T) {
	service := &Service{}
	service.ConfigureAccountRecovery(true, mailer.NewPreview(), "http://localhost:5173", 15*time.Minute, 24*time.Hour)
	profile := Record{"recovery_email_private": "recovery@example.com"}
	service.shapeOwnProfile(profile)
	if available, _ := profile["recovery_email_available"].(bool); !available {
		t.Fatal("preview sender must keep account recovery available")
	}
}

func TestNewAccountRecoverySenderAcceptsValidConfiguration(t *testing.T) {
	cfg := config.Config{
		AppEnv:                "development",
		AuthRecoveryEnabled:   true,
		AuthRecoveryTokenTTL:  15 * time.Minute,
		AuthRecoveryVerifyTTL: 24 * time.Hour,
		SMTPHost:              "smtp.resend.com",
		SMTPPort:              465,
		SMTPUsername:          "resend",
		SMTPPassword:          "test-api-key",
		SMTPFromAddress:       "no-reply@example.com",
		SMTPFromName:          "Consflow",
		SMTPTLSMode:           "tls",
		SMTPTimeout:           10 * time.Second,
	}
	sender, err := newAccountRecoverySender(cfg)
	if err != nil {
		t.Fatalf("valid recovery configuration rejected: %v", err)
	}
	if sender == nil {
		t.Fatal("valid recovery configuration did not create a sender")
	}
}

func TestShapeOwnProfileUsesOperationalRecoveryAvailability(t *testing.T) {
	service := &Service{
		recoveryEnabled: true,
		frontendOrigin:  "http://localhost:5173",
	}
	profile := Record{"recovery_email_private": "recovery@example.com"}
	service.shapeOwnProfile(profile)
	if available, _ := profile["recovery_email_available"].(bool); available {
		t.Fatal("recovery without a sender must be unavailable")
	}

	service.mailSender = recoveryAvailabilitySender{}
	profile = Record{"recovery_email_private": "recovery@example.com"}
	service.shapeOwnProfile(profile)
	if available, _ := profile["recovery_email_available"].(bool); !available {
		t.Fatal("configured sender and frontend origin must make recovery available")
	}

	service.frontendOrigin = ""
	profile = Record{"recovery_email_private": "recovery@example.com"}
	service.shapeOwnProfile(profile)
	if available, _ := profile["recovery_email_available"].(bool); available {
		t.Fatal("recovery without a frontend origin must be unavailable")
	}
}

func TestMaskEmail(t *testing.T) {
	tests := map[string]string{
		"abcdef@gmail.com": "abc***@g**.com",
		"ab@io.vn":         "ab***@i**.vn",
		"x@localhost":      "x***@l**",
		"invalid":          "***",
	}
	for input, want := range tests {
		if got := maskEmail(input); got != want {
			t.Errorf("maskEmail(%q)=%q, want %q", input, got, want)
		}
	}
}
