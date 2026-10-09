package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv reads KEY=VALUE lines from a .env file into the process
// environment, so secrets like ANTHROPIC_API_KEY can live in a git-ignored file
// instead of being typed into every shell. It's the Go equivalent of Node's
// `dotenv` package, kept to a few lines rather than adding a dependency.
//
// Rules: blank lines and lines starting with # are skipped; an optional
// "export " prefix is allowed; surrounding single or double quotes are removed.
// A variable that is already set in the environment wins, so CI secrets and
// one-off overrides (`ANTHROPIC_API_KEY=… job-radar run`) are never replaced.
// A missing file is not an error. Values are never logged or printed.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	line := 0
	for sc.Scan() {
		line++
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimPrefix(text, "export ")
		key, value, ok := strings.Cut(text, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			// Say where, never what: the line may contain a secret.
			return fmt.Errorf("%s line %d: expected KEY=VALUE", path, line)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		if _, set := os.LookupEnv(key); set {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("%s line %d: %w", path, line, err)
		}
	}
	return sc.Err()
}
