package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinuxGUIRefusesWaylandAndMissingSession(t *testing.T) {
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("DISPLAY", ":99")
	if _, err := desktop(context.Background(), "desktop_screenshot", nil); err == nil || !strings.Contains(err.Error(), "Wayland") {
		t.Fatalf("expected explicit Wayland refusal, got %v", err)
	}
	t.Setenv("XDG_SESSION_TYPE", "")
	t.Setenv("DISPLAY", "")
	if _, err := desktop(context.Background(), "desktop_move", nil); err == nil {
		t.Fatal("desktop accepted without a logged-in session")
	}
}

func TestLinuxDesktopBackendFailure(t *testing.T) {
	if _, err := desktopRun(context.Background(), exec.CommandContext(context.Background(), "/bin/sh", "-c", "printf 'X display unavailable' >&2; exit 7")); err == nil || !strings.Contains(err.Error(), "X display unavailable") {
		t.Fatalf("backend failure was hidden: %v", err)
	}
}

// Opt-in black-box test: an isolated Xvfb server and a real Tk text widget.
// This exercises X11 input, clipboard and screenshot utilities without touching
// an existing desktop. Run with RELAYDESK_X11_TEST=1 and the documented tools.
func TestLinuxX11BlackBox(t *testing.T) {
	if os.Getenv("RELAYDESK_X11_TEST") != "1" {
		t.Skip("set RELAYDESK_X11_TEST=1; requires Xvfb, xdotool, xclip, ImageMagick import, Python tkinter")
	}
	for _, name := range []string{"Xvfb", "xdotool", "xclip", "import", "python3"} {
		if _, err := exec.LookPath(name); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	x := exec.CommandContext(ctx, "Xvfb", "-displayfd", "3", "-screen", "0", "800x600x24", "-nolisten", "tcp")
	x.ExtraFiles = []*os.File{write}
	var xlog bytes.Buffer
	x.Stderr = &xlog
	if err := x.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = x.Process.Kill(); _ = x.Wait() })
	_ = write.Close()
	display := make(chan string, 1)
	go func() { b, _ := io.ReadAll(read); display <- strings.TrimSpace(string(b)) }()
	select {
	case n := <-display:
		if n == "" {
			t.Fatalf("Xvfb failed to allocate a display: %s", xlog.String())
		}
		t.Setenv("DISPLAY", ":"+n)
	case <-ctx.Done():
		t.Fatal("Xvfb startup timed out")
	}
	t.Setenv("XDG_SESSION_TYPE", "x11")
	state := filepath.Join(t.TempDir(), "widget.json")
	fixture := exec.CommandContext(ctx, "python3", "-c", x11Fixture, state)
	var fixtureLog bytes.Buffer
	fixture.Stderr = &fixtureLog
	if err := fixture.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fixture.Process.Kill(); _ = fixture.Wait() })
	type widgetState struct {
		Text                    string
		Clicks, Scrolls, Chords int
	}
	wait := func(check func(widgetState) bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			b, _ := os.ReadFile(state)
			var s widgetState
			if json.Unmarshal(b, &s) == nil && check(s) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("GUI state check failed: %s; fixture: %s", b, fixtureLog.String())
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
	wait(func(widgetState) bool { return true })
	call := func(action string, args map[string]any) any {
		t.Helper()
		start := time.Now()
		data, err := Execute(ctx, Command{Action: action, Args: args})
		t.Logf("%s completed in %v", action, time.Since(start))
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
		return data
	}
	shot := call("desktop_screenshot", nil).(map[string]any)
	b, err := base64.StdEncoding.DecodeString(shot["base64"].(string))
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil || config.Width != 800 || config.Height != 600 || shot["width"] != 800 || shot["height"] != 600 || shot["origin_x"] != 0 || shot["origin_y"] != 0 {
		t.Fatalf("invalid screenshot metadata: origin=(%v,%v) size=%vx%v; decoded %v; %v", shot["origin_x"], shot["origin_y"], shot["width"], shot["height"], config, err)
	}
	call("desktop_move", map[string]any{"x": float64(120), "y": float64(120)})
	location, err := exec.CommandContext(ctx, "xdotool", "getmouselocation", "--shell").Output()
	if err != nil || !strings.Contains(string(location), "X=120\nY=120\n") {
		t.Fatalf("pointer not moved: %s (%v)", location, err)
	}
	call("desktop_click", map[string]any{"x": float64(120), "y": float64(120)})
	wait(func(s widgetState) bool { return s.Clicks == 1 })
	text := "RelayDesk café 世界 🚀"
	call("desktop_type", map[string]any{"text": text})
	wait(func(s widgetState) bool { return s.Text == text })
	call("desktop_key", map[string]any{"key": "ctrl+l"})
	wait(func(s widgetState) bool { return s.Chords == 1 })
	call("desktop_key", map[string]any{"key": "escape"})
	call("desktop_scroll", map[string]any{"steps": float64(-2)})
	wait(func(s widgetState) bool { return s.Scrolls == 2 })
}

const x11Fixture = `import tkinter as tk
import json, os, sys
root = tk.Tk()
root.geometry("500x300+50+50")
root.title("RelayDesk isolated X11 integration test")
text = tk.Text(root)
text.pack(fill="both", expand=True)
counts = {"Clicks": 0, "Scrolls": 0, "Chords": 0}
def count(key):
    counts[key] += 1
text.bind("<Button-1>", lambda event: count("Clicks"))
text.bind("<Button-4>", lambda event: count("Scrolls"))
text.bind("<Button-5>", lambda event: count("Scrolls"))
text.bind("<MouseWheel>", lambda event: count("Scrolls"))
text.bind("<Control-l>", lambda event: count("Chords"))
root.update()
text.focus_force()
def save():
    data = dict(counts, Text=text.get("1.0", "end-1c"))
    with open(sys.argv[1] + ".tmp", "w") as f:
        json.dump(data, f)
    os.replace(sys.argv[1] + ".tmp", sys.argv[1])
    root.after(20, save)
save()
root.mainloop()
`
