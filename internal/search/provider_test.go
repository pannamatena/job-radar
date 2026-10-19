package search

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/pannamatena/job-radar/internal/llm"
)

type stubProvider struct{ model string }

func (s stubProvider) Search(context.Context, string, Options) (Result, error) { return Result{}, nil }

func TestNewRejectsAnUnknownProviderAndListsTheKnownOnes(t *testing.T) {
	_, err := New("nonesuch", "m")
	if err == nil || !strings.Contains(err.Error(), `"nonesuch" is not supported`) || !strings.Contains(err.Error(), "anthropic") {
		t.Errorf("want an error naming the provider and the supported ones, got %v", err)
	}
}

func TestNewIsCaseInsensitiveAndNeedsTheProvidersCredentials(t *testing.T) {
	t.Setenv(llm.EnvAPIKey, "")
	if _, err := New(" Anthropic ", "m"); err != llm.ErrNoAPIKey {
		t.Errorf("a known provider without credentials should say so, got %v", err)
	}
	t.Setenv(llm.EnvAPIKey, "test-key")
	s, err := New("anthropic", "m")
	if err != nil || s == nil {
		t.Fatalf("anthropic with a key: %v", err)
	}
	if _, ok := s.(*Anthropic); !ok {
		t.Errorf("got %T", s)
	}
}

// A new provider is one registry line: this proves the rest of the package
// works with any Searcher, with no change to the finder or the link check.
func TestAnotherProviderPlugsInWithOneRegistration(t *testing.T) {
	providers["stub"] = func(model string) (Searcher, error) { return stubProvider{model}, nil }
	defer delete(providers, "stub")

	s, err := New("stub", "some-model")
	if err != nil || !reflect.DeepEqual(s, stubProvider{"some-model"}) {
		t.Fatalf("got %v, %v", s, err)
	}
	if got := Providers(); !reflect.DeepEqual(got, []string{"anthropic", "stub"}) {
		t.Errorf("providers = %v", got)
	}
}
