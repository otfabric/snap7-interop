// Package s7client is a minimal independent ISO-on-TCP / S7 client (not go-s7comm).
package s7client

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"time"
)

// Outcome is a normalized probe result category.
type Outcome string

const (
	OutcomeSuccess           Outcome = "success"
	OutcomeAddressOutOfRange Outcome = "address-out-of-range"
	OutcomeWriteProtected    Outcome = "write-protected"
	OutcomeUnsupported       Outcome = "unsupported"
	OutcomeTransportFailure  Outcome = "transport-failure"
	OutcomeProtocolFailure   Outcome = "protocol-failure"
)

const (
	transportByte = 0x02
	transportBit  = 0x01
)

// Conn is a connected S7 session.
type Conn struct {
	conn   net.Conn
	pduRef uint16
}

// Dial connects and completes COTP + S7 setup.
func Dial(addr string, rack, slot int, timeout time.Duration) (*Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errTransport(), err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))

	c := &Conn{conn: conn, pduRef: 1}
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

func (c *Conn) Close() error { return c.conn.Close() }

// ReadArea reads bytes from a logical area.
func (c *Conn) ReadArea(areaType string, number, offset, length int) (data []byte, outcome Outcome, err error) {
	return c.read(areaType, number, offset*8, length, transportByte)
}

// ReadBit reads a single bit; returned data is one byte 0x00 or 0x01.
func (c *Conn) ReadBit(areaType string, number, offset, bit int) (data []byte, outcome Outcome, err error) {
	if bit < 0 || bit > 7 {
		return nil, OutcomeUnsupported, fmt.Errorf("bit out of range: %d", bit)
	}
	return c.read(areaType, number, offset*8+bit, 1, transportBit)
}

func (c *Conn) read(areaType string, number, bitOffset, length int, transport byte) ([]byte, Outcome, error) {
	areaCode, ok := areaCode(areaType)
	if !ok {
		return nil, OutcomeUnsupported, fmt.Errorf("unsupported area type %q", areaType)
	}
	item := encodeAnyPointer(areaCode, number, bitOffset, length, transport)
	param := append([]byte{0x04, 0x01}, item...)
	req := append(s7Header(0x01, c.nextRef(), len(param), 0), param...)
	if err := writeCOTPData(c.conn, req); err != nil {
		return nil, OutcomeTransportFailure, err
	}
	resp, err := readCOTPData(c.conn)
	if err != nil {
		return nil, OutcomeTransportFailure, err
	}
	hdrLen, paramLen, dataLen, err := parseS7Lengths(resp)
	if err != nil {
		return nil, OutcomeProtocolFailure, err
	}
	payload := resp[hdrLen+paramLen : hdrLen+paramLen+dataLen]
	if len(payload) < 4 {
		return nil, OutcomeProtocolFailure, fmt.Errorf("missing read data header")
	}
	if payload[0] != 0xFF {
		return nil, mapItemReturnCode(payload[0]), fmt.Errorf("read item return code 0x%02x", payload[0])
	}
	rawLen := int(binary.BigEndian.Uint16(payload[2:4]))
	nbytes := rawLen
	switch payload[1] {
	case 0x03, 0x04, 0x05:
		nbytes = (rawLen + 7) / 8
	}
	if 4+nbytes > len(payload) {
		return nil, OutcomeProtocolFailure, fmt.Errorf("truncated read payload")
	}
	out := make([]byte, nbytes)
	copy(out, payload[4:4+nbytes])
	if transport == transportBit && len(out) > 0 {
		out[0] &= 0x01
		out = out[:1]
	}
	return out, OutcomeSuccess, nil
}

// WriteArea writes bytes to a logical area.
func (c *Conn) WriteArea(areaType string, number, offset int, payload []byte) (outcome Outcome, err error) {
	return c.write(areaType, number, offset*8, len(payload), transportByte, payload, 0x04)
}

// WriteBit writes a single bit (value non-zero sets the bit).
func (c *Conn) WriteBit(areaType string, number, offset, bit int, set bool) (Outcome, error) {
	if bit < 0 || bit > 7 {
		return OutcomeUnsupported, fmt.Errorf("bit out of range: %d", bit)
	}
	val := byte(0)
	if set {
		val = 1
	}
	return c.write(areaType, number, offset*8+bit, 1, transportBit, []byte{val}, 0x03)
}

func (c *Conn) write(areaType string, number, bitOffset, elements int, transport byte, payload []byte, dataTransport byte) (Outcome, error) {
	areaCode, ok := areaCode(areaType)
	if !ok {
		return OutcomeUnsupported, fmt.Errorf("unsupported area type %q", areaType)
	}
	item := encodeAnyPointer(areaCode, number, bitOffset, elements, transport)
	param := append([]byte{0x05, 0x01}, item...)
	bitLen := len(payload) * 8
	if dataTransport == 0x03 {
		bitLen = 1
	}
	data := []byte{0x00, dataTransport, byte(bitLen >> 8), byte(bitLen)}
	data = append(data, payload...)
	req := append(s7Header(0x01, c.nextRef(), len(param), len(data)), param...)
	req = append(req, data...)
	if err := writeCOTPData(c.conn, req); err != nil {
		return OutcomeTransportFailure, err
	}
	resp, err := readCOTPData(c.conn)
	if err != nil {
		return OutcomeTransportFailure, err
	}
	hdrLen, paramLen, dataLen, err := parseS7Lengths(resp)
	if err != nil {
		return OutcomeProtocolFailure, err
	}
	dataPart := resp[hdrLen+paramLen : hdrLen+paramLen+dataLen]
	if len(dataPart) < 1 {
		return OutcomeProtocolFailure, fmt.Errorf("empty write response")
	}
	if dataPart[0] != 0xFF {
		return mapItemReturnCode(dataPart[0]), fmt.Errorf("write item return code 0x%02x", dataPart[0])
	}
	return OutcomeSuccess, nil
}

func mapItemReturnCode(code byte) Outcome {
	switch code {
	case 0x05, 0x0A: // invalid address / object does not exist
		return OutcomeAddressOutOfRange
	case 0x03: // access not allowed
		return OutcomeWriteProtected
	default:
		return OutcomeProtocolFailure
	}
}

func areaCode(areaType string) (byte, bool) {
	switch areaType {
	case "inputs":
		return 0x81, true
	case "outputs":
		return 0x82, true
	case "markers":
		return 0x83, true
	case "db":
		return 0x84, true
	default:
		return 0, false
	}
}

func encodeAnyPointer(areaCode byte, number, bitOffset, length int, transport byte) []byte {
	dbNum := number
	if areaCode != 0x84 {
		dbNum = 0
	}
	return []byte{
		0x12, 0x0A, 0x10, transport,
		byte(length >> 8), byte(length),
		byte(dbNum >> 8), byte(dbNum),
		areaCode,
		byte(bitOffset >> 16),
		byte(bitOffset >> 8),
		byte(bitOffset),
	}
}

func errTransport() error { return fmt.Errorf("%s", OutcomeTransportFailure) }

func (c *Conn) cotpConnect(rack, slot int) error {
	dstTSAP2 := byte((rack << 5) | slot)
	cotp := []byte{
		0x11, 0xE0, 0x00, 0x00, 0x00, 0x01, 0x00,
		0xC1, 0x02, 0x01, 0x00,
		0xC2, 0x02, 0x01, dstTSAP2,
		0xC0, 0x01, 0x0A,
	}
	if err := writeTPKT(c.conn, cotp); err != nil {
		return fmt.Errorf("%s: %w", OutcomeTransportFailure, err)
	}
	payload, err := readTPKT(c.conn)
	if err != nil {
		return fmt.Errorf("%s: %w", OutcomeTransportFailure, err)
	}
	if len(payload) < 2 || payload[1] != 0xD0 {
		return fmt.Errorf("%s: expected COTP CC, got %s", OutcomeProtocolFailure, hex.EncodeToString(payload))
	}
	return nil
}

func (c *Conn) setupComm(pduSize int) error {
	param := []byte{0xF0, 0x00, 0x00, 0x01, 0x00, 0x01, byte(pduSize >> 8), byte(pduSize)}
	req := append(s7Header(0x01, c.nextRef(), len(param), 0), param...)
	if err := writeCOTPData(c.conn, req); err != nil {
		return fmt.Errorf("%s: %w", OutcomeTransportFailure, err)
	}
	resp, err := readCOTPData(c.conn)
	if err != nil {
		return fmt.Errorf("%s: %w", OutcomeTransportFailure, err)
	}
	if len(resp) < 10 || resp[0] != 0x32 {
		return fmt.Errorf("%s: bad setup response: %s", OutcomeProtocolFailure, hex.EncodeToString(resp))
	}
	return nil
}

func (c *Conn) nextRef() uint16 {
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

func parseS7Lengths(resp []byte) (hdrLen, paramLen, dataLen int, err error) {
	if len(resp) < 10 || resp[0] != 0x32 {
		return 0, 0, 0, fmt.Errorf("short/invalid S7 PDU: %s", hex.EncodeToString(resp))
	}
	hdrLen = 10
	switch resp[1] {
	case 0x02, 0x03:
		hdrLen = 12
	}
	if len(resp) < hdrLen {
		return 0, 0, 0, fmt.Errorf("short S7 header")
	}
	paramLen = int(binary.BigEndian.Uint16(resp[6:8]))
	dataLen = int(binary.BigEndian.Uint16(resp[8:10]))
	if hdrLen+paramLen+dataLen > len(resp) {
		return 0, 0, 0, fmt.Errorf("S7 length overrun")
	}
	return hdrLen, paramLen, dataLen, nil
}

func writeTPKT(w io.Writer, payload []byte) error {
	total := len(payload) + 4
	hdr := []byte{0x03, 0x00, byte(total >> 8), byte(total)}
	_, err := w.Write(append(hdr, payload...))
	return err
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
	return writeTPKT(w, append([]byte{0x02, 0xF0, 0x80}, tsdu...))
}

func readCOTPData(r io.Reader) ([]byte, error) {
	payload, err := readTPKT(r)
	if err != nil {
		return nil, err
	}
	if len(payload) < 3 {
		return nil, fmt.Errorf("short COTP DT")
	}
	li := int(payload[0])
	if 1+li > len(payload) {
		return nil, fmt.Errorf("bad COTP LI")
	}
	return payload[1+li:], nil
}
