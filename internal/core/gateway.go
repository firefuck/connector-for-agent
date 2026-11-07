package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
