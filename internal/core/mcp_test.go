package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func waitJob(t *testing.T, g *Gateway) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		g.mu.Lock()
		for id := range g.jobs {
			g.mu.Unlock()
			return id
		}
		g.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job not dispatched")
	return ""
}

func TestMCPCancellationScopedToAgent(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	otherToken := agentToken(t, admin, []string{id})
	agent, _ := NewClient(admin.URL, token, false)
	other, _ := NewClient(admin.URL, otherToken, false)
	post(t, device, "/poll", map[string]any{})
	if _, err := g.Call(token, "session_acquire", map[string]any{"device_id": id}); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		var result map[string]any
		err := agent.Post(context.Background(), "/mcp", map[string]any{"jsonrpc": "2.0", "id": "action", "method": "tools/call", "params": map[string]any{"name": "system_info", "arguments": map[string]any{"device_id": id}}}, &result)
		if err == nil && !strings.Contains(mustJSON(result), "cancelled") {
			err = io.ErrUnexpectedEOF
		}
		done <- err
	}()
	jid := waitJob(t, g)
	// Delivered work must not be replayed even when the caller cancels.
	job := post(t, device, "/poll", map[string]any{})
	if job["id"] != jid {
		t.Fatal(job)
	}
	notification := map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": map[string]any{"requestId": "action"}}
	post(t, other, "/mcp", notification)
	if !post(t, device, "/check", map[string]any{"id": jid})["active"].(bool) {
		t.Fatal("another agent cancelled job")
	}
	post(t, agent, "/mcp", notification)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation stalled")
	}
	if post(t, device, "/check", map[string]any{"id": jid})["active"].(bool) {
		t.Fatal("cancelled job still active")
	}
	if post(t, device, "/poll", map[string]any{})["id"] != nil {
		t.Fatal("action replayed")
	}
	if err := device.Post(context.Background(), "/result", map[string]any{"id": jid, "result": Result{Data: "late"}}, nil); err == nil {
		t.Fatal("late result accepted")
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestMCPProtocolErrors(t *testing.T) {
	_, admin, _ := fixture(t)
	token := agentToken(t, admin, []string{"*"})
	for _, tc := range []struct {
		body, version string
		status, code  int
	}{
		{`{"jsonrpc":`, "", 400, -32700},
		{`[]`, "", 400, -32600},
		{`{"jsonrpc":"2.0","id":null,"method":"ping"}`, "", 400, -32600},
		{`{"jsonrpc":"2.0","id":1,"method":"ping"} {}`, "", 400, -32700},
		{`{"jsonrpc":"2.0","id":1,"method":"ping"}`, "unsupported", 400, 0},
		{`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"files_read","arguments":{}}}`, "", 200, -32602},
		{`{"jsonrpc":"2.0","id":1,"method":"no-such-method"}`, "", 200, -32601},
	} {
		r, _ := http.NewRequest("POST", admin.URL+"/mcp", strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("MCP-Protocol-Version", tc.version)
		resp, err := admin.HTTP.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		var result struct{ Error struct{ Code int } }
		_ = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		if resp.StatusCode != tc.status || result.Error.Code != tc.code {
			t.Fatalf("%s: status=%d code=%d", tc.body, resp.StatusCode, result.Error.Code)
		}
	}
}

func TestBridgeConcurrentPingAndCancellation(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	agent, _ := NewClient(admin.URL, token, false)
	post(t, device, "/poll", map[string]any{})
	g.Call(token, "session_acquire", map[string]any{"device_id": id})
	in, input := io.Pipe()
	output, out := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer in.Close()
	defer input.Close()
	defer output.Close()
	defer out.Close()
	done := make(chan error, 1)
	go func() { done <- RunMCPBridge(ctx, agent, in, out) }()
	responses := make(chan map[string]any, 4)
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			var m map[string]any
			if json.Unmarshal(scanner.Bytes(), &m) == nil {
				responses <- m
			}
		}
	}()
	write := func(v any) {
		t.Helper()
		if _, err := io.WriteString(input, mustJSON(v)+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	write(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "system_info", "arguments": map[string]any{"device_id": id}}})
	jid := waitJob(t, g)
	write(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "ping"})
	select {
	case r := <-responses:
		if r["id"] != float64(2) || r["result"] == nil {
			t.Fatal(r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ping blocked by running tool")
	}
	write(map[string]any{"jsonrpc": "2.0", "method": "notifications/cancelled", "params": map[string]any{"requestId": 1}})
	select {
	case r := <-responses:
		if r["id"] != float64(1) {
			t.Fatal(r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation blocked")
	}
	input.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("bridge did not exit")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if !post(t, device, "/check", map[string]any{"id": jid})["active"].(bool) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("HTTP disconnect left job active")
}

func TestGatewayCloseCancelsPending(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	post(t, device, "/poll", map[string]any{})
	g.Call(token, "session_acquire", map[string]any{"device_id": id})
	done := make(chan error, 1)
	go func() { _, err := g.Call(token, "system_info", map[string]any{"device_id": id}); done <- err }()
	waitJob(t, g)
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "shutting down") {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel jobs")
	}
	if _, err := g.Call(token, "devices_list", nil); err == nil {
		t.Fatal("closed gateway accepted call")
	}
	var output bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if RunMCPBridge(ctx, admin, strings.NewReader(""), &output) == nil {
		t.Fatal("cancelled bridge succeeded")
	}
}

type blockedOutput struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *blockedOutput) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}

func TestBridgeCancellationWithBlockedOutput(t *testing.T) {
	_, admin, _ := fixture(t)
	token := agentToken(t, admin, []string{"*"})
	agent, _ := NewClient(admin.URL, token, false)
	in, input := io.Pipe()
	defer in.Close()
	defer input.Close()
	output := &blockedOutput{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(output.release)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- RunMCPBridge(ctx, agent, in, output) }()
	io.WriteString(input, `{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n")
	select {
	case <-output.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("no response attempted")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled bridge succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked stdout prevented cancellation")
	}
}

func TestBridgeNotificationDoesNotBlockPing(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RPC
		json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "notifications/initialized" {
			close(entered)
			<-release
			w.WriteHeader(202)
			return
		}
		reply(w, 200, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": map[string]any{}})
	}))
	defer server.Close()
	defer close(release)
	c, _ := NewClient(server.URL, "", false)
	in, input := io.Pipe()
	output, out := io.Pipe()
	defer in.Close()
	defer input.Close()
	defer output.Close()
	defer out.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- RunMCPBridge(ctx, c, in, out) }()
	io.WriteString(input, `{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n")
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("notification not forwarded")
	}
	io.WriteString(input, `{"jsonrpc":"2.0","id":1,"method":"ping"}`+"\n")
	response := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(output)
		if s.Scan() {
			response <- s.Text()
		}
	}()
	select {
	case result := <-response:
		if !strings.Contains(result, `"result"`) {
			t.Fatal(result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notification blocked ping")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("notification blocked shutdown")
	}
}

type observedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedContext) Err() error {
	err := c.Context.Err()
	c.once.Do(func() { close(c.checked) })
	return err
}

func TestCancellationWhileWaitingForGatewayLock(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	token := agentToken(t, admin, []string{id})
	post(t, device, "/poll", map[string]any{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := &observedContext{Context: ctx, checked: make(chan struct{})}
	g.mu.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := g.CallContext(observed, token, "session_acquire", map[string]any{"device_id": id})
		done <- err
	}()
	<-observed.checked
	cancel()
	g.mu.Unlock()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled call stuck")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.leases) != 0 || len(g.jobs) != 0 {
		t.Fatal("cancelled request changed session state")
	}
}
