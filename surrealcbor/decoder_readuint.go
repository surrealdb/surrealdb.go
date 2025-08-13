package surrealcbor

import (
	"encoding/binary"
	"fmt"
	"io"
)

func (d *decoder) readUint() (uint64, error) {
	if d.pos >= len(d.data) {
		return 0, io.EOF
	}

	majorType := d.data[d.pos] >> 5
	additionalInfo := d.data[d.pos] & 0x1f
	d.pos++

	if additionalInfo < 24 {
		return uint64(additionalInfo), nil
	}

	switch additionalInfo {
	case 24:
		if d.pos >= len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		val := uint64(d.data[d.pos])
		d.pos++
		return val, nil
	case 25:
		if d.pos+1 >= len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		val := uint64(binary.BigEndian.Uint16(d.data[d.pos : d.pos+2]))
		d.pos += 2
		return val, nil
	case 26:
		if d.pos+3 >= len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		val := uint64(binary.BigEndian.Uint32(d.data[d.pos : d.pos+4]))
		d.pos += 4
		return val, nil
	case 27:
		if d.pos+7 >= len(d.data) {
			return 0, io.ErrUnexpectedEOF
		}
		val := binary.BigEndian.Uint64(d.data[d.pos : d.pos+8])
		d.pos += 8
		return val, nil
	default:
		return 0, fmt.Errorf("invalid additional info %d for major type %d", additionalInfo, majorType)
	}
}
