package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalPauseCancelsAndInvalidatesGeneration(t *testing.T) {
	s := NewLocalSession(false)
	epoch, _, _ := s.Snapshot()
	ctx, cancel, ok := s.Begin(context.Background(), epoch)
	if !ok {
		t.Fatal("active session rejected")
	}
	defer cancel()
	s.Change("pause")
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("Pause did not cancel local work")
	}
	s.Change("resume")
	if _, _, ok = s.Begin(context.Background(), epoch); ok {
		t.Fatal("old queued job survived Resume")
	}
	epoch, _, _ = s.Snapshot()
	_, cancel, ok = s.Begin(context.Background(), epoch)
	if !ok {
		t.Fatal("fresh session refused")
	}
	cancel()
	s.Change("stop")
	s.Change("resume")
	_, paused, stopped := s.Snapshot()
	if !paused || !stopped {
		t.Fatal("Resume overrode Stop")
	}
}
func TestPauseGatewayDropsJobsAndRequiresNewLease(t *testing.T) {
	g, admin, _ := fixture(t)
	defer g.Close()
	device, id := pairDevice(t, admin)
	post(t, device, "/poll", map[string]any{"local_epoch": "initial"})
	token := agentToken(t, admin, []string{id})
	if _, e := g.Call(token, "session_acquire", map[string]any{"device_id": id}); e != nil {
		t.Fatal(e)
	}
	done := make(chan error, 1)
	go func() { _, e := g.Call(token, "system_info", map[string]any{"device_id": id}); done <- e }()
	deadline := time.Now().Add(time.Second)
	for {
		g.mu.Lock()
		n := len(g.jobs)
		g.mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("job not queued")
		}
		time.Sleep(time.Millisecond)
	}
	post(t, device, "/connector-state", map[string]any{"local_epoch": "pause", "paused": true})
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("cancelled call succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("job survived pause")
	}
	if _, e := g.Call(token, "session_acquire", map[string]any{"device_id": id}); e == nil {
		t.Fatal("agent acquired paused device")
	}
	post(t, device, "/poll", map[string]any{"local_epoch": "resume"})
	if _, e := g.Call(token, "system_info", map[string]any{"device_id": id}); e == nil || !strings.Contains(e.Error(), "session_acquire") {
		t.Fatalf("old lease survived: %v", e)
	}
	if post(t, device, "/poll", map[string]any{"local_epoch": "resume"})["idle"] != true {
		t.Fatal("old action replayed")
	}
}
func TestDoctorReadOnlyAndRedacted(t *testing.T) {
	g, admin, _ := fixture(t)
	defer g.Close()
	device, id := pairDevice(t, admin)
	cfg := filepath.Join(t.TempDir(), "config.json")
	if e := SaveConfig(cfg, Config{Gateway: device.URL, Token: device.Token, ID: id}); e != nil {
		t.Fatal(e)
	}
	g.mu.Lock()
	before := g.State.Devices[id].LastSeen
	g.jobs["held"] = &job{Command: Command{ID: "held"}, device: id, done: make(chan Result, 1)}
	g.leases[id] = lease{Owner: "held"}
	g.mu.Unlock()
	report := Doctor(context.Background(), cfg)
	b, _ := json.Marshal(report)
	if strings.Contains(string(b), device.Token) || strings.Contains(string(b), device.URL) {
		t.Fatal("doctor exposed credentials or URL")
	}
	found := false
	for _, c := range report.Checks {
		if c.Name == "pairing" && c.State == "ready" {
			found = true
		}
	}
	if !found {
		t.Fatalf("valid authentication not reported: %s", b)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.State.Devices[id].LastSeen.Equal(before) || g.jobs["held"].delivered || len(g.leases) != 1 {
		t.Fatal("diagnostic probe mutated state")
	}
}
func TestDoctorNeverEchoesMaliciousGatewayError(t *testing.T) {
	secret := "super-private-token-do-not-log"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401); _, _ = w.Write([]byte(secret)) }))
	defer server.Close()
	cfg := filepath.Join(t.TempDir(), "config.json")
	if e := SaveConfig(cfg, Config{Gateway: server.URL, Token: secret, ID: "test"}); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(Doctor(context.Background(), cfg))
	if strings.Contains(string(b), secret) {
		t.Fatal("doctor echoed secret")
	}

}
func TestFrameInvalidation(t *testing.T) {
	s := NewLocalSession(false)
	calls := 0
	run := FrameExecutor(s, func(ctx context.Context, c Command) (any, error) {
		calls++
		if c.Action == "desktop_screenshot" {
			return map[string]any{"width": 800}, nil
		}
		return "ok", nil
	})
	input := Command{Action: "desktop_click", Args: map[string]any{}}
	if _, e := run(context.Background(), input); e == nil {
		t.Fatal("input without frame accepted")
	}
	frame, e := run(context.Background(), Command{Action: "desktop_screenshot"})
	if e != nil {
		t.Fatal(e)
	}
	input.Args["frame_id"] = frame.(map[string]any)["frame_id"]
	if _, e = run(context.Background(), input); e != nil {
		t.Fatal(e)
	}
	if _, e = run(context.Background(), input); e == nil {
		t.Fatal("frame reused")
	}
	run(context.Background(), Command{Action: "desktop_screenshot"})
	s.Change("pause")
	s.Change("resume")
	if _, e = run(context.Background(), input); e == nil {
		t.Fatal("frame survived local session change")
	}
	if calls != 3 {
		t.Fatalf("unexpected execution count %d", calls)
	}
}
func TestReadConfigRejectsOversize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversize")
	if e := os.WriteFile(path, []byte(strings.Repeat("x", maxControlBodyBytes+1)), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := ReadConfig(path); e == nil {
		t.Fatal("oversized config accepted")
	}
}

func TestFrameCannotCrossLeaseOrAgent(t *testing.T) {
	s := NewLocalSession(false)
	run := FrameExecutor(s, func(ctx context.Context, c Command) (any, error) { return map[string]any{}, nil })
	_, err := run(context.Background(), Command{Action: "desktop_screenshot", AgentID: "a", LeaseID: "first"})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []Command{{Action: "desktop_move", AgentID: "a", LeaseID: "second"}, {Action: "desktop_move", AgentID: "b", LeaseID: "first"}} {
		if _, e := run(context.Background(), input); e == nil {
			t.Fatal("frame crossed lease/agent")
		}
	}
}
