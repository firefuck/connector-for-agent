package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"
)

type DoctorCheck struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}
type DoctorReport struct {
	Schema   int           `json:"schema_version"`
	OS       string        `json:"os"`
	Checks   []DoctorCheck `json:"checks"`
	ExitCode int           `json:"exit_code"`
}

func Doctor(ctx context.Context, path string) DoctorReport {
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	r := DoctorReport{Schema: 1, OS: runtime.GOOS, Checks: []DoctorCheck{}}
	add := func(name, state, detail string) {
		r.Checks = append(r.Checks, DoctorCheck{name, state, detail})
		if state != "ready" {
			r.ExitCode = 1
		}
	}
	// Never return config contents, raw URLs or network error text to the report.
	cfg, err := ReadConfig(path)
	if err != nil {
		add("pairing", "missing", "Config absent or invalid. Pair this PC using relaydesk pair or the wizard.")
	} else if cfg.Token == "" || cfg.ID == "" {
		add("pairing", "missing", "Config lacks device identity or credential; pair again intentionally.")
	} else {
		add("config", "ready", "Device identity and credential are present; token not displayed.")
		c, e := NewClient(cfg.Gateway, cfg.Token, cfg.Wireguard)
		if e != nil {
			add("gateway", "unsupported", "Invalid gateway URL or HTTP trust policy; use HTTPS or a verified WireGuard route.")
		} else {
			probe, pc := context.WithTimeout(ctx, 5*time.Second)
			var out struct {
				DeviceID string `json:"device_id"`
				Paired   bool   `json:"paired"`
			}
			e = c.Post(probe, "/diagnose", map[string]any{}, &out)
			pc()
			if e != nil {
				var status *HTTPStatusError
				if errors.As(e, &status) && (status.Status == 401 || status.Status == 403) {
					add("pairing", "unsupported", "Gateway rejected the device credential. Check revocation; pair again intentionally if needed.")
				} else {
					add("gateway", "unknown", "Read-only authenticated probe failed. Check DNS/network, TLS trust and gateway version.")
				}
			} else if !out.Paired || out.DeviceID != cfg.ID {
				add("pairing", "unsupported", "Gateway identity does not match the saved configuration.")
			} else {
				add("pairing", "ready", "Gateway authenticated this device without taking jobs or leases.")
			}
		}
	}
	if runtime.GOOS == "linux" {
		if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
			add("session", "ready", "Wayland session selected.")
			c := exec.CommandContext(ctx, "/usr/bin/python3", "-c", linuxHelper, "--probe")
			var output capped
			output.max = 65536
			c.Stdout = &output
			if c.Run() != nil {
				add("portal", "missing", "Install python3-gi, gir1.2-gtk-3.0, gir1.2-gstreamer-1.0, gstreamer1.0-pipewire, gstreamer1.0-plugins-base/good and xdg-desktop-portal-gnome.")
			} else {
				var checks []DoctorCheck
				if json.Unmarshal(output.b, &checks) != nil {
					add("portal", "unknown", "Portal probe failed; verify the desktop session bus.")
				} else {
					for _, check := range checks {
						add(check.Name, check.State, check.Detail)
					}
				}
			}
			add("takeover", "unsupported", "GNOME portal does not expose reliable physical-input detection. Use local Pause or configure the GNOME pause shortcut.")
		} else if os.Getenv("DISPLAY") != "" {
			for _, tool := range []string{"xdotool", "import", "xclip", "xdg-open"} {
				if _, e := exec.LookPath(tool); e != nil {
					add(tool, "missing", "Install the corresponding X11 desktop utility.")
				} else {
					add(tool, "ready", "Executable available; desktop permission not exercised.")
				}
			}
			add("desktop_access", "unknown", "Read-only diagnosis does not capture or send input. Run the manual desktop acceptance test.")
		} else {
			add("desktop", "missing", "Start the connector inside the logged-in graphical user session.")
		}
	} else if runtime.GOOS == "windows" {
		if _, e := exec.LookPath("powershell.exe"); e != nil {
			add("powershell", "missing", "Windows PowerShell is required.")
		} else {
			add("powershell", "ready", "Windows PowerShell executable available.")
		}
		add("desktop_access", "unknown", "Run in the logged-in, unlocked user session. UAC secure desktop and disconnected RDP are not supported.")
	}
	return r
}
func (r DoctorReport) Text() string {
	s := fmt.Sprintf("RelayDesk doctor — %s (schema %d)\n", r.OS, r.Schema)
	for _, c := range r.Checks {
		s += fmt.Sprintf("[%s] %s: %s\n", c.State, c.Name, c.Detail)
	}
	return s
}
