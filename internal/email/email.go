// Package email sends aio's account email: verification links, temporary
// passwords and security notices (docs/rfc/RFC-003-password-reset-by-email.md).
package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/all-in-one/internal/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Message is one email, already rendered.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Sender delivers a rendered message.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Mailer renders a kind of email in the user's language and sends it.
type Mailer struct {
	sender Sender
	// host is aio's address as users see it (e.g. aio.example.com), named in
	// every email so users know why it comes from All-in-one.
	host string
	sent metric.Int64Counter
}

func NewMailer(sender Sender, host string) *Mailer {
	sent, _ := observability.Meter("email").Int64Counter("aio.email.sent",
		metric.WithDescription("Account emails sent, by kind and result"))
	return &Mailer{sender: sender, host: host, sent: sent}
}

// Send renders kind for locale ("en" or "id"; anything else is English) and
// sends it to the address in to.
func (m *Mailer) Send(ctx context.Context, kind Kind, to, locale string, data Data) error {
	data.Host = m.host
	msg, err := render(kind, locale, data)
	if err != nil {
		m.record(ctx, kind, "error")
		return err
	}
	msg.To = to
	ctx, span := otel.Tracer("github.com/all-in-one/internal/email").Start(ctx, "email.send")
	span.SetAttributes(attribute.String("email.kind", string(kind)))
	defer span.End()
	if err := m.sender.Send(ctx, msg); err != nil {
		span.RecordError(err)
		m.record(ctx, kind, "error")
		return fmt.Errorf("send %s email: %w", kind, err)
	}
	m.record(ctx, kind, "sent")
	return nil
}

func (m *Mailer) record(ctx context.Context, kind Kind, result string) {
	m.sent.Add(ctx, 1, metric.WithAttributes(attribute.String("kind", string(kind)), attribute.String("result", result)))
}

// MaskAddress hides most of an address for notices sent to someone else's
// mailbox: "nadia@example.com" → "n•••@example.com".
func MaskAddress(addr string) string {
	at := strings.LastIndex(addr, "@")
	if at < 1 {
		return "•••"
	}
	return addr[:1] + "•••" + addr[at:]
}
