package surrealcbor

import (
	"fmt"
	"io"
)

func (d *decoder) peekTag() (uint64, error) {
	// Save position
	savedPos := d.pos
	defer func() { d.pos = savedPos }()

	if d.pos >= len(d.data) {
		return 0, io.EOF
	}

	majorType := d.data[d.pos] >> 5
	if majorType != 6 {
		return 0, fmt.Errorf("not a tag")
	}

	// Read the tag number using the additional info
	additionalInfo := d.data[d.pos] & 0x1f
	d.pos++

	var tagNum uint64
	if additionalInfo < 24 {
		tagNum = uint64(additionalInfo)
	} else {
		var err error
		d.pos-- // Back up since readUint expects to read the header byte
		tagNum, err = d.readUint()
		if err != nil {
			return 0, err
		}
	}

	return tagNum, nil
}
