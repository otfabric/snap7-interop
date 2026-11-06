// SPDX-License-Identifier: MIT

// Command snap7-interop-otfabric serves a compiled snap7-interop fixture
// with OT Fabric's go-s7comm server, behind the same container contract as
// the native and python servers (docs/CONTAINER_CONTRACT.md). It is not an
// independent implementation: see README.md.
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/otfabric/go-s7comm/server"
)

// Set at build time (-ldflags -X).
var (
	adapterVersion   = "dev"
	upstreamVersion  = "unknown"
	upstreamRevision = "unknown"
)

const implementation = "otfabric"

// Exit codes (docs/CONTAINER_CONTRACT.md).
const (
	exitOK      = 0
	exitStartup = 1
	exitConfig  = 2
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Getenv, os.Stdout))
}

// identity is what the image reports about itself (`version`, the state
// snapshot and the server_ready event).
type identity struct {
	Name              string `json:"name"`
	AdapterVersion    string `json:"adapterVersion"`
	Upstream          string `json:"upstream"`
	UpstreamVersion   string `json:"upstreamVersion"`
	UpstreamRevision  string `json:"upstreamRevision"`
	IndependentEngine bool   `json:"independentEngine"`
}

func self() identity {
	return identity{
		Name:              implementation,
		AdapterVersion:    adapterVersion,
		Upstream:          "go-s7comm",
		UpstreamVersion:   upstreamVersion,
		UpstreamRevision:  upstreamRevision,
		IndependentEngine: false,
	}
}

// run is main without the process: args, environment and log output are
// injected so tests can drive it.
func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "version":
			_ = json.NewEncoder(out).Encode(self())
			return exitOK
		default:
			fmt.Fprintf(out, "usage: snap7-interop-otfabric [version]\n")
			return exitConfig
		}
	}
	lg := newLogger(out)
	fixturePath := getenv("SNAP_INTEROP_FIXTURE")
	if fixturePath == "" {
		lg.event("error", "fatal_error", "unknown", "error", "SNAP_INTEROP_FIXTURE is required")
		return exitConfig
	}
	listen := envOr(getenv, "SNAP_INTEROP_LISTEN_ADDRESS", "0.0.0.0")
	port, err := strconv.Atoi(envOr(getenv, "SNAP_INTEROP_PORT", "102"))
	if err != nil || port < 1 || port > 65535 {
		lg.event("error", "fatal_error", "unknown", "error", "invalid SNAP_INTEROP_PORT")
		return exitConfig
	}
	statePath := envOr(getenv, "SNAP_INTEROP_STATE_PATH", "/run/snap-interop/state.json")

	fx, err := loadFixture(fixturePath)
	if err != nil {
		lg.event("error", "fatal_error", "unknown", "error", err.Error())
		return exitStartup
	}
	lg.event("info", "fixture_loaded", fx.Fixture.ID, "compiled_sha256", fx.Fixture.CompiledSHA256)

	srv, err := newServer(fx)
	if err != nil {
		lg.event("error", "fatal_error", fx.Fixture.ID, "error", err.Error())
		return exitStartup
	}
	snap := &snapshotter{path: statePath, fx: fx, srv: srv}
	if err := snap.write("starting"); err != nil {
		lg.event("error", "fatal_error", fx.Fixture.ID, "error", err.Error())
		return exitStartup
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, os.Interrupt)
	defer stop()
	address := net.JoinHostPort(listen, strconv.Itoa(port))
	lg.event("info", "server_starting", fx.Fixture.ID, "listen_address", listen, "port", port)
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", address)
	if err != nil {
		lg.event("error", "fatal_error", fx.Fixture.ID, "error", err.Error())
		return exitStartup
	}
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ctx, ln) }()
	if err := snap.write("ready"); err != nil {
		lg.event("error", "fatal_error", fx.Fixture.ID, "error", err.Error())
		_ = srv.Close()
		<-served
		return exitStartup
	}
	id := self()
	lg.event("info", "server_ready", fx.Fixture.ID, "port", port, "upstream", id.Upstream,
		"upstream_version", id.UpstreamVersion, "independent_engine", id.IndependentEngine)

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			_ = snap.write("ready") // diagnostics must not stop the server
		case err := <-served:
			lg.event("info", "server_stopping", fx.Fixture.ID)
			_ = snap.write("stopped")
			if ctx.Err() != nil || errors.Is(err, server.ErrServerClosed) {
				lg.event("info", "server_stopped", fx.Fixture.ID)
				return exitOK
			}
			lg.event("error", "fatal_error", fx.Fixture.ID, "error", err.Error())
			return exitStartup
		}
	}
}

func envOr(getenv func(string) string, name, def string) string {
	if v := getenv(name); v != "" {
		return v
	}
	return def
}

// compiledFixture is the part of a compiled fixture the server needs
// (fixtures/compiled/*.compiled.json, format 1.0.0).
type compiledFixture struct {
	Format        string `json:"format"`
	FormatVersion string `json:"formatVersion"`
	Fixture       struct {
		ID             string `json:"id"`
		Revision       int    `json:"revision"`
		CompiledSHA256 string `json:"compiledSha256"`
	} `json:"fixture"`
	Server struct {
		PDUSize int `json:"pduSize"`
	} `json:"server"`
	Areas []fixtureArea `json:"areas"`
}

type fixtureArea struct {
	Type       string `json:"type"`
	Number     int    `json:"number"`
	Size       int    `json:"size"`
	Access     string `json:"access"`
	DataBase64 string `json:"dataBase64"`
	data       []byte
}

// maxFixtureAreaSize is the area limit of the reference servers' loaders.
const maxFixtureAreaSize = 1024 * 1024

func loadFixture(path string) (*compiledFixture, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- the fixture path is the container's configuration
	if err != nil {
		return nil, fmt.Errorf("read fixture: %w", err)
	}
	var fx compiledFixture
	if err := json.Unmarshal(raw, &fx); err != nil {
		return nil, fmt.Errorf("parse fixture: %w", err)
	}
	if fx.Format != "snap-interop-compiled-fixture" {
		return nil, errors.New("unsupported compiled fixture format")
	}
	if fx.FormatVersion != "1.0.0" {
		return nil, errors.New("unsupported compiled fixture formatVersion")
	}
	if fx.Fixture.ID == "" {
		return nil, errors.New("compiled fixture has no id")
	}
	if len(fx.Areas) == 0 {
		return nil, errors.New("compiled fixture has no areas")
	}
	if fx.Server.PDUSize == 0 {
		fx.Server.PDUSize = server.DefaultPDUSize
	}
	for i := range fx.Areas {
		a := &fx.Areas[i]
		if a.Size <= 0 || a.Size > maxFixtureAreaSize {
			return nil, fmt.Errorf("area %d: size %d out of range", i, a.Size)
		}
		data, err := base64.StdEncoding.DecodeString(a.DataBase64)
		if err != nil {
			return nil, fmt.Errorf("area %d: dataBase64: %w", i, err)
		}
		if len(data) != a.Size {
			return nil, fmt.Errorf("area %d: dataBase64 length %d, size %d", i, len(data), a.Size)
		}
		if a.Access == "" {
			a.Access = "read-write"
		}
		if a.Access != "read-write" && a.Access != "read-only" {
			return nil, fmt.Errorf("area %d: access %q", i, a.Access)
		}
		a.data = data
	}
	return &fx, nil
}

var areaTypes = map[string]server.Area{
	"db":      server.AreaDB,
	"markers": server.AreaMerkers,
	"inputs":  server.AreaInputs,
	"outputs": server.AreaOutputs,
}

func newServer(fx *compiledFixture) (*server.Server, error) {
	srv, err := server.New(server.Config{MaxPDUSize: fx.Server.PDUSize})
	if err != nil {
		return nil, err
	}
	for i, a := range fx.Areas {
		area, ok := areaTypes[a.Type]
		if !ok {
			return nil, fmt.Errorf("area %d: type %q is not served", i, a.Type)
		}
		opts := server.AreaOptions{ReadOnly: a.Access == "read-only"}
		if area == server.AreaDB {
			err = srv.RegisterDB(a.Number, a.data, opts)
		} else {
			err = srv.RegisterArea(area, a.data, opts)
		}
		if err != nil {
			return nil, fmt.Errorf("area %d: %w", i, err)
		}
	}
	return srv, nil
}

// snapshotter writes the diagnostic state snapshot atomically.
type snapshotter struct {
	path string
	fx   *compiledFixture
	srv  *server.Server
}

func (s *snapshotter) write(status string) error {
	type area struct {
		Type       string `json:"type"`
		Number     *int   `json:"number,omitempty"`
		Size       int    `json:"size"`
		Access     string `json:"access"`
		DataBase64 string `json:"dataBase64"`
	}
	areas := make([]area, 0, len(s.fx.Areas))
	for _, a := range s.fx.Areas {
		data, err := s.srv.Read(areaTypes[a.Type], a.Number, 0, a.Size)
		if err != nil {
			return err
		}
		item := area{Type: a.Type, Size: a.Size, Access: a.Access, DataBase64: base64.StdEncoding.EncodeToString(data)}
		if a.Type == "db" {
			n := a.Number
			item.Number = &n
		}
		areas = append(areas, item)
	}
	doc := map[string]any{
		"fixture": map[string]any{
			"id":             s.fx.Fixture.ID,
			"revision":       s.fx.Fixture.Revision,
			"compiledSha256": s.fx.Fixture.CompiledSHA256,
		},
		"implementation": self(),
		"serverStatus":   status,
		"lastUpdateTime": time.Now().UTC().Format(time.RFC3339),
		"areas":          areas,
	}
	body, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// logger writes one JSON object per line (docs/CONTAINER_CONTRACT.md, Log).
type logger struct {
	mu  sync.Mutex
	out io.Writer
}

func newLogger(out io.Writer) *logger { return &logger{out: out} }

func (l *logger) event(level, event, fixtureID string, kv ...any) {
	doc := map[string]any{
		"timestamp":       time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"level":           level,
		"implementation":  implementation,
		"event":           event,
		"fixture_id":      fixtureID,
		"adapter_version": adapterVersion,
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok {
			doc[k] = kv[i+1]
		}
	}
	line, err := json.Marshal(doc)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(append(line, '\n'))
}
