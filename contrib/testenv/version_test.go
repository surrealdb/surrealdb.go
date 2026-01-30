package testenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		major      int
		minor      int
		patch      int
		prerelease string
		isV3       bool
		thingFn    string
	}{
		{
			name:       "v2.6.0",
			input:      "2.6.0",
			major:      2,
			minor:      6,
			patch:      0,
			prerelease: "",
			isV3:       false,
			thingFn:    "type::thing",
		},
		{
			name:       "v3.0.0-beta.3",
			input:      "3.0.0-beta.3",
			major:      3,
			minor:      0,
			patch:      0,
			prerelease: "beta.3",
			isV3:       true,
			thingFn:    "type::record",
		},
		{
			name:       "v3.0.0-alpha.7",
			input:      "3.0.0-alpha.7",
			major:      3,
			minor:      0,
			patch:      0,
			prerelease: "alpha.7",
			isV3:       true,
			thingFn:    "type::record",
		},
		{
			name:       "surrealdb-2.6.0 (old format)",
			input:      "surrealdb-2.6.0",
			major:      2,
			minor:      6,
			patch:      0,
			prerelease: "",
			isV3:       false,
			thingFn:    "type::thing",
		},
		{
			name:       "v2.3.7",
			input:      "2.3.7",
			major:      2,
			minor:      3,
			patch:      7,
			prerelease: "",
			isV3:       false,
			thingFn:    "type::thing",
		},
		{
			name:       "v3.1.0",
			input:      "3.1.0",
			major:      3,
			minor:      1,
			patch:      0,
			prerelease: "",
			isV3:       true,
			thingFn:    "type::record",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := ParseVersion(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.major, v.Major, "Major version mismatch")
			require.Equal(t, tt.minor, v.Minor, "Minor version mismatch")
			require.Equal(t, tt.patch, v.Patch, "Patch version mismatch")
			require.Equal(t, tt.prerelease, v.Prerelease, "Prerelease mismatch")
			require.Equal(t, tt.isV3, v.IsV3OrLater(), "IsV3OrLater mismatch")
			require.Equal(t, tt.thingFn, v.ThingOrRecordFn(), "ThingOrRecordFn mismatch")
		})
	}
}

func TestParseVersion_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty string",
			input: "",
		},
		{
			name:  "only major",
			input: "3",
		},
		{
			name:  "only major.minor",
			input: "3.0",
		},
		{
			name:  "non-numeric major",
			input: "abc.0.0",
		},
		{
			name:  "non-numeric minor",
			input: "3.abc.0",
		},
		{
			name:  "non-numeric patch",
			input: "3.0.abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseVersion(tt.input)
			require.Error(t, err)
		})
	}
}

func TestSurrealDBVersion_String(t *testing.T) {
	tests := []struct {
		version  SurrealDBVersion
		expected string
	}{
		{
			version:  SurrealDBVersion{Major: 2, Minor: 6, Patch: 0},
			expected: "2.6.0",
		},
		{
			version:  SurrealDBVersion{Major: 3, Minor: 0, Patch: 0, Prerelease: "beta.3"},
			expected: "3.0.0-beta.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			require.Equal(t, tt.expected, tt.version.String())
		})
	}
}
