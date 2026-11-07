package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"
)

// RunMCPBridge allows cancellation and ping to be read while tools/call waits for
// a connector. Every message is forwarded once; host-control calls are never
// retried. Only JSON-RPC messages are written to output. Callers own the I/O
// streams and must close blocking readers/writers on cancellation; workers
// never wait for a blocked output write after context cancellation.
func RunMCPBridge(ctx context.Context, c *Client, input io.Reader, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx, stop := context.WithCancel(ctx)
	var workers sync.WaitGroup
	defer func() { stop(); workers.Wait() }()
	var pendingMu sync.Mutex
	enc := json.NewEncoder(output)
	pending := map[string]context.CancelFunc{}
	notifications := make(chan struct{}, 16)
	bridgeErrors := make(chan error, 1)
	abort := func(err error) {
		select {
		case bridgeErrors <- err:
		default:
		}
		stop()
	}
	type message struct {
		value any
		done  chan struct{}
	}
	outputQueue := make(chan message)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-outputQueue:
				err := enc.Encode(msg.value)
				close(msg.done)
				if err != nil {
					abort(err)
					return
				}
			}
		}
	}()
	write := func(v any) {
		msg := message{value: v, done: make(chan struct{})}
		select {
		case outputQueue <- msg:
		case <-ctx.Done():
			return
		}
		select {
		case <-msg.done:
		case <-ctx.Done():
		}
	}
	errReply := func(id json.RawMessage, message string) {
		write(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32000, "message": message}})
	}
	lines := make(chan []byte)
	scanDone := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(input)
		scanner.Buffer(make([]byte, 65536), 12<<20)
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- b:
			case <-ctx.Done():
				return
			}
		}
		scanDone <- scanner.Err()
		close(lines)
	}()
	for {
		select {
		case <-ctx.Done():
			select {
			case err := <-bridgeErrors:
				return err
			default:
				return ctx.Err()
			}
		case body, ok := <-lines:
			if !ok {
				return <-scanDone
			}
			var req RPC
			if json.Unmarshal(body, &req) != nil {
				write(map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32700, "message": "invalid JSON"}})
				continue
			}
			if len(req.ID) == 0 {
				if req.Method == "notifications/cancelled" {
					var p struct {
						RequestID json.RawMessage `json:"requestId"`
					}
					if json.Unmarshal(req.Params, &p) == nil {
						key, _ := requestIDKey(p.RequestID)
						pendingMu.Lock()
						if cancel := pending[key]; cancel != nil {
							cancel()
						}
						pendingMu.Unlock()
					}
				}
				select {
				case notifications <- struct{}{}:
				default:
					return errors.New("bridge notification backlog exceeded")
				}
				workers.Add(1)
				go func() {
					defer workers.Done()
					defer func() { <-notifications }()
					notify, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()
					if err := c.Post(notify, "/mcp", json.RawMessage(body), nil); err != nil && ctx.Err() == nil {
						abort(err)
					}
				}()
				continue
			}
			key, valid := requestIDKey(req.ID)
			if !valid {
				write(map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32600, "message": "invalid request ID"}})
				continue
			}
			work, cancel := context.WithCancel(ctx)
			pendingMu.Lock()
			if len(pending) >= 32 || pending[key] != nil {
				pendingMu.Unlock()
				cancel()
				errReply(req.ID, "bridge busy or duplicate request ID")
				continue
			}
			pending[key] = cancel
			pendingMu.Unlock()
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer cancel()
				defer func() { pendingMu.Lock(); delete(pending, key); pendingMu.Unlock() }()
				var result json.RawMessage
				if err := c.Post(work, "/mcp", json.RawMessage(body), &result); err != nil {
					errReply(req.ID, err.Error())
				} else if len(result) > 0 {
					write(result)
				}
			}()
		}
	}
}
