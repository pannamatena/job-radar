package config

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestEveryConfigFieldIsDocumented keeps docs/configuration.md honest: if a new
// config field is added without documenting it, this fails. Each field's YAML
// name must appear in the doc wrapped in backticks, e.g. `office_ids`.
func TestEveryConfigFieldIsDocumented(t *testing.T) {
	data, err := os.ReadFile("../../docs/configuration.md")
	if err != nil {
		t.Fatalf("reading configuration.md: %v", err)
	}
	doc := string(data)

	names := map[string]bool{}
	collectYAMLNames(reflect.TypeOf(Config{}), names)

	var missing []string
	for name := range names {
		if !strings.Contains(doc, "`"+name+"`") {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("these config fields are undocumented in docs/configuration.md (wrap each in backticks): %s",
			strings.Join(missing, ", "))
	}
}

// collectYAMLNames walks a struct type (recursing into nested structs and slice
// element structs) and records every field's YAML name.
func collectYAMLNames(t reflect.Type, out map[string]bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := strings.Split(f.Tag.Get("yaml"), ",")[0]
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name)
		}
		out[name] = true
		collectYAMLNames(f.Type, out)
	}
}
