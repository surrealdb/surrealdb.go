package models

import (
	"fmt"
	"testing"
	"time"
)

func TestReplacerBeForeEncode(t *testing.T) {
	d := map[string]interface{}{
		"duration": time.Duration(2000),
		"nested": map[string]interface{}{
			"duration": time.Duration(3000),
		},
	}

	newD := replacerBeforeEncode(d)
	fmt.Println(newD)
}
