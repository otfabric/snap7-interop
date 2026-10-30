package interop

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Report is the machine-readable interop suite report for one fixture.
type Report struct {
	FixtureID   string         `json:"fixtureId"`
	Status      string         `json:"status"`
	GeneratedAt string         `json:"generatedAt"`
	Native      EndpointResult `json:"native"`
	Python      EndpointResult `json:"python"`
	Diffs       []Diff         `json:"diffs,omitempty"`
}

// WriteReports writes report.json, report.md and junit.xml into outDir.
func WriteReports(outDir string, rep Report) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "report.json"), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "report.md"), []byte(markdownReport(rep)), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "junit.xml"), []byte(junitReport(rep)), 0o644)
}

func markdownReport(rep Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Interop report: %s\n\n", rep.FixtureID)
	fmt.Fprintf(&b, "- Status: **%s**\n", rep.Status)
	fmt.Fprintf(&b, "- Generated: %s\n", rep.GeneratedAt)
	fmt.Fprintf(&b, "- Native: `%s` (%s)\n", rep.Native.Addr, passLabel(rep.Native.Passed))
	fmt.Fprintf(&b, "- Python: `%s` (%s)\n\n", rep.Python.Addr, passLabel(rep.Python.Passed))
	if rep.Native.SkipReason != "" {
		fmt.Fprintf(&b, "Skip: %s\n\n", rep.Native.SkipReason)
	}
	if len(rep.Diffs) == 0 {
		b.WriteString("No divergences.\n")
	} else {
		b.WriteString("## Diffs\n\n")
		for _, d := range rep.Diffs {
			fmt.Fprintf(&b, "- **%s** area=%s#%d offset=%d\n", d.Operation, d.AreaType, d.AreaNumber, d.Offset)
			fmt.Fprintf(&b, "  - %s\n", d.Message)
			if d.Kind == "bytes" {
				fmt.Fprintf(&b, "  - expected: `%s`\n", d.Expected)
				fmt.Fprintf(&b, "  - native: `%s`\n", d.Native)
				fmt.Fprintf(&b, "  - python: `%s`\n", d.Python)
			}
		}
	}
	b.WriteString("\n## Steps\n\n")
	b.WriteString("| # | op | native | python |\n|---|----|--------|--------|\n")
	n := len(rep.Native.Steps)
	if len(rep.Python.Steps) > n {
		n = len(rep.Python.Steps)
	}
	for i := 0; i < n; i++ {
		var ns, ps StepResult
		if i < len(rep.Native.Steps) {
			ns = rep.Native.Steps[i]
		}
		if i < len(rep.Python.Steps) {
			ps = rep.Python.Steps[i]
		}
		op := ns.Operation
		if op == "" {
			op = ps.Operation
		}
		fmt.Fprintf(&b, "| %d | %s | %s %s | %s %s |\n",
			i, op,
			ns.Outcome, ns.BytesHex,
			ps.Outcome, ps.BytesHex,
		)
	}
	return b.String()
}

func passLabel(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func junitReport(rep Report) string {
	failures := 0
	switch rep.Status {
	case "fail", "infrastructure-error", "invalid":
		failures = 1
	}
	skipped := 0
	if rep.Status == "skip-unsupported" {
		skipped = 1
	}
	name := xmlEscape(rep.FixtureID)
	var body strings.Builder
	fmt.Fprintf(&body, `<?xml version="1.0" encoding="UTF-8"?>`+"\n")
	fmt.Fprintf(&body, `<testsuite name="snap7-interop" tests="1" failures="%d" skipped="%d" timestamp="%s">`+"\n",
		failures, skipped, xmlEscape(rep.GeneratedAt))
	fmt.Fprintf(&body, `  <testcase classname="interop" name="%s">`+"\n", name)
	switch rep.Status {
	case "fail", "infrastructure-error", "invalid":
		msg := "interop failed"
		if len(rep.Diffs) > 0 {
			msg = rep.Diffs[0].Message
		}
		fmt.Fprintf(&body, `    <failure message="%s">%s</failure>`+"\n", xmlEscape(msg), xmlEscape(msg))
	case "skip-unsupported":
		fmt.Fprintf(&body, `    <skipped message="%s"/>`+"\n", xmlEscape(rep.Native.SkipReason))
	}
	body.WriteString("  </testcase>\n</testsuite>\n")
	return body.String()
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
		`"`, "&quot;",
		`'`, "&apos;",
	)
	return r.Replace(s)
}

// NewReport builds a report from endpoint results.
func NewReport(fixtureID string, native, python EndpointResult, diffs []Diff, status string) Report {
	return Report{
		FixtureID:   fixtureID,
		Status:      status,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Native:      native,
		Python:      python,
		Diffs:       diffs,
	}
}
