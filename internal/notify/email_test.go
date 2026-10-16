package notify

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func sampleMessage() Message {
	return Message{
		Kind: KindAlert, Subject: "job-radar: 2 strong matches – Acme, Globex 👍",
		Sections: []Section{{Alerts: []Alert{
			{ID: "greenhouse:acme/1", Company: "Acme", Title: "Engineering Manager", Location: "Dublin", Track: "manager",
				WorkModel: "hybrid", Score: 5, Why: "Fits <b>well</b>", Gap: "Go", URL: "https://example.com/job/1", Scorer: "llm", New: true},
			{ID: "lever:globex/2", Company: "Globex", Title: "Tech Lead", Score: 4, URL: "javascript:alert(1)"},
		}}},
	}
}

func TestBuildMIME_RoundTrips(t *testing.T) {
	m := sampleMessage()
	date := time.Date(2026, 10, 17, 8, 0, 0, 0, time.UTC)
	raw, err := BuildMIME("bot.jobradar@example.com", []string{"me@example.com", "you@example.com"}, m.Subject, RenderText(m), RenderHTML(m), "<abc@example.com>", date)
	if err != nil {
		t.Fatal(err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("not a valid email: %v", err)
	}
	if got := msg.Header.Get("Reply-To"); got != "bot.jobradar@example.com" {
		t.Errorf("Reply-To = %q", got)
	}
	if got := msg.Header.Get("Message-ID"); got != "<abc@example.com>" {
		t.Errorf("Message-ID = %q", got)
	}
	subj, err := new(mime.WordDecoder).DecodeHeader(msg.Header.Get("Subject"))
	if err != nil || subj != m.Subject {
		t.Errorf("subject = %q (%v), want %q", subj, err, m.Subject)
	}
	mt, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/alternative" {
		t.Fatalf("content type %q %v", mt, err)
	}
	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types []string
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(quotedprintable.NewReader(p))
		types = append(types, strings.SplitN(p.Header.Get("Content-Type"), ";", 2)[0])
		if !strings.Contains(string(body), "Engineering Manager") {
			t.Errorf("part %s is missing the job title", types[len(types)-1])
		}
	}
	if len(types) != 2 || types[0] != "text/plain" || types[1] != "text/html" {
		t.Errorf("parts = %v, want plain text then html", types)
	}
}

func TestRender_EscapesAndNumbers(t *testing.T) {
	m := sampleMessage()
	h := RenderHTML(m)
	if strings.Contains(h, "<b>well</b>") || !strings.Contains(h, "&lt;b&gt;well&lt;/b&gt;") {
		t.Errorf("scorer text must be escaped in html: %s", h)
	}
	if strings.Contains(h, "javascript:") {
		t.Error("a javascript: link must never be rendered")
	}
	if !strings.Contains(h, `href="https://example.com/job/1"`) {
		t.Error("an https link should be clickable")
	}
	txt := RenderText(m)
	for _, want := range []string{"1. [5 AI] Acme — Engineering Manager", "2. [4] Globex — Tech Lead", "1 👍, 3 👎"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text missing %q:\n%s", want, txt)
		}
	}
	if strings.Contains(txt, "javascript:") {
		t.Error("unsafe url leaked into text")
	}
}

func TestEmailSend_RecordsNumberingAndNeverLeaksPassword(t *testing.T) {
	var got []byte
	var gotTo []string
	e := &Email{From: "bot.jobradar@example.com", Password: "supersecretpassword", To: []string{"me@example.com"},
		deliver: func(_ context.Context, _ string, to []string, msg []byte) error { gotTo, got = to, msg; return nil }}
	sent, err := e.Send(context.Background(), sampleMessage())
	if err != nil {
		t.Fatal(err)
	}
	if len(sent.Numbered) != 2 || sent.Numbered[0] != "greenhouse:acme/1" || sent.Numbered[1] != "lever:globex/2" {
		t.Errorf("Numbered = %v", sent.Numbered)
	}
	if !strings.HasPrefix(sent.MessageID, "<") || !strings.HasSuffix(sent.MessageID, "@example.com>") {
		t.Errorf("MessageID = %q", sent.MessageID)
	}
	if !strings.Contains(string(got), "Message-ID: "+sent.MessageID) || len(gotTo) != 1 {
		t.Error("the sent email should carry the recorded Message-ID")
	}
	if strings.Contains(string(got), "supersecretpassword") {
		t.Error("the password must never appear in the email")
	}
	if _, err := (&Email{From: "a@b.c"}).Send(context.Background(), sampleMessage()); err == nil {
		t.Error("no recipients should be an error")
	}
}

func TestTerminal_Prints(t *testing.T) {
	var sb strings.Builder
	sent, err := (&Terminal{W: &sb}).Send(context.Background(), sampleMessage())
	if err != nil || len(sent.Numbered) != 2 {
		t.Fatalf("%v %v", sent, err)
	}
	if !strings.Contains(sb.String(), "1. [5 (AI)] Acme") {
		t.Errorf("terminal output:\n%s", sb.String())
	}
}
