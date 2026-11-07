package core

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestFileToolsRejectPipesWithoutHanging(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "fifo")
	if err := syscall.Mkfifo(pipe, 0600); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"files_read", "files_write", "files_list"} {
		done := make(chan error, 1)
		go func() {
			_, err := Execute(context.Background(), Command{Action: action, Args: map[string]any{"path": pipe, "overwrite": true, "base64": "eA=="}})
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("pipe accepted")
			}
		case <-time.After(time.Second):
			t.Fatalf("%s hung on pipe", action)
		}
	}
}

func TestCredentialAndStateWritesDoNotFollowSymlinks(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	os.WriteFile(victim, []byte("unchanged"), 0644)
	config := filepath.Join(dir, "config.json")
	os.Symlink(victim, config)
	if err := SaveConfig(config, Config{Token: "synthetic-secret"}); err != nil {
		t.Fatal(err)
	}
	g, err := NewGateway(filepath.Join(dir, "state.json"), Secret())
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	os.Symlink(victim, g.Path+".tmp")
	if err := g.save(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(victim)
	if string(b) != "unchanged" {
		t.Fatal("symlink target overwritten")
	}
}
