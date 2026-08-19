package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Sender interface {
	Send(context.Context, Message) error
}

type SMTPConfig struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
	TLSMode     string
	Timeout     time.Duration
}

type SMTPSender struct {
	config SMTPConfig
}

func NewSMTP(config SMTPConfig) (*SMTPSender, error) {
	config.Host = strings.TrimSpace(config.Host)
	config.Username = strings.TrimSpace(config.Username)
	config.FromAddress = strings.ToLower(strings.TrimSpace(config.FromAddress))
	config.FromName = strings.TrimSpace(config.FromName)
	config.TLSMode = strings.ToLower(strings.TrimSpace(config.TLSMode))
	if config.Host == "" || config.Port < 1 || config.Port > 65535 {
		return nil, fmt.Errorf("SMTP host and port are required")
	}
	parsedFrom, err := mail.ParseAddress(config.FromAddress)
	if err != nil || parsedFrom.Address != config.FromAddress {
		return nil, fmt.Errorf("SMTP from address is invalid")
	}
	if config.Username == "" && config.Password != "" || config.Username != "" && config.Password == "" {
		return nil, fmt.Errorf("SMTP username and password must be configured together")
	}
	if config.TLSMode != "starttls" && config.TLSMode != "tls" && config.TLSMode != "none" {
		return nil, fmt.Errorf("SMTP TLS mode must be starttls, tls, or none")
	}
	if config.Timeout <= 0 {
		return nil, fmt.Errorf("SMTP timeout must be positive")
	}
	if strings.ContainsAny(config.FromName, "\r\n") {
		return nil, fmt.Errorf("SMTP from name must not contain line breaks")
	}
	return &SMTPSender{config: config}, nil
}

func (s *SMTPSender) Send(ctx context.Context, message Message) error {
	to := strings.ToLower(strings.TrimSpace(message.To))
	parsedTo, err := mail.ParseAddress(to)
	if err != nil || parsedTo.Address != to {
		return fmt.Errorf("recipient address is invalid")
	}
	if strings.TrimSpace(message.Subject) == "" || strings.TrimSpace(message.Text) == "" {
		return fmt.Errorf("mail subject and text are required")
	}
	if strings.ContainsAny(message.Subject, "\r\n") {
		return fmt.Errorf("mail subject must not contain line breaks")
	}

	deadline := time.Now().Add(s.config.Timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	dialer := net.Dialer{Timeout: s.config.Timeout}
	address := net.JoinHostPort(s.config.Host, strconv.Itoa(s.config.Port))
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer connection.Close()
	if err := connection.SetDeadline(deadline); err != nil {
		return fmt.Errorf("set SMTP deadline: %w", err)
	}

	tlsConfig := &tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}
	if s.config.TLSMode == "tls" {
		tlsConnection := tls.Client(connection, tlsConfig)
		if err := tlsConnection.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("negotiate SMTP TLS: %w", err)
		}
		connection = tlsConnection
	}
	client, err := smtp.NewClient(connection, s.config.Host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer client.Close()
	if s.config.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not advertise STARTTLS")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}
	if s.config.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(s.config.FromAddress); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP message: %w", err)
	}
	if err := writeMessage(writer, s.config, to, message); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("finish SMTP delivery: %w", err)
	}
	return nil
}

func writeMessage(writer io.Writer, config SMTPConfig, to string, message Message) error {
	buffered := bufio.NewWriter(writer)
	from := (&mail.Address{Name: config.FromName, Address: config.FromAddress}).String()
	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("utf-8", strings.TrimSpace(message.Subject)),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	if _, err := buffered.WriteString(strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.TrimSpace(message.Text) + "\r\n"); err != nil {
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := buffered.Flush(); err != nil {
		return fmt.Errorf("flush SMTP message: %w", err)
	}
	return nil
}
