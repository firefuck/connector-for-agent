#!/usr/bin/env python3
"""Black-box RelayDesk checks with the official MCP Python SDK.

    python -m pip install -r integration/requirements-mcp.txt
    python integration/test_mcp_sdk.py --binary dist/relaydesk-linux-amd64

Starts its own localhost gateway and connector with temporary state and synthetic
credentials. This does not connect to Hermes, user agents, or a real remote PC.
GUI automation and cancellation are tested separately.
"""

import argparse
import asyncio
import base64
from contextlib import asynccontextmanager, contextmanager
from datetime import timedelta
import json
from pathlib import Path
import secrets
import socket
import subprocess
import os
import tempfile
import time

import httpx
from importlib.metadata import version
from jsonschema.validators import validator_for
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client
from mcp.client.streamable_http import streamablehttp_client


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def free_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


@contextmanager
def fixture(binary):
    """Own every process, credential, and path used by this integration test."""
    with tempfile.TemporaryDirectory(prefix="relaydesk-mcp-sdk-") as directory:
        root = Path(directory)
        base = f"http://127.0.0.1:{free_port()}"
        admin = secrets.token_hex(32)
        children = []
        with (root / "gateway.log").open("wb") as gateway_log, (root / "connector.log").open("wb") as connector_log:
            try:
                gateway = subprocess.Popen(
                    [str(binary), "gateway", "--listen", base.removeprefix("http://"), "--state", str(root / "state.json")],
                    env={**os.environ, "RELAYDESK_ADMIN_TOKEN": admin},
                    stdin=subprocess.DEVNULL,
                    stdout=gateway_log,
                    stderr=subprocess.STDOUT,
                )
                children.append(gateway)
                with httpx.Client(base_url=base, timeout=5, trust_env=False) as http:
                    deadline = time.monotonic() + 10
                    while True:
                        require(gateway.poll() is None, "test gateway exited during startup")
                        try:
                            if http.get("/health").status_code == 200:
                                break
                        except httpx.TransportError:
                            pass
                        require(time.monotonic() < deadline, "test gateway did not become healthy")
                        time.sleep(0.05)

                    def admin_post(path, payload):
                        response = http.post(path, json=payload, headers={"Authorization": f"Bearer {admin}"})
                        require(response.status_code == 200, f"admin {path} returned HTTP {response.status_code}")
                        return response.json()

                    pairing = admin_post("/admin/pair", {"name": "sdk-test-connector"})
                    config = root / "connector.json"
                    paired = subprocess.run(
                        [str(binary), "pair", "--gateway", base, "--name", "sdk-test-connector", "--config", str(config)],
                        env={**os.environ, "RELAYDESK_PAIR_CODE": pairing["code"]},
                        stdin=subprocess.DEVNULL,
                        capture_output=True,
                        timeout=10,
                        check=False,
                    )
                    require(paired.returncode == 0, "CLI pairing failed")
                    device_id = json.loads(config.read_text())["id"]
                    token = admin_post("/admin/agent", {"name": "sdk-full", "devices": [device_id], "tools": ["*"]})["token"]
                    restricted = admin_post("/admin/agent", {"name": "sdk-restricted", "devices": [device_id], "tools": ["devices_list"]})["token"]
                    connector = subprocess.Popen(
                        [str(binary), "connector", "--headless", "--config", str(config)],
                        stdin=subprocess.DEVNULL,
                        stdout=connector_log,
                        stderr=subprocess.STDOUT,
                    )
                    children.append(connector)
                    deadline = time.monotonic() + 10
                    while True:
                        require(connector.poll() is None, "test connector exited during startup")
                        devices = admin_post("/admin/list", {})["devices"]
                        if any(d["id"] == device_id and not d["last_seen"].startswith("0001-") for d in devices):
                            break
                        require(time.monotonic() < deadline, "test connector did not become online")
                        time.sleep(0.05)
                    yield {"binary": binary, "base": base, "root": root, "device_id": device_id, "token": token, "restricted": restricted}
            finally:
                for process in reversed(children):
                    if process.poll() is None:
                        process.terminate()
                        try:
                            process.wait(timeout=5)
                        except subprocess.TimeoutExpired:
                            process.kill()
                            process.wait(timeout=5)


def http_client_factory(headers=None, timeout=None, auth=None):
    # All test traffic is loopback, independent of shell proxy configuration.
    return httpx.AsyncClient(headers=headers, timeout=timeout or 30, auth=auth, follow_redirects=False, trust_env=False)


@asynccontextmanager
async def session_for(mode, test, token):
    if mode == "http":
        async with streamablehttp_client(
            test["base"] + "/mcp",
            headers={"Authorization": f"Bearer {token}"},
            timeout=15,
            httpx_client_factory=http_client_factory,
        ) as (read, write, _):
            async with ClientSession(read, write, read_timeout_seconds=timedelta(seconds=15)) as session:
                yield session
    else:
        params = StdioServerParameters(
            command=str(test["binary"]),
            args=["mcp", "--gateway", test["base"]],
            env={**os.environ, "RELAYDESK_AGENT_TOKEN": token},
        )
        async with stdio_client(params) as (read, write):
            async with ClientSession(read, write, read_timeout_seconds=timedelta(seconds=15)) as session:
                yield session


def text_json(result):
    require(not result.isError, "tool failed: " + " ".join(getattr(c, "text", "") for c in result.content))
    texts = [block.text for block in result.content if block.type == "text"]
    require(len(texts) == 1, "expected one JSON text result")
    return json.loads(texts[0])


async def exercise_transport(mode, test):
    async with session_for(mode, test, test["token"]) as session:
        initialized = await session.initialize()
        require(initialized.protocolVersion == "2025-03-26", "unexpected negotiated MCP version")
        require(initialized.serverInfo.name == "relaydesk", "unexpected MCP server identity")
        await session.send_ping()
        listing = await session.list_tools()
        schemas = {}
        for tool in listing.tools:
            validator = validator_for(tool.inputSchema)
            validator.check_schema(tool.inputSchema)
            schemas[tool.name] = validator(tool.inputSchema)
            if tool.outputSchema is not None:
                validator_for(tool.outputSchema).check_schema(tool.outputSchema)
        require({"devices_list", "session_acquire", "session_release", "files_read", "files_write", "system_exec"} <= schemas.keys(), "required tools missing")

        async def call(name, arguments):
            schemas[name].validate(arguments)
            return text_json(await session.call_tool(name, arguments))

        devices = await call("devices_list", {})
        require(any(d["id"] == test["device_id"] for d in devices), "paired connector not discoverable")
        arguments = {"device_id": test["device_id"]}
        require((await call("session_acquire", arguments))["lease_seconds"] == 300, "lease acquisition failed")
        try:
            path = test["root"] / f"roundtrip-{mode}-世界.txt"
            contents = "RelayDesk — halo dunia 👋 世界\n".encode("utf-8")
            wrote = await call("files_write", {**arguments, "path": str(path), "base64": base64.b64encode(contents).decode("ascii")})
            require(wrote["bytes"] == len(contents), "Unicode file write length mismatch")
            read = await call("files_read", {**arguments, "path": str(path)})
            require(base64.b64decode(read["base64"]) == contents, "Unicode file roundtrip mismatch")
            executed = await call("system_exec", {**arguments, "command": "echo relaydesk-sdk-ok", "timeout_seconds": 5})
            require(executed["exit_code"] == 0 and executed["output"].strip() == "relaydesk-sdk-ok", "command result mismatch")
        finally:
            require(await call("session_release", arguments) == "released", "lease release failed")
        after_release = await session.call_tool("system_exec", {**arguments, "command": "echo forbidden-without-lease"})
        require(after_release.isError, "command without lease accepted")
        print(f"PASS {mode}: initialize, ping, {len(schemas)} valid tool schemas, discovery, lease, Unicode files, exec, release", flush=True)

    async with session_for(mode, test, test["restricted"]) as session:
        await session.initialize()
        listed = await session.list_tools()
        require([tool.name for tool in listed.tools] == ["devices_list"], "restricted tool list leaked forbidden tools")
        result = await session.call_tool("system_exec", {"device_id": test["device_id"], "command": "echo forbidden"})
        require(result.isError and any("forbidden" in getattr(c, "text", "") for c in result.content), "cached forbidden tool call was not rejected")
        print(f"PASS {mode}: restricted tool discovery and server-side denial", flush=True)


async def run_tests(test):
    await exercise_transport("http", test)
    await exercise_transport("stdio", test)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, default=Path(__file__).resolve().parents[1] / "dist" / "relaydesk-linux-amd64")
    args = parser.parse_args()
    binary = args.binary.resolve()
    require(binary.is_file(), f"binary missing: {binary}; run packaging/build.sh first")
    require(version("mcp") == "1.30.0", "install integration/requirements-mcp.txt (official SDK mcp==1.30.0 required)")
    print(f"Official MCP Python SDK {version('mcp')}; jsonschema {version('jsonschema')}", flush=True)
    with fixture(binary) as test:
        asyncio.run(run_tests(test))
    print("PASS temporary gateway/connector cleanup; no user agents or remote PCs contacted", flush=True)


if __name__ == "__main__":
    main()
