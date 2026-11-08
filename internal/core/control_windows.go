package core

import (
	"context"
	"errors"
	"path/filepath"
	"syscall"
)

// Share mode zero keeps one connector per config across Windows sessions.
func startLocalControl(ctx context.Context, config string, action func(string), session *LocalSession) (func(), error) {
	path, e := filepath.Abs(config + ".lock")
	if e != nil {
		return nil, e
	}
	name, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return nil, e
	}
	handle, e := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if e != nil {
		return nil, errors.New("cannot lock connector configuration; another connector may be running")
	}
	return func() { syscall.CloseHandle(handle) }, nil
}
func LocalControl(config, op string) (any, error) {
	return nil, errors.New("use the local RelayDesk window or Ctrl+Alt+P on Windows")
}
