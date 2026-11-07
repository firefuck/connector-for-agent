package core

import (
	"context"
	"testing"
	"time"
)

// Exercise public entry points with two live sessions: cancelling one must
// neither cancel the other nor allow a cancelled command to be replayed.
func TestSessionCancellationIsolation(t *testing.T) {
	for _, operation := range []string{"permissions", "revoke-agent", "revoke-device", "release"} {
		t.Run(operation, func(t *testing.T) {
			g, admin, _ := fixture(t)
			defer g.Close()
			var devices [2]*Client
			var ids, tokens, agents, jobs [2]string
			var done [2]chan error
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			for i := range devices {
				devices[i], ids[i] = pairDevice(t, admin)
				post(t, devices[i], "/poll", map[string]any{})
				a := post(t, admin, "/admin/agent", map[string]any{"devices": []string{ids[i]}, "tools": []string{"*"}})
				tokens[i], agents[i] = a["token"].(string), a["id"].(string)
				if _, err := g.Call(tokens[i], "session_acquire", map[string]any{"device_id": ids[i]}); err != nil {
					t.Fatal(err)
				}
				done[i] = make(chan error, 1)
				go func(i int) {
					_, err := g.CallContext(ctx, tokens[i], "system_info", map[string]any{"device_id": ids[i]})
					done[i] <- err
				}(i)
				deadline := time.Now().Add(3 * time.Second)
				for jobs[i] == "" {
					r := post(t, devices[i], "/poll", map[string]any{})
					jobs[i], _ = r["id"].(string)
					if time.Now().After(deadline) {
						t.Fatal("job not delivered")
					}
					if jobs[i] == "" {
						time.Sleep(time.Millisecond)
					}
				}
			}
			switch operation {
			case "permissions":
				post(t, admin, "/admin/permissions", map[string]any{"id": agents[0], "tools": []string{}})
			case "revoke-agent":
				post(t, admin, "/admin/revoke", map[string]any{"id": agents[0], "kind": "agent"})
			case "revoke-device":
				post(t, admin, "/admin/revoke", map[string]any{"id": ids[0], "kind": "device"})
			case "release":
				if _, err := g.Call(tokens[0], "session_release", map[string]any{"device_id": ids[0]}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done[0]:
				if err == nil {
					t.Fatal("cancelled call succeeded")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("call was not cancelled")
			}
			if err := devices[0].Post(ctx, "/result", map[string]any{"id": jobs[0], "result": Result{Data: "late"}}, nil); err == nil {
				t.Fatal("cancelled result accepted")
			}
			if operation != "revoke-device" && post(t, devices[0], "/poll", map[string]any{})["idle"] != true {
				t.Fatal("cancelled action replayed")
			}
			if post(t, devices[1], "/check", map[string]any{"id": jobs[1]})["active"] != true {
				t.Fatal("other agent's job cancelled")
			}
			g.mu.Lock()
			_, firstLease := g.leases[ids[0]]
			secondLease := g.leases[ids[1]]
			g.mu.Unlock()
			if firstLease || secondLease.Owner != agents[1] {
				t.Fatal("wrong leases cancelled")
			}
			post(t, devices[1], "/result", map[string]any{"id": jobs[1], "result": Result{Data: "completed"}})
			select {
			case err := <-done[1]:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("unrelated session blocked")
			}
			if _, err := g.Call(tokens[1], "session_release", map[string]any{"device_id": ids[1]}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
