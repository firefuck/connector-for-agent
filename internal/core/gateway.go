package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Device struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Key          string    `json:"key,omitempty"`
	Revoked      bool      `json:"revoked"`
	LastSeen     time.Time `json:"last_seen"`
	Platform     string    `json:"platform"`
	Capabilities []string  `json:"capabilities"`
}
type Agent struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Key     string   `json:"key,omitempty"`
	Devices []string `json:"devices"`
	Tools   []string `json:"tools"` // nil is the legacy unrestricted policy; [] denies all.
	Revoked bool     `json:"revoked"`
}
type Pair struct {
	Name    string
	Expires time.Time
}
type State struct {
	Devices map[string]*Device
	Agents  map[string]*Agent
	Pairs   map[string]Pair
}
type Command struct {
	ID       string         `json:"id"`
	Action   string         `json:"action"`
	Args     map[string]any `json:"args"`
	Deadline time.Time      `json:"deadline"`
}
type Result struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}
type job struct {
	Command
	owner     string
	device    string
	done      chan Result
	delivered bool
}
type lease struct {
	Owner string    `json:"owner"`
	Until time.Time `json:"until"`
}
type Gateway struct {
	mu          sync.Mutex
	State       State
	Path, Admin string
	jobs        map[string]*job
	leases      map[string]lease
	audit       *os.File
	closed      bool
	mcpRequests map[string]context.CancelFunc
}

func Secret() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func NewGateway(path, admin string) (*Gateway, error) {
	if len(admin) < 32 {
		return nil, errors.New("RELAYDESK_ADMIN_TOKEN must contain at least 32 characters")
	}
	g := &Gateway{Path: path, Admin: Hash(admin), jobs: map[string]*job{}, leases: map[string]lease{}}
	g.State = State{Devices: map[string]*Device{}, Agents: map[string]*Agent{}, Pairs: map[string]Pair{}}
	b, e := os.ReadFile(path)
	if e == nil {
		if e = json.Unmarshal(b, &g.State); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if g.State.Devices == nil {
		g.State.Devices = map[string]*Device{}
	}
	if g.State.Agents == nil {
		g.State.Agents = map[string]*Agent{}
	}
	if g.State.Pairs == nil {
		g.State.Pairs = map[string]Pair{}
	}
	for id, d := range g.State.Devices {
		if d == nil || d.ID != id {
			return nil, errors.New("invalid device in gateway state")
		}
	}
	for id, a := range g.State.Agents {
		if a == nil || a.ID != id {
			return nil, errors.New("invalid agent in gateway state")
		}
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return nil, e
	}
	g.audit, e = os.OpenFile(path+".audit.jsonl", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	return g, e
}
func (g *Gateway) save() error {
	b, e := json.MarshalIndent(g.State, "", "  ")
	if e != nil {
		return e
	}
	return writePrivateFile(g.Path, b)
}
func (g *Gateway) log(event, actor, device, action string) {
	_ = json.NewEncoder(g.audit).Encode(map[string]any{"time": time.Now().UTC(), "event": event, "actor": actor, "device": device, "action": action})
}
func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func fail(w http.ResponseWriter, status int, msg string) {
	reply(w, status, map[string]string{"error": msg})
}

// Snapshot while holding the state lock, but never hold it during socket I/O.
// A slow reader must not block cancellation, polling or every other device.
func (g *Gateway) respondLocked(w http.ResponseWriter, fn func() (int, any)) {
	status, body, err := func() (int, []byte, error) {
		g.mu.Lock()
		defer g.mu.Unlock()
		if g.closed {
			return 503, []byte(`{"error":"gateway shutting down"}`), nil
		}
		status, value := fn()
		body, err := json.Marshal(value)
		return status, body, err
	}()
	if err != nil {
		fail(w, 500, "cannot encode response")
		return
	}
	reply(w, status, json.RawMessage(body))
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	limit := int64(64 << 10)
	if r.URL.Path == "/result" {
		limit = 12 << 20
	} else if r.URL.Path == "/pair" {
		limit = 4 << 10
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	var extra any
	if dec.Decode(v) != nil || dec.Decode(&extra) != io.EOF {
		fail(w, 400, "invalid JSON or body too large")
		return false
	}
	return true
}
func bearer(r *http.Request) string {
	s := r.Header.Get("Authorization")
	if !strings.HasPrefix(s, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(s, "Bearer ")
}
func (g *Gateway) agent(key string) *Agent {
	if key == "" {
		return nil
	}
	h := Hash(key)
	for _, a := range g.State.Agents {
		if a.Key == h && !a.Revoked {
			return a
		}
	}
	return nil
}
func (g *Gateway) device(key string) *Device {
	if key == "" {
		return nil
	}
	h := Hash(key)
	for _, d := range g.State.Devices {
		if d.Key == h && !d.Revoked {
			return d
		}
	}
	return nil
}
func allowed(a *Agent, id string) bool {
	for _, d := range a.Devices {
		if d == id || d == "*" {
			return true
		}
	}
	return false
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
	if o := r.Header.Get("Origin"); o != "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		if o != scheme+"://"+r.Host {
			fail(w, 403, "origin rejected")
			return
		}
	}
	if r.URL.Path == "/health" && r.Method == "GET" {
		reply(w, 200, map[string]string{"status": "ok", "version": "0.1.0"})
		return
	}
	if r.URL.Path == "/" && r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, dashboard)
		return
	}
	if r.Method == "GET" && (r.URL.Path == "/dashboard.js" || r.URL.Path == "/dashboard.css") {
		if r.URL.Path == "/dashboard.js" {
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			fmt.Fprint(w, dashboardJS)
		} else {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			fmt.Fprint(w, dashboardCSS)
		}
		return
	}
	if r.URL.Path == "/mcp" {
		g.mcp(w, r)
		return
	}
	if r.Method != "POST" {
		fail(w, 405, "POST required")
		return
	}
	if strings.HasPrefix(r.URL.Path, "/admin/") {
		g.admin(w, r)
		return
	}
	if r.URL.Path == "/pair" {
		g.pair(w, r)
		return
	}
	if r.URL.Path == "/poll" || r.URL.Path == "/result" || r.URL.Path == "/check" {
		g.connector(w, r)
		return
	}
	fail(w, 404, "not found")
}
func (g *Gateway) admin(w http.ResponseWriter, r *http.Request) {
	if Hash(bearer(r)) != g.Admin {
		fail(w, 401, "unauthorized")
		return
	}
	var in struct {
		Name    string   `json:"name"`
		Devices []string `json:"devices"`
		Tools   []string `json:"tools"`
		ID      string   `json:"id"`
		Kind    string   `json:"kind"`
	}
	if !decode(w, r, &in) {
		return
	}
	g.respondLocked(w, func() (int, any) {
		switch r.URL.Path {
		case "/admin/list":
			ds := []Device{}
			for _, d := range g.State.Devices {
				v := *d
				v.Key = ""
				ds = append(ds, v)
			}
			as := []Agent{}
			for _, a := range g.State.Agents {
				v := *a
				v.Key = ""
				as = append(as, v)
			}
			return 200, map[string]any{"devices": ds, "agents": as, "leases": g.leases}
		case "/admin/pair":
			code := Secret()
			g.State.Pairs[Hash(code)] = Pair{in.Name, time.Now().Add(10 * time.Minute)}
			if e := g.save(); e != nil {
				delete(g.State.Pairs, Hash(code))
				return 500, map[string]string{"error": "cannot save state"}
			}
			return 200, map[string]any{"code": code, "expires_in": 600}
		case "/admin/agent":
			if len(in.Devices) == 0 {
				return 400, map[string]string{"error": "devices must contain IDs or explicit *"}
			}
			if in.Tools == nil {
				return 400, map[string]string{"error": "tools must be explicit; use [] to deny all or [\"*\"] to allow all"}
			}
			if e := validateToolPolicy(in.Tools); e != nil {
				return 400, map[string]string{"error": e.Error()}
			}
			token := Secret()
			a := &Agent{ID: Secret()[:16], Name: in.Name, Key: Hash(token), Devices: in.Devices, Tools: in.Tools}
			g.State.Agents[a.ID] = a
			if e := g.save(); e != nil {
				delete(g.State.Agents, a.ID)
				return 500, map[string]string{"error": "cannot save state"}
			}
			g.log("agent_created", "admin", "", a.ID)
			return 200, map[string]string{"id": a.ID, "token": token}
		case "/admin/permissions":
			a := g.State.Agents[in.ID]
			if a == nil || a.Revoked {
				return 404, map[string]string{"error": "unknown or revoked agent"}
			}
			if in.Tools == nil {
				return 400, map[string]string{"error": "tools must be an explicit array; use [] to deny all or [\"*\"] to allow all"}
			}
			if e := validateToolPolicy(in.Tools); e != nil {
				return 400, map[string]string{"error": e.Error()}
			}
			old := a.Tools
			a.Tools = in.Tools
			if e := g.save(); e != nil {
				a.Tools = old
				return 500, map[string]string{"error": "cannot save permissions; previous policy retained"}
			}
			// Invalidate the entire session so an action accepted under the old
			// policy cannot remain queued. Connectors observe cancellation via /check.
			for id, j := range g.jobs {
				if j.owner == a.ID {
					select {
					case j.done <- Result{Error: "agent permissions changed"}:
					default:
					}
					delete(g.jobs, id)
				}
			}
			for id, l := range g.leases {
				if l.Owner == a.ID {
					delete(g.leases, id)
				}
			}
			g.log("permissions_changed", "admin", "", a.ID)
			return 200, map[string]bool{"ok": true}
		case "/admin/revoke":
			found := false
			if in.Kind == "device" {
				if d := g.State.Devices[in.ID]; d != nil {
					d.Revoked = true
					found = true
				}
			} else if in.Kind == "agent" {
				if a := g.State.Agents[in.ID]; a != nil {
					a.Revoked = true
					found = true
				}
			}
			if !found {
				return 404, map[string]string{"error": "unknown identity"}
			}
			for id, j := range g.jobs {
				if (in.Kind == "device" && j.device == in.ID) || (in.Kind == "agent" && j.owner == in.ID) {
					select {
					case j.done <- Result{Error: "access revoked"}:
					default:
					}
					delete(g.jobs, id)
				}
			}
			for id, l := range g.leases {
				if (in.Kind == "device" && id == in.ID) || (in.Kind == "agent" && l.Owner == in.ID) {
					delete(g.leases, id)
				}
			}
			if e := g.save(); e != nil {
				return 500, map[string]string{"error": "revoked in memory; persistence failed"}
			}
			g.log("revoke", "admin", in.ID, in.Kind)
			return 200, map[string]bool{"ok": true}
		}
		return 404, map[string]string{"error": "unknown admin operation"}
	})
}
func (g *Gateway) pair(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code string `json:"code"`
		Name string `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	g.respondLocked(w, func() (int, any) {
		p, ok := g.State.Pairs[Hash(in.Code)]
		if !ok || time.Now().After(p.Expires) {
			return 403, map[string]string{"error": "invalid or expired pairing code"}
		}
		token := Secret()
		id := Secret()[:16]
		name := in.Name
		if name == "" {
			name = p.Name
		}
		d := &Device{ID: id, Name: name, Key: Hash(token)}
		g.State.Devices[id] = d
		delete(g.State.Pairs, Hash(in.Code))
		if e := g.save(); e != nil {
			delete(g.State.Devices, id)
			g.State.Pairs[Hash(in.Code)] = p
			return 500, map[string]string{"error": "cannot save state"}
		}
		g.log("pair", "admin", id, "")
		return 200, map[string]string{"id": id, "token": token}
	})
}
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
func (g *Gateway) Call(key, name string, args map[string]any) (any, error) {
	return g.CallContext(context.Background(), key, name, args)
}

func (g *Gateway) CallContext(ctx context.Context, key, name string, args map[string]any) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateTool(name, args); err != nil {
		return nil, err
	}
	g.mu.Lock()
	if err := ctx.Err(); err != nil {
		g.mu.Unlock()
		return nil, err
	}
	if g.closed {
		g.mu.Unlock()
		return nil, errors.New("gateway shutting down")
	}
	a := g.agent(key)
	if a == nil {
		g.mu.Unlock()
		return nil, errors.New("unauthorized agent")
	}
	if !toolAllowed(a, name) {
		g.mu.Unlock()
		return nil, errors.New("tool forbidden by agent policy")
	}
	owner := a.ID
	id, _ := args["device_id"].(string)
	if name == "devices_list" {
		ds := []Device{}
		for _, d := range g.State.Devices {
			if !d.Revoked && allowed(a, d.ID) {
				v := *d
				v.Key = ""
				ds = append(ds, v)
			}
		}
		g.mu.Unlock()
		return ds, nil
	}
	d := g.State.Devices[id]
	if d == nil || d.Revoked || !allowed(a, id) {
		g.mu.Unlock()
		return nil, errors.New("device unavailable or forbidden")
	}
	l := g.leases[id]
	now := time.Now()
	if l.Owner != "" && l.Owner != owner && now.Before(l.Until) {
		g.mu.Unlock()
		return nil, errors.New("device is controlled by another agent")
	}
	if name == "session_acquire" {
		if now.Sub(d.LastSeen) > 10*time.Second {
			g.mu.Unlock()
			return nil, errors.New("device offline")
		}
		g.leases[id] = lease{owner, now.Add(5 * time.Minute)}
		g.mu.Unlock()
		return map[string]any{"lease_seconds": 300}, nil
	}
	if l.Owner != owner || now.After(l.Until) {
		g.mu.Unlock()
		return nil, errors.New("call session_acquire first")
	}
	if name == "session_release" {
		for jid, j := range g.jobs {
			if j.device == id {
				select {
				case j.done <- Result{Error: "session released"}:
				default:
				}
				delete(g.jobs, jid)
			}
		}
		delete(g.leases, id)
		g.mu.Unlock()
		return "released", nil
	}
	valid := false
	for _, t := range Tools() {
		if t["name"] == name {
			valid = true
		}
	}
	if !valid {
		g.mu.Unlock()
		return nil, errors.New("unknown tool")
	}
	if now.Sub(d.LastSeen) > 10*time.Second {
		g.mu.Unlock()
		return nil, errors.New("device offline")
	}
	for _, j := range g.jobs {
		if j.device == id {
			g.mu.Unlock()
			return nil, errors.New("device has an in-flight action")
		}
	}
	seconds := 60.0
	if n, ok := args["timeout_seconds"].(float64); ok {
		seconds = n
	}
	if seconds < 1 || seconds > 300 {
		g.mu.Unlock()
		return nil, errors.New("timeout_seconds must be 1..300")
	}
	j := &job{Command: Command{Secret()[:24], name, args, now.Add(time.Duration(seconds) * time.Second)}, owner: owner, device: id, done: make(chan Result, 1)}
	g.jobs[j.ID] = j
	g.leases[id] = lease{owner, j.Deadline.Add(time.Minute)}
	g.log("dispatch", owner, id, name)
	g.mu.Unlock()
	timer := time.NewTimer(time.Until(j.Deadline))
	defer timer.Stop()
	var result Result
	select {
	case result = <-j.done:
	case <-ctx.Done():
		result.Error = "request cancelled; verify state before retrying"
	case <-timer.C:
		result.Error = "deadline exceeded; verify state before retrying"
	}
	g.mu.Lock()
	delete(g.jobs, j.ID)
	g.log("finished", owner, id, name)
	g.mu.Unlock()
	if result.Error != "" {
		return nil, errors.New(result.Error)
	}
	return result.Data, nil
}
