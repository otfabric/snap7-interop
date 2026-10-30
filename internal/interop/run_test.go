package interop_test

import (
	"encoding/json"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/otfabric/snap7-interop/internal/interop"
	"github.com/otfabric/snap7-interop/internal/s7client"
	"github.com/otfabric/snap7-interop/internal/s7fake"
)

const readFixture = `{
  "fixture": {"id": "read-one"},
  "expectations": [{"operation": "read", "area": {"type": "db", "number": 1}, "result": {"hex": "0102"}}]
}`

func runConfig(t *testing.T, native, python string) interop.RunConfig {
	t.Helper()
	return interop.RunConfig{
		FixturePath: writeJSON(t, readFixture),
		NativeAddr:  native,
		PythonAddr:  python,
		ReportDir:   t.TempDir(),
		Timeout:     time.Second,
		ReadyWait:   100 * time.Millisecond,
	}
}

func TestRunFixture_pass(t *testing.T) {
	cfg := runConfig(t, server(t, memory()).Addr(), server(t, memory()).Addr())
	rep, err := interop.RunFixture(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != "pass" || rep.FixtureID != "read-one" || len(rep.Diffs) != 0 {
		t.Fatalf("report %+v", rep)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.ReportDir, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got interop.Report
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Status != "pass" || len(got.Native.Steps) != 1 || got.Native.Steps[0].BytesHex != "0102" {
		t.Fatalf("report.json %+v", got)
	}
	md, err := os.ReadFile(filepath.Join(cfg.ReportDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "No divergences.") || !strings.Contains(string(md), "| 0 | read | success 0102 | success 0102 |") {
		t.Fatalf("report.md:\n%s", md)
	}
	junit, err := os.ReadFile(filepath.Join(cfg.ReportDir, "junit.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Failures int `xml:"failures,attr"`
		Skipped  int `xml:"skipped,attr"`
	}
	if err := xml.Unmarshal(junit, &suite); err != nil {
		t.Fatalf("junit.xml: %v", err)
	}
	if suite.Failures != 0 || suite.Skipped != 0 {
		t.Fatalf("junit %+v", suite)
	}
}

// chdir is t.Chdir, which needs Go 1.24.
func chdir(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRunFixture_defaultsAndReportDir(t *testing.T) {
	chdir(t, t.TempDir())
	cfg := runConfig(t, server(t, memory()).Addr(), server(t, memory()).Addr())
	cfg.ReportDir, cfg.Timeout, cfg.ReadyWait = "", 0, 0
	if _, err := interop.RunFixture(cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join("reports", "read-one", "report.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRunFixture_divergence(t *testing.T) {
	other := s7fake.NewMemory(map[s7fake.Area][]byte{db1: {9, 9}})
	cfg := runConfig(t, server(t, memory()).Addr(), server(t, other).Addr())
	rep, err := interop.RunFixture(cfg)
	if err == nil || !strings.Contains(err.Error(), "status=fail") {
		t.Fatalf("err %v", err)
	}
	if rep.Status != "fail" || len(rep.Diffs) != 1 || rep.Diffs[0].Kind != "bytes" {
		t.Fatalf("report %+v", rep)
	}
	md, err := os.ReadFile(filepath.Join(cfg.ReportDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"## Diffs", "- python: `0909`", "- native: `0102`"} {
		if !strings.Contains(string(md), want) {
			t.Fatalf("report.md lacks %q:\n%s", want, md)
		}
	}
	junit, err := os.ReadFile(filepath.Join(cfg.ReportDir, "junit.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(junit), `failures="1"`) || !strings.Contains(string(junit), "<failure message=") {
		t.Fatalf("junit.xml:\n%s", junit)
	}
}

func TestRunFixture_skip(t *testing.T) {
	cfg := runConfig(t, server(t, memory()).Addr(), server(t, memory()).Addr())
	cfg.FixturePath = writeJSON(t, `{"fixture":{"id":"x"},"requires":["area.timers"]}`)
	rep, err := interop.RunFixture(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Status != "skip-unsupported" {
		t.Fatalf("report %+v", rep)
	}
	junit, err := os.ReadFile(filepath.Join(cfg.ReportDir, "junit.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(junit), `skipped="1"`) || !strings.Contains(string(junit), "<skipped message=") {
		t.Fatalf("junit.xml:\n%s", junit)
	}
	md, err := os.ReadFile(filepath.Join(cfg.ReportDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "Skip: skip-unsupported") {
		t.Fatalf("report.md:\n%s", md)
	}
}

func TestRunFixture_notReady(t *testing.T) {
	gone := server(t, memory())
	addr := gone.Addr()
	_ = gone.Close()
	up := server(t, memory()).Addr()

	rep, err := interop.RunFixture(runConfig(t, addr, up))
	if err == nil || !strings.Contains(err.Error(), "native not ready") || rep.Status != "infrastructure-error" {
		t.Fatalf("native: %+v, %v", rep, err)
	}
	rep, err = interop.RunFixture(runConfig(t, up, addr))
	if err == nil || !strings.Contains(err.Error(), "python not ready") || rep.Status != "infrastructure-error" {
		t.Fatalf("python: %+v, %v", rep, err)
	}
}

func TestRunFixture_errors(t *testing.T) {
	cfg := runConfig(t, "", "")
	cfg.FixturePath = filepath.Join(t.TempDir(), "missing.json")
	if _, err := interop.RunFixture(cfg); !os.IsNotExist(err) {
		t.Fatalf("no fixture: %v", err)
	}
	// The report directory cannot be created: a file is in the way.
	cfg = runConfig(t, server(t, memory()).Addr(), server(t, memory()).Addr())
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.ReportDir = filepath.Join(blocked, "sub")
	if rep, err := interop.RunFixture(cfg); err == nil || rep.Status != "pass" {
		t.Fatalf("%+v, %v", rep, err)
	}
}

func TestWriteReports_errors(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"report.json", "report.md", "junit.xml"} {
		sub := filepath.Join(dir, name+"-case")
		if err := os.MkdirAll(filepath.Join(sub, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := interop.WriteReports(sub, interop.Report{}); err == nil {
			t.Fatalf("%s: want an error", name)
		}
	}
}

func TestRetainFailureArtifacts(t *testing.T) {
	src := filepath.Join(t.TempDir(), "native.log")
	if err := os.WriteFile(src, []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "report")
	err := interop.RetainFailureArtifacts(dir, map[string]string{"native.log": src, "python.log": filepath.Join(dir, "missing")})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "native.log")); err != nil || string(got) != "log" {
		t.Fatalf("%q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "python.log")); !os.IsNotExist(err) {
		t.Fatalf("a missing log must be skipped: %v", err)
	}
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := interop.RetainFailureArtifacts(filepath.Join(blocked, "x"), nil); err == nil {
		t.Fatal("want an error")
	}
}

func TestCompareEndpoints(t *testing.T) {
	step := func(outcome s7client.Outcome, hex string) interop.StepResult {
		return interop.StepResult{Operation: "read", AreaType: "db", AreaNumber: 1, Outcome: outcome, BytesHex: hex}
	}
	ok := s7client.OutcomeSuccess
	bad := s7client.OutcomeAddressOutOfRange
	tests := []struct {
		name   string
		native interop.EndpointResult
		python interop.EndpointResult
		same   bool
		kinds  string
	}{
		{"equal", interop.EndpointResult{Steps: []interop.StepResult{step(ok, "01")}}, interop.EndpointResult{Steps: []interop.StepResult{step(ok, "01")}}, true, ""},
		{"both skip alike", interop.EndpointResult{SkipReason: "s"}, interop.EndpointResult{SkipReason: "s"}, true, ""},
		{"one skips", interop.EndpointResult{SkipReason: "s"}, interop.EndpointResult{}, false, "skip"},
		{"outcome", interop.EndpointResult{Steps: []interop.StepResult{step(ok, "01")}}, interop.EndpointResult{Steps: []interop.StepResult{step(bad, "")}}, false, "outcome"},
		{"python shorter", interop.EndpointResult{Steps: []interop.StepResult{step(ok, ""), step(ok, "")}}, interop.EndpointResult{Steps: []interop.StepResult{step(ok, "")}}, false, "length"},
		{"native shorter", interop.EndpointResult{}, interop.EndpointResult{Steps: []interop.StepResult{step(ok, "")}}, false, "length"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs, same := interop.CompareEndpoints(tt.native, tt.python)
			var kinds []string
			for _, d := range diffs {
				kinds = append(kinds, d.Kind)
				if d.Message == "" {
					t.Fatalf("diff without a message: %+v", d)
				}
			}
			if same != tt.same || strings.Join(kinds, " ") != tt.kinds {
				t.Fatalf("same %v, diffs %+v", same, diffs)
			}
		})
	}
}

func TestClassifyFixture(t *testing.T) {
	pass := interop.EndpointResult{Passed: true}
	fail := interop.EndpointResult{}
	broken := interop.EndpointResult{Steps: []interop.StepResult{{Outcome: s7client.OutcomeTransportFailure}}}
	tests := []struct {
		name      string
		native    interop.EndpointResult
		python    interop.EndpointResult
		compareOK bool
		invalid   bool
		want      string
	}{
		{"invalid", pass, pass, true, true, "invalid"},
		{"pass", pass, pass, true, false, "pass"},
		{"divergence", pass, pass, false, false, "fail"},
		{"a step failed", fail, pass, true, false, "fail"},
		{"a transport failure", pass, broken, true, false, "infrastructure-error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := interop.ClassifyFixture(tt.native, tt.python, tt.compareOK, tt.invalid); got != tt.want {
				t.Fatalf("%q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewReportTimestamp(t *testing.T) {
	rep := interop.NewReport("x", interop.EndpointResult{}, interop.EndpointResult{}, nil, "pass")
	if _, err := time.Parse(time.RFC3339, rep.GeneratedAt); err != nil {
		t.Fatal(err)
	}
}
