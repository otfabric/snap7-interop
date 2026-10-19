package fixture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otfabric/snap7-interop/internal/fixture"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// internal/fixture -> repo root
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

func newValidator(t *testing.T) *fixture.Validator {
	t.Helper()
	v, err := fixture.NewValidator(fixture.DefaultSchemaPath(repoRoot(t)))
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func TestValidateCanonicalFixtures(t *testing.T) {
	v := newValidator(t)
	root := repoRoot(t)
	entries, err := filepath.Glob(filepath.Join(root, "fixtures", "canonical", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no canonical fixtures found")
	}
	for _, path := range entries {
		t.Run(filepath.Base(path), func(t *testing.T) {
			r := v.ValidateFile(path)
			if !r.Valid {
				t.Fatalf("expected valid, got: %v", r.Diagnostics)
			}
		})
	}
}

func TestValidateInvalidFixtures(t *testing.T) {
	v := newValidator(t)
	root := repoRoot(t)

	cases := []struct {
		file string
		code string
	}{
		{"invalid-bounds.json", fixture.CodeBounds},
		{"invalid-overlap.json", fixture.CodeOverlap},
		{"invalid-type-range.json", fixture.CodeTypeRange},
		{"invalid-duplicate-area.json", fixture.CodeDuplicateArea},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join(root, "fixtures", "invalid", tc.file)
			r := v.ValidateFile(path)
			if r.Valid {
				t.Fatal("expected invalid fixture")
			}
			found := false
			for _, d := range r.Diagnostics {
				if d.Code == tc.code {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected diagnostic %s, got %#v", tc.code, r.Diagnostics)
			}
		})
	}
}

func TestRejectUnknownFields(t *testing.T) {
	v := newValidator(t)
	root := repoRoot(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "unknown.json")
	content := `{
  "$schema": "` + filepath.ToSlash(filepath.Join(root, "fixtures", "schema", "fixture.schema.json")) + `",
  "schemaVersion": "1.0.0",
  "id": "unknown-field",
  "description": "has an unknown field",
  "server": {"rack": 0, "slot": 2},
  "areas": [{"type": "db", "number": 1, "size": 8, "access": "read-write"}],
  "notARealField": true
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	r := v.ValidateFile(path)
	if r.Valid {
		t.Fatal("expected unknown field to fail")
	}
}
