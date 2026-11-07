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
	for id, j := range g.jobs {
		select {
		case j.done <- Result{Error: "gateway shutting down; verify state before retrying"}:
		default:
		}
		delete(g.jobs, id)
	}
	for _, cancel := range g.mcpRequests {
		cancel()
	}
	clear(g.leases)
	return g.audit.Close()
}
