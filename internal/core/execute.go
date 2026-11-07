package core

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

func value(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func integer(m map[string]any, k string) int  { f, _ := m[k].(float64); return int(f) }

type capped struct {
	mu        sync.Mutex
	b         []byte
	max       int
	truncated bool
}

func (b *capped) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	left := b.max - len(b.b)
	if left < n {
		b.truncated = true
		p = p[:left]
	}
	b.b = append(b.b, p...)
	return n, nil
}
func run(ctx context.Context, c *exec.Cmd) (map[string]any, error) {
	b := &capped{max: maxShellOutputBytes}
	c.Stdout = b
	c.Stderr = b
	configureProcess(c)
	e := c.Run()
	code := 0
	if e != nil {
		var ee *exec.ExitError
		if errors.As(e, &ee) {
			code = ee.ExitCode()
		} else {
			return nil, e
		}
	}
	return map[string]any{"output": string(b.b), "exit_code": code, "truncated": b.truncated, "cancelled": ctx.Err() != nil}, nil
}
func Execute(ctx context.Context, c Command) (any, error) {
	a := c.Args
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	switch c.Action {
	case "system_info":
		host, _ := os.Hostname()
		return map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "hostname": host, "capabilities": Capabilities(), "display": os.Getenv("DISPLAY"), "session_type": os.Getenv("XDG_SESSION_TYPE"), "desktop_coordinates": "Windows: virtual desktop can have negative origin. Linux: X11 root coordinates."}, nil
	case "system_exec":
		command := value(a, "command")
		if strings.TrimSpace(command) == "" {
			return nil, errors.New("command required")
		}
		cmd := shell(ctx, command)
		cmd.Dir = value(a, "cwd")
		return run(ctx, cmd)
	case "files_read":
		f, e := openRegularFile(value(a, "path"), os.O_RDONLY)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
		if len(b) > maxFileBytes {
			return nil, errors.New("file exceeds 4 MiB")
		}
		return map[string]any{"base64": base64.StdEncoding.EncodeToString(b), "bytes": len(b)}, e
	case "files_write":
		b, e := base64.StdEncoding.DecodeString(value(a, "base64"))
		if e != nil {
			return nil, e
		}
		if len(b) > maxFileBytes {
			return nil, errors.New("file exceeds 4 MiB")
		}
		flags := os.O_CREATE | os.O_WRONLY | os.O_EXCL
		if overwrite, _ := a["overwrite"].(bool); overwrite {
			flags = os.O_CREATE | os.O_WRONLY
		}
		f, e := openRegularFile(value(a, "path"), flags)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		if e = f.Truncate(0); e != nil {
			return nil, e
		}
		n, e := f.Write(b)
		ce := f.Close()
		if e == nil {
			e = ce
		}
		return map[string]any{"bytes": n}, e
	case "files_list":
		path := value(a, "path")
		info, e := os.Stat(path)
		if e != nil {
			return nil, e
		}
		if !info.IsDir() {
			return nil, errors.New("directory required")
		}
		f, e := os.OpenFile(path, os.O_RDONLY|nonblockFileFlag, 0)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		ds, e := f.ReadDir(maxDirectoryEntries)
		if e != nil && !errors.Is(e, io.EOF) {
			return nil, e
		}
		out := []map[string]any{}
		for _, d := range ds {
			out = append(out, map[string]any{"name": d.Name(), "directory": d.IsDir()})
		}
		return out, nil
	case "browser_open":
		u, e := url.Parse(value(a, "url"))
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, errors.New("absolute http/https URL required")
		}
		return openURL(ctx, u.String())
	default:
		if strings.HasPrefix(c.Action, "desktop_") {
			return desktop(ctx, c.Action, a)
		}
		return nil, fmt.Errorf("unsupported action: %s", c.Action)
	}
}

func openRegularFile(path string, flags int) (*os.File, error) {
	// Preflight also avoids opening named pipes on Windows. Recheck the handle
	// before reading/truncating; Linux additionally opens nonblocking to close
	// the FIFO replacement race between stat and open.
	info, err := os.Stat(path)
	if err == nil && !info.Mode().IsRegular() {
		return nil, errors.New("only regular files are supported")
	}
	if err != nil && !(os.IsNotExist(err) && flags&os.O_CREATE != 0) {
		return nil, err
	}
	f, err := os.OpenFile(path, flags|nonblockFileFlag, 0600)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("only regular files are supported")
	}
	return f, nil
}
