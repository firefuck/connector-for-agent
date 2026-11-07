package core

// Close stops dispatch, cancels pending work, and closes the audit log. A
// connector discovers cancellation on its next /check; completed effects cannot
// be undone. Call before shutting down the HTTP server.
func (g *Gateway) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil
	}
	g.closed = true
	g.cancelSessionsLocked("", "", "gateway shutting down; verify state before retrying")
	for _, cancel := range g.mcpRequests {
		cancel()
	}
	return g.audit.Close()
}

// cancelSessionsLocked invalidates matching jobs and leases atomically. Empty
// owner/device selects every owner/device; callers must hold g.mu. The same
// cancellation path serves release, policy changes, revoke and shutdown.
func (g *Gateway) cancelSessionsLocked(owner, device, reason string) {
	for id, j := range g.jobs {
		if (owner == "" || j.owner == owner) && (device == "" || j.device == device) {
			select {
			case j.done <- Result{Error: reason}:
			default:
			}
			delete(g.jobs, id)
		}
	}
	for id, l := range g.leases {
		if (owner == "" || l.Owner == owner) && (device == "" || id == device) {
			delete(g.leases, id)
		}
	}
}
