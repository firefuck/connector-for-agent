package core

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

type ConnectorStatus struct {
	State      string `json:"state"`
	DeviceID   string `json:"device_id,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
	AgentID    string `json:"agent_id,omitempty"`
	AgentName  string `json:"agent_name,omitempty"`
}
type ConnectorOptions struct {
	Session      *LocalSession
	Status       func(ConnectorStatus)
	Execute      func(context.Context, Command) (any, error)
	Capabilities func() []string
}

func RunConnector(ctx context.Context, c *Client) error {
	return RunConnectorWithOptions(ctx, c, ConnectorOptions{})
}
func RunConnectorWithOptions(ctx context.Context, c *Client, o ConnectorOptions) error {
	if o.Session == nil {
		o.Session = NewLocalSession(false)
	}
	if o.Execute == nil {
		o.Execute = Execute
	}
	if o.Capabilities == nil {
		o.Capabilities = Capabilities
	}
	status := func(v ConnectorStatus) {
		if o.Status != nil {
			o.Status(v)
		}
	}
	body := func() map[string]any {
		epoch, paused, _ := o.Session.Snapshot()
		return map[string]any{"platform": runtime.GOOS, "capabilities": o.Capabilities(), "local_epoch": epoch, "paused": paused}
	}
	wait := func(d time.Duration) {
		select {
		case <-ctx.Done():
		case <-o.Session.wake:
		case <-time.After(d):
		}
	}
	defer func() {
		o.Session.Change("stop")
		sync, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.Post(sync, "/connector-state", body(), nil)
	}()
	delay := time.Second
	for ctx.Err() == nil {
		_, _, stopped := o.Session.Snapshot()
		if stopped {
			return nil
		}
		pctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		var poll struct {
			Command
			DeviceID string `json:"device_id"`
		}
		e := c.Post(pctx, "/poll", body(), &poll)
		cancel()
		if e != nil {
			_, paused, _ := o.Session.Snapshot()
			state := "offline"
			if paused {
				state = "paused_offline"
			}
			status(ConnectorStatus{State: state})
			fmt.Fprintln(os.Stderr, "Gateway unavailable; no action replay")
			wait(delay)
			if delay < 16*time.Second {
				delay *= 2
			}
			continue
		}
		delay = time.Second
		_, paused, _ := o.Session.Snapshot()
		state := "idle"
		if paused {
			state = "paused"
		}
		info := ConnectorStatus{State: state, DeviceID: poll.DeviceID, DeviceName: poll.DeviceName, AgentID: poll.AgentID, AgentName: poll.AgentName}
		status(info)
		if poll.ID == "" {
			wait(time.Second)
			continue
		}
		deadline, dc := context.WithDeadline(ctx, poll.Deadline)
		work, cancelWork, ok := o.Session.Begin(deadline, poll.LocalEpoch)
		if !ok {
			dc()
			continue
		}
		finished := make(chan struct{})
		checked := make(chan struct{})
		go func() {
			defer close(checked)
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-finished:
					return
				case <-work.Done():
					return
				case <-ticker.C:
					var active struct {
						Active bool `json:"active"`
					}
					check, cc := context.WithTimeout(work, 3*time.Second)
					request := body()
					request["id"] = poll.ID
					err := c.Post(check, "/check", request, &active)
					cc()
					if err != nil || !active.Active {
						cancelWork()
						return
					}
				}
			}
		}()
		info.State = "controlling"
		switch {
		case poll.Action == "desktop_screenshot":
			info.State = "viewing"
		case poll.Action == "system_exec":
			info.State = "shell"
		case strings.HasPrefix(poll.Action, "files_"):
			info.State = "files"
		case poll.Action == "system_info":
			info.State = "inspecting"
		}
		status(info)
		data, err := o.Execute(work, poll.Command)
		if work.Err() != nil {
			err = work.Err()
		}
		close(finished)
		cancelWork()
		dc()
		<-checked
		o.Session.End()
		result := Result{Data: data}
		if err != nil {
			result = Result{Error: err.Error()}
		}
		send, sc := context.WithTimeout(ctx, 5*time.Second)
		request := body()
		request["id"], request["result"] = poll.ID, result
		e = c.Post(send, "/result", request, nil)
		sc()
		if e != nil {
			fmt.Fprintln(os.Stderr, "Result not delivered; action will NOT be replayed")
		}
	}
	return nil
}
