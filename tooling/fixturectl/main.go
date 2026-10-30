package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/otfabric/snap7-interop/internal/fixture"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 {
		usage()
		return 2
	}

	switch args[0] {
	case "validate":
		return cmdValidate(args[1:])
	case "compile":
		return cmdCompile(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `fixturectl — snap7-interop fixture tooling

Usage:
  fixturectl validate [--schema PATH] [--json] FILE [FILE...]
  fixturectl compile  [--schema PATH] [--out DIR] FILE [FILE...]

`)
}

func cmdValidate(args []string) int {
	schemaPath := ""
	jsonOut := false
	files := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--schema":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--schema requires a path")
				return 2
			}
			i++
			schemaPath = args[i]
		case "--json":
			jsonOut = true
		case "--help", "-h":
			usage()
			return 0
		default:
			files = append(files, args[i])
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "validate requires at least one fixture file")
		return 2
	}

	v, err := loadValidator(schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load schema: %v\n", err)
		return 2
	}

	results := make([]fixture.Result, 0, len(files))
	failed := false
	for _, f := range files {
		r := v.ValidateFile(f)
		results = append(results, r)
		if !r.Valid {
			failed = true
		}
	}

	if jsonOut {
		if err := fixture.WriteJSON(os.Stdout, results); err != nil {
			fmt.Fprintf(os.Stderr, "write json: %v\n", err)
			return 2
		}
	} else if err := fixture.WriteText(os.Stdout, results); err != nil {
		fmt.Fprintf(os.Stderr, "write text: %v\n", err)
		return 2
	}

	if failed {
		return 1
	}
	return 0
}

func cmdCompile(args []string) int {
	schemaPath := ""
	outDir := ""
	files := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--schema":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--schema requires a path")
				return 2
			}
			i++
			schemaPath = args[i]
		case "--out":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--out requires a directory")
				return 2
			}
			i++
			outDir = args[i]
		case "--help", "-h":
			usage()
			return 0
		default:
			files = append(files, args[i])
		}
	}

	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "compile requires at least one fixture file")
		return 2
	}

	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "locate repo root: %v\n", err)
		return 2
	}
	if outDir == "" {
		outDir = filepath.Join(root, "fixtures", "compiled")
	}

	v, err := loadValidator(schemaPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load schema: %v\n", err)
		return 2
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create out dir: %v\n", err)
		return 2
	}

	failed := false
	for _, f := range files {
		cr, err := v.CompileFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s\n%v\n", f, err)
			failed = true
			continue
		}
		base := strings.TrimSuffix(filepath.Base(f), filepath.Ext(f))
		outPath := filepath.Join(outDir, base+".compiled.json")
		if err := os.WriteFile(outPath, cr.Bytes, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", outPath, err)
			failed = true
			continue
		}
		fmt.Printf("OK  %s -> %s\n", f, outPath)
	}

	if failed {
		return 1
	}
	return 0
}

func loadValidator(schemaPath string) (*fixture.Validator, error) {
	if schemaPath == "" {
		root, err := findRepoRoot()
		if err != nil {
			return nil, err
		}
		schemaPath = fixture.DefaultSchemaPath(root)
	}
	return fixture.NewValidator(schemaPath)
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "fixtures", "schema", "fixture.schema.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("fixtures/schema/fixture.schema.json not found from %s", wd)
		}
		dir = parent
	}
}
