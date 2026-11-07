package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type observedBody struct{ read bool }

func (b *observedBody) Read([]byte) (int, error) { b.read = true; return 0, io.EOF }
func (*observedBody) Close() error               { return nil }

func TestConnectorAuthenticatesBeforeReadingBody(t *testing.T) {
	g, _, _ := fixture(t)
	for _, path := range []string{"/poll", "/check", "/result"} {
		body := &observedBody{}
		r := httptest.NewRequest("POST", path, body)
		r.Header.Set("Authorization", "Bearer invalid")
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != 401 || body.read {
			t.Fatalf("%s: code=%d body read=%v", path, w.Code, body.read)
		}
	}
}

func TestJSONBoundariesBeforeMutation(t *testing.T) {
	g, admin, _ := fixture(t)
	device, _ := pairDevice(t, admin)
	for _, tc := range []struct{ path, token, body string }{
		{"/admin/pair", admin.Token, `{"name":"trailing"} {}`},
		{"/admin/pair", admin.Token, `{"name":"padding"}` + strings.Repeat(" ", 64<<10)},
		{"/pair", "", `{"code":"invalid"} {}`},
		{"/pair", "", `{"code":"` + strings.Repeat("a", 4096) + `"}`},
		{"/poll", device.Token, `{} garbage`},
		{"/result", device.Token, `{} {}`},
	} {
		r := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
		r.Header.Set("Authorization", "Bearer "+tc.token)
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatalf("%s accepted malformed/oversized JSON: %d", tc.path, w.Code)
		}
	}
	if len(g.State.Pairs) != 0 {
		t.Fatal("invalid request mutated pairing state")
	}
}

func TestNewAgentsRequireExplicitPolicy(t *testing.T) {
	g, admin, _ := fixture(t)
	for _, input := range []map[string]any{{"devices": []string{"*"}}, {"devices": []string{"*"}, "tools": nil}} {
		if admin.Post(context.Background(), "/admin/agent", input, nil) == nil {
			t.Fatal("implicit full access accepted")
		}
	}
	if len(g.State.Agents) != 0 {
		t.Fatal("failed request created agent")
	}
	token := post(t, admin, "/admin/agent", map[string]any{"devices": []string{"*"}, "tools": []string{}})["token"].(string)
	if len(g.ToolsForAgent(token)) != 0 {
		t.Fatal("empty policy widened")
	}
}

type slowResponse struct {
	header          http.Header
	started, resume chan struct{}
	once            sync.Once
}

func (w *slowResponse) Header() http.Header { return w.header }
func (*slowResponse) WriteHeader(int)       {}
func (w *slowResponse) Write(b []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	<-w.resume
	return len(b), nil
}

func TestSlowResponsesDoNotHoldGatewayLock(t *testing.T) {
	for _, path := range []string{"/admin/list", "/pair", "/poll"} {
		t.Run(path, func(t *testing.T) {
			g, admin, _ := fixture(t)
			device, _ := pairDevice(t, admin)
			body, token := `{}`, admin.Token
			if path == "/poll" {
				token = device.Token
			}
			if path == "/pair" {
				p := post(t, admin, "/admin/pair", map[string]string{"name": "slow"})
				body = mustJSON(map[string]any{"code": p["code"]})
				token = ""
			}
			w := &slowResponse{header: http.Header{}, started: make(chan struct{}), resume: make(chan struct{})}
			var unblock sync.Once
			release := func() { unblock.Do(func() { close(w.resume) }) }
			defer release()
			r := httptest.NewRequest("POST", path, strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+token)
			done := make(chan struct{})
			go func() { g.ServeHTTP(w, r); close(done) }()
			select {
			case <-w.started:
			case <-time.After(time.Second):
				t.Fatal("response never started")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			if err := device.Post(ctx, "/poll", map[string]any{}, nil); err != nil {
				t.Fatalf("slow %s blocked another device: %v", path, err)
			}
			release()
			<-done
		})
	}
}

func TestResultRequiresDeliveryOwnershipAndLiveDeadline(t *testing.T) {
	g, admin, _ := fixture(t)
	device, id := pairDevice(t, admin)
	other, _ := pairDevice(t, admin)
	j := &job{Command: Command{ID: "job", Deadline: time.Now().Add(time.Minute)}, device: id, done: make(chan Result, 1)}
	g.mu.Lock()
	g.jobs[j.ID] = j
	g.mu.Unlock()
	input := map[string]any{"id": j.ID, "result": Result{Data: "ok"}}
	if device.Post(context.Background(), "/result", input, nil) == nil {
		t.Fatal("undelivered result accepted")
	}
	g.mu.Lock()
	j.delivered = true
	g.mu.Unlock()
	if other.Post(context.Background(), "/result", input, nil) == nil {
		t.Fatal("wrong device result accepted")
	}
	g.mu.Lock()
	j.Deadline = time.Now().Add(-time.Second)
	g.mu.Unlock()
	if device.Post(context.Background(), "/result", input, nil) == nil {
		t.Fatal("expired result accepted")
	}
	g.mu.Lock()
	j.Deadline = time.Now().Add(time.Minute)
	g.mu.Unlock()
	post(t, device, "/result", input)
	select {
	case result := <-j.done:
		if result.Data != "ok" {
			t.Fatal(result)
		}
	default:
		t.Fatal("valid result lost")
	}
}

func TestSaveConfigPrivateReplacementAndFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connector.json")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Gateway: "https://example.invalid", Token: "synthetic-token", ID: "pc"}
	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("credential permissions: %o", info.Mode().Perm())
	}
	restored, err := ReadConfig(path)
	if err != nil || restored != cfg {
		t.Fatalf("read: %+v %v", restored, err)
	}
	directory := filepath.Join(t.TempDir(), "directory")
	os.Mkdir(directory, 0700)
	if err := writePrivateFile(directory, []byte("secret")); err == nil {
		t.Fatal("expected rename failure")
	}
	entries, _ := os.ReadDir(filepath.Dir(directory))
	if len(entries) != 1 {
		t.Fatal("temporary credential left behind")
	}
}

func TestStateNullMapsAndInvalidEntries(t *testing.T) {
	for _, tc := range []struct {
		data  string
		valid bool
	}{{`{"Devices":null,"Agents":null,"Pairs":null}`, true}, {`{"Devices":{"pc":null}}`, false}, {`{"Agents":{"a":{"id":"other"}}}`, false}} {
		path := filepath.Join(t.TempDir(), "state.json")
		os.WriteFile(path, []byte(tc.data), 0600)
		g, err := NewGateway(path, Secret())
		if !tc.valid {
			if err == nil {
				g.Close()
				t.Fatal("invalid state accepted")
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if g.State.Devices == nil || g.State.Agents == nil || g.State.Pairs == nil {
			t.Fatal("null maps not initialized")
		}
		g.Close()
	}
}

func TestBrowserGuardsAndSecretRedaction(t *testing.T) {
	g, admin, _ := fixture(t)
	device, _ := pairDevice(t, admin)
	agent := post(t, admin, "/admin/agent", map[string]any{"devices": []string{"*"}, "tools": []string{"*"}})
	r := httptest.NewRequest("POST", "/admin/list", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+admin.Token)
	w := httptest.NewRecorder()
	g.ServeHTTP(w, r)
	for _, secret := range []string{admin.Token, device.Token, agent["token"].(string), Hash(device.Token), Hash(agent["token"].(string))} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("credential or hash in admin listing")
		}
	}
	for _, origin := range []string{"https://attacker.invalid", "null"} {
		r := httptest.NewRequest("POST", "/admin/list", strings.NewReader(`{}`))
		r.Header.Set("Origin", origin)
		r.Header.Set("Authorization", "Bearer "+admin.Token)
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("cross-origin accepted")
		}
	}
	for _, path := range []string{"/", "/dashboard.js", "/dashboard.css"} {
		w := httptest.NewRecorder()
		g.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
			t.Fatalf("missing CSP %s", path)
		}
	}
}

func TestClientRejectsOversizedSuccess(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(bytes.Repeat([]byte(" "), (12<<20)+1)) }))
	defer s.Close()
	c, _ := NewClient(s.URL, "", false)
	if c.Post(context.Background(), "/", map[string]any{}, nil) == nil {
		t.Fatal("oversized response silently truncated")
	}
}

func TestClientRedactsErrorCredentialsAndTerminalControls(t *testing.T) {
	token := Secret()
	for _, raw := range []bool{false, true} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(403)
			if raw {
				io.WriteString(w, token+" raw proxy response")
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"error": "denied " + token + "\x1b[31m\r\n" + strings.Repeat("x", 4096)})
		}))
		c, _ := NewClient(s.URL, token, false)
		err := c.Post(context.Background(), "/", map[string]any{}, nil)
		s.Close()
		if err == nil || strings.Contains(err.Error(), token) || strings.ContainsAny(err.Error(), "\x1b\r\n") || len(err.Error()) > 1100 {
			t.Fatal("unsafe or unbounded gateway error")
		}
	}
}

func TestRegularFileAndBoundedDirectoryOperations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	write := Command{Action: "files_write", Args: map[string]any{"path": path, "base64": "aGVsbG8="}}
	if _, err := Execute(context.Background(), write); err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(context.Background(), write); err == nil {
		t.Fatal("overwrite without permission")
	}
	write.Args["overwrite"] = true
	write.Args["base64"] = "eA=="
	if _, err := Execute(context.Background(), write); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "x" {
		t.Fatal("overwrite did not truncate")
	}
	for _, action := range []string{"files_read", "files_write"} {
		if _, err := Execute(context.Background(), Command{Action: action, Args: map[string]any{"path": dir, "base64": "", "overwrite": true}}); err == nil {
			t.Fatal("directory accepted as file")
		}
	}
	for i := 0; i < 2005; i++ {
		name, _ := json.Marshal(i)
		if err := os.WriteFile(filepath.Join(dir, string(name)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	result, err := Execute(context.Background(), Command{Action: "files_list", Args: map[string]any{"path": dir}})
	if err != nil || len(result.([]map[string]any)) != 2000 {
		t.Fatalf("bounded list: %v", err)
	}
}
