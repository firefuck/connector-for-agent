package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
)

// RunInteractiveConnector keeps control UI and backend in child processes with
// private pipes. No local Resume API or remote Resume MCP tool is provided.
func RunInteractiveConnector(ctx context.Context, c *Client, config string, headless, autostart bool) error {
	marker := config + ".stopped"
	if autostart {
		if _, e := os.Stat(marker); e == nil {
			fmt.Fprintln(os.Stderr, "Locally stopped; start RelayDesk manually to reconnect")
			return nil
		}
	} else if e := os.Remove(marker); e != nil && !os.IsNotExist(e) {
		return e
	}
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	session := NewLocalSession(!headless)
	action := func(op string) {
		if op == "stop" {
			if e := writePrivateFile(marker, []byte("Stopped locally. Explicit start is required.\n")); e != nil {
				fmt.Fprintln(os.Stderr, "Could not save local Stop marker; disable autostart manually")
			}
			session.Change(op)
			stop()
			return
		}
		session.Change(op)
	}
	external := make(chan string, 4)
	localAction := func(op string) {
		action(op)
		select {
		case external <- op:
		default:
		}
	}
	closeControl, e := startLocalControl(ctx, config, localAction, session)
	if e != nil {
		return e
	}
	defer closeControl()
	o := ConnectorOptions{Session: session}
	if !headless {
		helper, e := StartNativeHelper(ctx, action)
		if e != nil {
			return e
		}
		defer helper.Close()
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case op := <-external:
					if !helper.Send(map[string]any{"op": op}) {
						helper.Close()
						return
					}
				}
			}
		}()
		o.Status = helper.Status
		o.Execute = FrameExecutor(session, func(work context.Context, cmd Command) (any, error) {
			if runtime.GOOS == "linux" && os.Getenv("XDG_SESSION_TYPE") == "wayland" && strings.HasPrefix(cmd.Action, "desktop_") {
				args := make(map[string]any, len(cmd.Args))
				for k, v := range cmd.Args {
					if k != "frame_id" {
						args[k] = v
					}
				}
				cmd.Args = args
				return helper.Execute(work, cmd)
			}
			data, err := Execute(work, cmd)
			if cmd.Action == "system_info" && err == nil {
				if info, ok := data.(map[string]any); ok {
					info["capabilities"] = o.Capabilities()
					info["local_control"] = "window"
					info["desktop_coordinates"] = "Read origin, dimensions, frame_id and coordinate_hint from each screenshot. Wayland coordinates are relative to the selected monitor."
				}
			}
			return data, err
		})
		o.Capabilities = func() []string {
			v := Capabilities()
			if runtime.GOOS == "linux" && os.Getenv("XDG_SESSION_TYPE") == "wayland" {
				v = append(v, "desktop_wayland_portal", "local_pause", "local_stop")
			}
			return v
		}
	} else if os.Getenv("XDG_SESSION_TYPE") == "wayland" {
		return errors.New("Wayland desktop requires the local control window; remove --headless")
	}
	return RunConnectorWithOptions(ctx, c, o)
}
