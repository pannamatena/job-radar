package notify

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
)

// Email sends messages over SMTP from the job-radar account (ADR-0019). The app
// password is held in memory only and is never logged or written into an error.
type Email struct {
	Host     string
	Port     int
	From     string // the job-radar address
	Password string // the app password
	To       []string

	// deliver is replaced in tests; nil means a real SMTP connection.
	deliver func(ctx context.Context, from string, to []string, msg []byte) error
	now     func() time.Time
}

// Send builds the plain-text and HTML versions and delivers them to every
// recipient in one message.
func (e *Email) Send(ctx context.Context, m Message) (Sent, error) {
	if len(e.To) == 0 {
		return Sent{}, fmt.Errorf("email: no recipients configured (email.to)")
	}
	now := time.Now
	if e.now != nil {
		now = e.now
	}
	id, err := messageID(e.From)
	if err != nil {
		return Sent{}, err
	}
	raw, err := BuildMIME(e.From, e.To, m.Subject, RenderText(m), RenderHTML(m), id, now())
	if err != nil {
		return Sent{}, err
	}
	send := e.deliver
	if send == nil {
		send = e.smtpDeliver
	}
	if err := send(ctx, e.From, e.To, raw); err != nil {
		return Sent{}, err
	}
	return Sent{MessageID: id, Numbered: Numbered(m)}, nil
}

// BuildMIME assembles a multipart/alternative email: plain text first, HTML
// second, both quoted-printable UTF-8. The From is also the Reply-To, so
// replies come back to the job-radar account where they can be read.
func BuildMIME(from string, to []string, subject, text, htmlBody, msgID string, date time.Time) ([]byte, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, part := range []struct{ ctype, content string }{
		{"text/plain; charset=utf-8", text},
		{"text/html; charset=utf-8", htmlBody},
	} {
		h := textproto.MIMEHeader{}
		h.Set("Content-Type", part.ctype)
		h.Set("Content-Transfer-Encoding", "quoted-printable")
		w, err := mw.CreatePart(h)
		if err != nil {
			return nil, err
		}
		qp := quotedprintable.NewWriter(w)
		if _, err := qp.Write([]byte(part.content)); err != nil {
			return nil, err
		}
		if err := qp.Close(); err != nil {
			return nil, err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	hdr := func(k, v string) { out.WriteString(k + ": " + v + "\n") }
	hdr("From", from)
	hdr("To", strings.Join(to, ", "))
	hdr("Reply-To", from)
	hdr("Subject", mime.QEncoding.Encode("utf-8", strings.NewReplacer("\r", " ", "\n", " ").Replace(subject)))
	hdr("Date", date.Format(time.RFC1123Z))
	hdr("Message-ID", msgID)
	hdr("MIME-Version", "1.0")
	hdr("Content-Type", `multipart/alternative; boundary="`+mw.Boundary()+`"`)
	out.WriteString("\n")
	out.Write(bytes.ReplaceAll(body.Bytes(), []byte("\r\n"), []byte("\n")))
	return bytes.ReplaceAll(out.Bytes(), []byte("\n"), []byte("\r\n")), nil
}

func messageID(from string) (string, error) {
	domain := "job-radar.local"
	if a, err := mail.ParseAddress(from); err == nil {
		if _, d, ok := strings.Cut(a.Address, "@"); ok && d != "" {
			domain = d
		}
	}
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("email: message id: %w", err)
	}
	return "<" + hex.EncodeToString(b) + "@" + domain + ">", nil
}

// smtpDeliver connects with STARTTLS (port 587), logs in with the app password
// and sends. Port 465 (implicit TLS) is also supported.
func (e *Email) smtpDeliver(ctx context.Context, from string, to []string, msg []byte) error {
	addr := net.JoinHostPort(e.Host, strconv.Itoa(e.Port))
	d := net.Dialer{Timeout: 20 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("email: can't reach %s: %w", addr, err)
	}
	if e.Port == 465 {
		conn = tls.Client(conn, &tls.Config{ServerName: e.Host})
	}
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	c, err := smtp.NewClient(conn, e.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("email: SMTP greeting from %s failed: %w", addr, err)
	}
	defer c.Close()
	if e.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return fmt.Errorf("email: %s doesn't offer STARTTLS; refusing to send a password unencrypted", addr)
		}
		if err := c.StartTLS(&tls.Config{ServerName: e.Host}); err != nil {
			return fmt.Errorf("email: starting TLS with %s failed: %w", addr, err)
		}
	}
	if err := c.Auth(smtp.PlainAuth("", e.From, e.Password, e.Host)); err != nil {
		return fmt.Errorf("email: %s rejected the login for %s (check RADAR_EMAIL_APP_PASSWORD is an app password, with no spaces): %w", e.Host, e.From, err)
	}
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("email: sender rejected: %w", err)
	}
	for _, r := range to {
		if err := c.Rcpt(r); err != nil {
			return fmt.Errorf("email: recipient %s rejected: %w", r, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("email: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("email: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: sending failed: %w", err)
	}
	return c.Quit()
}
