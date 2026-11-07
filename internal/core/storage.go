package core

import (
	"os"
	"path/filepath"
)

// Replace through a private, unpredictable file. WriteFile on an existing path
// preserves its old mode and follows symlinks, which is unsafe for credentials.
func writePrivateFile(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".relaydesk-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
