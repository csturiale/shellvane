# 0003. Plugins

- Status: accepted
- Date: 2026-11-01

## Context

Users want Shellvane to reach the systems around their servers: cloud
inventories (AWS EC2, Google Cloud, Azure, Proxmox, Kubernetes, Ansible,
Terraform, NetBox), credential brokers (HashiCorp Vault, Teleport,
Boundary), views of what runs on a host (Docker containers, systemd
services, logs) and one-click actions (restart a service, deploy, open a web
console). Building each one into the core would make it grow without end,
and many users need only one or two.

## Decision

A plugin is a program with a manifest, run by Shellvane as a child process
that speaks JSON-RPC 2.0, one message per line, on its stdin and stdout.

- **Location**: `<data dir>/plugins/<id>/plugin.json`. Plugins are installed
  from a folder or a .zip, or from a store: an index JSON served over HTTPS
  (default `https://shellvane.com/plugins/index.json`) whose packages carry
  a SHA-256 that the download must match. Nothing is downloaded or run
  without the user installing it; built-in plugins are installed (enabled)
  one by one too.
- **Process**: Shellvane starts the plugin for each call; it reads one request,
  may send `host.exec` requests and read their answers, writes its result
  and exits. No long-running processes to supervise; calls time out after
  two minutes.
- **Manifest**: `id`, `name`, `version`, `description`, `command` (argv,
  relative to the plugin folder), `capabilities` and `settings` (fields the
  user fills in; `secret` ones are kept in the vault, never in a file).
- **Consent**: a plugin is disabled until the user enables it, after seeing
  its capabilities. It runs with the user's permissions, like the commands
  Shellvane already runs (password, certificate and proxy commands).
- **Capabilities** and their methods:
  - `inventory`: `inventory.list` returns hosts (name, address, port, user,
    group path, tags, notes); Shellvane keeps them in a group of their own,
    refreshed on demand, without duplicates.
  - `credentials`: `credentials.get` returns a password, a private key or a
    certificate for a host about to connect (the brokers).
  - `panel`: `panel.render` returns tables to show for a connected host; the
    data comes from commands the plugin asks Shellvane to run on that host over
    the existing connection (`host.exec`), so plugins never get SSH access.
  - `actions`: `actions.list` and `actions.run` add buttons to hosts; an
    action returns output, or a command to type in a new terminal.
- **Built-in plugins**: the integrations Shellvane ships (the list above) use
  the same Go interface as external ones, so they can be enabled one by one
  and an external plugin can replace any of them. They rely on the vendors'
  own CLIs (`aws`, `gcloud`, `az`, `kubectl`, `ansible-inventory`,
  `terraform`, `vault`, `tsh`, `boundary`) or on their HTTP APIs (Proxmox,
  NetBox) with a token from the vault.

## Consequences

- Any language works for external plugins, and a crashed plugin cannot take
  Shellvane down: every call is a fresh process with a time limit.
- Plugins are as trusted as the user's own scripts. WebAssembly plugins, with
  a capability-based sandbox, remain possible later behind the same
  interface.
- The protocol is documented in the guide and versioned (`protocol: 1` in
  the manifest).
