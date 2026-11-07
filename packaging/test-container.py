#!/usr/bin/env python3
"""Build and smoke-test an isolated gateway container; never contacts user PCs.

Requires Docker, Compose and Python 3. Uses only a temporary named volume and a
random loopback port; all resources are removed, including after test failure.
Tokens exist only in process memory and the test container's environment.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor
import json
import os
from pathlib import Path
import secrets
import subprocess
import time
import urllib.error
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", default="relaydesk-container-test")
    parser.add_argument("--skip-build", action="store_true")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    suffix = secrets.token_hex(6)
    name = "relaydesk-smoke-" + suffix
    volume = name + "-data"
    token = secrets.token_hex(32)
    env = dict(os.environ, RELAYDESK_ADMIN_TOKEN=token)

    def docker(*argv, **kwargs):
        return subprocess.run(["docker", *argv], check=True, text=True,
                              env=env, cwd=root, **kwargs)

    # Check the exact Compose definition used by operators without printing its
    # interpolated credentials.
    config = json.loads(docker("compose", "config", "--format", "json",
                               capture_output=True).stdout)
    service = config["services"]["gateway"]
    assert service["read_only"] is True
    assert "ALL" in service["cap_drop"]
    assert "no-new-privileges:true" in service["security_opt"]
    assert service["ports"][0]["host_ip"] == (env.get("RELAYDESK_BIND_IP") or "127.0.0.1")
    if not args.skip_build:
        docker("build", "-t", args.image, ".")
    assert json.loads(docker("image", "inspect", args.image,
                              capture_output=True).stdout)[0]["Config"]["User"] == "10001:10001"

    base = ""

    def request(path, data=None, auth=None, expected=200):
        body = None if data is None else json.dumps(data).encode()
        headers = {"Content-Type": "application/json", "Accept": "application/json, text/event-stream"}
        if auth:
            headers["Authorization"] = "Bearer " + auth
        req = urllib.request.Request(base + path, data=body, headers=headers)
        try:
            with urllib.request.urlopen(req, timeout=5) as response:
                status, raw = response.status, response.read()
        except urllib.error.HTTPError as response:
            status, raw = response.code, response.read()
        assert status == expected, f"{path}: HTTP {status}, expected {expected}"
        return json.loads(raw) if raw else None

    def ready():
        end = time.monotonic() + 15
        while True:
            try:
                assert request("/health")["status"] == "ok"
                return
            except (OSError, AssertionError):
                if time.monotonic() >= end:
                    raise RuntimeError("container health endpoint did not become ready") from None
                time.sleep(0.1)

    try:
        docker("volume", "create", volume, capture_output=True)
        docker("run", "-d", "--name", name, "--read-only", "--cap-drop=ALL",
               "--security-opt=no-new-privileges:true", "--tmpfs", "/tmp:size=16m",
               "--mount", f"type=volume,src={volume},dst=/data",
               "-p", "127.0.0.1::8787", "-e", "RELAYDESK_ADMIN_TOKEN", args.image,
               capture_output=True)
        port = docker("port", name, "8787/tcp", capture_output=True).stdout.strip().rsplit(":", 1)[1]
        base = "http://127.0.0.1:" + port
        ready()
        assert docker("exec", name, "id", "-u", capture_output=True).stdout.strip() == "10001"
        # An immutable root filesystem still permits state writes in /data.
        read_only = subprocess.run(["docker", "exec", name, "touch", "/relaydesk-smoke"],
                                   capture_output=True, env=env)
        assert read_only.returncode != 0
        request("/admin/list", {}, secrets.token_hex(32), expected=401)
        pairing = request("/admin/pair", {"name": "container-smoke"}, token)
        device = request("/pair", {"code": pairing["code"]})
        request("/pair", {"code": pairing["code"]}, expected=403)
        agent = request("/admin/agent", {"name": "smoke-agent", "devices": [device["id"]],
                                         "tools": ["devices_list"]}, token)
        request("/poll", {"platform": "smoke"}, device["token"])
        listing = request("/mcp", {"jsonrpc": "2.0", "id": 1, "method": "tools/list"}, agent["token"])
        assert [tool["name"] for tool in listing["result"]["tools"]] == ["devices_list"]
        state = docker("exec", name, "cat", "/data/state.json", capture_output=True).stdout
        for secret in (token, device["token"], agent["token"], pairing["code"]):
            assert secret not in state, "plaintext credential in persisted state"
        assert docker("exec", name, "stat", "-c", "%a", "/data/state.json",
                      capture_output=True).stdout.strip() == "600"
        # Check HEALTHCHECK independently instead of waiting its full interval.
        docker("exec", name, "wget", "-Y", "off", "-q", "-O", "/dev/null", "http://127.0.0.1:8787/health")
        worker = request("/admin/agent", {"name": "pending-job", "devices": [device["id"]],
                                          "tools": ["session_acquire", "system_exec"]}, token)

        def call(request_id, tool, arguments):
            return request("/mcp", {"jsonrpc": "2.0", "id": request_id, "method": "tools/call",
                                    "params": {"name": tool, "arguments": arguments}}, worker["token"])

        acquired = call(2, "session_acquire", {"device_id": device["id"]})
        assert not acquired["result"].get("isError", False)
        # Simulate delivery to a connector, without executing a host command.
        # SIGTERM must resolve the waiting MCP call before exiting.
        with ThreadPoolExecutor(max_workers=1) as pool:
            pending = pool.submit(call, 3, "system_exec", {"device_id": device["id"],
                                                          "command": "exit 99", "timeout_seconds": 120})
            end = time.monotonic() + 3
            while True:
                job = request("/poll", {}, device["token"])
                if job.get("id"):
                    assert job["action"] == "system_exec"
                    break
                assert time.monotonic() < end, "no pending job was dispatched"
                time.sleep(0.01)
            docker("stop", "--timeout", "10", name, capture_output=True)
            assert pending.result(timeout=5)["result"]["isError"] is True
        assert docker("inspect", "--format", "{{.State.ExitCode}}", name,
                      capture_output=True).stdout.strip() == "0", "SIGTERM did not exit cleanly"
        docker("start", name, capture_output=True)
        # Docker can allocate a new random published port when starting again.
        port = docker("port", name, "8787/tcp", capture_output=True).stdout.strip().rsplit(":", 1)[1]
        base = "http://127.0.0.1:" + port
        ready()
        assert request("/poll", {}, device["token"]).get("idle") is True, "job replayed after restart"
        listing = request("/admin/list", {}, token)
        assert any(d["id"] == device["id"] for d in listing["devices"])
        assert any(a["id"] == agent["id"] for a in listing["agents"])
        request("/admin/revoke", {"kind": "device", "id": device["id"]}, token)
        request("/poll", {}, device["token"], expected=401)
        print("PASS: container build, Compose policy, health, non-root/read-only, auth, pairing,")
        print("      hashed state, restart persistence, revocation and clean SIGTERM with pending MCP")
    finally:
        subprocess.run(["docker", "rm", "-f", name], env=env, capture_output=True)
        subprocess.run(["docker", "volume", "rm", volume], env=env, capture_output=True)


if __name__ == "__main__":
    main()
