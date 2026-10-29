package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/all-in-one/internal/config"
)

const smtpTimeout = 20 * time.Second

// SMTPSender sends through an SMTP server with STARTTLS (port 587) or
// implicit TLS (port 465). It never sends over a plaintext connection.
type SMTPSender struct {
	cfg  config.SMTPConfig
	from *mail.Address
}

func NewSMTPSender(cfg config.EmailConfig) (*SMTPSender, error) {
	from, err := mail.ParseAddress(cfg.From)
	if err != nil {
		return nil, fmt.Errorf("email.from: %w", err)
	}
	return &SMTPSender{cfg: cfg.SMTP, from: from}, nil
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	body, err := buildMIME(s.from, to, msg)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	tlsConfig := &tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(ctx, smtpTimeout)
	defer cancel()

	var conn net.Conn
	if s.cfg.Port == 465 {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if s.cfg.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("%s does not offer STARTTLS; refusing to send in plaintext", addr)
		}
		if err := c.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if s.cfg.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(s.from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// buildMIME writes a multipart/alternative message (plain text and HTML).
func buildMIME(from, to *mail.Address, msg Message) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	id := make([]byte, 12)
	_, _ = rand.Read(id)
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]
	subject := strings.NewReplacer("\r", " ", "\n", " ").Replace(msg.Subject)

	headers := []string{
		"From: " + from.String(),
		"To: " + to.String(),
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"Date: " + time.Now().Format(time.RFC1123Z),
		"Message-ID: <" + hex.EncodeToString(id) + "@" + domain + ">",
		"MIME-Version: 1.0",
		"Content-Type: multipart/alternative; boundary=" + mw.Boundary(),
		"Auto-Submitted: auto-generated",
	}
	head := strings.Join(headers, "\r\n") + "\r\n\r\n"

	for _, part := range []struct{ ctype, body string }{
		{"text/plain; charset=utf-8", msg.Text},
		{"text/html; charset=utf-8", msg.HTML},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(pw)
		if _, err := qp.Write([]byte(part.body)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}
	return append([]byte(head), buf.Bytes()...), nil
}
