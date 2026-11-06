#!/usr/bin/env python3
"""Example Shellvane plugin: a panel with the host's filesystems.

Shellvane starts the plugin for each call and writes one JSON-RPC 2.0
request on stdin. The plugin may ask Shellvane to run commands on the host
(host.exec) before writing its answer on stdout, one message per line.
"""
import json
import sys


def send(message):
    sys.stdout.write(json.dumps(message) + "\n")
    sys.stdout.flush()


def host_exec(command):
    """Runs a command on the host over Shellvane's SSH connection."""
    send({"jsonrpc": "2.0", "id": "exec", "method": "host.exec", "params": {"command": command}})
    reply = json.loads(sys.stdin.readline())
    if "error" in reply:
        raise RuntimeError(reply["error"]["message"])
    return reply["result"]["output"], reply["result"]["code"]


def panel():
    out, code = host_exec("df -hP -x tmpfs -x devtmpfs -x squashfs -x overlay 2>/dev/null || df -hP")
    rows = []
    for line in out.splitlines()[1:]:
        f = line.split()
        if len(f) >= 6:
            rows.append([f[5], f[0], f[1], f[2], f[3], f[4]])
    return {
        "title": "Disk usage",
        "tables": [{
            "title": "Filesystems",
            "columns": ["Mounted on", "Device", "Size", "Used", "Free", "Use%"],
            "rows": rows,
            "actions": [{"id": "largest", "label": "Largest folders"}],
        }],
    }


def run(action, item):
    if action == "largest":
        # A command for a new terminal on the host.
        safe = item.replace("'", "")
        return {"command": "sudo du -xh --max-depth=2 '%s' 2>/dev/null | sort -rh | head -n 30" % safe}
    raise ValueError("unknown action " + action)


def main():
    request = json.loads(sys.stdin.readline())
    params = request.get("params") or {}
    try:
        if request["method"] == "panel.render":
            result = panel()
        elif request["method"] == "actions.list":
            result = {"actions": []}
        elif request["method"] == "actions.run":
            result = run(params.get("action"), params.get("item", ""))
        else:
            raise ValueError("unsupported method " + request["method"])
        send({"jsonrpc": "2.0", "id": request["id"], "result": result})
    except Exception as e:  # reported to the user by Shellvane
        send({"jsonrpc": "2.0", "id": request["id"], "error": {"code": 1, "message": str(e)}})


if __name__ == "__main__":
    main()
