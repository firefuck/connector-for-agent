# RelayDesk project instructions

Read README.md, docs/STATUS.md, and docs/CODEX_HANDOFF.md first. Do not use Context.dev. Keep user communication in casual Indonesian. Preserve the two existing remote agents and existing WireGuard/CHR network topology. Never claim GUI, RDP, Wayland or installer behavior was tested without running on those systems. No production deployment or credentials are present.

Use gofmt, go test -race ./..., go vet ./..., and packaging/build.sh after meaningful code changes. The gateway handles powerful host-control commands: preserve authentication, per-device authorization, exclusive leases, bounded bodies, no automatic replay, token redaction, and cancellation. Keep tests focused on meaningful behavior. Avoid adding a second AI agent or requiring a model on each PC.
