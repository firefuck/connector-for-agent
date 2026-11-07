package core

import "fmt"

func toolAllowed(a *Agent, name string) bool {
	if a.Tools == nil {
		return true
	}
	for _, tool := range a.Tools {
		if tool == "*" || tool == name {
			return true
		}
	}
	return false
}

func validateToolPolicy(policy []string) error {
	known := map[string]bool{"*": true}
	for _, tool := range Tools() {
		known[tool["name"].(string)] = true
	}
	for _, name := range policy {
		if !known[name] {
			return fmt.Errorf("unknown tool permission: %s", name)
		}
	}
	return nil
}

// ToolsForAgent returns the tool schema visible under the current policy.
// Call independently enforces that policy, including for legacy clients that
// cached a previous tools/list response.
func (g *Gateway) ToolsForAgent(key string) []map[string]any {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := []map[string]any{}
	a := g.agent(key)
	if a == nil {
		return out
	}
	for _, tool := range Tools() {
		if toolAllowed(a, tool["name"].(string)) {
			out = append(out, tool)
		}
	}
	return out
}
