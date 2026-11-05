package surrealdump

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// replacement is a file written beside the file at a path, which replaces that
// file only once it is committed. A write which fails part way so leaves the
// file it was to replace as it was, rather than truncated: the last good dump
// is kept when a dump breaks off.
type replacement struct {
	file *os.File
	// target is the file replaced: the path given, or the file it links to.
	target string
	// direct is set for a target which is not a regular file, such as a pipe,
	// which is written to as it is.
	direct bool
	done   bool
}

// createReplacement starts the file which replaces path. It gets path's
// permissions when path exists, and otherwise perm, less the umask, as
// os.OpenFile would give a new file.
func createReplacement(path string, perm os.FileMode) (*replacement, error) {
	target := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		target = resolved
	}
	info, err := os.Stat(target)
	switch {
	case err == nil && !info.Mode().IsRegular():
		f, openErr := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if openErr != nil {
			return nil, openErr
		}
		return &replacement{file: f, target: target, direct: true}, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}

	dir, base := filepath.Split(target)
	for range 10 {
		name := filepath.Join(dir, "."+base+"."+rand.Text()+".partial")
		f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info != nil {
			if err := f.Chmod(info.Mode().Perm()); err != nil {
				_ = f.Close()
				_ = os.Remove(name)
				return nil, err
			}
		}
		return &replacement{file: f, target: target}, nil
	}
	return nil, fmt.Errorf("could not create a file beside %s to write to", target)
}

// commit replaces the target with what was written.
func (r *replacement) commit() error {
	if r.direct {
		r.done = true
		return r.file.Close()
	}
	if err := r.file.Sync(); err != nil {
		return err
	}
	if err := r.file.Close(); err != nil {
		return err
	}
	if err := os.Rename(r.file.Name(), r.target); err != nil {
		return err
	}
	r.done = true
	return nil
}

// discard drops what was written, unless it was committed, and leaves the
// target as it was.
func (r *replacement) discard() {
	if r.done {
		return
	}
	_ = r.file.Close()
	if !r.direct {
		_ = os.Remove(r.file.Name())
	}
}

// writeFileReplacing writes data to path by way of a replacement.
func writeFileReplacing(path string, data []byte, perm os.FileMode) error {
	r, err := createReplacement(path, perm)
	if err != nil {
		return err
	}
	defer r.discard()
	if _, err := r.file.Write(data); err != nil {
		return err
	}
	return r.commit()
}
