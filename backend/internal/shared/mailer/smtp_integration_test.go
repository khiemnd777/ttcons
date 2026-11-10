package mailer

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// This test uses a disposable local SMTP server and a private test CA. It does
// not contact an external provider or accept real credentials.
func TestSMTPDeliverWithVerifiedTLS(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "local SMTP test"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(parsed)
	// Go on macOS uses the platform trust store instead of SSL_CERT_FILE.
	// Force the documented test-only fallback store, preserving full verification.
	t.Setenv("GODEBUG", "x509usefallbackroots=1")
	x509.SetFallbackRoots(roots)
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	result := make(chan error, 1)
	go func() {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			result <- acceptErr
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		reader := bufio.NewReader(conn)
		write := func(s string) error { _, writeErr := fmt.Fprint(conn, s); return writeErr }
		if err := write("220 local test SMTP\r\n"); err != nil {
			result <- err
			return
		}
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				result <- readErr
				return
			}
			upper := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(upper, "EHLO "):
				if err := write("250-local.test\r\n250 AUTH PLAIN\r\n"); err != nil {
					result <- err
					return
				}
			case strings.HasPrefix(upper, "AUTH PLAIN "):
				parts := strings.SplitN(strings.TrimSpace(line), " ", 3)
				credential, decodeErr := base64.StdEncoding.DecodeString(parts[2])
				if decodeErr != nil || string(credential) != "\x00test-only\x00test-only" {
					result <- fmt.Errorf("unexpected test SMTP credentials")
					return
				}
				if err := write("235 authenticated\r\n"); err != nil {
					result <- err
					return
				}
			case strings.HasPrefix(upper, "MAIL FROM:") || strings.HasPrefix(upper, "RCPT TO:"):
				if strings.HasPrefix(upper, "MAIL FROM:") && !strings.Contains(upper, "SENDER@EXAMPLE.TEST") || strings.HasPrefix(upper, "RCPT TO:") && !strings.Contains(upper, "RECIPIENT@EXAMPLE.TEST") {
					result <- fmt.Errorf("unexpected SMTP envelope")
					return
				}
				if err := write("250 accepted\r\n"); err != nil {
					result <- err
					return
				}
			case upper == "DATA":
				if err := write("354 continue\r\n"); err != nil {
					result <- err
					return
				}
				data, readErr := textproto.NewReader(reader).ReadDotBytes()
				if readErr != nil {
					result <- readErr
					return
				}
				text := string(data)
				if !strings.Contains(text, "To: recipient@example.test") || !strings.Contains(text, "synthetic message") {
					result <- fmt.Errorf("unexpected SMTP message")
					return
				}
				if err := write("250 queued\r\n"); err != nil {
					result <- err
					return
				}
			case upper == "QUIT":
				result <- write("221 closing\r\n")
				return
			default:
				result <- fmt.Errorf("unexpected SMTP command")
				return
			}
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	sender, err := NewSMTP(SMTPConfig{Host: "127.0.0.1", Port: port, Username: "test-only", Password: "test-only", FromAddress: "sender@example.test", FromName: "Test", TLSMode: "tls", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(context.Background(), Message{To: "recipient@example.test", Subject: "Synthetic", Text: "synthetic message"}); err != nil {
		t.Fatalf("verified TLS SMTP delivery: %v", err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}
