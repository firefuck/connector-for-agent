package core

import "time"

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
