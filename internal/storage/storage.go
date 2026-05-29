// Package storage provides file-backed implementations of the service stores:
// a review store (file-per-app) and an app registry, both persisted with an
// atomic temp-file + rename and rebuilt from disk on boot.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	registryFileName = "apps.json"
	maxFileBytes     = 50 << 20
)

// readJSONFile reads a data file, rejecting anything past maxFileBytes so a
// corrupt or oversized file can't exhaust memory before decoding.
func readJSONFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxFileBytes {
		return nil, fmt.Errorf("%s exceeds the %d-byte limit", filepath.Base(path), maxFileBytes)
	}
	return os.ReadFile(path)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data)
}

// atomicWrite writes data to a temp file in the target's directory, fsyncs it,
// renames it into place, then fsyncs the directory so a reader — or a reboot
// after a crash — sees either the old or new file whole, never a partial one.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return fsyncDir(dir)
}

// fsyncDir flushes a directory entry so a rename survives a crash. A directory
// that can't be opened read-only on some platforms is not fatal to durability.
func fsyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return nil
		}
		return err
	}
	defer d.Close()
	return d.Sync()
}
