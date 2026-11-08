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
	"strings"
	"time"
)

type HTTPStatusError struct {
	Status  int
	Message string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("gateway HTTP %d: %s", e.Status, e.Message)
}

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
		return &HTTPStatusError{Status: resp.StatusCode, Message: message}
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
	f, e := openRegularFile(path, os.O_RDONLY)
	if e != nil {
		return c, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, maxControlBodyBytes+1))
	if len(b) > maxControlBodyBytes {
		return c, errors.New("configuration too large")
	}
	if e == nil {
		e = json.Unmarshal(b, &c)
	}
	return c, e
}
