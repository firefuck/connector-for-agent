package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"relaydesk"
	"relaydesk/internal/core"
	"strings"
	"syscall"
	"time"
)

type exitStatus int

func (e exitStatus) Error() string { return "diagnosis incomplete" }
func main() {
	if e := mainRun(); e != nil {
		if code, ok := e.(exitStatus); ok {
			os.Exit(int(code))
		}
		fmt.Fprintln(os.Stderr, "Error:", e)
		os.Exit(1)
	}
}
func mainRun() error {
	if len(os.Args) < 2 {
		return wizard()
	}
	switch os.Args[1] {
	case "version":
		fmt.Println(relaydesk.Name, relaydesk.Version)
		return nil
	case "license":
		fmt.Print(relaydesk.NoticeText(), relaydesk.LicenseText())
		return nil
	case "help", "--help", "-h":
		fmt.Println(relaydesk.Name + ": gateway | pair | connector | control | doctor | admin | mcp | token | version | license\nRun a command with --help for flags. No arguments starts the pairing wizard.")
		return nil
	case "token":
		fmt.Println(core.Secret())
		return nil
	}
	f := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	jsonOutput := f.Bool("json", false, "Machine-readable diagnosis")
	headless := f.Bool("headless", false, "Run without the local window (X11/shell testing only)")
	autostart := f.Bool("autostart", false, "Respect a previous local Stop")
	gateway := f.String("gateway", os.Getenv("RELAYDESK_GATEWAY"), "Gateway URL")
	wg := f.Bool("wireguard-http", os.Getenv("RELAYDESK_WIREGUARD_HTTP") == "1", "Allow HTTP over your WireGuard network")
	config := f.String("config", core.DefaultConfig(), "Connector credential file")
	name := f.String("name", "", "Device or agent name")
	code := f.String("code", "", "One-time pairing code; prefer RELAYDESK_PAIR_CODE")
	listen := f.String("listen", "127.0.0.1:8787", "Gateway bind address")
	state := f.String("state", "data/state.json", "Gateway persistent state")
	cert := f.String("tls-cert", "", "TLS certificate PEM")
	key := f.String("tls-key", "", "TLS private key PEM")
	duration := f.Duration("duration", 0, "Connector session duration (0 = persistent process)")
	devices := f.String("devices", "", "Comma-separated device IDs or *")
	tools := f.String("tools", "*", "Comma-separated tool permissions; * = all, empty = none")
	id := f.String("id", "", "Identity to revoke")
	kind := f.String("kind", "", "device or agent")
	op := f.String("op", "list", "Admin operation: list, pair, agent, permissions, revoke")
	if e := f.Parse(os.Args[2:]); e != nil {
		return e
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch os.Args[1] {
	case "doctor":
		report := core.Doctor(ctx, *config)
		if *jsonOutput {
			if e := json.NewEncoder(os.Stdout).Encode(report); e != nil {
				return e
			}
		} else {
			fmt.Print(report.Text())
		}
		if report.ExitCode != 0 {
			return exitStatus(report.ExitCode)
		}
		return nil
	case "control":
		result, e := core.LocalControl(*config, *op)
		if e != nil {
			return e
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	case "gateway":
		g, e := core.NewGateway(*state, os.Getenv("RELAYDESK_ADMIN_TOKEN"))
		if e != nil {
			return e
		}
		defer g.Close()
		s := &http.Server{Addr: *listen, Handler: g, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 330 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16384}
		shutdownDone := make(chan struct{})
		go func() {
			defer close(shutdownDone)
			<-ctx.Done()
			_ = g.Close()
			c, cc := context.WithTimeout(context.Background(), 5*time.Second)
			defer cc()
			if s.Shutdown(c) != nil {
				_ = s.Close()
			}
		}()
		fmt.Fprintln(os.Stderr, relaydesk.Name+" gateway listening on", *listen)
		if *cert != "" {
			e = s.ListenAndServeTLS(*cert, *key)
		} else {
			e = s.ListenAndServe()
		}
		if errors.Is(e, http.ErrServerClosed) {
			<-shutdownDone
			return nil
		}
		return e
	case "pair":
		if *code == "" {
			*code = os.Getenv("RELAYDESK_PAIR_CODE")
		}
		if *code == "" {
			return errors.New("pairing code required")
		}
		if _, e := os.Stat(*config); e == nil {
			return errors.New("configuration already exists; use a new --config or remove the old one intentionally")
		}
		c, e := core.NewClient(strings.TrimRight(*gateway, "/"), "", *wg)
		if e != nil {
			return e
		}
		var out struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		}
		if e = c.Post(ctx, "/pair", map[string]string{"code": *code, "name": *name}, &out); e != nil {
			return e
		}
		if e = core.SaveConfig(*config, core.Config{Gateway: c.URL, Token: out.Token, ID: out.ID, Wireguard: *wg}); e != nil {
			return e
		}
		fmt.Println("Paired device:", out.ID, "\nSaved configuration:", *config)
		return nil
	case "connector":
		cfg, e := core.ReadConfig(*config)
		if e != nil {
			return e
		}
		c, e := core.NewClient(cfg.Gateway, cfg.Token, cfg.Wireguard)
		if e != nil {
			return e
		}
		if *duration > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *duration)
			defer cancel()
		}
		fmt.Fprintln(os.Stderr, "Connecting device:", cfg.ID, "— Ctrl+C to disconnect")
		return core.RunInteractiveConnector(ctx, c, *config, *headless, *autostart)
	case "admin":
		c, e := core.NewClient(strings.TrimRight(*gateway, "/"), os.Getenv("RELAYDESK_ADMIN_TOKEN"), *wg)
		if e != nil {
			return e
		}
		ds := []string{}
		for _, s := range strings.Split(*devices, ",") {
			if strings.TrimSpace(s) != "" {
				ds = append(ds, strings.TrimSpace(s))
			}
		}
		ts := []string{}
		for _, tool := range strings.Split(*tools, ",") {
			if tool = strings.TrimSpace(tool); tool != "" {
				ts = append(ts, tool)
			}
		}
		var out any
		e = c.Post(ctx, "/admin/"+*op, map[string]any{"name": *name, "devices": ds, "tools": ts, "id": *id, "kind": *kind}, &out)
		if e != nil {
			return e
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(b))
		return nil
	case "mcp":
		c, e := core.NewClient(strings.TrimRight(*gateway, "/"), os.Getenv("RELAYDESK_AGENT_TOKEN"), *wg)
		if e != nil {
			return e
		}
		go func() { <-ctx.Done(); _ = os.Stdin.Close(); _ = os.Stdout.Close() }()
		return core.RunMCPBridge(ctx, c, os.Stdin, os.Stdout)
	}
	return errors.New("unknown command; use relaydesk help")
}
func wizard() error {
	reader := bufio.NewReader(os.Stdin)
	ask := func(q string) string { fmt.Print(q); s, _ := reader.ReadString('\n'); return strings.TrimSpace(s) }
	fmt.Printf("%s connector %s\nWindows / Linux X11 / GNOME Wayland. Local controls open after pairing.\n", relaydesk.Name, relaydesk.Version)
	path := core.DefaultConfig()
	if _, e := os.Stat(path); os.IsNotExist(e) {
		u := ask("Gateway URL: ")
		code := ask("Pairing code: ")
		name := ask("Device name: ")
		wg := false
		if strings.HasPrefix(u, "http://") {
			wg = strings.EqualFold(ask("Is this your trusted WireGuard network? (yes/no): "), "yes")
		}
		c, e := core.NewClient(strings.TrimRight(u, "/"), "", wg)
		if e != nil {
			return e
		}
		var out struct{ ID, Token string }
		if e = c.Post(context.Background(), "/pair", map[string]string{"code": code, "name": name}, &out); e != nil {
			return e
		}
		if e = core.SaveConfig(path, core.Config{Gateway: c.URL, Token: out.Token, ID: out.ID, Wireguard: wg}); e != nil {
			return e
		}
	}
	cfg, e := core.ReadConfig(path)
	if e != nil {
		return e
	}
	c, e := core.NewClient(cfg.Gateway, cfg.Token, cfg.Wireguard)
	if e != nil {
		return e
	}
	fmt.Println("Config:", filepath.Clean(path))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return core.RunInteractiveConnector(ctx, c, path, false, false)
}
