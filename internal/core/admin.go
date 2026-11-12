package core

import (
	"crypto/subtle"
	"net/http"
	"time"
)

func (g *Gateway) admin(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(Hash(bearer(r))), []byte(g.Admin)) != 1 {
		g.authFailed(r)
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
			for hash, p := range g.State.Pairs {
				if time.Now().After(p.Expires) {
					delete(g.State.Pairs, hash)
				}
			}
			code := Secret()
			g.State.Pairs[Hash(code)] = Pair{in.Name, time.Now().Add(pairingLifetime)}
			if e := g.save(); e != nil {
				delete(g.State.Pairs, Hash(code))
				return 500, map[string]string{"error": "cannot save state"}
			}
			return 200, map[string]any{"code": code, "expires_in": int(pairingLifetime / time.Second)}
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
			// A queued action authorized under the old policy must not survive.
			g.cancelSessionsLocked(a.ID, "", "agent permissions changed")
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
			if in.Kind == "device" {
				g.cancelSessionsLocked("", in.ID, "access revoked")
			} else {
				g.cancelSessionsLocked(in.ID, "", "access revoked")
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
	rejected := false
	defer func() {
		if rejected {
			g.authFailed(r)
		}
	}()
	g.respondLocked(w, func() (int, any) {
		p, ok := g.State.Pairs[Hash(in.Code)]
		if !ok || time.Now().After(p.Expires) {
			rejected = true
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
