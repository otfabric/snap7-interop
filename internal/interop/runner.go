package interop

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunConfig configures a dual-endpoint fixture run.
type RunConfig struct {
	FixturePath string
	NativeAddr  string
	PythonAddr  string
	ReportDir   string
	Timeout     time.Duration
	ReadyWait   time.Duration
}

// RunFixture executes one compiled fixture against both endpoints and writes reports.
func RunFixture(cfg RunConfig) (Report, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.ReadyWait == 0 {
		cfg.ReadyWait = 20 * time.Second
	}
	fx, err := LoadCompiledFixture(cfg.FixturePath)
	if err != nil {
		return Report{}, err
	}
	if cfg.ReportDir == "" {
		cfg.ReportDir = filepath.Join("reports", fx.ID)
	}

	if err := WaitReady(cfg.NativeAddr, fx.Rack, fx.Slot, cfg.ReadyWait); err != nil {
		rep := NewReport(fx.ID,
			EndpointResult{Name: "native", Addr: cfg.NativeAddr, Passed: false},
			EndpointResult{Name: "python", Addr: cfg.PythonAddr, Passed: false},
			nil, "infrastructure-error")
		_ = WriteReports(cfg.ReportDir, rep)
		return rep, fmt.Errorf("native not ready: %w", err)
	}
	if err := WaitReady(cfg.PythonAddr, fx.Rack, fx.Slot, cfg.ReadyWait); err != nil {
		rep := NewReport(fx.ID,
			EndpointResult{Name: "native", Addr: cfg.NativeAddr, Passed: true},
			EndpointResult{Name: "python", Addr: cfg.PythonAddr, Passed: false},
			nil, "infrastructure-error")
		_ = WriteReports(cfg.ReportDir, rep)
		return rep, fmt.Errorf("python not ready: %w", err)
	}

	native := RunEndpoint("native", cfg.NativeAddr, fx, cfg.Timeout)
	python := RunEndpoint("python", cfg.PythonAddr, fx, cfg.Timeout)
	diffs, compareOK := CompareEndpoints(native, python)
	status := ClassifyFixture(native, python, compareOK, false)
	rep := NewReport(fx.ID, native, python, diffs, status)
	if err := WriteReports(cfg.ReportDir, rep); err != nil {
		return rep, err
	}
	if status != "pass" && status != "skip-unsupported" {
		return rep, fmt.Errorf("fixture %s status=%s", fx.ID, status)
	}
	return rep, nil
}

// RetainFailureArtifacts copies server logs into the report directory when present.
func RetainFailureArtifacts(reportDir string, logPaths map[string]string) error {
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return err
	}
	for name, src := range logPaths {
		raw, err := os.ReadFile(src)
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(reportDir, name), raw, 0o644)
	}
	return nil
}
