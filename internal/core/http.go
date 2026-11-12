package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"relaydesk"
	"strings"
)

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
	limit := int64(maxControlBodyBytes)
	if r.URL.Path == "/result" {
		limit = maxMessageBytes
	} else if r.URL.Path == "/pair" {
		limit = maxPairBodyBytes
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
		reply(w, 200, map[string]string{"status": "ok", "version": relaydesk.Version})
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
	if g.authBlocked(w, r) {
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
	if r.URL.Path == "/diagnose" {
		g.diagnose(w, r)
		return
	}
	if r.URL.Path == "/connector-state" || r.URL.Path == "/poll" || r.URL.Path == "/result" || r.URL.Path == "/check" {
		g.connector(w, r)
		return
	}
	fail(w, 404, "not found")
}
