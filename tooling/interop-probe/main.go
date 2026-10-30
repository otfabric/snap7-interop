// interop-probe is the independent snap7-interop S7 probe and equivalence runner.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/otfabric/snap7-interop/internal/interop"
	"github.com/otfabric/snap7-interop/internal/s7client"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "smoke":
		os.Exit(cmdSmoke(os.Args[2:]))
	case "run", "interop":
		os.Exit(cmdRun(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		// Backward-compatible smoke flags without a subcommand.
		os.Exit(cmdSmoke(os.Args[1:]))
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `interop-probe — independent S7 smoke probe and interop runner

Usage:
  interop-probe smoke [flags]
  interop-probe run --fixture FILE --native ADDR --python ADDR [--out DIR]

Smoke flags:
  -addr HOST:PORT       (default 127.0.0.1:1102)
  -rack N -slot N -db N -offset N -length N
  -expect-hex HEX
  -write-hex HEX
  -timeout DURATION

`)
}

func cmdSmoke(args []string) int {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:1102", "host:port")
	rack := fs.Int("rack", 0, "rack")
	slot := fs.Int("slot", 2, "slot")
	db := fs.Int("db", 1, "db number")
	offset := fs.Int("offset", 0, "offset")
	length := fs.Int("length", 8, "length")
	expectHex := fs.String("expect-hex", "", "expected hex")
	writeHex := fs.String("write-hex", "", "write hex")
	timeout := fs.Duration("timeout", 5*time.Second, "timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	c, err := s7client.Dial(*addr, *rack, *slot, *timeout)
	if err != nil {
		fail("connect: %v", err)
	}
	defer func() { _ = c.Close() }()

	data, outcome, err := c.ReadArea("db", *db, *offset, *length)
	if err != nil || outcome != s7client.OutcomeSuccess {
		fail("read: outcome=%s err=%v", outcome, err)
	}
	fmt.Printf("READ db=%d offset=%d length=%d hex=%s\n", *db, *offset, *length, hex.EncodeToString(data))

	if *expectHex != "" {
		want, err := hex.DecodeString(strings.ReplaceAll(*expectHex, " ", ""))
		if err != nil {
			fail("expect-hex: %v", err)
		}
		if hex.EncodeToString(data) != hex.EncodeToString(want) {
			fail("byte mismatch: got %s want %s", hex.EncodeToString(data), hex.EncodeToString(want))
		}
		fmt.Println("OK expect-hex match")
	}

	if *writeHex != "" {
		payload, err := hex.DecodeString(strings.ReplaceAll(*writeHex, " ", ""))
		if err != nil {
			fail("write-hex: %v", err)
		}
		outcome, err := c.WriteArea("db", *db, *offset, payload)
		if err != nil || outcome != s7client.OutcomeSuccess {
			fail("write: outcome=%s err=%v", outcome, err)
		}
		fmt.Printf("WRITE db=%d offset=%d hex=%s\n", *db, *offset, hex.EncodeToString(payload))
		got, outcome, err := c.ReadArea("db", *db, *offset, len(payload))
		if err != nil || outcome != s7client.OutcomeSuccess {
			fail("re-read: outcome=%s err=%v", outcome, err)
		}
		if hex.EncodeToString(got) != hex.EncodeToString(payload) {
			fail("write verify mismatch: got %s want %s", hex.EncodeToString(got), hex.EncodeToString(payload))
		}
		fmt.Println("OK write verified")
	}
	return 0
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fixture := fs.String("fixture", "", "compiled fixture path")
	native := fs.String("native", "127.0.0.1:1102", "native endpoint")
	python := fs.String("python", "127.0.0.1:2102", "python endpoint")
	out := fs.String("out", "", "report directory")
	timeout := fs.Duration("timeout", 5*time.Second, "per-request timeout")
	ready := fs.Duration("ready-wait", 20*time.Second, "readiness wait")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *fixture == "" {
		fmt.Fprintln(os.Stderr, "--fixture is required")
		return 2
	}
	rep, err := interop.RunFixture(interop.RunConfig{
		FixturePath: *fixture,
		NativeAddr:  *native,
		PythonAddr:  *python,
		ReportDir:   *out,
		Timeout:     *timeout,
		ReadyWait:   *ready,
	})
	reportDir := *out
	if reportDir == "" {
		reportDir = filepath.Join("reports", rep.FixtureID)
	}
	fmt.Printf("fixture=%s status=%s reports=%s\n", rep.FixtureID, rep.Status, reportDir)
	for _, d := range rep.Diffs {
		fmt.Printf("DIFF: %s\n", d.Message)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		return 1
	}
	if rep.Status == "skip-unsupported" {
		fmt.Println("SKIP unsupported capabilities")
	}
	return 0
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}
