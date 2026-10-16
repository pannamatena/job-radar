// Package mailbox reads replies to job-radar's emails from the job-radar
// account over IMAP (ADR-0019, ADR-0035). It only ever reads: it never deletes,
// moves or marks messages. Everything it returns is untrusted text.
package mailbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Reply is one received message, reduced to what the feedback reader needs.
type Reply struct {
	MessageID string
	// Refs are the Message-IDs this message answers (In-Reply-To and References).
	Refs []string
	From string // the sender's address, lower-cased
	Body string // the plain-text body
	Date time.Time
}

// Reader fetches replies. The IMAP implementation is below; tests use a fake.
type Reader interface {
	// Replies returns messages received since the given time from any of the
	// given sender addresses.
	Replies(ctx context.Context, since time.Time, from []string) ([]Reply, error)
}

// IMAP reads from an IMAP server over TLS.
type IMAP struct {
	Host     string
	Port     int
	User     string
	Password string
}

// maxMessageBytes stops a huge message being read into memory.
const maxMessageBytes = 1 << 20

// Replies connects, looks in the inbox for mail from each sender since the date,
// and parses what it finds. It uses only read access (the inbox is opened
// read-only).
func (m *IMAP) Replies(ctx context.Context, since time.Time, from []string) ([]Reply, error) {
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	c, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, fmt.Errorf("mailbox: can't reach %s: %w", addr, err)
	}
	defer c.Close()
	if err := c.Login(m.User, strings.ReplaceAll(m.Password, " ", "")).Wait(); err != nil {
		return nil, fmt.Errorf("mailbox: %s rejected the login for %s (is IMAP switched on for the account, and is the password an app password?): %w", m.Host, m.User, err)
	}
	// The closure matters: `defer c.Logout().Wait()` would send LOGOUT right now
	// and only defer the wait, closing the connection before the inbox is opened.
	defer func() { _ = c.Logout().Wait() }()
	if _, err := c.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return nil, fmt.Errorf("mailbox: opening the inbox: %w", err)
	}

	var out []Reply
	seen := map[string]bool{}
	for _, sender := range from {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		crit := &imap.SearchCriteria{
			Since:  since,
			Header: []imap.SearchCriteriaHeaderField{{Key: "From", Value: sender}},
		}
		res, err := c.Search(crit, nil).Wait()
		if err != nil {
			return nil, fmt.Errorf("mailbox: searching: %w", err)
		}
		seqs := res.AllSeqNums()
		if len(seqs) == 0 {
			continue
		}
		body := &imap.FetchItemBodySection{Peek: true}
		msgs, err := c.Fetch(imap.SeqSetNum(seqs...), &imap.FetchOptions{BodySection: []*imap.FetchItemBodySection{body}}).Collect()
		if err != nil {
			return nil, fmt.Errorf("mailbox: fetching: %w", err)
		}
		for _, msg := range msgs {
			raw := msg.FindBodySection(body)
			if len(raw) == 0 || len(raw) > maxMessageBytes {
				continue
			}
			r, err := ParseReply(raw)
			if err != nil || seen[r.MessageID] {
				continue
			}
			seen[r.MessageID] = true
			out = append(out, r)
		}
	}
	return out, nil
}

// ParseReply reads a raw email into a Reply.
func ParseReply(raw []byte) (Reply, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return Reply{}, fmt.Errorf("mailbox: not an email: %w", err)
	}
	var r Reply
	r.MessageID = strings.TrimSpace(msg.Header.Get("Message-Id"))
	if a, err := mail.ParseAddress(msg.Header.Get("From")); err == nil {
		r.From = strings.ToLower(a.Address)
	}
	r.Date, _ = msg.Header.Date()
	r.Refs = append(messageIDs(msg.Header.Get("In-Reply-To")), messageIDs(msg.Header.Get("References"))...)
	r.Body = plainText(msg.Header.Get("Content-Type"), msg.Header.Get("Content-Transfer-Encoding"), msg.Body, 0)
	return r, nil
}

// messageIDs pulls the <id> tokens out of a header value.
func messageIDs(v string) []string {
	var out []string
	for _, f := range strings.Fields(v) {
		if strings.HasPrefix(f, "<") && strings.HasSuffix(f, ">") {
			out = append(out, f)
		}
	}
	return out
}

// plainText finds the text/plain part of a (possibly multipart) body.
func plainText(contentType, encoding string, body io.Reader, depth int) string {
	mt, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mt = "text/plain"
	}
	if strings.HasPrefix(mt, "multipart/") && depth < 4 {
		mr := multipart.NewReader(body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err != nil {
				return ""
			}
			if s := plainText(p.Header.Get("Content-Type"), p.Header.Get("Content-Transfer-Encoding"), p, depth+1); s != "" {
				return s
			}
		}
	}
	if mt != "text/plain" {
		return ""
	}
	var rd io.Reader = io.LimitReader(body, maxMessageBytes)
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "quoted-printable":
		rd = quotedprintable.NewReader(rd)
	case "base64":
		rd = base64.NewDecoder(base64.StdEncoding, newlineStripper{rd})
	}
	b, _ := io.ReadAll(rd)
	return string(b)
}

// newlineStripper removes line breaks so the base64 decoder accepts wrapped text.
type newlineStripper struct{ r io.Reader }

func (n newlineStripper) Read(p []byte) (int, error) {
	buf := make([]byte, len(p))
	k, err := n.r.Read(buf)
	j := 0
	for i := 0; i < k; i++ {
		if buf[i] != '\r' && buf[i] != '\n' {
			p[j] = buf[i]
			j++
		}
	}
	return j, err
}
