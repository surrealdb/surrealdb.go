package surrealcbor

import (
	"fmt"
	"io"
)

// skipCBORItem skips over a complete CBOR item, advancing d.pos to the next item
func (d *decoder) skipCBORItem() error {
	if d.pos >= len(d.data) {
		return io.EOF
	}

	majorType := d.data[d.pos] >> 5
	additionalInfo := d.data[d.pos] & 0x1f

	// Skip the initial byte
	d.pos++

	// Skip additional bytes for the length/value
	switch majorType {
	case 0, 1: // Unsigned/Negative integer
		return d.skipInteger(additionalInfo)
	case 2, 3: // Byte string / Text string
		return d.skipString(additionalInfo)
	case 4: // Array
		return d.skipArray(additionalInfo)
	case 5: // Map
		return d.skipMap(additionalInfo)
	case 6: // Tag
		return d.skipTag(additionalInfo)
	case 7: // Simple/Float
		return d.skipSimpleFloat(additionalInfo)
	default:
		return fmt.Errorf("unknown major type %d", majorType)
	}
}

func (d *decoder) skipInteger(additionalInfo byte) error {
	if additionalInfo < 24 {
		// Value is in additionalInfo, no extra bytes
	} else if additionalInfo == 24 {
		d.pos++ // 1 extra byte
	} else if additionalInfo == 25 {
		d.pos += 2 // 2 extra bytes
	} else if additionalInfo == 26 {
		d.pos += 4 // 4 extra bytes
	} else if additionalInfo == 27 {
		d.pos += 8 // 8 extra bytes
	} else {
		return fmt.Errorf("invalid additional info for integer: %d", additionalInfo)
	}
	if d.pos > len(d.data) {
		return io.EOF
	}
	return nil
}

func (d *decoder) skipString(additionalInfo byte) error {
	length, err := d.getLength(additionalInfo)
	if err != nil {
		return err
	}
	if length < 0 {
		return d.skipIndefiniteItems()
	}
	d.pos += int(length)
	if d.pos > len(d.data) {
		return io.EOF
	}
	return nil
}

func (d *decoder) skipArray(additionalInfo byte) error {
	count, err := d.getLength(additionalInfo)
	if err != nil {
		return err
	}
	if count < 0 {
		return d.skipIndefiniteItems()
	}
	for i := int64(0); i < count; i++ {
		if err := d.skipCBORItem(); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoder) skipMap(additionalInfo byte) error {
	count, err := d.getLength(additionalInfo)
	if err != nil {
		return err
	}
	if count < 0 {
		return d.skipIndefiniteMapItems()
	}
	for i := int64(0); i < count; i++ {
		// Skip key
		if err := d.skipCBORItem(); err != nil {
			return err
		}
		// Skip value
		if err := d.skipCBORItem(); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoder) skipTag(additionalInfo byte) error {
	// Skip tag number bytes
	if err := d.skipInteger(additionalInfo); err != nil {
		return err
	}
	// Skip the tagged item
	return d.skipCBORItem()
}

func (d *decoder) skipSimpleFloat(additionalInfo byte) error {
	if additionalInfo < 20 {
		// Simple value, no extra bytes
	} else if additionalInfo <= 23 {
		// False/True/Null/Undefined, no extra bytes
	} else if additionalInfo == 24 {
		d.pos++ // 1 extra byte for simple value
	} else if additionalInfo == 25 {
		d.pos += 2 // Half-precision float
	} else if additionalInfo == 26 {
		d.pos += 4 // Single-precision float
	} else if additionalInfo == 27 {
		d.pos += 8 // Double-precision float
	} else if additionalInfo == 31 {
		return fmt.Errorf("unexpected break marker")
	} else {
		return fmt.Errorf("invalid additional info for simple/float: %d", additionalInfo)
	}
	if d.pos > len(d.data) {
		return io.EOF
	}
	return nil
}

func (d *decoder) skipIndefiniteItems() error {
	for {
		if d.pos >= len(d.data) {
			return io.EOF
		}
		if d.data[d.pos] == 0xff { // break marker
			d.pos++
			break
		}
		if err := d.skipCBORItem(); err != nil {
			return err
		}
	}
	return nil
}

func (d *decoder) skipIndefiniteMapItems() error {
	for {
		if d.pos >= len(d.data) {
			return io.EOF
		}
		if d.data[d.pos] == 0xff { // break marker
			d.pos++
			break
		}
		// Skip key
		if err := d.skipCBORItem(); err != nil {
			return err
		}
		// Skip value
		if err := d.skipCBORItem(); err != nil {
			return err
		}
	}
	return nil
}

// getLength extracts the length from additional info, handling extended lengths
// Returns -1 for indefinite length
func (d *decoder) getLength(additionalInfo byte) (int64, error) {
	if additionalInfo < 24 {
		return int64(additionalInfo), nil
	}

	if additionalInfo == 31 {
		return -1, nil // Indefinite length
	}

	// We need to go back one position to read the length properly
	oldPos := d.pos - 1
	d.pos = oldPos
	length, err := d.readUint()
	if err != nil {
		return 0, err
	}
	// Check for overflow when converting uint64 to int64
	const maxInt64 = 1<<63 - 1
	if length > maxInt64 {
		return 0, fmt.Errorf("length overflow: %d", length)
	}
	return int64(length), nil //nolint:gosec
}
