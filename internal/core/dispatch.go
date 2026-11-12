package core

import (
	"context"
	"errors"
	"time"
)

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
	if d.Paused {
		g.mu.Unlock()
		return nil, errors.New("device paused locally; local user must resume")
	}
	l := g.leases[id]
	now := time.Now()
	if l.Owner != "" && l.Owner != owner && now.Before(l.Until) {
		g.mu.Unlock()
		return nil, errors.New("device is controlled by another agent")
	}
	if name == "session_acquire" {
		if now.Sub(d.LastSeen) > deviceOnlineWindow {
			g.mu.Unlock()
			return nil, errors.New("device offline")
		}
		g.leases[id] = lease{ID: Secret()[:24], Owner: owner, Until: now.Add(defaultLeaseDuration)}
		g.mu.Unlock()
		return map[string]any{"lease_seconds": int(defaultLeaseDuration / time.Second)}, nil
	}
	if l.Owner != owner || now.After(l.Until) {
		g.mu.Unlock()
		return nil, errors.New("call session_acquire first")
	}
	if name == "session_release" {
		g.cancelSessionsLocked("", id, "session released")
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
	if now.Sub(d.LastSeen) > deviceOnlineWindow {
		g.mu.Unlock()
		return nil, errors.New("device offline")
	}
	for _, j := range g.jobs {
		if j.device == id {
			g.mu.Unlock()
			return nil, errors.New("device has an in-flight action")
		}
	}
	seconds := defaultActionTimeout.Seconds()
	if n, ok := args["timeout_seconds"].(float64); ok {
		seconds = n
	}
	if seconds < 1 || seconds > maxActionTimeout.Seconds() {
		g.mu.Unlock()
		return nil, errors.New("timeout_seconds must be 1..300")
	}
	j := &job{Command: Command{DeviceID: id, LeaseID: l.ID, ID: Secret()[:24], Action: name, Args: args, Deadline: now.Add(time.Duration(seconds) * time.Second), LocalEpoch: d.LocalEpoch, DeviceName: d.Name, AgentID: a.ID, AgentName: a.Name}, owner: owner, device: id, done: make(chan Result, 1)}
	g.jobs[j.ID] = j
	// Each action renews the lease. It must outlive the action, and it must
	// never end earlier than the lease the agent already held.
	until := now.Add(defaultLeaseDuration)
	if completion := j.Deadline.Add(leaseCompletionGrace); completion.After(until) {
		until = completion
	}
	g.leases[id] = lease{ID: l.ID, Owner: owner, Until: until}
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
