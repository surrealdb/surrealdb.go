package surrealdb

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const (
	// MinimumServerVersion is the oldest SurrealDB version this SDK supports.
	MinimumServerVersion = "2.1.0"
	// MaximumServerVersion is the first SurrealDB version this SDK does not
	// support. Every version before it, 4.x included, is supported.
	MaximumServerVersion = "5.0.0"
)

// UnsupportedVersionError is returned, when [WithVersionCheck] is on, when
// connecting to a server whose version is outside the range this SDK supports.
// Pass [WithVersionCheck](false) to connect anyway.
type UnsupportedVersionError struct {
	// Version is the version the server reported.
	Version string
	// Minimum is the oldest version supported.
	Minimum string
	// Maximum is the first version not supported.
	Maximum string
}

func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("SurrealDB server version %q is not supported by this SDK (supported: >= %s, < %s); "+
		"to connect anyway, turn the check off with surrealdb.WithVersionCheck(false)",
		e.Version, e.Minimum, e.Maximum)
}

// IsVersionSupported reports whether this SDK supports a SurrealDB server of the
// given version, such as "3.3.4", "surrealdb-2.6.0" or "3.4.0-nightly+20261014".
// Only the major, minor and patch numbers are compared, so a pre-release of a
// supported version is supported.
func IsVersionSupported(version string) bool {
	return IsVersionInRange(version, MinimumServerVersion, MaximumServerVersion)
}

// IsVersionInRange reports whether a SurrealDB version is at least minimum and
// below maximum, such as whether a server has a feature which first came in
// 3.3.0:
//
//	surrealdb.IsVersionInRange(v.Version, "3.3.0", surrealdb.MaximumServerVersion)
//
// Versions are compared as [IsVersionSupported] compares them. A version or
// bound which cannot be read, such as "nonsense", is in no range.
func IsVersionInRange(version, minimum, maximum string) bool {
	v, ok := parseRelease(version)
	if !ok {
		return false
	}
	lo, ok := parseRelease(minimum)
	if !ok {
		return false
	}
	hi, ok := parseRelease(maximum)
	if !ok {
		return false
	}
	return compareRelease(v, lo) >= 0 && compareRelease(v, hi) < 0
}

func parseRelease(version string) ([3]int, bool) {
	var out [3]int
	v := strings.TrimSpace(version)
	v = strings.TrimPrefix(v, "surrealdb-")
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

func compareRelease(a, b [3]int) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

// checkVersion returns an *UnsupportedVersionError if the server's version is
// outside the supported range.
func (db *DB) checkVersion(ctx context.Context) error {
	v, err := db.Version(ctx)
	if err != nil {
		return fmt.Errorf("checking the server version (turn the check off with surrealdb.WithVersionCheck(false)): %w", err)
	}
	if !IsVersionSupported(v.Version) {
		return &UnsupportedVersionError{
			Version: v.Version,
			Minimum: MinimumServerVersion,
			Maximum: MaximumServerVersion,
		}
	}
	return nil
}
