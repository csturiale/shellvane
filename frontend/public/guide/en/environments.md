# Environments {#top}

An environment starts, in order and with one click, everything you need to work on a system: a cloud login on this computer, an SSH session to a jump host with its commands, tunnels, local terminals whose kubectl goes through a tunnel, and a layout.

Typical case: a GKE cluster reachable only from a bastion. The environment logs in with `gcloud`, connects to the bastion, runs `get-credentials` there, opens a SOCKS tunnel, and opens local terminals where `kubectl` talks to the cluster through the tunnel.

[Open Environments](shellvane:view/environments)

## Steps {#steps}

Every step is optional; steps without settings are skipped.

1. **Login on this computer** — a check command and a login command.
2. **Jump host** — an SSH session to a saved host, with extra commands after its own startup script and a kubectl context for that pane.
3. **Tunnels** — saved tunnels, started after the connection; the step waits until they run.
4. **Local kubectl** — local terminals with a kubectl context, optionally through one of the tunnels.
5. **Layout** — a saved workspace to open.

The Environments panel shows each step as it runs. If one fails, the run stops there, with the reason and **Retry from here**. **Stop** closes the terminals and stops the tunnels the environment opened.

## Cloud login {#login}

The **check** command must succeed only when you are logged in, e.g. `gcloud auth print-access-token`, `aws sts get-caller-identity --profile {{profile}}`, `az account show`. If it fails, the **login** command (`gcloud auth login`, `aws sso login`, `az login`) runs in a local terminal so you can complete it (browser, code), and the environment waits until the check succeeds (up to 10 minutes). Presets fill both fields.

The check and the preparation run in your own shell (zsh, bash…), reading its startup files like a terminal does, so tools put on the PATH by `~/.zshrc` are found. While the step waits, it shows why the check does not pass yet; **Continue without checking** goes on anyway.

Each running step says what it is doing (checking the login, the preparation command, connecting the tunnels, opening terminals) and for how long. Steps without a terminal — the check, the preparation, the tunnels — run in the background, so this line is where to follow them.

When the jump host has no saved password (and no key or team login), its password is asked in the usual dialog as soon as the tunnel connects: tick **Save in vault** and the next starts are automatic. The step waits up to five minutes for the answer.

Logins needed on the jump host go in its commands, e.g. `gcloud auth login --no-launch-browser`.

## Restored sessions and more terminals {#terminals}

Every terminal an environment opens shows its badge in the header — the environment's name, and its state when it is not ready. When Shellvane restores the previous session, those terminals come back as plain shells: the environment then starts again on them by itself — login check (opening the login if needed), tunnels, kubectl route — without opening them twice. Until it is ready the badge says so, in red.

- **Login expired**: the login check runs again every 5 minutes. When it fails (for example `gcloud` asks to authenticate again) the badge and the Environments panel say *login expired*: click the badge or **Log in again and resume**.
- **One more terminal**: click a ready environment's badge, use **New terminal in this environment** in the pane menu, or the terminal button of the environment in the panel. It opens a local shell with the environment's kubectl context and tunnel (or its remote host when it has no local terminals). Open as many as you need.
- The variables the environment ran with are saved with the session, so a restore reuses them.
- **Closing the last terminal** of a started environment stops it: the tunnels it started close, but not those already running before it, started at launch or used by another started environment. A notice offers **Undo** for a few seconds (login and tunnels again, without terminals). Environments that open no terminals keep running until **Stop**. Turn it off in **Settings › Sessions › Stop an environment with its last terminal**.

## Variables {#variables}

Write `{{name}}` in any command, context or namespace. Variables are asked when the environment starts, with a default value or a list of choices, e.g. `cluster` with choices `prod-eu, prod-us`.

## kubectl through a tunnel {#kubectl}

Choose a tunnel in **Local kubectl**:

- a **dynamic (SOCKS)** tunnel: the context's cluster gets `proxy-url: socks5://…`;
- a **local** tunnel to the API server (e.g. local `6443` → `10.0.0.2:443`): the cluster's address becomes `https://127.0.0.1:6443`, keeping the original TLS name so the certificate is still verified.

As for every local pane, this is written in the pane's own kubectl file: your kubeconfig is not changed. The context must exist in your local kubeconfig: put `gcloud container clusters get-credentials …` in the **Prepare** field of step 1 and it runs at every start, after the login.

### Example: private GKE cluster behind a jump host {#gke}

The usual script (find the cluster, get its credentials, `ssh -L port:endpoint:443 jumphost`, patch the kubeconfig with `127.0.0.1` and `tls-server-name`) becomes an environment:

1. Save the jump host as a host, and a tunnel on it: **dynamic (SOCKS)**, or **local** `25901` → `<cluster endpoint>:443`.
2. Create an environment and click **GKE via jump host**: it adds the variables `project`, `region` and `cluster`, the gcloud login, the `get-credentials` preparation and the context `gke_{{project}}_{{region}}_{{cluster}}`.
3. In step 3 tick the tunnel, in step 4 choose it as the kubectl tunnel. Use **Choices** on the variables to pick the cluster when the environment starts.

Starting it logs in when needed, writes the context, opens the tunnel and opens local terminals where `kubectl`, `helm` and `k9s` reach the cluster; the address and TLS name are set in the pane's own kubectl file, so your kubeconfig keeps the real endpoint. One environment per stage (sit, uat, prd), each with its own jump host and tunnel.

## Share {#share}

In the app from termoraplus.com, the team button next to the environment shares it: members get it with its tunnels, kept up to date, and approve its commands before it runs on their computer.


The copy icon puts the environment on the clipboard as YAML, with host, tunnel and workspace **names** instead of internal IDs and without secrets. **Import** reads it back and links the names to your own hosts, tunnels and workspaces; several environments can be pasted at once, separated by lines with `---`. Tunnels imported from an OpenSSH config are named after their host and ports (`bastion SOCKS 1080`, `bastion L 8080 → db:5432`), so environments can refer to them.
