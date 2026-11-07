package core

import (
	"net/http"
	"time"
)

func (g *Gateway) connector(w http.ResponseWriter, r *http.Request) {
	// Reject unauthenticated uploads before reading or allocating their bodies.
	g.mu.Lock()
	authorized := g.device(bearer(r)) != nil
	g.mu.Unlock()
	if !authorized {
		fail(w, 401, "device revoked or unknown")
		return
	}
	var in struct {
		ID           string   `json:"id"`
		Platform     string   `json:"platform"`
		Capabilities []string `json:"capabilities"`
		Result       Result   `json:"result"`
	}
	if !decode(w, r, &in) {
		return
	}
	g.respondLocked(w, func() (int, any) {
		d := g.device(bearer(r))
		if d == nil {
			return 401, map[string]string{"error": "device revoked or unknown"}
		}
		d.LastSeen = time.Now()
		d.Platform = in.Platform
		d.Capabilities = in.Capabilities
		if r.URL.Path == "/result" {
			j := g.jobs[in.ID]
			if j == nil || j.device != d.ID || !j.delivered || !time.Now().Before(j.Deadline) {
				return 409, map[string]string{"error": "job expired or cancelled"}
			}
			select {
			case j.done <- in.Result:
			default:
			}
			return 200, map[string]bool{"ok": true}
		}
		if r.URL.Path == "/check" {
			j := g.jobs[in.ID]
			active := j != nil && j.device == d.ID && time.Now().Before(j.Deadline)
			return 200, map[string]bool{"active": active}
		}
		for _, j := range g.jobs {
			if j.device == d.ID && !j.delivered && time.Now().Before(j.Deadline) {
				j.delivered = true
				return 200, j.Command
			}
		}
		return 200, map[string]any{"idle": true}
	})
}
