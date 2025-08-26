package surrealdump

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ScanChains scans a directory for dump files, builds chains from manifests, and validates them.
// It returns all valid chains found in the directory.
// All chains are validated to ensure they form valid sequences (no gaps in versionstamps).
func ScanChains(dir string) ([]*Chain, error) {
	// Scan directory for manifests
	manifests, err := scanDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(manifests) == 0 {
		// Return empty slice, not an error - caller can decide how to handle
		return []*Chain{}, nil
	}

	// Build chains from manifests
	chains := buildChains(manifests)

	// Always validate all chains to ensure correctness
	for _, chain := range chains {
		if err := chain.Validate(); err != nil {
			return nil, fmt.Errorf("chain validation failed for %s.%s: %w",
				chain.FullDump.Namespace, chain.FullDump.Database, err)
		}
	}

	return chains, nil
}

// scanDirectory scans a directory for dump files and their manifests.
// This is a private function - callers should use ScanChains instead.
//
// Do not use this directly to work with invalid chains.
// If chains are invalid, they should be fixed at the source, not worked around.
// Invalid chains cannot be used for restoration and should not be displayed to users.
func scanDirectory(dir string) ([]*Manifest, error) {
	var manifests []*Manifest

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for .cbor files
		if filepath.Ext(path) == ".cbor" {
			// Try to read the manifest (mandatory)
			manifest, err := ReadManifest(path)
			if err == nil {
				manifests = append(manifests, manifest)
			}
			// Skip dumps without manifests - they are considered invalid
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	// Sort by start versionstamp (full dumps have 0, incrementals have their starting point)
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].StartVersionstamp < manifests[j].StartVersionstamp
	})

	return manifests, nil
}

// findLatestVersionstamp searches for the latest versionstamp from previous dumps
// in the specified directory for the given namespace and database.
func findLatestVersionstamp(dir, namespace, database string) (uint64, error) {
	// Use ScanChains to get all dump chains
	chains, err := ScanChains(dir)
	if err != nil {
		return 0, err
	}

	var latestVs uint64
	for _, chain := range chains {
		// Check if this chain matches our namespace/database
		if chain.FullDump != nil &&
			chain.FullDump.Namespace == namespace &&
			chain.FullDump.Database == database {
			// Get the latest versionstamp from this chain
			points := chain.GetRestorationPoints()
			if len(points) > 0 {
				// The last point is the latest versionstamp
				lastVs := points[len(points)-1]
				if lastVs > latestVs {
					latestVs = lastVs
				}
			}
		}
	}

	return latestVs, nil
}
