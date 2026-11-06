package core

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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
	return run(ctx, exec.CommandContext(ctx, "xdg-open", u))
}
func desktop(ctx context.Context, action string, a map[string]any) (any, error) {
	if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		return nil, errors.New("Wayland control is not implemented; use an Xorg/X11 login session")
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
		return map[string]any{"mimeType": "image/png", "base64": base64.StdEncoding.EncodeToString(b.b)}, nil
	case "desktop_type":
		c := exec.CommandContext(ctx, "xclip", "-selection", "clipboard", "-quiet", "-loops", "1")
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
		result, err := run(ctx, exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", "ctrl+v"))
		time.Sleep(250 * time.Millisecond)
		return result, err
	case "desktop_key":
		key := strings.ToLower(value(a, "key"))
		repl := strings.NewReplacer("enter", "Return", "win", "super", "esc", "Escape", "backspace", "BackSpace", "delete", "Delete", "pageup", "Prior", "pagedown", "Next", "left", "Left", "right", "Right", "up", "Up", "down", "Down", "tab", "Tab", "home", "Home", "end", "End")
		return run(ctx, exec.CommandContext(ctx, "xdotool", "key", "--clearmodifiers", repl.Replace(key)))
	case "desktop_move", "desktop_click":
		x := strconv.Itoa(integer(a, "x"))
		y := strconv.Itoa(integer(a, "y"))
		args := []string{"mousemove", "--sync", x, y}
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
		return run(ctx, exec.CommandContext(ctx, "xdotool", args...))
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
		return run(ctx, exec.CommandContext(ctx, "xdotool", "click", "--repeat", strconv.Itoa(n), button))
	}
	return nil, errors.New("unsupported desktop action")
}
