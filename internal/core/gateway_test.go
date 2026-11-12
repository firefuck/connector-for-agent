package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Gateway, *Client, string) {
	t.Helper()
	admin := Secret()
	g, e := NewGateway(filepath.Join(t.TempDir(), "state.json"), admin)
	if e != nil {
		t.Fatal(e)
	}
	s := httptest.NewServer(g)
	t.Cleanup(s.Close)
	t.Cleanup(func() { g.audit.Close() })
	c, _ := NewClient(s.URL, admin, false)
	return g, c, admin
}
func post(t *testing.T, c *Client, p string, in any) map[string]any {
	t.Helper()
	var out map[string]any
	if e := c.Post(context.Background(), p, in, &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func pairDevice(t *testing.T, c *Client) (*Client, string) {
	p := post(t, c, "/admin/pair", map[string]string{"name": "test"})
	dc, _ := NewClient(c.URL, "", false)
	d := post(t, dc, "/pair", map[string]any{"code": p["code"]})
	if e := dc.Post(context.Background(), "/pair", map[string]any{"code": p["code"]}, nil); e == nil {
		t.Fatal("pairing code replay accepted")
	}
	dc.Token = d["token"].(string)
	return dc, d["id"].(string)
}
func agentToken(t *testing.T, c *Client, ds []string) string {
	return post(t, c, "/admin/agent", map[string]any{"name": "agent", "devices": ds, "tools": []string{"*"}})["token"].(string)
}
func TestAuthPairingPersistenceAndACL(t *testing.T) {
	g, admin, root := fixture(t)
	bad, _ := NewClient(admin.URL, Secret(), false)
	if e := bad.Post(context.Background(), "/admin/list", map[string]any{}, nil); e == nil {
		t.Fatal("unauthorized admin")
	}
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	denied := agentToken(t, admin, []string{"other"})
	if _, e := g.Call(denied, "session_acquire", map[string]any{"device_id": id}); e == nil {
		t.Fatal("ACL bypass")
	}
	post(t, device, "/poll", map[string]any{})
	if _, e := g.Call(token, "session_acquire", map[string]any{"device_id": id}); e != nil {
		t.Fatal(e)
	}
	another := agentToken(t, admin, []string{id})
	if _, e := g.Call(another, "session_acquire", map[string]any{"device_id": id}); e == nil {
		t.Fatal("lease collision")
	}
	raw, _ := os.ReadFile(g.Path)
	if strings.Contains(string(raw), token) || strings.Contains(string(raw), device.Token) {
		t.Fatal("plaintext token persisted")
	}
	restored, e := NewGateway(g.Path, root)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.audit.Close()
	if restored.agent(token) == nil || restored.device(device.Token) == nil {
		t.Fatal("identities lost on restart")
	}
	post(t, admin, "/admin/revoke", map[string]string{"kind": "device", "id": id})
	if e := device.Post(context.Background(), "/poll", map[string]any{}, nil); e == nil {
		t.Fatal("revoked device accepted")
	}
}
func TestEndToEndConnectorAndMCP(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go RunConnector(ctx, device)
	deadline := time.Now().Add(5 * time.Second)
	for {
		g.mu.Lock()
		online := !g.State.Devices[id].LastSeen.IsZero()
		g.mu.Unlock()
		if online {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("connector offline")
		}
		time.Sleep(10 * time.Millisecond)
	}
	agent, _ := NewClient(admin.URL, token, false)
	call := func(name string, args map[string]any) map[string]any {
		return post(t, agent, "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	}
	r := post(t, agent, "/mcp", map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	if r["result"] == nil {
		t.Fatal(r)
	}
	a := map[string]any{"device_id": id}
	call("session_acquire", a)
	path := filepath.Join(t.TempDir(), "hello.txt")
	a = map[string]any{"device_id": id, "path": path, "base64": base64.StdEncoding.EncodeToString([]byte("hello 世界"))}
	r = call("files_write", a)
	b, _ := json.Marshal(r)
	if strings.Contains(string(b), "isError") {
		t.Fatal(string(b))
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != "hello 世界" {
		t.Fatal("file write failed")
	}
	r = call("system_exec", map[string]any{"device_id": id, "command": "printf relaydesk-ok"})
	b, _ = json.Marshal(r)
	if !strings.Contains(string(b), "relaydesk-ok") {
		t.Fatal(string(b))
	}
	r = call("files_read", map[string]any{"device_id": id, "path": path})
	b, _ = json.Marshal(r)
	if !strings.Contains(string(b), base64.StdEncoding.EncodeToString(actual)) {
		t.Fatal(string(b))
	}
	call("session_release", map[string]any{"device_id": id})
	if _, e := g.Call(token, "system_info", map[string]any{"device_id": id}); e == nil {
		t.Fatal("control without lease")
	}
}
func TestRevocationCancelsInFlight(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	post(t, device, "/poll", map[string]any{})
	g.Call(token, "session_acquire", map[string]any{"device_id": id})
	done := make(chan error, 1)
	go func() {
		_, e := g.Call(token, "system_exec", map[string]any{"device_id": id, "command": "sleep 100"})
		done <- e
	}()
	deadline := time.Now().Add(time.Second)
	for {
		g.mu.Lock()
		n := len(g.jobs)
		g.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("no job")
		}
		time.Sleep(time.Millisecond)
	}
	post(t, admin, "/admin/revoke", map[string]string{"kind": "device", "id": id})
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("revocation succeeded action")
		}
	case <-time.After(time.Second):
		t.Fatal("revocation did not cancel")
	}
}
func TestRejectInsecureURL(t *testing.T) {
	if _, e := NewClient("http://example.com", "", false); e == nil {
		t.Fatal("insecure URL accepted")
	}
	if _, e := NewClient("http://10.1.1.1:8787", "", true); e != nil {
		t.Fatal(e)
	}
}
func TestTimeoutKillsProcess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	Execute(ctx, Command{Action: "system_exec", Args: map[string]any{"command": "sleep 30"}})
	if time.Since(start) > 3*time.Second {
		t.Fatal("subprocess failed to stop")
	}
}
func TestToolInputValidation(t *testing.T) {
	for _, a := range []map[string]any{{"device_id": "x"}, {"device_id": "x", "x": "bad", "y": float64(1)}, {"device_id": "x", "x": float64(1.5), "y": float64(1)}} {
		if validateTool("desktop_click", a) == nil {
			t.Fatal("bad coordinates accepted")
		}
	}
}

// A lease is granted for five minutes. Running an action must renew it, never
// cut it down to the action timeout plus grace.
func TestActionRenewsLeaseWithoutShorteningIt(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	post(t, device, "/poll", map[string]any{})
	if _, e := g.Call(token, "session_acquire", map[string]any{"device_id": id}); e != nil {
		t.Fatal(e)
	}
	g.mu.Lock()
	granted := g.leases[id]
	g.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		_, e := g.Call(token, "system_info", map[string]any{"device_id": id})
		done <- e
	}()
	var command map[string]any
	for deadline := time.Now().Add(2 * time.Second); command["id"] == nil; {
		if time.Now().After(deadline) {
			t.Fatal("job not delivered")
		}
		command = post(t, device, "/poll", map[string]any{})
	}
	g.mu.Lock()
	during := g.leases[id]
	g.mu.Unlock()
	post(t, device, "/result", map[string]any{"id": command["id"], "result": Result{Data: "ok"}})
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if during.ID != granted.ID || during.Until.Before(granted.Until) {
		t.Fatalf("action shortened the lease: granted until %v, now %v", granted.Until, during.Until)
	}
}

func TestExpiredPairingCodesArePurged(t *testing.T) {
	g, admin, _ := fixture(t)
	g.mu.Lock()
	g.State.Pairs["expired"] = Pair{"old", time.Now().Add(-time.Minute)}
	g.mu.Unlock()
	post(t, admin, "/admin/pair", map[string]string{"name": "new"})
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, kept := g.State.Pairs["expired"]; kept || len(g.State.Pairs) != 1 {
		t.Fatalf("expired pairing code retained: %d codes", len(g.State.Pairs))
	}
}
