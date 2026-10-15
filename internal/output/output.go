// Package output guards and writes the output file (R-10, R-19, T-03).
package output

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ErrSameFile is returned when the output path refers to the input file.
var ErrSameFile = errors.New("output path is the same file as the input")

// ErrExists is returned when the output exists and overwriting was not allowed.
var ErrExists = errors.New("output file already exists (use --force to overwrite)")

// Check refuses an output path that resolves to the input file, or that
// already exists when force is false.
func Check(input, out string, force bool) error {
	inReal, err := filepath.EvalSymlinks(input)
	if err != nil {
		return err
	}
	outReal, err := resolve(out)
	if err != nil {
		return err
	}
	if inReal == outReal {
		return ErrSameFile
	}
	inInfo, err := os.Stat(inReal)
	if err != nil {
		return err
	}
	if outInfo, err := os.Stat(outReal); err == nil && os.SameFile(inInfo, outInfo) {
		return ErrSameFile
	}
	if _, err := os.Lstat(out); err == nil {
		if !force {
			return ErrExists
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// resolve returns the real path of p, which need not exist yet.
func resolve(p string) (string, error) {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r, nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(abs))
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(abs)), nil
}

// WriteAtomic writes data to a new temp file next to path, syncs it, and
// renames it over path. On any error the temp file is removed, so no
// partial output is left behind. A symlink at path is replaced, not followed.
func WriteAtomic(path string, data []byte, force bool) (err error) {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Chmod(0o644); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if !force {
		if _, statErr := os.Lstat(path); statErr == nil {
			return ErrExists
		}
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("renaming temp file into place: %w", err)
	}
	return nil
}
