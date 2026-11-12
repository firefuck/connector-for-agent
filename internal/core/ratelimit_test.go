package core

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFailedAuthenticationIsRateLimited(t *testing.T) {
	g, admin, _ := fixture(t)
	wrong, _ := NewClient(admin.URL, Secret(), false)
	status := func(c *Client, path string) int {
		var he *HTTPStatusError
		if e := c.Post(context.Background(), path, map[string]any{"code": "wrong"}, nil); errors.As(e, &he) {
			return he.Status
		} else if e != nil {
			t.Fatal(e)
		}
		return 200
	}
	// Every credential type feeds the same per-address budget.
	paths := []string{"/admin/list", "/pair", "/poll", "/diagnose"}
	for i := 0; i < maxAuthFailures; i++ {
		if got := status(wrong, paths[i%len(paths)]); got != 401 && got != 403 {
			t.Fatalf("attempt %d: HTTP %d", i, got)
		}
	}
	if got := status(wrong, "/admin/list"); got != 429 {
		t.Fatalf("guessing continued after the limit: HTTP %d", got)
	}
	// A valid credential must not reveal itself while the address is blocked.
	if got := status(admin, "/admin/list"); got != 429 {
		t.Fatalf("valid credential answered during block: HTTP %d", got)
	}
	g.limiter.mu.Lock()
	for _, f := range g.limiter.clients {
		f.since = time.Now().Add(-authFailureWindow)
	}
	g.limiter.mu.Unlock()
	if got := status(admin, "/admin/list"); got != 200 {
		t.Fatalf("block outlived its window: HTTP %d", got)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	audit, _ := os.ReadFile(g.Path + ".audit.jsonl")
	if strings.Count(string(audit), `"auth_blocked"`) != 1 {
		t.Fatalf("block not audited exactly once: %s", audit)
	}
}

func TestAuthLimiterIsPerClientAndBounded(t *testing.T) {
	var l authLimiter
	now := time.Now()
	for i := 0; i < maxAuthFailures; i++ {
		l.fail("10.0.0.1", now)
	}
	if l.retryAfter("10.0.0.1", now) <= 0 || l.retryAfter("10.0.0.2", now) != 0 {
		t.Fatal("limit is not scoped to the failing address")
	}
	// Successful clients below the limit are never delayed.
	l.fail("10.0.0.2", now)
	if l.retryAfter("10.0.0.2", now) != 0 {
		t.Fatal("single failure blocked a client")
	}
	for i := 0; i < 2*maxTrackedClients; i++ {
		l.fail(Secret(), now)
	}
	if len(l.clients) > maxTrackedClients {
		t.Fatalf("limiter tracked %d addresses", len(l.clients))
	}
}

func TestAuditLogRotatesWithOnePreviousGeneration(t *testing.T) {
	g, _, _ := fixture(t)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.log("first", "admin", "", "")
	g.auditBytes = maxAuditBytes // Pretend the current file is full.
	g.log("second", "admin", "", "")
	g.log("third", "admin", "", "")
	old, e1 := os.ReadFile(g.Path + ".audit.jsonl.1")
	current, e2 := os.ReadFile(g.Path + ".audit.jsonl")
	if e1 != nil || e2 != nil {
		t.Fatal(e1, e2)
	}
	if !strings.Contains(string(old), `"first"`) || strings.Contains(string(current), `"first"`) ||
		!strings.Contains(string(current), `"second"`) || !strings.Contains(string(current), `"third"`) {
		t.Fatalf("unexpected rotation: old=%q current=%q", old, current)
	}
	if info, _ := os.Stat(g.Path + ".audit.jsonl"); info.Mode().Perm() != 0600 || g.auditBytes != info.Size() {
		t.Fatalf("rotated audit log mode %v, tracked %d of %d bytes", info.Mode().Perm(), g.auditBytes, info.Size())
	}
}

func TestHTTPAllowedOnlyForLoopbackWithoutWireGuardFlag(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1:8787", "http://[::1]:8787", "http://localhost:8787"} {
		if _, e := NewClient(u, "", false); e != nil {
			t.Errorf("%s rejected: %v", u, e)
		}
	}
	for _, u := range []string{"http://10.20.0.10:8787", "http://[fd00::1]:8787", "http://127.0.0.1.example.com:8787"} {
		if _, e := NewClient(u, "", false); e == nil {
			t.Errorf("%s accepted without --wireguard-http", u)
		}
	}
}
