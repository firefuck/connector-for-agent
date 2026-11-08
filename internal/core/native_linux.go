package core

import (
	"context"
	"os/exec"
)

func nativeCommand(ctx context.Context) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "/usr/bin/python3", "-u", "-c", linuxHelper), nil
}
