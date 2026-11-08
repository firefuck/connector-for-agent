package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func controlPath(config string) (string, error) {
	path, e := filepath.Abs(config)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256([]byte(path))
	base, e := os.UserCacheDir()
	if e != nil {
		return "", e
	}
	dir := filepath.Join(base, "relaydesk", "control")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	st, e := os.Lstat(dir)
	if e != nil {
		return "", e
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok || sys.Uid != uint32(os.Getuid()) || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return "", errors.New("local control directory must be owned by this user with mode 0700")
	}
	return filepath.Join(dir, hex.EncodeToString(sum[:12])+".sock"), nil
}
func startLocalControl(ctx context.Context, config string, action func(string), session *LocalSession) (func(), error) {
	path, e := controlPath(config)
	if e != nil {
		return nil, e
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("a connector is already running for this config")
	}
	success := false
	defer func() {
		if !success {
			lock.Close()
		}
	}()
	if c, err := net.DialTimeout("unix", path, 200*time.Millisecond); err == nil {
		c.Close()
		return nil, errors.New("a connector is already running for this config")
	}
	if st, err := os.Lstat(path); err == nil {
		if st.Mode()&os.ModeSocket == 0 {
			return nil, errors.New("unexpected file at control socket path")
		}
		if err = os.Remove(path); err != nil {
			return nil, err
		}
	}
	l, e := net.Listen("unix", path)
	if e != nil {
		return nil, e
	}
	if e = os.Chmod(path, 0600); e != nil {
		l.Close()
		return nil, e
	}
	go func() {
		for {
			c, e := l.Accept()
			if e != nil {
				return
			}
			func() {
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(time.Second))
				var v struct {
					Op string `json:"op"`
				}
				if json.NewDecoder(io.LimitReader(c, 4096)).Decode(&v) != nil {
					return
				}
				if v.Op == "stop" {
					_ = json.NewEncoder(c).Encode(map[string]bool{"paused": true, "stopped": true})
					action("stop")
					return
				}
				if v.Op == "pause" {
					action(v.Op)
				} else if v.Op != "status" {
					_ = json.NewEncoder(c).Encode(map[string]string{"error": "only pause, stop and status are available; resume in the local window"})
					return
				}
				_, paused, stopped := session.Snapshot()
				_ = json.NewEncoder(c).Encode(map[string]bool{"paused": paused, "stopped": stopped})
			}()
		}
	}()
	success = true
	return func() { l.Close(); lock.Close() }, nil
}
func LocalControl(config, op string) (any, error) {
	path, e := controlPath(config)
	if e != nil {
		return nil, e
	}
	c, e := net.DialTimeout("unix", path, time.Second)
	if e != nil {
		return nil, errors.New("no running connector for this configuration")
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if e = json.NewEncoder(c).Encode(map[string]string{"op": op}); e != nil {
		return nil, e
	}
	var v map[string]any
	e = json.NewDecoder(io.LimitReader(c, 4096)).Decode(&v)
	if message, ok := v["error"].(string); ok {
		return nil, errors.New(message)
	}
	return v, e
}
