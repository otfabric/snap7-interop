package fixture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func schemaPath(t testing.TB) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "fixtures", "schema", "fixture.schema.json"),
		filepath.Join("fixtures", "schema", "fixture.schema.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatal("fixture.schema.json not found")
	return ""
}

func FuzzValidate(f *testing.F) {
	v, err := NewValidator(schemaPath(f))
	if err != nil {
		f.Fatal(err)
	}

	seeds := []string{
		`{"schemaVersion":"1.0.0","id":"basic-db","description":"x","server":{"rack":0,"slot":2},"areas":[{"type":"db","number":1,"size":8,"access":"read-write","initial":{"hex":"01020304"}}]}`,
		`{"schemaVersion":"1.0.0","id":"bad","description":"x","server":{"rack":0,"slot":2},"areas":[]}`,
		`not-json`,
		`{}`,
		`{"schemaVersion":"1.0.0","id":"../escape","description":"x","server":{"rack":0,"slot":2},"areas":[{"type":"db","number":1,"size":1,"access":"read-write"}]}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Bound input size to keep the fuzzer practical.
		if len(data) > 64*1024 {
			return
		}
		tmp := filepath.Join(t.TempDir(), "fuzz.json")
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = v.ValidateFile(tmp)
	})
}

func FuzzCompile(f *testing.F) {
	v, err := NewValidator(schemaPath(f))
	if err != nil {
		f.Fatal(err)
	}

	seedPath := filepath.Join("..", "..", "fixtures", "canonical", "basic-db.json")
	if raw, err := os.ReadFile(seedPath); err == nil {
		f.Add(raw)
	} else {
		f.Add([]byte(`{"schemaVersion":"1.0.0","id":"basic-db","description":"x","server":{"rack":0,"slot":2},"areas":[{"type":"db","number":1,"size":8,"access":"read-write","initial":{"hex":"01020304"}}]}`))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64*1024 {
			return
		}
		var probe any
		if err := json.Unmarshal(data, &probe); err != nil {
			return
		}
		tmp := filepath.Join(t.TempDir(), "fuzz.json")
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			t.Fatal(err)
		}
		res := v.ValidateFile(tmp)
		if !res.Valid {
			return
		}
		_, _ = v.CompileFile(tmp)
	})
}
