package model

import (
	"fmt"
	"testing"
	"time"
)

func TestReplacer(t *testing.T) {
	d := map[string]interface{}{
		"duration": time.Duration(2000),
		"nested": map[string]interface{}{
			"duration": time.Duration(3000),
		},
	}

	newD := replacerBeforeEncode(d)
	fmt.Println(newD)
}
