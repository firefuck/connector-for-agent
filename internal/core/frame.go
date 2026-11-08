package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

// FrameExecutor is scoped to one connector. Every attempted input consumes the
// last frame; Pause/Resume changes the generation and invalidates old frames.
func FrameExecutor(session *LocalSession, execute func(context.Context, Command) (any, error)) func(context.Context, Command) (any, error) {
	var id, epoch, owner, leaseID string
	var captured time.Time
	return func(ctx context.Context, c Command) (any, error) {
		generation, _, _ := session.Snapshot()
		if strings.HasPrefix(c.Action, "desktop_") && c.Action != "desktop_screenshot" {
			if id == "" || epoch != generation || owner != c.AgentID || leaseID != c.LeaseID || time.Since(captured) > 30*time.Second {
				return nil, errors.New("take a fresh screenshot before each desktop input")
			}
			if supplied := value(c.Args, "frame_id"); supplied != "" && supplied != id {
				return nil, errors.New("stale frame_id; take a new screenshot")
			}
			id = ""
		}
		result, err := execute(ctx, c)
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if c.Action == "desktop_screenshot" {
			id = ""
			if err == nil {
				if data, ok := result.(map[string]any); ok {
					id = Secret()[:24]
					epoch = generation
					owner = c.AgentID
					leaseID = c.LeaseID
					captured = time.Now()
					if previous, ok := data["session_id"]; ok {
						data["backend_session_id"] = previous
					}
					data["frame_id"], data["captured_at"], data["session_id"], data["lease_id"] = id, captured.UTC().Format(time.RFC3339Nano), generation, c.LeaseID
				}
			}
		}
		return result, err
	}
}
