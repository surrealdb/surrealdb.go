package surrealdump

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dumpOf(content string, vs uint64, err error) func(io.Writer) (*uint64, error) {
	return func(w io.Writer) (*uint64, error) {
		if _, werr := w.Write([]byte(content)); werr != nil {
			return nil, werr
		}
		if err != nil {
			return nil, err
		}
		return &vs, nil
	}
}

// A dump which breaks off leaves the dump it was to replace, and its manifest,
// as they were. Full and Incremental used to truncate the file first, so the
// last good dump was lost, while its manifest still vouched for it.
func TestWriteDump_failedDumpKeepsTheLastGoodOne(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "backup.cbor")
	d := &Dumper{namespace: "app", database: "main"}

	require.NoError(t, d.writeDump(path, ManifestTypeFull, 0, dumpOf("a good dump", 7, nil)))
	manifest, err := os.ReadFile(path + ".manifest.json")
	require.NoError(t, err)

	err = d.writeDump(path, ManifestTypeFull, 0, dumpOf("part of a", 0, errors.New("connection lost")))
	require.ErrorContains(t, err, "connection lost")

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "a good dump", string(got))
	again, err := os.ReadFile(path + ".manifest.json")
	require.NoError(t, err)
	assert.Equal(t, string(manifest), string(again))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{"backup.cbor", "backup.cbor.manifest.json"}, names, "nothing is left behind")

	require.NoError(t, d.writeDump(path, ManifestTypeFull, 0, dumpOf("a newer dump", 9, nil)))
	got, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "a newer dump", string(got), "a dump which completes replaces the last one")
	m, err := ReadManifest(path)
	require.NoError(t, err)
	assert.Equal(t, int64(len("a newer dump")), m.Size)
	assert.Equal(t, uint64(9), m.EndVersionstamp)
}

// The new dump keeps the permissions of the one it replaces, and a dump to a
// symbolic link replaces the file the link points to, not the link.
func TestWriteDump_keepsModeAndWritesThroughSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.cbor")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o600))
	require.NoError(t, os.Chmod(target, 0o640)) //nolint:gosec // a mode other than the default, to check it is kept
	link := filepath.Join(dir, "latest.cbor")
	require.NoError(t, os.Symlink(target, link))

	d := &Dumper{namespace: "app", database: "main"}
	require.NoError(t, d.writeDump(link, ManifestTypeFull, 0, dumpOf("new", 1, nil)))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the link is still a link")
	got, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "new", string(got))
	info, err = os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
}
