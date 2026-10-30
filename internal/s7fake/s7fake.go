// Package s7fake is an in-process S7 server for the harness's own tests: enough
// of ISO-on-TCP, COTP and S7 to serve reads and writes from memory. It is no
// interop evidence, which comes from the Snap7 and python-snap7 servers.
package s7fake

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
)

// Area names one memory area: "db" with a number, or "inputs", "outputs" or
// "markers" with number 0.
type Area struct {
	Type   string
	Number int
}

// Item return codes of the S7 protocol.
const (
	codeOK           = 0xFF
	codeAccessDenied = 0x03
	codeBadAddress   = 0x05
	codeNoObject     = 0x0A
)

// Memory answers COTP and S7 requests from its areas.
type Memory struct {
	mu       sync.Mutex
	areas    map[Area][]byte
	readOnly map[Area]bool
}

// NewMemory returns a Memory holding copies of areas.
func NewMemory(areas map[Area][]byte) *Memory {
	m := &Memory{areas: make(map[Area][]byte, len(areas)), readOnly: map[Area]bool{}}
	for a, b := range areas {
		m.areas[a] = append([]byte(nil), b...)
	}
	return m
}

// SetReadOnly makes writes to a fail with an access error.
func (m *Memory) SetReadOnly(a Area) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readOnly[a] = true
}

// Bytes returns a copy of the current content of a.
func (m *Memory) Bytes(a Area) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.areas[a]...)
}

// Reply answers one TPKT payload. A nil answer closes the connection.
func (m *Memory) Reply(payload []byte) []byte {
	if len(payload) < 2 {
		return nil
	}
	if payload[1] == 0xE0 {
		return []byte{0x06, 0xD0, 0x00, 0x00, 0x00, 0x01, 0x00}
	}
	if len(payload) < 13 || payload[3] != 0x32 {
		return nil
	}
	s7 := payload[3:]
	paramLen := int(binary.BigEndian.Uint16(s7[6:8]))
	dataLen := int(binary.BigEndian.Uint16(s7[8:10]))
	if len(s7) < 10+paramLen+dataLen || paramLen < 1 {
		return nil
	}
	param, data := s7[10:10+paramLen], s7[10+paramLen:10+paramLen+dataLen]
	ref := s7[4:6]
	switch param[0] {
	case 0xF0:
		return ack(ref, append([]byte(nil), param...), nil)
	case 0x04:
		if len(param) < 14 {
			return nil
		}
		return ack(ref, []byte{0x04, 0x01}, m.read(param[2:14]))
	case 0x05:
		if len(param) < 14 || len(data) < 4 {
			return nil
		}
		return ack(ref, []byte{0x05, 0x01}, []byte{m.write(param[2:14], data[4:])})
	default:
		return nil
	}
}

// ack builds a COTP data TPDU holding an S7 ack-data PDU.
func ack(ref, param, data []byte) []byte {
	out := []byte{0x02, 0xF0, 0x80, 0x32, 0x03, 0x00, 0x00, ref[0], ref[1]}
	out = binary.BigEndian.AppendUint16(out, uint16(len(param)))
	out = binary.BigEndian.AppendUint16(out, uint16(len(data)))
	out = append(out, 0x00, 0x00)
	return append(append(out, param...), data...)
}

// item decodes an S7ANY address: the area, the bit offset, the element count
// and whether it addresses a bit.
func item(b []byte) (a Area, bitOffset, count int, bit, ok bool) {
	count = int(binary.BigEndian.Uint16(b[4:6]))
	number := int(binary.BigEndian.Uint16(b[6:8]))
	switch b[8] {
	case 0x81:
		a = Area{Type: "inputs"}
	case 0x82:
		a = Area{Type: "outputs"}
	case 0x83:
		a = Area{Type: "markers"}
	case 0x84:
		a = Area{Type: "db", Number: number}
	default:
		return a, 0, 0, false, false
	}
	bitOffset = int(b[9])<<16 | int(b[10])<<8 | int(b[11])
	return a, bitOffset, count, b[3] == 0x01, true
}

func (m *Memory) read(b []byte) []byte {
	a, bitOffset, count, bit, ok := item(b)
	if !ok {
		return []byte{codeNoObject, 0x00, 0x00, 0x00}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.areas[a]
	if !ok {
		return []byte{codeNoObject, 0x00, 0x00, 0x00}
	}
	start := bitOffset / 8
	if bit {
		if start >= len(mem) {
			return []byte{codeBadAddress, 0x00, 0x00, 0x00}
		}
		return []byte{codeOK, 0x03, 0x00, 0x01, mem[start] >> (bitOffset % 8) & 1}
	}
	if start+count > len(mem) {
		return []byte{codeBadAddress, 0x00, 0x00, 0x00}
	}
	out := []byte{codeOK, 0x04}
	out = binary.BigEndian.AppendUint16(out, uint16(count*8))
	return append(out, mem[start:start+count]...)
}

func (m *Memory) write(b, value []byte) byte {
	a, bitOffset, count, bit, ok := item(b)
	if !ok {
		return codeNoObject
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	mem, ok := m.areas[a]
	switch {
	case !ok:
		return codeNoObject
	case m.readOnly[a]:
		return codeAccessDenied
	}
	start := bitOffset / 8
	if bit {
		if start >= len(mem) || len(value) < 1 {
			return codeBadAddress
		}
		mask := byte(1) << (bitOffset % 8)
		mem[start] &^= mask
		if value[0] != 0 {
			mem[start] |= mask
		}
		return codeOK
	}
	if start+count > len(mem) || len(value) < count {
		return codeBadAddress
	}
	copy(mem[start:], value[:count])
	return codeOK
}

// Server accepts TCP connections and answers each TPKT payload.
type Server struct {
	l       net.Listener
	reply   func([]byte) []byte
	limit   atomic.Int64
	wg      sync.WaitGroup
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	closing bool
}

// Serve starts a server on a loopback port. A nil answer from reply closes the
// connection.
func Serve(reply func(payload []byte) []byte) (*Server, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("s7fake: %w", err)
	}
	s := &Server{l: l, reply: reply, conns: map[net.Conn]struct{}{}}
	s.limit.Store(-1)
	s.wg.Add(1)
	go s.accept()
	return s, nil
}

// Addr returns the host:port the server listens on.
func (s *Server) Addr() string { return s.l.Addr().String() }

// Limit makes the server close every connection after the next n at once.
func (s *Server) Limit(n int) { s.limit.Store(int64(n)) }

// Close stops the server and waits for its goroutines.
func (s *Server) Close() error {
	s.mu.Lock()
	s.closing = true
	for c := range s.conns {
		_ = c.Close()
	}
	s.mu.Unlock()
	err := s.l.Close()
	s.wg.Wait()
	return err
}

func (s *Server) accept() {
	defer s.wg.Done()
	for {
		c, err := s.l.Accept()
		if err != nil {
			return
		}
		if s.limit.Load() == 0 {
			_ = c.Close()
			continue
		}
		s.limit.Add(-1)
		s.mu.Lock()
		if s.closing {
			s.mu.Unlock()
			_ = c.Close()
			continue
		}
		s.conns[c] = struct{}{}
		s.wg.Add(1)
		s.mu.Unlock()
		go s.serve(c)
	}
}

func (s *Server) serve(c net.Conn) {
	defer s.wg.Done()
	defer func() {
		s.mu.Lock()
		delete(s.conns, c)
		s.mu.Unlock()
		_ = c.Close()
	}()
	for {
		hdr := make([]byte, 4)
		if _, err := io.ReadFull(c, hdr); err != nil {
			return
		}
		n := int(binary.BigEndian.Uint16(hdr[2:4]))
		if n < 4 {
			return
		}
		payload := make([]byte, n-4)
		if _, err := io.ReadFull(c, payload); err != nil {
			return
		}
		resp := s.reply(payload)
		if resp == nil {
			return
		}
		out := []byte{0x03, 0x00}
		out = binary.BigEndian.AppendUint16(out, uint16(len(resp)+4))
		if _, err := c.Write(append(out, resp...)); err != nil {
			return
		}
	}
}
