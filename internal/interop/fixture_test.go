package interop_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/snap7-interop/internal/interop"
	"github.com/otfabric/snap7-interop/internal/s7client"
)

func writeJSON(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "x.compiled.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// Every committed fixture loads, with each expectation in a form the runner
// executes.
func TestLoadCompiledFixture_committed(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "fixtures", "compiled", "*.compiled.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("%d fixtures, %v", len(paths), err)
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			fx, err := interop.LoadCompiledFixture(p)
			if err != nil {
				t.Fatal(err)
			}
			if fx.ID == "" || fx.Slot != 2 {
				t.Fatalf("fixture %+v", fx)
			}
			if strings.TrimSuffix(filepath.Base(p), ".compiled.json") != fx.ID {
				t.Fatalf("id %q in %s", fx.ID, p)
			}
			for i, e := range fx.Expectations {
				if e.Operation == "" || e.WantOutcome == "" {
					t.Fatalf("expectation %d: %+v", i, e)
				}
			}
		})
	}
}

func TestLoadCompiledFixture_fields(t *testing.T) {
	path := writeJSON(t, `{
  "fixture": {"id": "x", "revision": 3},
  "server": {"rack": 1, "slot": 4},
  "requires": ["operation.read"],
  "expectations": [
    {"operation": "read", "area": {"type": "db", "number": 7}, "offset": 2, "result": {"hex": "01 02"}},
    {"operation": "read", "area": {"type": "db", "number": 7}, "offset": 2, "length": 9, "result": {"hex": "01"}},
    {"operation": "write", "area": {"type": "markers"}, "bit": 3, "clients": 2, "data": {"hex": "01"}, "result": "accepted"}
  ]
}`)
	fx, err := interop.LoadCompiledFixture(path)
	if err != nil {
		t.Fatal(err)
	}
	if fx.ID != "x" || fx.Revision != 3 || fx.Rack != 1 || fx.Slot != 4 || len(fx.Requires) != 1 {
		t.Fatalf("fixture %+v", fx)
	}
	a, b, c := fx.Expectations[0], fx.Expectations[1], fx.Expectations[2]
	if a.AreaNum != 7 || a.Offset != 2 || a.Length != 2 || !a.HasBytes || a.WantOutcome != s7client.OutcomeSuccess {
		t.Fatalf("first %+v", a)
	}
	if b.Length != 9 {
		t.Fatalf("an explicit length must stay: %+v", b)
	}
	if !c.HasBit || c.Bit != 3 || c.Clients != 2 || len(c.Data) != 1 || c.WantOutcome != s7client.OutcomeSuccess {
		t.Fatalf("third %+v", c)
	}
}

func TestLoadCompiledFixture_defaults(t *testing.T) {
	fx, err := interop.LoadCompiledFixture(writeJSON(t, `{}`))
	if err != nil {
		t.Fatal(err)
	}
	if fx.Revision != 1 || fx.Rack != 0 || fx.Slot != 2 || fx.ID != "" {
		t.Fatalf("fixture %+v", fx)
	}
}

func TestLoadCompiledFixture_resultStrings(t *testing.T) {
	for in, want := range map[string]s7client.Outcome{
		"accepted":             s7client.OutcomeSuccess,
		"success":              s7client.OutcomeSuccess,
		"address-out-of-range": s7client.OutcomeAddressOutOfRange,
		"write-protected":      s7client.OutcomeWriteProtected,
		"unsupported":          s7client.OutcomeUnsupported,
		"transport-failure":    s7client.OutcomeTransportFailure,
		"protocol-failure":     s7client.OutcomeProtocolFailure,
		"something-else":       "something-else",
	} {
		raw, _ := json.Marshal(map[string]any{"expectations": []any{map[string]any{"operation": "read", "result": in}}})
		fx, err := interop.LoadCompiledFixture(writeJSON(t, string(raw)))
		if err != nil {
			t.Fatal(err)
		}
		if got := fx.Expectations[0].WantOutcome; got != want {
			t.Fatalf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestLoadCompiledFixture_errors(t *testing.T) {
	if _, err := interop.LoadCompiledFixture(filepath.Join(t.TempDir(), "missing.json")); !os.IsNotExist(err) {
		t.Fatalf("missing: %v", err)
	}
	tests := []struct{ name, in, want string }{
		{"not JSON", `{`, "unexpected"},
		{"expectation not an object", `{"expectations":[1]}`, "not an object"},
		{"no result", `{"expectations":[{"operation":"read"}]}`, "unsupported result"},
		{"data without hex", `{"expectations":[{"data":{},"result":"accepted"}]}`, "requires hex"},
		{"result with bad hex", `{"expectations":[{"result":{"hex":"zz"}}]}`, "invalid byte"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := interop.LoadCompiledFixture(writeJSON(t, tt.in)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err %v, want %q", err, tt.want)
			}
		})
	}
}
