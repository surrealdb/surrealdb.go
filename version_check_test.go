package surrealdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsVersionSupported(t *testing.T) {
	supported := []string{
		"2.1.0", "surrealdb-2.6.0", "v3.0.1", "3.3.4", "3.4.0-nightly+20261014.abcdef0",
		"4.0.0", "4.0.0-alpha.1", "4.9.99", "3.3",
	}
	unsupported := []string{
		"1.5.6", "2.0.4", "2.0.99", "5.0.0", "5.0.0-alpha.1", "6.1.0",
		"", "nonsense", "3", "3.x.0", "-1.2.3",
	}

	for _, v := range supported {
		assert.True(t, IsVersionSupported(v), v)
	}
	for _, v := range unsupported {
		assert.False(t, IsVersionSupported(v), v)
	}
}
