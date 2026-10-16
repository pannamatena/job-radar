package mailbox

import (
	"strings"
	"testing"
)

func TestParseReply_MultipartQuotedPrintable(t *testing.T) {
	raw := "From: Sam Rivera <Sam@Example.com>\r\nTo: bot@example.com\r\nSubject: Re: job-radar digest\r\n" +
		"Message-ID: <reply1@example.com>\r\nIn-Reply-To: <sent1@example.com>\r\nReferences: <old@example.com> <sent1@example.com>\r\n" +
		"Date: Fri, 17 Oct 2026 09:00:00 +0100\r\nMIME-Version: 1.0\r\n" +
		"Content-Type: multipart/alternative; boundary=\"b1\"\r\n\r\n" +
		"--b1\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n1 =F0=9F=91=8D, 3 =F0=9F=91=8E\r\n\r\n> quoted\r\n" +
		"--b1\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n<p>1 up</p>\r\n--b1--\r\n"
	r, err := ParseReply([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if r.MessageID != "<reply1@example.com>" || r.From != "sam@example.com" {
		t.Errorf("id/from = %q / %q", r.MessageID, r.From)
	}
	if len(r.Refs) != 3 || r.Refs[0] != "<sent1@example.com>" {
		t.Errorf("refs = %v", r.Refs)
	}
	if !strings.Contains(r.Body, "1 👍, 3 👎") || strings.Contains(r.Body, "<p>") {
		t.Errorf("body = %q", r.Body)
	}
}

func TestParseReply_PlainAndBase64(t *testing.T) {
	plain := "From: a@example.com\r\nMessage-ID: <p@x>\r\nContent-Type: text/plain\r\n\r\n2 down\r\n"
	r, err := ParseReply([]byte(plain))
	if err != nil || strings.TrimSpace(r.Body) != "2 down" {
		t.Errorf("plain: %q %v", r.Body, err)
	}
	b64 := "From: a@example.com\r\nMessage-ID: <b@x>\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\nMSB1cApvaw==\r\n"
	r, err = ParseReply([]byte(b64))
	if err != nil || !strings.Contains(r.Body, "1 up") {
		t.Errorf("base64: %q %v", r.Body, err)
	}
}

func TestParseReply_HTMLOnlyGivesNoBody(t *testing.T) {
	raw := "From: a@example.com\r\nMessage-ID: <h@x>\r\nContent-Type: text/html\r\n\r\n<p>1 up</p>\r\n"
	r, err := ParseReply([]byte(raw))
	if err != nil || r.Body != "" {
		t.Errorf("html-only mail should give an empty body, got %q (%v)", r.Body, err)
	}
	if _, err := ParseReply([]byte("not an email at all")); err == nil {
		t.Error("garbage should be an error")
	}
}
