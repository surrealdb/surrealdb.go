package surrealdump

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// ManifestType represents the type of a dump manifest
type ManifestType string

// Manifest type constants
const (
	ManifestTypeFull        ManifestType = "full"
	ManifestTypeIncremental ManifestType = "incremental"
)

// Manifest represents metadata about a dump file for chain validation
type Manifest struct {
	// File information
	Filename  string       `json:"filename"`
	Type      ManifestType `json:"type"` // ManifestTypeFull or ManifestTypeIncremental
	CreatedAt time.Time    `json:"created_at"`
	Size      int64        `json:"size"`

	// Database context
	Namespace string `json:"namespace"`
	Database  string `json:"database"`

	// EndVersionstamp designates the ending versionstamp of this dump,
	// which does not necessarily correspond to the latest versionstamp in the database.
	// In case this is a full dump, it should be the versionstamp that is considered larger
	// than any change feed entries' versionstamps.
	// In case this is an incremental dump, it should be the versionstamp that corresponds
	// to the maximum versionstamp of the changes captured in the dump.
	EndVersionstamp uint64 `json:"end_versionstamp"`

	// StartVersionstamp designates the starting versionstamp of this dump,
	// which is 0 for initial full dumps, non-zero for subsequent full dumps,
	// and the previous dump's EndVersionstamp for incremental dumps.
	StartVersionstamp uint64 `json:"start_versionstamp,omitempty"`

	// Checksum for integrity
	SHA256 string `json:"sha256,omitempty"`
}

// Validate validates the manifest fields for consistency and completeness
func (m *Manifest) Validate() error {
	// Validate manifest type
	if m.Type != ManifestTypeFull && m.Type != ManifestTypeIncremental {
		return fmt.Errorf("invalid manifest type: %s", m.Type)
	}

	// Validate required fields
	if m.Namespace == "" {
		return fmt.Errorf("manifest missing namespace")
	}

	if m.Database == "" {
		return fmt.Errorf("manifest missing database")
	}

	// Validate versionstamp fields based on type
	if m.Type == ManifestTypeFull {
		// Full dumps should have StartVersionstamp as 0
		if m.StartVersionstamp != 0 {
			return fmt.Errorf("full manifest should have zero StartVersionstamp")
		}
		// EndVersionstamp of 0 is valid (could be the initial state)
	}

	if m.Type == ManifestTypeIncremental {
		// Incremental dumps must have a StartVersionstamp (where they start from)
		if m.StartVersionstamp == 0 {
			return fmt.Errorf("incremental manifest missing start versionstamp")
		}
		// EndVersionstamp must be greater than StartVersionstamp
		if m.EndVersionstamp <= m.StartVersionstamp {
			return fmt.Errorf("incremental manifest EndVersionstamp must be greater than StartVersionstamp")
		}
	}

	return nil
}

// WriteManifest writes a manifest file alongside the dump
func WriteManifest(dumpPath string, manifest *Manifest) error {
	manifestPath := dumpPath + ".manifest.json"

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return os.WriteFile(manifestPath, data, 0600)
}

// ReadManifest reads a manifest file for a dump
// It returns an error if the manifest doesn't exist (manifests are mandatory)
// or if the manifest data is invalid
func ReadManifest(dumpPath string) (*Manifest, error) {
	manifestPath := dumpPath + ".manifest.json"

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifest not found for %s (manifests are mandatory)", dumpPath)
		}
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	// Validate the manifest
	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return &manifest, nil
}

// CanApplyIncremental checks if an incremental dump can be applied to the current state
func CanApplyIncremental(currentVersionstamp uint64, incrementalManifest *Manifest) error {
	if incrementalManifest.Type != ManifestTypeIncremental {
		return fmt.Errorf("not an incremental dump")
	}

	if incrementalManifest.StartVersionstamp != currentVersionstamp {
		return fmt.Errorf("incremental dump expects start versionstamp %d, but current is %d",
			incrementalManifest.StartVersionstamp, currentVersionstamp)
	}

	return nil
}
