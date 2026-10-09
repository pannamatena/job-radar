package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# a comment\n\nJR_TEST_A=plain\nexport JR_TEST_B=\"quoted value\"\nJR_TEST_C='single'\nJR_TEST_KEEP=from-file\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JR_TEST_KEEP", "from-env") // already set → must win
	for _, k := range []string{"JR_TEST_A", "JR_TEST_B", "JR_TEST_C"} {
		t.Setenv(k, "") // register cleanup, then unset
		os.Unsetenv(k)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}
	want := map[string]string{"JR_TEST_A": "plain", "JR_TEST_B": "quoted value", "JR_TEST_C": "single", "JR_TEST_KEEP": "from-env"}
	for k, v := range want {
		if got := os.Getenv(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestLoadDotEnv_MissingFileIsFine(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "nope.env")); err != nil {
		t.Errorf("missing .env should not be an error, got %v", err)
	}
}

func TestLoadDotEnv_BadLineNeverEchoesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("sk-ant-secret-without-key\n"), 0o600)
	err := LoadDotEnv(path)
	if err == nil {
		t.Fatal("expected an error for a line without =")
	}
	if strings.Contains(err.Error(), "sk-ant") {
		t.Errorf("error must not echo the line's content: %v", err)
	}
}
