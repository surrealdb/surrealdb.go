package models

import (
	"fmt"
	"testing"
	"time"
)

func TestReplacerBeForeEncode(t *testing.T) {
	d := map[string]any{
		"duration": time.Duration(2000),
		"nested": map[string]any{
			"duration": time.Duration(3000),
		},
	}

	newD := replacerBeforeEncode(d)
	fmt.Println(newD)
}
