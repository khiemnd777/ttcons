package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"consflow/backend/internal/shared/config"
	"consflow/backend/internal/shared/mailer"
	sharedstorage "consflow/backend/internal/shared/storage"
	"github.com/gofiber/fiber/v2"
)

type bootstrapTestSender struct {
	mu      sync.Mutex
	message mailer.Message
	fail    bool
}

func (s *bootstrapTestSender) Send(_ context.Context, m mailer.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("smtp unavailable")
	}
	s.message = m
	return nil
}

func TestDeveloperBootstrapSMTPRequiresRealTLSAndSender(t *testing.T) {
	cfg := config.Config{AppEnv: "production", SMTPHost: "smtp.example.test", SMTPPort: 587, SMTPUsername: "account", SMTPPassword: "test-only", SMTPFromAddress: "khiem@knasoftware.com", SMTPTLSMode: "starttls", SMTPTimeout: time.Second, AuthRecoveryTokenTTL: time.Minute, AuthRecoveryVerifyTTL: time.Minute}
	if sender, err := newDeveloperBootstrapSender(cfg); err != nil || sender == nil {
		t.Fatalf("valid SMTP configuration: %v", err)
	}
	cfg.SMTPTLSMode = "none"
	if sender, err := newDeveloperBootstrapSender(cfg); err == nil || sender != nil {
		t.Fatal("insecure SMTP accepted")
	}
	cfg.SMTPTLSMode = "tls"
	cfg.SMTPFromAddress = "other@example.test"
	if sender, err := newDeveloperBootstrapSender(cfg); err == nil || sender != nil {
		t.Fatal("unexpected sender accepted")
	}
}

func TestDeveloperBootstrapMailConfigPreservesDatabaseAuthority(t *testing.T) {
	startup := config.Config{SMTPHost: "smtp.example.test", SMTPUsername: "environment-user", SMTPPassword: "test-only", SMTPFromAddress: "khiem@knasoftware.com"}
	store := NewSystemSettingsStore(nil, startup)
	tests := []struct {
		name         string
		runtime      config.Config
		wantHost     string
		wantUsername string
	}{
		{name: "empty database uses environment", runtime: config.Config{}, wantHost: "smtp.example.test", wantUsername: "environment-user"},
		{name: "configured database wins", runtime: config.Config{SMTPHost: "database.example.test", SMTPUsername: "database-user", SMTPFromAddress: "khiem@knasoftware.com"}, wantHost: "database.example.test", wantUsername: "database-user"},
		{name: "partial database must not fall back", runtime: config.Config{SMTPUsername: "database-user"}, wantHost: "", wantUsername: "database-user"},
		{name: "database password alone must not fall back", runtime: config.Config{SMTPPassword: "test-only-database"}, wantHost: "", wantUsername: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			selected := developerBootstrapMailConfig(tc.runtime, store)
			if selected.SMTPHost != tc.wantHost || selected.SMTPUsername != tc.wantUsername {
				t.Fatal("incorrect SMTP configuration selected")
			}
		})
	}
}

func TestDeveloperBootstrapStartupWithPreviouslyEmptySettings(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "bootstrap_startup_", []string{"system_settings", "users", "roles", "user_roles", "developer_bootstrap_otp"})
	ctx := context.Background()
	empty := config.Load()
	empty.StoragePath = t.TempDir()
	empty.SettingsEncryptionKey = "test-only-settings-encryption-key-123456"
	empty.RateLimitEnabled = false // Isolated transport fixture does not start Redis.
	empty.SMTPHost, empty.SMTPUsername, empty.SMTPPassword, empty.SMTPFromAddress = "", "", "", ""
	empty.AuthRecoveryEnabled = false
	if _, err := NewSystemSettingsStore(db, empty).Bootstrap(ctx, empty); err != nil {
		t.Fatalf("initialize empty settings: %v", err)
	}

	startup := empty
	startup.SMTPHost = "127.0.0.1"
	startup.SMTPPort = 1 // No SMTP server: the resulting delivery failure proves the sender was initialized.
	startup.SMTPUsername = "test-only"
	startup.SMTPPassword = "test-only"
	startup.SMTPFromAddress = "khiem@knasoftware.com"
	startup.SMTPTLSMode = "tls"
	store := NewSystemSettingsStore(db, startup)
	runtime, err := store.Bootstrap(ctx, startup)
	if err != nil {
		t.Fatalf("load persisted settings: %v", err)
	}
	if runtime.SMTPHost != "" {
		t.Fatal("fixture did not retain empty persisted SMTP settings")
	}
	local, err := sharedstorage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	registry, err := sharedstorage.NewRegistry(local)
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	closeRoutes, err := RegisterRoutes(app, db, nil, nil, runtime, registry, nil, store)
	if err != nil {
		t.Fatal(err)
	}
	defer closeRoutes()
	res, err := app.Test(httptest.NewRequest("POST", "/api/auth/bootstrap/otp", nil), 10000)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 503 || !strings.Contains(string(body), "Không gửi được mã xác minh") {
		t.Fatalf("startup SMTP sender was not used, status=%d response=%s", res.StatusCode, body)
	}
}

func TestDeveloperBootstrapFlowPostgres(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "bootstrap_test_", []string{"users", "roles", "user_roles", "developer_bootstrap_otp"})
	ctx := context.Background()
	sender := &bootstrapTestSender{}
	svc := NewService(NewRepository(db), "test-jwt")
	svc.bootstrapKey = bytes.Repeat([]byte{42}, 32)
	svc.bootstrapSender = sender
	h := NewHandler(svc)
	app := fiber.New()
	app.Get("/api/bootstrap/status", h.DeveloperBootstrapStatus)
	app.Post("/api/auth/bootstrap/otp", h.SendDeveloperBootstrapOTP)
	app.Post("/api/auth/bootstrap/complete", h.CompleteDeveloperBootstrap)
	app.Use("/api", bootstrapGate(db))
	app.Get("/api/protected", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	call := func(method, path string, body any) (int, string) {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, 10000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(raw)
	}
	if status, _ := call("GET", "/api/protected", nil); status != 423 {
		t.Fatalf("locked status %d", status)
	}
	sender.fail = true
	if status, _ := call("POST", "/api/auth/bootstrap/otp", nil); status != 503 {
		t.Fatalf("mail failure status %d", status)
	}
	var failedCode []byte
	if err := db.QueryRowContext(ctx, `SELECT code_mac FROM developer_bootstrap_otp`).Scan(&failedCode); err != nil || len(failedCode) != 0 {
		t.Fatal("mail failure left an active code")
	}
	sender.fail = false
	if _, err := db.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET sent_at=now()-interval '2 minutes'`); err != nil {
		t.Fatal(err)
	}
	if status, _ := call("POST", "/api/auth/bootstrap/otp", nil); status != 200 {
		t.Fatalf("send status %d", status)
	}
	if sender.message.To != developerBootstrapEmail {
		t.Fatal("incorrect recipient")
	}
	code := regexp.MustCompile(`\b[0-9]{8}\b`).FindString(sender.message.Text)
	if len(code) != 8 {
		t.Fatal("missing OTP in test sender")
	}
	if status, _ := call("POST", "/api/auth/bootstrap/otp", nil); status != 429 {
		t.Fatalf("cooldown status %d", status)
	}
	request := map[string]string{"otp": "notvalid", "full_name": "Test Developer", "password": "password-test-123", "password_confirmation": "password-test-123"}
	if status, _ := call("POST", "/api/auth/bootstrap/complete", request); status != 400 {
		t.Fatalf("invalid input status %d", status)
	}
	request["otp"] = "99999999"
	if code == request["otp"] {
		request["otp"] = "00000000"
	}
	for i := 0; i < 5; i++ {
		if status, _ := call("POST", "/api/auth/bootstrap/complete", request); status != 400 {
			t.Fatalf("invalid OTP status %d", status)
		}
	}
	request["otp"] = code
	if status, _ := call("POST", "/api/auth/bootstrap/complete", request); status != 400 {
		t.Fatalf("attempt limit status %d", status)
	}
	if _, err := db.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET attempts=0, expires_at=now()-interval '1 second'`); err != nil {
		t.Fatal(err)
	}
	if status, _ := call("POST", "/api/auth/bootstrap/complete", request); status != 400 {
		t.Fatalf("expired OTP status %d", status)
	}
	if _, err := db.ExecContext(ctx, `UPDATE developer_bootstrap_otp SET sent_at=now()-interval '2 minutes'`); err != nil {
		t.Fatal(err)
	}
	if status, _ := call("POST", "/api/auth/bootstrap/otp", nil); status != 200 {
		t.Fatalf("resend status %d", status)
	}
	request["otp"] = regexp.MustCompile(`\b[0-9]{8}\b`).FindString(sender.message.Text)
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		go func() { status, _ := call("POST", "/api/auth/bootstrap/complete", request); results <- status }()
	}
	a, b := <-results, <-results
	if !((a == 200 && b == 409) || (a == 409 && b == 200)) {
		t.Fatalf("concurrent completion statuses %d and %d", a, b)
	}
	if status, _ := call("GET", "/api/protected", nil); status != 200 {
		t.Fatalf("unlocked status %d", status)
	}
	if status, _ := call("POST", "/api/auth/bootstrap/complete", request); status != 409 {
		t.Fatalf("replay status %d", status)
	}
	streamApp := fiber.New()
	streamApp.Get("/stream", func(c *fiber.Ctx) error {
		h.writeActivityStream(c, func() {}, func(event ActivityStreamEvent) (string, ActivityStreamEvent, bool) { return "event", event, true })
		return nil
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start local test stream: %v", err)
	}
	go func() { _ = streamApp.Listener(listener) }()
	defer streamApp.Shutdown()
	client := &http.Client{Timeout: 8 * time.Second}
	stream, err := client.Get("http://" + listener.Addr().String() + "/stream")
	if err != nil {
		t.Fatalf("open test stream: %v", err)
	}
	defer stream.Body.Close()
	reader := bufio.NewReader(stream.Body)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("read stream start: %v", readErr)
		}
		if line == "\n" {
			break
		}
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET status='inactive' WHERE email=$1`, developerBootstrapEmail); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		t.Fatalf("stream should close when Developer disappears: %v", err)
	}
	if status, _ := call("GET", "/api/protected", nil); status != 423 {
		t.Fatalf("relocked status %d", status)
	}
	if status, body := call("POST", "/api/auth/bootstrap/otp", nil); status != 409 || !strings.Contains(body, "BOOTSTRAP_EMAIL_CONFLICT") {
		t.Fatalf("conflict status %d", status)
	}
}
