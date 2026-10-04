// Package logging configures the tool's structured logger (log/slog) with one
// non-negotiable guarantee: secrets never reach the logs.
//
// Two layers of defence:
//  1. Any attribute whose key looks like a secret (password, token, api key,
//     secret) has its value redacted.
//  2. The literal values of known secret environment variables (anything
//     matching *_API_KEY, *_PASSWORD, *_TOKEN, *_SECRET, plus a few named
//     ones) are redacted wherever they appear in a logged string — so even if
//     a secret is accidentally interpolated into a message, it is scrubbed.
//
// This matters because job-radar runs in GitHub Actions, where logs may be
// visible, and because it handles email app passwords and an LLM API key
// (BUILD_PLAN.md §9a point 2).
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Redacted is the placeholder written in place of any secret.
const Redacted = "[REDACTED]"

// secretKeyHints are substrings that mark an attribute key as sensitive.
var secretKeyHints = []string{"password", "passwd", "api_key", "apikey", "token", "secret"}

// Options configures New.
type Options struct {
	Level  slog.Level
	JSON   bool      // JSON output (used in CI); otherwise human-readable text
	Writer io.Writer // defaults to os.Stderr
	// SecretValues are literal strings to scrub from all output. If nil, they
	// are collected from the environment via SecretsFromEnv.
	SecretValues []string
}

// New builds a redacting *slog.Logger.
func New(opts Options) *slog.Logger {
	w := opts.Writer
	if w == nil {
		w = os.Stderr
	}
	secrets := opts.SecretValues
	if secrets == nil {
		secrets = SecretsFromEnv(os.Environ())
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       opts.Level,
		ReplaceAttr: redactor(secrets),
	}
	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(w, handlerOpts)
	} else {
		h = slog.NewTextHandler(w, handlerOpts)
	}
	return slog.New(h)
}

// redactor returns a slog ReplaceAttr function that scrubs secrets.
func redactor(secretValues []string) func(groups []string, a slog.Attr) slog.Attr {
	return func(_ []string, a slog.Attr) slog.Attr {
		if isSecretKey(a.Key) {
			return slog.String(a.Key, Redacted)
		}
		if a.Value.Kind() == slog.KindString {
			if scrubbed, changed := scrub(a.Value.String(), secretValues); changed {
				return slog.String(a.Key, scrubbed)
			}
		}
		return a
	}
}

func isSecretKey(key string) bool {
	k := strings.ToLower(key)
	for _, hint := range secretKeyHints {
		if strings.Contains(k, hint) {
			return true
		}
	}
	return false
}

// scrub replaces any occurrence of a secret value with the placeholder.
func scrub(s string, secretValues []string) (string, bool) {
	changed := false
	for _, v := range secretValues {
		if v == "" {
			continue
		}
		if strings.Contains(s, v) {
			s = strings.ReplaceAll(s, v, Redacted)
			changed = true
		}
	}
	return s, changed
}

// SecretsFromEnv extracts the values of environment variables whose names look
// like secrets, given an environment in "KEY=VALUE" form (os.Environ()).
func SecretsFromEnv(environ []string) []string {
	var out []string
	for _, kv := range environ {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			continue
		}
		key, val := kv[:eq], kv[eq+1:]
		if val == "" {
			continue
		}
		if isSecretEnvName(key) {
			out = append(out, val)
		}
	}
	return out
}

func isSecretEnvName(name string) bool {
	u := strings.ToUpper(name)
	for _, suffix := range []string{"_API_KEY", "_PASSWORD", "_APP_PASSWORD", "_TOKEN", "_SECRET"} {
		if strings.HasSuffix(u, suffix) {
			return true
		}
	}
	switch u {
	case "ANTHROPIC_API_KEY", "RADAR_EMAIL_APP_PASSWORD":
		return true
	}
	return false
}
