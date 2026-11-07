package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"relaydesk"
	"runtime"
	"strings"
	"time"
)

type Client struct {
	URL, Token string
	HTTP       *http.Client
}

func NewClient(raw, token string, wireguard bool) (*Client, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid gateway URL")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (wireguard || u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")) {
		return nil, errors.New("HTTPS required; --wireguard-http permits HTTP on your trusted WireGuard route")
	}
	return &Client{raw, token, &http.Client{Timeout: 320 * time.Second, CheckRedirect: func(r *http.Request, v []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Post(ctx context.Context, path string, in, out any) error {
	b, e := json.Marshal(in)
	if e != nil {
		return e
	}
	r, e := http.NewRequestWithContext(ctx, "POST", c.URL+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	r.Header.Set("Authorization", "Bearer "+c.Token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	resp, e := c.HTTP.Do(r)
	if e != nil {
		return e
	}
	defer resp.Body.Close()
	body, e := io.ReadAll(io.LimitReader(resp.Body, maxMessageBytes+1))
	if e != nil {
		return e
	}
	if len(body) > maxMessageBytes {
		return errors.New("gateway response exceeds 12 MiB")
	}
	if resp.StatusCode >= 300 {
		var details struct {
			Error string `json:"error"`
		}
		message := http.StatusText(resp.StatusCode)
		if json.Unmarshal(body, &details) == nil && details.Error != "" {
			message = details.Error
		}
		if c.Token != "" {
			message = strings.ReplaceAll(message, c.Token, "[REDACTED]")
		}
		message = strings.Map(func(r rune) rune {
			if r < 32 || (r >= 127 && r <= 159) {
				return ' '
			}
			return r
		}, message)
		if len(message) > 1024 {
			message = message[:1024] + "…"
		}
		return fmt.Errorf("gateway HTTP %d: %s", resp.StatusCode, message)
	}
	if out == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}

type Config struct {
	Gateway   string `json:"gateway"`
	Token     string `json:"token"`
	ID        string `json:"id"`
	Wireguard bool   `json:"wireguard_http"`
}

func DefaultConfig() string {
	p, e := os.UserConfigDir()
	if e != nil {
		p = "."
	}
	return filepath.Join(p, relaydesk.ID, "connector.json")
}
func SaveConfig(path string, c Config) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return writePrivateFile(path, b)
}
func ReadConfig(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
func RunConnector(ctx context.Context, c *Client) error {
	delay := time.Second
	for ctx.Err() == nil {
		pctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		var raw json.RawMessage
		e := c.Post(pctx, "/poll", map[string]any{"platform": runtime.GOOS, "capabilities": Capabilities()}, &raw)
		cancel()
		if e != nil {
			fmt.Fprintln(os.Stderr, "Connection unavailable:", e)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(delay):
			}
			if delay < 20*time.Second {
				delay *= 2
			}
			continue
		}
		delay = time.Second
		var cmd Command
		if json.Unmarshal(raw, &cmd) != nil || cmd.ID == "" {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Second):
			}
			continue
		}
		if time.Now().After(cmd.Deadline) {
			continue
		}
		work, cancelWork := context.WithDeadline(ctx, cmd.Deadline)
		finished := make(chan struct{})
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-finished:
					return
				case <-work.Done():
					return
				case <-ticker.C:
					var state struct {
						Active bool `json:"active"`
					}
					check, cc := context.WithTimeout(work, 3*time.Second)
					e := c.Post(check, "/check", map[string]any{"id": cmd.ID, "platform": runtime.GOOS, "capabilities": Capabilities()}, &state)
					cc()
					if e != nil || !state.Active {
						cancelWork()
						return
					}
				}
			}
		}()
		fmt.Fprintln(os.Stderr, "Running", cmd.Action, "job", cmd.ID)
		data, err := Execute(work, cmd)
		close(finished)
		cancelWork()
		result := Result{Data: data}
		if err != nil {
			result = Result{Error: err.Error()}
		}
		send, sc := context.WithTimeout(ctx, 5*time.Second)
		e = c.Post(send, "/result", map[string]any{"id": cmd.ID, "platform": runtime.GOOS, "capabilities": Capabilities(), "result": result}, nil)
		sc()
		if e != nil {
			fmt.Fprintln(os.Stderr, "Result not delivered; action will NOT be replayed:", e)
		}
	}
	return nil
}
