package core

import "net/http"

// Read-only authentication probe: no heartbeat, dequeue, lease or state write.
func (g *Gateway) diagnose(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		fail(w, 503, "gateway shutting down")
		return
	}
	d := g.device(bearer(r))
	if d == nil {
		g.mu.Unlock()
		fail(w, 401, "device revoked or unknown")
		return
	}
	id, name, paused := d.ID, d.Name, d.Paused
	g.mu.Unlock()
	reply(w, 200, map[string]any{"device_id": id, "device_name": name, "paired": true, "paused": paused})
}
