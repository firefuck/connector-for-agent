package core

import (
	"context"
	"os/exec"
)

func nativeCommand(ctx context.Context) (*exec.Cmd, error) { return ps(ctx, windowsHelper), nil }
