package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const nonblockFileFlag = syscall.O_NONBLOCK

func Capabilities() []string {
	v := []string{"system_exec", "files"}
	if os.Getenv("XDG_SESSION_TYPE") != "wayland" && os.Getenv("DISPLAY") != "" {
		v = append(v, "desktop_x11", "browser_open")
	}
	return v
}
func shell(ctx context.Context, s string) *exec.Cmd {
	return exec.CommandContext(ctx, "/bin/sh", "-c", s)
}
func configureProcess(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
	c.WaitDelay = 2 * time.Second
}
func openURL(ctx context.Context, u string) (any, error) {
	return desktopRun(ctx, exec.CommandContext(ctx, "xdg-open", u))
}

// Desktop operations must surface backend failures as tool errors, rather than
// reporting a successful action containing a nonzero shell exit code.
func desktopRun(ctx context.Context, c *exec.Cmd) (map[string]any, error) {
	result, err := run(ctx, c)
	if err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if result["exit_code"] != 0 {
		return nil, fmt.Errorf("desktop backend failed (exit %v): %s", result["exit_code"], strings.TrimSpace(result["output"].(string)))
	}
	return result, nil
}
func desktop(ctx context.Context, action string, a map[string]any) (any, error) {
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		return nil, errors.New("Wayland GUI control is not implemented yet; shell and file tools remain available")
	}
	if os.Getenv("DISPLAY") == "" {
		return nil, errors.New("connector must run inside the logged-in desktop session")
	}
	switch action {
	case "desktop_screenshot":
		c := exec.CommandContext(ctx, "import", "-window", "root", "png:-")
		var b capped
		b.max = 8 << 20
		c.Stdout = &b
		configureProcess(c)
		if e := c.Run(); e != nil {
			return nil, e
		}
		if b.truncated {
			return nil, errors.New("screenshot exceeds 8 MiB")
		}
		config, err := png.DecodeConfig(bytes.NewReader(b.b))
		if err != nil {
			return nil, fmt.Errorf("invalid screenshot PNG: %w", err)
		}
		return map[string]any{"mimeType": "image/png", "base64": base64.StdEncoding.EncodeToString(b.b), "origin_x": 0, "origin_y": 0, "width": config.Width, "height": config.Height}, nil
	case "desktop_type":
		// Toolkits can request TARGETS and then UTF8_STRING separately. Keep the
		// selection alive through both requests; the deferred cleanup terminates
		// this foreground clipboard owner after the paste has been delivered.
		c := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-quiet", "-loops", "0")
		c.Stdin = strings.NewReader(value(a, "text"))
		if e := c.Start(); e != nil {
			return nil, e
		}
		defer func() {
			if c.Process != nil {
				_ = c.Process.Kill()
			}
			_ = c.Wait()
		}()
		time.Sleep(100 * time.Millisecond)
		result, err := desktopRun(ctx, exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+v"))
		time.Sleep(250 * time.Millisecond)
		return result, err
	case "desktop_key":
		key := strings.ToLower(value(a, "key"))
		aliases := map[string]string{"enter": "Return", "win": "super", "esc": "Escape", "escape": "Escape", "backspace": "BackSpace", "delete": "Delete", "pageup": "Prior", "pagedown": "Next", "left": "Left", "right": "Right", "up": "Up", "down": "Down", "tab": "Tab", "home": "Home", "end": "End"}
		parts := strings.Split(key, "+")
		for i, part := range parts {
			if part == "" || strings.HasPrefix(part, "-") || strings.ContainsAny(part, " \t\r\n") {
				return nil, errors.New("invalid key chord")
			}
			if alias, ok := aliases[part]; ok {
				parts[i] = alias
			}
		}
		return desktopRun(ctx, exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", strings.Join(parts, "+")))
	case "desktop_move", "desktop_click":
		x := strconv.Itoa(integer(a, "x"))
		y := strconv.Itoa(integer(a, "y"))
		// --sync waits for movement and can hang when already at this point.
		// X11 processes move and click requests in order on this connection.
		args := []string{"mousemove", x, y}
		if action == "desktop_click" {
			button := 1
			switch value(a, "button") {
			case "right":
				button = 3
			case "middle":
				button = 2
			case "", "left":
			default:
				return nil, errors.New("unknown button")
			}
			count := integer(a, "count")
			if count == 0 {
				count = 1
			}
			if count < 1 || count > 2 {
				return nil, errors.New("count must be 1 or 2")
			}
			args = append(args, "click", "--repeat", strconv.Itoa(count), "--delay", "100", strconv.Itoa(button))
		}
		return desktopRun(ctx, exec.CommandContext(ctx, "xdotool", args...))
	case "desktop_scroll":
		n := integer(a, "steps")
		if n < -100 || n > 100 {
			return nil, errors.New("steps must be -100..100")
		}
		button := "4"
		if n < 0 {
			n = -n
			button = "5"
		}
		if n == 0 {
			return "ok", nil
		}
		return desktopRun(ctx, exec.CommandContext(ctx, "xdotool", "click", "--repeat", strconv.Itoa(n), button))
	}
	return nil, errors.New("unsupported desktop action")
}
