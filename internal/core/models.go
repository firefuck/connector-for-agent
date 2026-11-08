package core

import "time"

type Device struct {
	LocalEpoch   string    `json:"local_epoch,omitempty"`
	Paused       bool      `json:"paused"`
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
	DeviceID   string         `json:"device_id,omitempty"`
	LeaseID    string         `json:"lease_id,omitempty"`
	LocalEpoch string         `json:"local_epoch,omitempty"`
	DeviceName string         `json:"device_name,omitempty"`
	AgentID    string         `json:"agent_id,omitempty"`
	AgentName  string         `json:"agent_name,omitempty"`
	ID         string         `json:"id"`
	Action     string         `json:"action"`
	Args       map[string]any `json:"args"`
	Deadline   time.Time      `json:"deadline"`
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
	ID    string    `json:"id,omitempty"`
	Owner string    `json:"owner"`
	Until time.Time `json:"until"`
}
