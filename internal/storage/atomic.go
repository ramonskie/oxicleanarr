package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// writeFileAtomic writes data to path atomically: it writes to a temporary file
// in the same directory, flushes it, then renames it over the target. A crash
// mid-write leaves the previous file intact instead of a truncated one.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if tmpName != "" {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	tmpName = "" // renamed; nothing left to clean up
	return nil
}

// backupCorruptFile renames a corrupt data file aside so its bytes are preserved
// for manual recovery instead of being silently overwritten with empty state.
func backupCorruptFile(path string) (string, error) {
	backup := fmt.Sprintf("%s.corrupt.%d", path, time.Now().UnixNano())
	if err := os.Rename(path, backup); err != nil {
		return "", err
	}
	return backup, nil
}
