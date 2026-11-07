package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestToolPolicyEnforcementAndDiscovery(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	post(t, device, "/poll", map[string]any{})
	policy := []string{"devices_list", "session_acquire", "session_release", "files_read"}
	a := post(t, admin, "/admin/agent", map[string]any{"devices": []string{id}, "tools": policy})
	token := a["token"].(string)
	tools := g.ToolsForAgent(token)
	if len(tools) != len(policy) {
		t.Fatalf("listed %d tools, want %d", len(tools), len(policy))
	}
	for _, tool := range tools {
		if tool["name"] == "system_exec" {
			t.Fatal("forbidden tool advertised")
		}
	}
	if len(g.ToolsForAgent(Secret())) != 0 {
		t.Fatal("unauthenticated tool discovery")
	}
	if _, e := g.Call(token, "session_acquire", map[string]any{"device_id": id}); e != nil {
		t.Fatal(e)
	}
	if _, e := g.Call(token, "system_exec", map[string]any{"device_id": id, "command": "echo forbidden"}); e == nil || !strings.Contains(e.Error(), "forbidden") {
		t.Fatalf("forbidden tool accepted: %v", e)
	}
	g.mu.Lock()
	n := len(g.jobs)
	g.mu.Unlock()
	if n != 0 {
		t.Fatal("forbidden tool was dispatched")
	}
	if _, e := g.Call(token, "session_release", map[string]any{"device_id": id}); e != nil {
		t.Fatal(e)
	}
}

func TestEmptyAndLegacyToolPolicies(t *testing.T) {
	g, admin, root := fixture(t)
	_, id := pairDevice(t, admin)
	legacy := agentToken(t, admin, []string{id})
	all := post(t, admin, "/admin/agent", map[string]any{"devices": []string{id}, "tools": []string{"*"}})["token"].(string)
	deny := post(t, admin, "/admin/agent", map[string]any{"devices": []string{id}, "tools": []string{}})["token"].(string)
	if len(g.ToolsForAgent(legacy)) != len(Tools()) || len(g.ToolsForAgent(all)) != len(Tools()) || len(g.ToolsForAgent(deny)) != 0 {
		t.Fatal("nil, wildcard, or empty policy semantics incorrect")
	}
	if _, e := g.Call(deny, "devices_list", nil); e == nil {
		t.Fatal("empty tool policy accepted discovery")
	}
	// Simulate a state written before tool policies existed, where tools is absent.
	b, e := os.ReadFile(g.Path)
	if e != nil {
		t.Fatal(e)
	}
	var state map[string]json.RawMessage
	if e = json.Unmarshal(b, &state); e != nil {
		t.Fatal(e)
	}
	var agents map[string]map[string]any
	if e = json.Unmarshal(state["Agents"], &agents); e != nil {
		t.Fatal(e)
	}
	for _, a := range agents {
		if a["key"] == Hash(legacy) {
			delete(a, "tools")
		}
	}
	state["Agents"], _ = json.Marshal(agents)
	b, _ = json.Marshal(state)
	if e = os.WriteFile(g.Path, b, 0600); e != nil {
		t.Fatal(e)
	}
	restored, e := NewGateway(g.Path, root)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.audit.Close()
	if len(restored.ToolsForAgent(legacy)) != len(Tools()) || len(restored.ToolsForAgent(deny)) != 0 {
		t.Fatal("policy changed after state reload")
	}
}

func TestPermissionUpdateCancelsSessionAndPersists(t *testing.T) {
	g, admin, root := fixture(t)
	device, id := pairDevice(t, admin)
	post(t, device, "/poll", map[string]any{})
	a := post(t, admin, "/admin/agent", map[string]any{"devices": []string{id}, "tools": []string{"*"}})
	token := a["token"].(string)
	if _, e := g.Call(token, "session_acquire", map[string]any{"device_id": id}); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() {
		_, e := g.Call(token, "system_exec", map[string]any{"device_id": id, "command": "echo should-not-run"})
		done <- e
	}()
	var jobID string
	deadline := time.Now().Add(time.Second)
	for jobID == "" {
		g.mu.Lock()
		for jid := range g.jobs {
			jobID = jid
		}
		g.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("job not queued")
		}
		time.Sleep(time.Millisecond)
	}
	post(t, admin, "/admin/permissions", map[string]any{"id": a["id"], "tools": []string{}})
	select {
	case e := <-done:
		if e == nil || !strings.Contains(e.Error(), "permissions changed") {
			t.Fatalf("unexpected cancellation: %v", e)
		}
	case <-time.After(time.Second):
		t.Fatal("permission update did not cancel pending action")
	}
	if post(t, device, "/check", map[string]any{"id": jobID})["active"] != false {
		t.Fatal("connector still sees cancelled action")
	}
	if post(t, device, "/poll", map[string]any{})["idle"] != true {
		t.Fatal("cancelled action replayed")
	}
	g.mu.Lock()
	_, leased := g.leases[id]
	g.mu.Unlock()
	if leased {
		t.Fatal("lease survived permission change")
	}
	restored, e := NewGateway(g.Path, root)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.audit.Close()
	if len(restored.ToolsForAgent(token)) != 0 {
		t.Fatal("empty updated policy lost on restart")
	}
}

func TestPermissionValidationAndSaveRollback(t *testing.T) {
	g, admin, _ := fixture(t)
	_, id := pairDevice(t, admin)
	for _, tools := range [][]string{{"typo_exec"}, {""}} {
		if e := admin.Post(context.Background(), "/admin/agent", map[string]any{"devices": []string{id}, "tools": tools}, nil); e == nil {
			t.Fatal("invalid tool permission accepted")
		}
	}
	a := post(t, admin, "/admin/agent", map[string]any{"devices": []string{id}, "tools": []string{}})
	for _, tools := range []any{nil, []string{"unknown"}} {
		if e := admin.Post(context.Background(), "/admin/permissions", map[string]any{"id": a["id"], "tools": tools}, nil); e == nil {
			t.Fatal("invalid permission update accepted")
		}
	}
	g.mu.Lock()
	oldPath := g.Path
	g.Path = filepath.Join(t.TempDir(), "missing-directory", "state.json")
	g.mu.Unlock()
	e := admin.Post(context.Background(), "/admin/permissions", map[string]any{"id": a["id"], "tools": []string{"*"}}, nil)
	g.mu.Lock()
	g.Path = oldPath
	g.mu.Unlock()
	if e == nil {
		t.Fatal("expected save failure")
	}
	if len(g.ToolsForAgent(a["token"].(string))) != 0 {
		t.Fatal("failed save widened permissions in memory")
	}
}
