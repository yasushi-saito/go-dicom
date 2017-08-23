package dicom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type Encoder struct {
	err error
	buf *bytes.Buffer
	bo  binary.ByteOrder
}

func NewEncoder(bo binary.ByteOrder) *Encoder {
	return &Encoder{
		err: nil,
		buf: &bytes.Buffer{},
		bo:  bo}
}

func (e *Encoder) SetError(err error) {
	if e.err == nil {
		e.err = err
	}
}

func (e *Encoder) Finish() ([]byte, error) {
	return e.buf.Bytes(), e.err
}

func (e *Encoder) EncodeByte(v byte) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeUInt16(v uint16) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeUInt32(v uint32) {
	binary.Write(e.buf, e.bo, &v)
}

func (e *Encoder) EncodeString(v string) {
	e.buf.Write([]byte(v))
}

// Encode an array of zero bytes.
func (e *Encoder) EncodeZeros(len int) {
	// TODO(saito) reuse the buffer!
	zeros := make([]byte, len)
	e.buf.Write(zeros)
}

// Copy the given data to the output.
func (e *Encoder) EncodeBytes(v []byte) {
	e.buf.Write(v)
}

type Decoder struct {
	in  io.Reader
	err error

	bo       binary.ByteOrder
	implicit bool
	limit    int64

	oldBos       []binary.ByteOrder
	oldImplicits []bool
	oldLimits    []int64

	// Cumulative # bytes read.
	pos int64
	// Max bytes to read. PushLimit() will add a new limit, and PopLimit()
	// will restore the old limit. The newest limit is at the end.
	//
	// INVARIANT: limits[] store values in decreasing order.
	// limits []int64
}

// limit is the maximum number of read from "in". Don't pass just an arbitrary
// large number as the limit. The underlying code assumes that "limit"
// accurately bounds the end of the data.
func NewDecoder(
	in io.Reader,
	limit int64,
	bo binary.ByteOrder,
	implicit bool) *Decoder {
	return &Decoder{
		in:       in,
		err:      nil,
		bo:       bo,
		implicit: implicit,
		pos:      0,
		limit:    limit,
	}
}

func NewBytesDecoder(data []byte, bo binary.ByteOrder, implicit bool) *Decoder {
	return NewDecoder(bytes.NewBuffer(data), int64(len(data)), bo, implicit)
}

func (d *Decoder) SetError(err error) {
	if d.err == nil {

		d.err = err
	}
}

func (d *Decoder) PushTranslationSyntax(bo binary.ByteOrder, implicit bool) {
	d.oldBos = append(d.oldBos, d.bo)
	d.oldImplicits = append(d.oldImplicits, d.implicit)

	d.bo = bo
	d.implicit = implicit
}

func (d *Decoder) PopTranslationSyntax() {
	d.implicit = d.oldImplicits[len(d.oldImplicits)-1]
	d.bo = d.oldBos[len(d.oldBos)-1]

	d.oldImplicits = d.oldImplicits[:len(d.oldImplicits)-1]
	d.oldBos = d.oldBos[:len(d.oldBos)-1]
}

// Temporarily override the end of the buffer.
//
// REQUIRES: limit must be smaller than the current limit
func (d *Decoder) PushLimit(bytes int64) {
	doassert(bytes >= 0)
	d.oldLimits = append(d.oldLimits, d.limit)
	d.limit = d.pos + bytes
}

// Restore the old limit overridden by PushLimit.
func (d *Decoder) PopLimit() {
	d.limit = d.oldLimits[len(d.oldLimits)-1]
	d.oldLimits = d.oldLimits[:len(d.oldLimits)-1]
}

// Pos() returns the cumulative number of bytes read so far.
func (d *Decoder) Pos() int64 { return d.pos }

func (d *Decoder) Error() error { return d.err }

// Finish() must be called after using the decoder. It returns any error
// encountered during decoding.
func (d *Decoder) Finish() error {
	if d.err != nil {
		return d.err
	}
	if d.Len() != 0 {
		return fmt.Errorf("Decoder found junk (%d bytes remaining)", d.Len())
	}
	return nil
}

// io.Reader implementation
func (d *Decoder) Read(p []byte) (int, error) {
	desired := d.Len()
	if desired == 0 {
		if len(p) == 0 {
			return 0, nil
		}
		return 0, io.EOF
	}
	if desired < int64(len(p)) {
		p = p[:desired]
		desired = int64(len(p))
	}
	n, err := d.in.Read(p)
	if err == nil {
		d.pos += int64(n)
	}
	return n, err
}

// Len() returns the number of bytes yet unread.
func (d *Decoder) Len() int64 {
	return d.limit - d.pos
}

// DecodeByte() reads a single byte from the buffer. On EOF, it returns a junk
// value, and sets an error to be returned by Error() or Finish().
func (d *Decoder) DecodeByte() (v byte) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
		return 0
	}
	return v
}

func (d *Decoder) DecodeUInt32() (v uint32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeInt32() (v int32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeUInt16() (v uint16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeInt16() (v int16) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeFloat32() (v float32) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeFloat64() (v float64) {
	err := binary.Read(d, d.bo, &v)
	if err != nil {
		d.err = err
	}
	return v
}

func (d *Decoder) DecodeString(length int) string {
	return string(d.DecodeBytes(length))
}

func (d *Decoder) DecodeBytes(length int) []byte {
	v := make([]byte, length)
	remaining := v
	for len(remaining) > 0 {
		n, err := d.Read(v)
		if err != nil {
			d.err = err
			break
		}
		remaining = remaining[n:]
	}
	//doassert(d.err==nil)
	if len(remaining) > 0 {
		d.err = fmt.Errorf("DecodeBytes: requested %d, remaining %d",
			length, len(remaining))
		panic(d.err) // TODO(saito) remove
	}
	return v
}

func (d *Decoder) Skip(bytes int) {
	junk := make([]byte, bytes)
	n, err := d.Read(junk)
	if err != nil {
		d.err = err
		return
	}
	if n != bytes {
		d.err = fmt.Errorf("Failed to skip %d bytes (read %d bytes instead)", bytes, n)
		return
	}
}
