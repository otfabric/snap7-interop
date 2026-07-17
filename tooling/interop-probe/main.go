// Minimal independent S7 smoke probe (not go-s7comm).
// Speaks enough ISO-on-TCP / COTP / S7 to read and write DB bytes.
package main

import (
	"encoding/binary"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:1102", "host:port of S7 server")
	rack := flag.Int("rack", 0, "PLC rack")
	slot := flag.Int("slot", 2, "PLC slot")
	db := flag.Int("db", 1, "DB number")
	offset := flag.Int("offset", 0, "byte offset")
	length := flag.Int("length", 8, "read length")
	expectHex := flag.String("expect-hex", "", "expected read bytes as hex (optional)")
	writeHex := flag.String("write-hex", "", "if set, write these bytes then re-read")
	timeout := flag.Duration("timeout", 5*time.Second, "dial/IO timeout")
	flag.Parse()

	c, err := dialS7(*addr, *rack, *slot, *timeout)
	if err != nil {
		fail("connect: %v", err)
	}
	defer c.Close()

	data, err := c.ReadDB(*db, *offset, *length)
	if err != nil {
		fail("read: %v", err)
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
		if err := c.WriteDB(*db, *offset, payload); err != nil {
			fail("write: %v", err)
		}
		fmt.Printf("WRITE db=%d offset=%d hex=%s\n", *db, *offset, hex.EncodeToString(payload))
		got, err := c.ReadDB(*db, *offset, len(payload))
		if err != nil {
			fail("re-read: %v", err)
		}
		if hex.EncodeToString(got) != hex.EncodeToString(payload) {
			fail("write verify mismatch: got %s want %s", hex.EncodeToString(got), hex.EncodeToString(payload))
		}
		fmt.Println("OK write verified")
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
	os.Exit(1)
}

type s7Conn struct {
	conn   net.Conn
	pduRef uint16
}

func dialS7(addr string, rack, slot int, timeout time.Duration) (*s7Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	c := &s7Conn{conn: conn, pduRef: 1}
	if err := c.cotpConnect(rack, slot); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := c.setupComm(480); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

func (c *s7Conn) Close() error { return c.conn.Close() }

func (c *s7Conn) cotpConnect(rack, slot int) error {
	dstTSAP := byte(0x01)
	dstTSAP2 := byte((rack << 5) | slot)
	// COTP CR with TPDU size + TSAPs (src 0x0100, dst rack/slot).
	cotp := []byte{
		0x11,       // LI
		0xE0,       // CR
		0x00, 0x00, // DST-REF
		0x00, 0x01, // SRC-REF
		0x00,       // class
		0xC1, 0x02, 0x01, 0x00,
		0xC2, 0x02, dstTSAP, dstTSAP2,
		0xC0, 0x01, 0x0A, // TPDU size 1024
	}
	if err := writeTPKT(c.conn, cotp); err != nil {
		return err
	}
	payload, err := readTPKT(c.conn)
	if err != nil {
		return err
	}
	if len(payload) < 2 || payload[1] != 0xD0 {
		return fmt.Errorf("expected COTP CC, got %s", hex.EncodeToString(payload))
	}
	return nil
}

func (c *s7Conn) setupComm(pduSize int) error {
	param := []byte{
		0xF0, 0x00,
		0x00, 0x01, // maxAmq calling
		0x00, 0x01, // maxAmq called
		byte(pduSize >> 8), byte(pduSize),
	}
	req := append(s7Header(0x01, c.nextRef(), len(param), 0), param...)
	if err := writeCOTPData(c.conn, req); err != nil {
		return err
	}
	resp, err := readCOTPData(c.conn)
	if err != nil {
		return err
	}
	if len(resp) < 10 || resp[0] != 0x32 {
		return fmt.Errorf("bad setup response: %s", hex.EncodeToString(resp))
	}
	return nil
}

func (c *s7Conn) ReadDB(db, offset, length int) ([]byte, error) {
	// S7 Any-pointer item for DB bytes.
	item := []byte{
		0x12, 0x0A, 0x10, 0x02, // spec, len, syntax, transport=byte
		byte(length >> 8), byte(length),
		byte(db >> 8), byte(db),
		0x84, // area DB
		byte((offset * 8) >> 16),
		byte((offset * 8) >> 8),
		byte(offset * 8),
	}
	param := append([]byte{0x04, 0x01}, item...)
	req := append(s7Header(0x01, c.nextRef(), len(param), 0), param...)
	if err := writeCOTPData(c.conn, req); err != nil {
		return nil, err
	}
	resp, err := readCOTPData(c.conn)
	if err != nil {
		return nil, err
	}
	hdrLen, paramLen, dataLen, err := parseS7Lengths(resp)
	if err != nil {
		return nil, err
	}
	data := resp[hdrLen+paramLen : hdrLen+paramLen+dataLen]
	if len(data) < 4 {
		return nil, fmt.Errorf("missing read data header")
	}
	if data[0] != 0xFF {
		return nil, fmt.Errorf("read item return code 0x%02x (raw=%s)", data[0], hex.EncodeToString(resp))
	}
	// Response transport 0x04 = byte/octet; length is in bits on the wire.
	rawLen := int(binary.BigEndian.Uint16(data[2:4]))
	nbytes := rawLen
	switch data[1] {
	case 0x03, 0x04, 0x05: // bit, byte, int — length in bits
		nbytes = (rawLen + 7) / 8
	}
	if 4+nbytes > len(data) {
		return nil, fmt.Errorf("truncated read payload")
	}
	out := make([]byte, nbytes)
	copy(out, data[4:4+nbytes])
	return out, nil
}

func (c *s7Conn) WriteDB(db, offset int, payload []byte) error {
	item := []byte{
		0x12, 0x0A, 0x10, 0x02,
		byte(len(payload) >> 8), byte(len(payload)),
		byte(db >> 8), byte(db),
		0x84,
		byte((offset * 8) >> 16),
		byte((offset * 8) >> 8),
		byte(offset * 8),
	}
	param := append([]byte{0x05, 0x01}, item...)
	data := []byte{0x00, 0x04, byte((len(payload) * 8) >> 8), byte(len(payload) * 8)}
	data = append(data, payload...)
	req := append(s7Header(0x01, c.nextRef(), len(param), len(data)), param...)
	req = append(req, data...)
	if err := writeCOTPData(c.conn, req); err != nil {
		return err
	}
	resp, err := readCOTPData(c.conn)
	if err != nil {
		return err
	}
	hdrLen, paramLen, dataLen, err := parseS7Lengths(resp)
	if err != nil {
		return err
	}
	dataPart := resp[hdrLen+paramLen : hdrLen+paramLen+dataLen]
	if len(dataPart) < 1 || dataPart[0] != 0xFF {
		return fmt.Errorf("write rejected: %s (raw=%s)", hex.EncodeToString(dataPart), hex.EncodeToString(resp))
	}
	return nil
}

// parseS7Lengths returns header length (10 for Job, 12 for Ack/AckData), param length and data length.
func parseS7Lengths(resp []byte) (hdrLen, paramLen, dataLen int, err error) {
	if len(resp) < 10 || resp[0] != 0x32 {
		return 0, 0, 0, fmt.Errorf("short/invalid S7 PDU: %s", hex.EncodeToString(resp))
	}
	hdrLen = 10
	switch resp[1] {
	case 0x02, 0x03: // Ack / AckData include error class/code
		hdrLen = 12
	}
	if len(resp) < hdrLen {
		return 0, 0, 0, fmt.Errorf("short S7 header")
	}
	paramLen = int(binary.BigEndian.Uint16(resp[6:8]))
	dataLen = int(binary.BigEndian.Uint16(resp[8:10]))
	if hdrLen+paramLen+dataLen > len(resp) {
		return 0, 0, 0, fmt.Errorf("S7 length overrun: hdr=%d param=%d data=%d raw=%d", hdrLen, paramLen, dataLen, len(resp))
	}
	return hdrLen, paramLen, dataLen, nil
}

func (c *s7Conn) nextRef() uint16 {
	c.pduRef++
	return c.pduRef
}

func s7Header(rosctr byte, pduRef uint16, paramLen, dataLen int) []byte {
	h := make([]byte, 10)
	h[0] = 0x32
	h[1] = rosctr
	binary.BigEndian.PutUint16(h[4:6], pduRef)
	binary.BigEndian.PutUint16(h[6:8], uint16(paramLen))
	binary.BigEndian.PutUint16(h[8:10], uint16(dataLen))
	return h
}

func writeTPKT(w io.Writer, payload []byte) error {
	total := len(payload) + 4
	hdr := []byte{0x03, 0x00, byte(total >> 8), byte(total)}
	if _, err := w.Write(append(hdr, payload...)); err != nil {
		return err
	}
	return nil
}

func readTPKT(r io.Reader) ([]byte, error) {
	hdr := make([]byte, 4)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return nil, err
	}
	if hdr[0] != 0x03 {
		return nil, fmt.Errorf("bad TPKT version 0x%02x", hdr[0])
	}
	n := int(binary.BigEndian.Uint16(hdr[2:4]))
	if n < 4 {
		return nil, fmt.Errorf("bad TPKT length %d", n)
	}
	payload := make([]byte, n-4)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func writeCOTPData(w io.Writer, tsdu []byte) error {
	// DT TPDU: LI=2, type=0xF0, EOT=0x80
	payload := append([]byte{0x02, 0xF0, 0x80}, tsdu...)
	return writeTPKT(w, payload)
}

func readCOTPData(r io.Reader) ([]byte, error) {
	payload, err := readTPKT(r)
	if err != nil {
		return nil, err
	}
	if len(payload) < 3 {
		return nil, fmt.Errorf("short COTP DT")
	}
	// LI then type; for DT type high nibble 0xF.
	li := int(payload[0])
	if 1+li > len(payload) {
		return nil, fmt.Errorf("bad COTP LI")
	}
	return payload[1+li:], nil
}
