package core

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

//go:embed native/linux.py
var linuxHelper string

//go:embed native/windows.ps1
var windowsHelper string

type NativeHelper struct {
	cmd     *exec.Cmd
	input   io.WriteCloser
	queue   chan any
	done    chan struct{}
	ready   chan struct{}
	mu      sync.Mutex
	pending map[string]chan Result
	action  func(string)
}

func StartNativeHelper(ctx context.Context, action func(string)) (*NativeHelper, error) {
	cmd, err := nativeCommand(ctx)
	if err != nil {
		return nil, err
	}
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		return nil, err
	}
	cmd.Stderr = os.Stderr
	if err = cmd.Start(); err != nil {
		input.Close()
		output.Close()
		return nil, err
	}
	h := &NativeHelper{cmd: cmd, input: input, queue: make(chan any, 16), done: make(chan struct{}), ready: make(chan struct{}), pending: map[string]chan Result{}, action: action}
	go func() {
		enc := json.NewEncoder(input)
		for {
			select {
			case <-h.done:
				return
			case <-ctx.Done():
				return
			case v := <-h.queue:
				if enc.Encode(v) != nil {
					return
				}
			}
		}
	}()
	go func() {
		defer close(h.done)
		defer action("stop") // Losing the visible local control must fail closed.
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 65536), maxMessageBytes)
		var once sync.Once
		for scanner.Scan() {
			var m struct {
				Event, ID string
				Data      any
				Error     string
			}
			if json.Unmarshal(scanner.Bytes(), &m) != nil {
				break
			}
			switch m.Event {
			case "ready":
				once.Do(func() { close(h.ready) })
			case "pause", "resume", "stop":
				action(m.Event)
			default:
				h.mu.Lock()
				ch := h.pending[m.ID]
				h.mu.Unlock()
				if ch != nil {
					select {
					case ch <- Result{Data: m.Data, Error: m.Error}:
					default:
					}
				}
			}
		}
		input.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()
	select {
	case <-h.ready:
		return h, nil
	case <-h.done:
		return nil, errors.New("local control window failed to start; run relaydesk doctor")
	case <-ctx.Done():
		h.Close()
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		h.Close()
		return nil, errors.New("local control window timed out; run relaydesk doctor")
	}
}
func (h *NativeHelper) Close() {
	h.input.Close()
	if h.cmd.Process != nil {
		_ = h.cmd.Process.Kill()
	}
}
func (h *NativeHelper) Send(v any) bool {
	select {
	case <-h.done:
		return false
	case h.queue <- v:
		return true
	default:
		return false
	}
}
func (h *NativeHelper) Status(s ConnectorStatus) { h.Send(map[string]any{"op": "status", "status": s}) }
func (h *NativeHelper) Execute(ctx context.Context, c Command) (any, error) {
	id := Secret()[:24]
	ch := make(chan Result, 1)
	h.mu.Lock()
	h.pending[id] = ch
	h.mu.Unlock()
	defer func() { h.mu.Lock(); delete(h.pending, id); h.mu.Unlock() }()
	if !h.Send(map[string]any{"op": "execute", "id": id, "action": c.Action, "args": c.Args}) {
		return nil, errors.New("desktop helper unavailable")
	}
	select {
	case r := <-ch:
		if r.Error != "" {
			return nil, errors.New(r.Error)
		}
		return r.Data, nil
	case <-h.done:
		return nil, errors.New("desktop helper stopped")
	case <-ctx.Done():
		if !h.Send(map[string]any{"op": "cancel", "id": id}) {
			h.Close()
		}
		return nil, ctx.Err()
	}
}
