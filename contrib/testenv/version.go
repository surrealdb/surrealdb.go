package testenv

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	surrealdb "github.com/surrealdb/surrealdb.go"
)

// SurrealDBVersion represents a parsed SurrealDB version.
type SurrealDBVersion struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string // e.g., "alpha.7", "beta.1", etc.
}

// GetVersion retrieves and parses the SurrealDB server version.
func GetVersion(ctx context.Context, db *surrealdb.DB) (*SurrealDBVersion, error) {
	v, err := db.Version(ctx)
	if err != nil {
		return nil, err
	}
	return ParseVersion(v.Version)
}

// ParseVersion parses version strings like "3.0.0-beta.3" or "2.6.0".
// It also handles the older "surrealdb-X.Y.Z" format.
func ParseVersion(version string) (*SurrealDBVersion, error) {
	// Remove "surrealdb-" prefix if present (older format)
	version = strings.TrimPrefix(version, "surrealdb-")

	// Split on hyphen for prerelease
	parts := strings.SplitN(version, "-", 2)
	mainVersion := parts[0]
	prerelease := ""
	if len(parts) > 1 {
		prerelease = parts[1]
	}

	// Parse major.minor.patch
	vparts := strings.Split(mainVersion, ".")
	if len(vparts) < 3 {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	major, err := strconv.Atoi(vparts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", vparts[0])
	}

	minor, err := strconv.Atoi(vparts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", vparts[1])
	}

	patch, err := strconv.Atoi(vparts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", vparts[2])
	}

	return &SurrealDBVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
	}, nil
}

// IsV3OrLater returns true if the version is 3.0.0 or later.
func (v *SurrealDBVersion) IsV3OrLater() bool {
	return v.Major >= 3
}

// ThingOrRecordFn returns "type::thing" for v2.x and "type::record" for v3.x.
// This is useful for composing version-appropriate SurrealQL queries.
func (v *SurrealDBVersion) ThingOrRecordFn() string {
	if v.IsV3OrLater() {
		return "type::record"
	}
	return "type::thing"
}

// String returns the version as a string.
func (v *SurrealDBVersion) String() string {
	if v.Prerelease != "" {
		return fmt.Sprintf("%d.%d.%d-%s", v.Major, v.Minor, v.Patch, v.Prerelease)
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
