package surrealdump

import "fmt"

// Chain represents a valid sequence of dumps for restoration
type Chain struct {
	FullDump         *Manifest
	IncrementalDumps []*Manifest
	// TotalSize tracks the total bytes of all dumps in the chain.
	// This is intended to be used for reporting.
	TotalSize int64
	// LatestVersionstamp is the end versionstamp of the last dump,
	// which is the max possible restoration point.
	LatestVersionstamp uint64
}

// buildChains builds valid dump chains from a set of manifests.
// This is a private function - callers should use ScanChains instead.
//
// This function assumes manifests are already loaded and does not validate chains.
// Chain validation is performed separately to allow for better error reporting.
func buildChains(manifests []*Manifest) []*Chain {
	var chains []*Chain

	// Group by namespace and database
	groups := make(map[string][]*Manifest)
	for _, m := range manifests {
		key := fmt.Sprintf("%s/%s", m.Namespace, m.Database)
		groups[key] = append(groups[key], m)
	}

	// Build chains for each group
	for _, group := range groups {
		// Find full dumps
		for _, manifest := range group {
			if manifest.Type != ManifestTypeFull {
				continue
			}
			chain := &Chain{
				FullDump:           manifest,
				IncrementalDumps:   []*Manifest{},
				TotalSize:          manifest.Size,
				LatestVersionstamp: manifest.EndVersionstamp,
			}

			// Find compatible incremental dumps
			currentVs := manifest.EndVersionstamp
			usedIncrementals := make(map[*Manifest]bool)

			for {
				found := false
				for _, inc := range group {
					// Skip if already used in this chain
					if usedIncrementals[inc] {
						continue
					}

					// Use CanApplyIncremental to check if this incremental can be applied
					if CanApplyIncremental(currentVs, inc) == nil {
						chain.IncrementalDumps = append(chain.IncrementalDumps, inc)
						chain.TotalSize += inc.Size
						chain.LatestVersionstamp = inc.EndVersionstamp
						currentVs = inc.EndVersionstamp
						usedIncrementals[inc] = true
						found = true
						break
					}
				}
				if !found {
					break
				}
			}

			chains = append(chains, chain)
		}
	}

	return chains
}

// Validate validates that the dump chain is consistent
func (c *Chain) Validate() error {
	if c.FullDump == nil {
		return fmt.Errorf("chain missing full dump")
	}

	expectedVs := c.FullDump.EndVersionstamp

	for i, inc := range c.IncrementalDumps {
		if inc.StartVersionstamp != expectedVs {
			return fmt.Errorf("incremental dump %d has mismatched start versionstamp: expected %d, got %d",
				i, expectedVs, inc.StartVersionstamp)
		}

		// StartVersionstamp should be less than EndVersionstamp for incremental dumps
		if inc.StartVersionstamp >= inc.EndVersionstamp {
			return fmt.Errorf("incremental dump %d has invalid versionstamp range: base %d >= max %d",
				i, inc.StartVersionstamp, inc.EndVersionstamp)
		}

		expectedVs = inc.EndVersionstamp
	}

	return nil
}

// GetRestorationPoints returns end versionstamps associated to available restoration points from a chain
func (c *Chain) GetRestorationPoints() []uint64 {
	var points []uint64

	// Add the full dump point
	points = append(points, c.FullDump.EndVersionstamp)

	// Add each incremental point
	for _, inc := range c.IncrementalDumps {
		points = append(points, inc.EndVersionstamp)
	}

	return points
}

// GetManifestsForVersionstamp returns the dump manifests needed to restore to a specific versionstamp
func (c *Chain) GetManifestsForVersionstamp(targetVs uint64) ([]*Manifest, error) {
	if targetVs < c.FullDump.EndVersionstamp {
		return nil, fmt.Errorf("target versionstamp %d is before the full dump at %d",
			targetVs, c.FullDump.EndVersionstamp)
	}

	dumps := []*Manifest{c.FullDump}

	for _, inc := range c.IncrementalDumps {
		if inc.EndVersionstamp <= targetVs {
			dumps = append(dumps, inc)
		} else {
			break
		}
	}

	return dumps, nil
}
