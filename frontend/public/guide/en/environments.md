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

### Login password from a credential plugin {#login-secret}

When the sign-in page that the login opens needs a password kept in a password manager, set **Password for the login**. The field appears once a credential plugin is configured in **Plugins** (CyberArk, Conjur, HashiCorp Vault…) or a login is shared in a team. Choose the plugin configuration and the account **as it is in the vault**: its **user** (for example the directory account `a123456`) and, when it is not the plugin's own, its **Safe**; the **address** is optional and needed only when several accounts share the user. This account need not be the one you sign in with: put the sign-in email in the login command, e.g. `gcloud auth login alice.admin@example.com --update-adc`, and the sign-in page opens on that account. Variables work in these fields.

When the check fails, Shellvane first reads the account's password from the plugin, signing in to it if needed (for CyberArk, with its MFA or the browser sign-in; the session is then reused), copies it to the clipboard for 45 seconds (configurable in Settings) and only then runs the login: paste the password in the sign-in page. The password is never saved, shown or written in the terminal, and the clipboard is cleared afterwards unless you copied something else. If the password cannot be read, the step says why and the login runs anyway, so you can type it. While the password is being read, **Skip the password** goes on without it (for example when the vault opened in your browser and you prefer to copy the password from there).

**Open the sign-in page in a private window** (on by default with the Google Cloud and Azure presets) makes `gcloud`, `az` and `aws sso` open their page in an incognito or InPrivate window of Chrome, Edge, Brave or Firefox, through the `BROWSER` variable of the login terminal; with **Automatic** that is your default browser, or the first of these when the default (Safari) has no private window: a Google or Microsoft session already open in your browser then cannot sign in with the wrong account. Safari has no private window that can be opened this way, so another installed browser is used; without any, the usual browser opens.

In **Settings → Environment logins** choose the browser of sign-in pages (automatic, one of the list, or a **custom command** with `%s` for the link, e.g. a browser with a dedicated profile) and how many seconds a login password stays on the clipboard (45 by default). Each environment can pick another browser in its step 1.

Not every tool reads `BROWSER`: many written in Go (kubelogin, `vault login -method=oidc`, `tsh login`) open the system's browser directly, and some only print the link (`gcloud auth login --no-launch-browser`, device codes). For those, tick **Open the sign-in links printed in the login terminal**: Shellvane recognizes OAuth, OpenID Connect, SAML and device-code links in the terminal's output and opens each once in the chosen browser (at most 5 per terminal). Use the tool's option to print the link instead of opening it, or the page opens twice.

**Use this browser in every local terminal** (Settings) applies the same choice to the local terminals you open by hand, with its own **private window** and **open sign-in links** options; an environment's login keeps the environment's choice.

### Different Google accounts at the same time {#gcloud}

gcloud keeps its account, project and application default credentials in one configuration folder, so by default every terminal uses the same Google account. Give an environment a **gcloud profile** (e.g. `prod-admin`) and its check, login, preparation and local terminals (with kubectl and its GKE plugin) use a configuration folder of their own (`CLOUDSDK_CONFIG`): the environment "prod" can stay signed in as `alice-admin@…` while "dev" uses another account, both open together. Environments with the same profile share the login. The Google Cloud preset logs in with `gcloud auth login --update-adc`, which also refreshes the application default credentials of the profile.

## Restored sessions and more terminals {#terminals}

Every terminal an environment opens shows its badge in the header — the environment's name, and its state when it is not ready. When Shellvane restores the previous session, those terminals come back as plain shells: the environment then starts again on them by itself — login check (opening the login if needed), tunnels, kubectl route — without opening them twice. Until it is ready the badge says so, in red.

- **Login expired**: the login check runs again every 5 minutes. When it fails (for example `gcloud` asks to authenticate again) the badge and the Environments panel say *login expired*: click the badge or **Log in again and resume**.
- **One more terminal**: click a ready environment's badge, use **New terminal in this environment** in the pane menu, or the terminal button of the environment in the panel. It opens a local shell with the environment's kubectl context and tunnel (or its remote host when it has no local terminals). Open as many as you need. From the command palette, **kubectl on GKE shop prd** (any environment with a local kubectl context) does the same, and starts the environment first when it is not up.
- The variables the environment ran with are saved with the session, so a restore reuses them.
- **Closing the last terminal** of a started environment stops it: the tunnels it started close, but not those already running before it, started at launch or used by another started environment. A notice offers **Undo** for a few seconds (login and tunnels again, without terminals). Environments that open no terminals keep running until **Stop**. Turn it off in **Settings › Sessions › Stop an environment with its last terminal**.

## Variables {#variables}

Write `{{name}}` in any command, context or namespace. Variables are asked when the environment starts, with a default value or a list of choices, e.g. `cluster` with choices `prod-eu, prod-us`.


### Personal values {#personal}

Tick **personal** on a variable whose value belongs to each person, such as the sign-in email (`gcloud auth login {{googleEmail}} --update-adc`) or the user and safe in the vault (the **Password for the login** fields accept variables too). A personal value is asked the first time, then remembered on this computer and not asked again; **My values** (the person icon of the environment) changes it. It is never stored in the environment: exports and team shares carry only the variable's name, so everyone in the team uses the same environment with their own values, and receives its updates without a local copy to keep in sync.

Next to each personal value, choose where it is remembered:

- **Remember everywhere** (the default): the value is kept by name, and every environment, [snippet](shellvane:topic/snippets#variables) and [procedure](shellvane:topic/procedures#placeholders) that uses the same name gets it. Type `googleEmail` once and all your GKE environments have it.
- **Only here**: this environment keeps its own value, which wins over the one used everywhere. Use it for a second account, e.g. `name@client.com` for one customer's clusters while the others keep `name@company.com`. Choosing **Remember everywhere** again drops it.

When environments use personal values you have not given yet (for example after joining a team), the panel lists them and **Fill them in** asks them all at once.

**Settings › Personal values** lists the values kept everywhere: change, add or remove them there. Deleting an environment forgets its own values only.

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

In the app from shellvane.com, the team button next to the environment shares it: members get it with its tunnels, kept up to date, and approve its commands before it runs on their computer.


The copy icon puts the environment on the clipboard as YAML, with host, tunnel and workspace **names** instead of internal IDs and without secrets. **Import** reads it back and links the names to your own hosts, tunnels and workspaces; several environments can be pasted at once, separated by lines with `---`. Tunnels imported from an OpenSSH config are named after their host and ports (`bastion SOCKS 1080`, `bastion L 8080 → db:5432`), so environments can refer to them.
