// SPDX-License-Identifier: MIT

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/otfabric/go-s7comm/client"
)

// syncBuffer is a log sink safe for the server goroutine and the test.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func writeFixture(t *testing.T, mutate func(map[string]any)) string {
	t.Helper()
	doc := map[string]any{
		"format":        "snap-interop-compiled-fixture",
		"formatVersion": "1.0.0",
		"fixture":       map[string]any{"id": "unit", "revision": 1, "compiledSha256": "abc"},
		"server":        map[string]any{"rack": 0, "slot": 2, "pduSize": 480},
		"areas": []any{
			map[string]any{"type": "db", "number": 1, "size": 8, "access": "read-write", "dataBase64": base64.StdEncoding.EncodeToString([]byte{1, 2, 3, 4, 5, 6, 7, 8})},
			map[string]any{"type": "db", "number": 2, "size": 2, "access": "read-only", "dataBase64": base64.StdEncoding.EncodeToString([]byte{0xAA, 0xBB})},
			map[string]any{"type": "markers", "size": 4, "dataBase64": base64.StdEncoding.EncodeToString(make([]byte, 4))},
			map[string]any{"type": "inputs", "size": 2, "access": "read-write", "dataBase64": base64.StdEncoding.EncodeToString([]byte{9, 9})},
			map[string]any{"type": "outputs", "size": 2, "access": "read-write", "dataBase64": base64.StdEncoding.EncodeToString([]byte{7, 7})},
		},
	}
	if mutate != nil {
		mutate(doc)
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestVersion(t *testing.T) {
	var out bytes.Buffer
	if code := run(context.Background(), []string{"version"}, env(nil), &out); code != exitOK {
		t.Fatalf("exit %d", code)
	}
	var id identity
	if err := json.Unmarshal(out.Bytes(), &id); err != nil || id.Name != "otfabric" || id.Upstream != "go-s7comm" || id.IndependentEngine {
		t.Fatalf("version = %s (%v)", out.String(), err)
	}
	if code := run(context.Background(), []string{"bogus"}, env(nil), &out); code != exitConfig {
		t.Errorf("unknown command: exit %d", code)
	}
}

func TestStartupFailures(t *testing.T) {
	good := writeFixture(t, nil)
	for name, tc := range map[string]struct {
		env  map[string]string
		want int
	}{
		"no fixture":         {map[string]string{}, exitConfig},
		"bad port":           {map[string]string{"SNAP_INTEROP_FIXTURE": good, "SNAP_INTEROP_PORT": "x"}, exitConfig},
		"port 0":             {map[string]string{"SNAP_INTEROP_FIXTURE": good, "SNAP_INTEROP_PORT": "0"}, exitConfig},
		"missing file":       {map[string]string{"SNAP_INTEROP_FIXTURE": "/nonexistent.json"}, exitStartup},
		"bad listen":         {map[string]string{"SNAP_INTEROP_FIXTURE": good, "SNAP_INTEROP_LISTEN_ADDRESS": "256.0.0.1", "SNAP_INTEROP_STATE_PATH": filepath.Join(t.TempDir(), "s.json")}, exitStartup},
		"state not writable": {map[string]string{"SNAP_INTEROP_FIXTURE": good, "SNAP_INTEROP_STATE_PATH": "/dev/null/x/state.json"}, exitStartup},
	} {
		var out bytes.Buffer
		if code := run(context.Background(), nil, env(tc.env), &out); code != tc.want {
			t.Errorf("%s: exit %d, want %d (%s)", name, code, tc.want, out.String())
		}
		if !strings.Contains(out.String(), `"event":"fatal_error"`) {
			t.Errorf("%s: no fatal_error event: %s", name, out.String())
		}
	}
}

func TestFixtureValidation(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"format":    func(d map[string]any) { d["format"] = "other" },
		"version":   func(d map[string]any) { d["formatVersion"] = "2.0.0" },
		"no id":     func(d map[string]any) { d["fixture"] = map[string]any{} },
		"no areas":  func(d map[string]any) { d["areas"] = []any{} },
		"size":      func(d map[string]any) { d["areas"].([]any)[0].(map[string]any)["size"] = 0 },
		"base64":    func(d map[string]any) { d["areas"].([]any)[0].(map[string]any)["dataBase64"] = "!!" },
		"length":    func(d map[string]any) { d["areas"].([]any)[0].(map[string]any)["size"] = 9 },
		"access":    func(d map[string]any) { d["areas"].([]any)[0].(map[string]any)["access"] = "write-only" },
		"area type": func(d map[string]any) { d["areas"].([]any)[0].(map[string]any)["type"] = "timers" },
		"DB 0":      func(d map[string]any) { d["areas"].([]any)[0].(map[string]any)["number"] = 0 },
		"PDU":       func(d map[string]any) { d["server"] = map[string]any{"pduSize": 100} },
	} {
		fx, err := loadFixture(writeFixture(t, mutate))
		if err == nil {
			_, err = newServer(fx)
		}
		if err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := loadFixture(filepath.Join(t.TempDir(), "x")); err == nil {
		t.Error("missing file accepted")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	_ = os.WriteFile(bad, []byte("{"), 0o600)
	if _, err := loadFixture(bad); err == nil {
		t.Error("bad JSON accepted")
	}
	fx, err := loadFixture(writeFixture(t, func(d map[string]any) { delete(d, "server") }))
	if err != nil || fx.Server.PDUSize != 480 {
		t.Errorf("default PDU: %v %v", fx, err)
	}
}

func TestServesTheFixture(t *testing.T) {
	port := freePort(t)
	state := filepath.Join(t.TempDir(), "run", "state.json")
	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan int, 1)
	go func() {
		done <- run(ctx, nil, env(map[string]string{
			"SNAP_INTEROP_FIXTURE":        writeFixture(t, nil),
			"SNAP_INTEROP_LISTEN_ADDRESS": "127.0.0.1",
			"SNAP_INTEROP_PORT":           strconv.Itoa(port),
			"SNAP_INTEROP_STATE_PATH":     state,
		}), logs)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(logs.String(), `"event":"server_ready"`) {
		if time.Now().After(deadline) {
			t.Fatalf("not ready: %s", logs.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	c := client.New("127.0.0.1", client.WithPort(port), client.WithRackSlot(0, 2), client.WithTimeout(2*time.Second))
	if err := c.Connect(ctx); err != nil {
		t.Fatal(err)
	}
	res, err := c.ReadDB(ctx, 1, 0, 8)
	if err != nil || res.Err() != nil || !bytes.Equal(res.Data, []byte{1, 2, 3, 4, 5, 6, 7, 8}) {
		t.Fatalf("ReadDB: %v %v", err, res)
	}
	if err := c.WriteDB(ctx, 1, 0, []byte{0x42}); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteDB(ctx, 2, 0, []byte{0}); err == nil {
		t.Error("write to read-only DB2 accepted")
	}
	_ = c.Close()

	time.Sleep(700 * time.Millisecond) // one snapshot tick
	var doc struct {
		ServerStatus   string   `json:"serverStatus"`
		Implementation identity `json:"implementation"`
		Areas          []struct {
			Type       string `json:"type"`
			DataBase64 string `json:"dataBase64"`
		} `json:"areas"`
	}
	b, err := os.ReadFile(state)
	if err != nil || json.Unmarshal(b, &doc) != nil {
		t.Fatalf("state: %v %s", err, b)
	}
	if doc.ServerStatus != "ready" || doc.Implementation.IndependentEngine || len(doc.Areas) != 5 {
		t.Errorf("state %+v", doc)
	}
	if db1, _ := base64.StdEncoding.DecodeString(doc.Areas[0].DataBase64); len(db1) == 0 || db1[0] != 0x42 {
		t.Errorf("snapshot DB1 = %x", db1)
	}

	cancel()
	select {
	case code := <-done:
		if code != exitOK {
			t.Errorf("exit %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("did not stop")
	}
	events := map[string]bool{}
	sc := bufio.NewScanner(strings.NewReader(logs.String()))
	for sc.Scan() {
		var ev map[string]any
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("log line not JSON: %s", sc.Text())
		}
		if ev["implementation"] != "otfabric" || ev["fixture_id"] == nil || ev["timestamp"] == nil {
			t.Errorf("log fields: %v", ev)
		}
		events[ev["event"].(string)] = true
	}
	for _, e := range []string{"fixture_loaded", "server_starting", "server_ready", "server_stopping", "server_stopped"} {
		if !events[e] {
			t.Errorf("no %s event", e)
		}
	}
}
