package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otfabric/snap7-interop/internal/fixture"
)

// capture runs fn and returns what it wrote to stdout and stderr.
func capture(t *testing.T, fn func() int) (code int, stdout, stderr string) {
	t.Helper()
	dir := t.TempDir()
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	out, errf := open("out"), open("err")
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = out, errf
	defer func() { os.Stdout, os.Stderr = oldOut, oldErr }()
	code = fn()
	read := func(f *os.File) string {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return code, read(out), read(errf)
}

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

func repo(parts ...string) string {
	return filepath.Join(append([]string{"..", ".."}, parts...)...)
}

func goodFixture() string { return repo("fixtures", "canonical", "basic-db.json") }
func badFixture() string  { return repo("fixtures", "invalid", "invalid-bounds.json") }

func TestRunDispatch(t *testing.T) {
	tests := []struct {
		args []string
		code int
		want string
	}{
		{nil, 2, "Usage:"},
		{[]string{"help"}, 0, "Usage:"},
		{[]string{"-h"}, 0, "Usage:"},
		{[]string{"explode"}, 2, `unknown command "explode"`},
	}
	for _, tt := range tests {
		code, _, stderr := capture(t, func() int { return run(tt.args) })
		if code != tt.code || !strings.Contains(stderr, tt.want) {
			t.Fatalf("%v: %d, %q", tt.args, code, stderr)
		}
	}
}

func TestValidate(t *testing.T) {
	code, stdout, _ := capture(t, func() int { return run([]string{"validate", goodFixture()}) })
	if code != 0 || !strings.HasPrefix(stdout, "OK  ") {
		t.Fatalf("%d, %q", code, stdout)
	}
	code, stdout, _ = capture(t, func() int { return run([]string{"validate", goodFixture(), badFixture()}) })
	if code != 1 || !strings.Contains(stdout, "FAIL ") || !strings.Contains(stdout, fixture.CodeBounds) {
		t.Fatalf("%d, %q", code, stdout)
	}
}

func TestValidateJSON(t *testing.T) {
	schema := repo("fixtures", "schema", "fixture.schema.json")
	code, stdout, _ := capture(t, func() int {
		return run([]string{"validate", "--schema", schema, "--json", badFixture()})
	})
	if code != 1 {
		t.Fatalf("exit %d", code)
	}
	var got []fixture.Result
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("%v: %q", err, stdout)
	}
	if len(got) != 1 || got[0].Valid {
		t.Fatalf("%+v", got)
	}
}

func TestValidateUsage(t *testing.T) {
	tests := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"--help"}, 0, "Usage:"},
		{nil, 2, "at least one"},
		{[]string{"--schema"}, 2, "requires a path"},
		{[]string{"--schema", filepath.Join(t.TempDir(), "x.json"), goodFixture()}, 2, "load schema"},
	}
	for _, tt := range tests {
		code, _, stderr := capture(t, func() int { return cmdValidate(tt.args) })
		if code != tt.code || !strings.Contains(stderr, tt.want) {
			t.Fatalf("%v: %d, %q", tt.args, code, stderr)
		}
	}
}

func TestCompile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "nested", "out")
	code, stdout, _ := capture(t, func() int { return run([]string{"compile", "--out", out, goodFixture()}) })
	want := filepath.Join(out, "basic-db.compiled.json")
	if code != 0 || !strings.Contains(stdout, want) {
		t.Fatalf("%d, %q", code, stdout)
	}
	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(repo("fixtures", "compiled", "basic-db.compiled.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(committed) {
		t.Fatal("compile output differs from the committed compiled fixture")
	}
}

func TestCompileFailures(t *testing.T) {
	code, _, stderr := capture(t, func() int { return cmdCompile([]string{"--out", t.TempDir(), badFixture()}) })
	if code != 1 || !strings.Contains(stderr, "FAIL ") {
		t.Fatalf("%d, %q", code, stderr)
	}
	// The output path is taken by a directory.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "basic-db.compiled.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr = capture(t, func() int { return cmdCompile([]string{"--out", dir, goodFixture()}) })
	if code != 1 || !strings.Contains(stderr, "write ") {
		t.Fatalf("%d, %q", code, stderr)
	}
}

func TestCompileUsage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		args []string
		code int
		want string
	}{
		{[]string{"-h"}, 0, "Usage:"},
		{nil, 2, "at least one"},
		{[]string{"--schema"}, 2, "requires a path"},
		{[]string{"--out"}, 2, "requires a directory"},
		{[]string{"--schema", filepath.Join(t.TempDir(), "x.json"), goodFixture()}, 2, "load schema"},
		{[]string{"--out", filepath.Join(file, "sub"), goodFixture()}, 2, "create out dir"},
	}
	for _, tt := range tests {
		code, _, stderr := capture(t, func() int { return cmdCompile(tt.args) })
		if code != tt.code || !strings.Contains(stderr, tt.want) {
			t.Fatalf("%v: %d, %q", tt.args, code, stderr)
		}
	}
}

func TestRepoRootNotFound(t *testing.T) {
	good, err := filepath.Abs(goodFixture())
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, t.TempDir())
	if _, err := findRepoRoot(); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err %v", err)
	}
	code, _, stderr := capture(t, func() int { return cmdValidate([]string{good}) })
	if code != 2 || !strings.Contains(stderr, "load schema") {
		t.Fatalf("validate: %d, %q", code, stderr)
	}
	code, _, stderr = capture(t, func() int { return cmdCompile([]string{good}) })
	if code != 2 || !strings.Contains(stderr, "locate repo root") {
		t.Fatalf("compile: %d, %q", code, stderr)
	}
}

func TestRepoRootFromASubdirectory(t *testing.T) {
	root, err := findRepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "fixtures", "schema", "fixture.schema.json")); err != nil {
		t.Fatal(err)
	}
}
