package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestRedactsSecretKeyedAttributes(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Writer: &buf, JSON: true})
	log.Info("sending", "app_password", "hunter2-secret", "api_key", "sk-ant-123")

	out := buf.String()
	if strings.Contains(out, "hunter2-secret") || strings.Contains(out, "sk-ant-123") {
		t.Fatalf("secret-keyed values leaked:\n%s", out)
	}
	if !strings.Contains(out, Redacted) {
		t.Fatalf("expected %s placeholder:\n%s", Redacted, out)
	}
}

func TestRedactsKnownSecretEnvValuesInMessages(t *testing.T) {
	// This is the test BUILD_PLAN.md §9a point 2 calls for: the logger must
	// redact the values of known secret env vars, even when interpolated into
	// an otherwise innocent-looking attribute.
	environ := []string{
		"ANTHROPIC_API_KEY=sk-ant-SUPERSECRET",
		"RADAR_EMAIL_APP_PASSWORD=abcd efgh ijkl mnop",
		"HOME=/home/user", // not a secret; must not be collected
	}
	secrets := SecretsFromEnv(environ)
	if len(secrets) != 2 {
		t.Fatalf("SecretsFromEnv found %d secrets, want 2: %v", len(secrets), secrets)
	}

	var buf bytes.Buffer
	log := New(Options{Writer: &buf, JSON: true, SecretValues: secrets})
	log.Info("debugging", "detail", "connecting with key sk-ant-SUPERSECRET and pw abcd efgh ijkl mnop")

	out := buf.String()
	if strings.Contains(out, "sk-ant-SUPERSECRET") {
		t.Errorf("API key leaked into a message:\n%s", out)
	}
	if strings.Contains(out, "abcd efgh ijkl mnop") {
		t.Errorf("app password leaked into a message:\n%s", out)
	}
	if strings.Contains(out, "/home/user") {
		// sanity: /home/user shouldn't be treated as secret, but it also
		// shouldn't appear here at all since we didn't log it.
		t.Errorf("unexpected content:\n%s", out)
	}
}

func TestSecretsFromEnvMatchesSuffixPatterns(t *testing.T) {
	environ := []string{
		"SOME_TOKEN=tok",
		"MY_SECRET=s",
		"DB_PASSWORD=p",
		"SERVICE_API_KEY=k",
		"PLAIN=value",
		"EMPTY_TOKEN=",
	}
	got := SecretsFromEnv(environ)
	want := map[string]bool{"tok": true, "s": true, "p": true, "k": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want 4 values %v", got, want)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("unexpected secret value %q", v)
		}
	}
}

func TestNonSecretAttrsPassThrough(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Writer: &buf, JSON: true, SecretValues: []string{}})
	log.Info("run", slog.Int("new_postings", 3), slog.String("company", "Acme"))
	out := buf.String()
	if !strings.Contains(out, "new_postings") || !strings.Contains(out, "Acme") {
		t.Fatalf("normal attributes should pass through:\n%s", out)
	}
}
