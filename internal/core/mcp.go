package core

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func Tools() []map[string]any {
	defs := []struct {
		name, desc string
		props      map[string]any
		required   []string
	}{
		{"devices_list", "List authorized devices. Last seen older than 10 seconds means offline.", nil, nil},
		{"session_acquire", "Acquire exclusive five-minute device lease before control.", nil, []string{"device_id"}},
		{"session_release", "Release lease and cancel pending work.", nil, []string{"device_id"}},
		{"system_info", "OS, user, session and backend capabilities.", nil, []string{"device_id"}},
		{"desktop_screenshot", "Capture entire virtual desktop as PNG. Windows or X11 only.", nil, []string{"device_id"}},
		{"desktop_click", "Click at absolute desktop coordinates. button: left, right, middle; count: 1 or 2.", map[string]any{"x": num(), "y": num(), "button": str(), "count": num()}, []string{"device_id", "x", "y"}},
		{"desktop_move", "Move mouse to absolute coordinates.", map[string]any{"x": num(), "y": num()}, []string{"device_id", "x", "y"}},
		{"desktop_scroll", "Scroll signed wheel steps, positive is up.", map[string]any{"steps": num()}, []string{"device_id", "steps"}},
		{"desktop_type", "Type literal Unicode text (Linux uses clipboard paste).", map[string]any{"text": str()}, []string{"device_id", "text"}},
		{"desktop_key", "Press chord, e.g. ctrl+l, ctrl+shift+p, enter, alt+tab, win+r.", map[string]any{"key": str()}, []string{"device_id", "key"}},
		{"browser_open", "Open an http/https URL in default browser.", map[string]any{"url": str()}, []string{"device_id", "url"}},
		{"system_exec", "Execute shell command as connector user; capped output. Non-interactive, no PTY.", map[string]any{"command": str(), "cwd": str(), "timeout_seconds": num()}, []string{"device_id", "command"}},
		{"files_read", "Read file, max 4 MiB, returns base64.", map[string]any{"path": str()}, []string{"device_id", "path"}},
		{"files_write", "Write base64 file content, max 4 MiB. Overwrites only when overwrite=true.", map[string]any{"path": str(), "base64": str(), "overwrite": map[string]any{"type": "boolean"}}, []string{"device_id", "path", "base64"}},
		{"files_list", "List directory entries, maximum 2000.", map[string]any{"path": str()}, []string{"device_id", "path"}},
	}
	out := []map[string]any{}
	for _, d := range defs {
		p := map[string]any{}
		if d.name != "devices_list" {
			p["device_id"] = str()
		}
		for k, v := range d.props {
			p[k] = v
		}
		req := d.required
		if req == nil {
			req = []string{}
		}
		out = append(out, map[string]any{"name": d.name, "description": d.desc, "inputSchema": map[string]any{"type": "object", "properties": p, "required": req, "additionalProperties": false}})
	}
	return out
}
func str() map[string]any { return map[string]any{"type": "string"} }
func num() map[string]any { return map[string]any{"type": "integer"} }

type RPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (g *Gateway) mcp(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	ok := g.agent(bearer(r)) != nil
	g.mu.Unlock()
	if !ok {
		fail(w, 401, "unauthorized agent")
		return
	}
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		fail(w, 405, "SSE stream not offered")
		return
	}
	var req RPC
	if !decode(w, r, &req) {
		return
	}
	if req.JSONRPC != "2.0" {
		reply(w, 400, map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32600, "message": "invalid request"}})
		return
	}
	if len(req.ID) == 0 {
		reply(w, 202, nil)
		return
	}
	res := map[string]any{"jsonrpc": "2.0", "id": req.ID}
	var result any
	switch req.Method {
	case "initialize":
		result = map[string]any{"protocolVersion": "2025-03-26", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "relaydesk", "version": "0.1.0"}, "instructions": "Use devices_list, then session_acquire. Release after task. Inspect screenshots; do not assume GUI actions succeeded. Only Windows interactive sessions and Linux X11 desktop automation are supported."}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": Tools()}
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if json.Unmarshal(req.Params, &p) != nil {
			res["error"] = map[string]any{"code": -32602, "message": "invalid params"}
			reply(w, 200, res)
			return
		}
		data, e := g.Call(bearer(r), p.Name, p.Arguments)
		if e != nil {
			result = map[string]any{"isError": true, "content": []any{map[string]string{"type": "text", "text": e.Error()}}}
		} else if m, ok := data.(map[string]any); ok && m["mimeType"] == "image/png" {
			result = map[string]any{"content": []any{map[string]any{"type": "image", "mimeType": "image/png", "data": m["base64"]}, map[string]any{"type": "text", "text": fmt.Sprintf("Virtual desktop origin: (%v, %v), dimensions: %v x %v. Add origin to screenshot-relative coordinates. Linux X11 origin defaults to (0,0).", m["origin_x"], m["origin_y"], m["width"], m["height"])}}}
		} else {
			b, _ := json.Marshal(data)
			result = map[string]any{"content": []any{map[string]string{"type": "text", "text": string(b)}}}
		}
	default:
		res["error"] = map[string]any{"code": -32601, "message": fmt.Sprintf("unknown method %s", req.Method)}
		reply(w, 200, res)
		return
	}
	res["result"] = result
	reply(w, 200, res)
}

func validateTool(name string, args map[string]any) error {
	for _, t := range Tools() {
		if t["name"] != name {
			continue
		}
		schema := t["inputSchema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		for _, k := range schema["required"].([]string) {
			if _, ok := args[k]; !ok {
				return fmt.Errorf("missing argument: %s", k)
			}
		}
		for k, v := range args {
			p, ok := props[k]
			if !ok {
				return fmt.Errorf("unknown argument: %s", k)
			}
			typ := p.(map[string]any)["type"]
			valid := false
			switch typ {
			case "string":
				_, valid = v.(string)
			case "boolean":
				_, valid = v.(bool)
			case "integer":
				n, ok := v.(float64)
				valid = ok && n >= -2147483648 && n <= 2147483647 && float64(int64(n)) == n
			}
			if !valid {
				return fmt.Errorf("invalid argument type: %s", k)
			}
		}
		return nil
	}
	return fmt.Errorf("unknown tool: %s", name)
}
