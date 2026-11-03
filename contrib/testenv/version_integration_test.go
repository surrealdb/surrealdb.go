package testenv

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetVersion_Integration(t *testing.T) {
	tests := []struct {
		dockerTag      string
		expectedMajor  int
		expectedMinor  int
		expectedPatch  int
		expectedPrerel string
	}{
		{"v4.0.0-beta.1", 4, 0, 0, "beta.1"},
	}

	for _, tt := range tests {
		t.Run(tt.dockerTag, func(t *testing.T) {
			// SetupVersionTest skips the test if Docker is not available.
			db, cleanup := SetupVersionTest(t, tt.dockerTag)
			defer cleanup()

			v, err := GetVersion(context.Background(), db)
			require.NoError(t, err)

			require.Equal(t, tt.expectedMajor, v.Major, "Major version mismatch")
			require.Equal(t, tt.expectedMinor, v.Minor, "Minor version mismatch")
			require.Equal(t, tt.expectedPatch, v.Patch, "Patch version mismatch")
			require.Equal(t, tt.expectedPrerel, v.Prerelease, "Prerelease mismatch")
		})
	}
}
