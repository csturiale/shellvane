# Plugins {#top}

Plugins bring in hosts from the systems that already know them: clouds, Kubernetes, configuration management, virtualization and source-of-truth tools. Set one up once; every **Refresh hosts** keeps a host group in step with the source. Other plugins add [panels](shellvane:topic/plugins#panels) to connected hosts.

[Open Plugins](shellvane:view/plugins)

## Install {#install}

Nothing runs until you install it. The Plugins view lists:

- the plugins **included with Shellvane**: **Install** enables one, **Uninstall** disables it again (delete its configurations first);
- the **store**: plugins published at an address (by default `https://shellvane.com/plugins/index.json`; any other store can be set). **Install** downloads the package, checks its SHA-256 against the store's and installs it; **Update** appears when the store has a newer version;
- **Choose a .zip…** installs a plugin's `.zip` (with a `plugin.json`) picked with the file chooser, e.g. one someone shared or one you wrote; **Install from file** takes a typed path, also of a plugin folder while you write one.

The **+** next to the title brings back this catalog, at the store.

External plugins run on this computer with your permissions, like your own scripts: install only those you trust.

## Available plugins {#list}

| Plugin | Reads | Needs |
| --- | --- | --- |
| AWS EC2 | running instances, grouped by region; tags become tags | `aws` CLI (profiles and SSO) |
| Google Cloud Compute | running instances of a project, by zone; labels become tags | `gcloud` |
| Azure VMs | running VMs, by resource group | `az` |
| Kubernetes nodes | the nodes of a context, with their roles | `kubectl` |
| Ansible inventory | static or dynamic inventories, keeping the group tree, `ansible_host`, `ansible_user`, `ansible_port` | `ansible-inventory` |
| Terraform state | machines of AWS, Google, Azure, DigitalOcean, Hetzner, Linode, Vultr | `terraform` |
| Proxmox VE | running VMs and containers, by node, addresses from the guest agent | an API token |
| NetBox | devices and VMs with a primary IP, by site or cluster, with an optional filter | an API token |

A plugin whose tool is missing says so; tools installed with Homebrew or in `~/.local/bin` are found on macOS too (Shellvane reads your login shell's `PATH`).

## Set up and refresh {#setup}

1. Click **+** and pick a plugin. Give it a name: the same plugin can be added several times, e.g. two AWS profiles.
2. Fill in its settings. **Address** chooses the public or the private address; **SSH user** is set on every host.
3. **Refresh hosts** saves and fills the group named after the plugin, with subgroups by region, zone, node or site.

A refresh updates name, address, user and port from the source, adds new machines and removes those that are gone. What you change on the hosts stays: favorites, keys, colors, jump hosts and every other setting. Deleting the plugin keeps its hosts as ordinary ones.

## Secrets {#secrets}

API tokens are kept in the [vault](shellvane:topic/vault), never in the settings: the vault must be unlocked to save or use them. Leave a secret field empty to keep the saved value. For a Proxmox server with a self-signed certificate, paste its SHA-256 **fingerprint**: only that certificate is accepted.

## Docker, services and logs {#panels}

Once installed, on a connected host the puzzle button in the pane header (or **Docker, services and logs** in the pane's menu) opens panels that run over the same SSH connection, without installing anything on the server:

- **Docker**: the containers, running ones first, with **Logs** and **Shell** (opened in a new pane), **Start**, **Stop** and **Restart** (asking first).
- **systemd**: failed and running services, with **Status** and **Journal** in a new pane, and **Start**, **Stop**, **Restart** typed there with `sudo`, so it can ask for its password.
- **Logs**: the recent errors from the journal (or `/var/log/syslog`, `/var/log/messages`), and **Follow the log** in a new pane.

The filter narrows the rows. Container and unit names are checked before they reach a command.

## Credentials from a broker {#credentials}

Credential plugins give the login at every connection, so no long-lived password or key is stored. Install and configure one, then choose it in the host's or group's advanced options under **Credentials from a plugin or a team** (groups pass it on to their hosts).

- **HashiCorp Vault**: with an **SSH role**, Shellvane makes a new key for each connection and Vault's SSH secrets engine signs it (the certificate is valid for the host's user); with a **KV secret path** (e.g. `secret/data/ssh/{{host}}`) it reads `password` (and `username`) from the secret. The token is kept in Shellvane's vault; Enterprise namespaces and a pinned certificate are supported.
- **CyberArk** (Privileged Access Manager): with an **Application ID**, Shellvane asks the **Central Credential Provider** (`/AIMWebService`) for the account, by **Object** name (e.g. `{{host}}-{{user}}`) or by the host's address and user, optionally within a **Safe**; a **client certificate** and key (PEM files) are sent when the provider requires one. Without an Application ID it logs on to the **PVWA** with your **CyberArk user and password** (CyberArk, LDAP, RADIUS or Windows authentication; the password is kept in Shellvane's vault), finds the account whose address and user match the host, retrieves its password or SSH key with the **reason** you set. **MFA**: with an authenticator push (RADIUS) the connection waits up to two minutes while you approve it on the phone; when the PVWA asks for a code, Shellvane asks you for it during the connection (or, with **Code** set to `append`, together with the password, joined by the separator, `,` by default). The PVWA session is then kept in memory and reused, so MFA is asked once and not at every connection, until the PVWA ends it or it stays unused for 15 minutes; it is never saved. With **SAML** or **OIDC** authentication (Entra ID, Okta, CyberArk Identity…) no user or password is set in Shellvane: at the first connection a sign-in window opens on the PVWA's sign-in page (`/PasswordVault/v10/logon/saml` or `…/oidc`, or the **Sign-in page** you set); sign in there with your company account and its MFA (the authenticator code or approval), and the window closes by itself. Shellvane exchanges the SAML response for a PVWA session, or takes over the session of the PVWA web app once the PVWA accepts it; that session is reused like the others. Accounts that need dual control or a ticket must be approved in CyberArk first.
- **CyberArk Conjur** (Open Source, Enterprise/Secrets Manager, Cloud): Shellvane authenticates with the **login** (`host/…` or a user) and its **API key** (kept in the vault; another **authenticator** such as `authn-ldap/service` can be set), then reads the host's **private key variable** or **password variable** and, if set, the **user variable**, e.g. `ssh/{{host}}/password`.
- **Teleport**: after `tsh login`, Shellvane uses the certificate and key of the profile and connects through `tsh proxy ssh`, on the nodes' port (usually 3022). If you are not logged in, the error says which `tsh login` to run.
- **Boundary**: Shellvane authorizes a session on the target, uses the credentials Boundary brokers (user and password, or a key) and connects through `boundary connect` (it needs `nc`).

If the plugin's token is in a locked vault, Shellvane opens the vault first (keychain, Touch ID or the master password). External plugins can broker logins too, with `credentials.get`.

## Write a plugin {#write}

A plugin is a folder with a `plugin.json` and a program in any language:

```json
{
  "id": "disk-usage", "name": "Disk usage", "version": "1.0.0",
  "description": "Filesystems of the host.",
  "protocol": 1,
  "command": ["python3", "main.py"],
  "capabilities": ["panel", "actions"],
  "settings": [{ "key": "token", "label": "API token", "secret": true, "required": true }]
}
```

Shellvane starts `command` in the plugin's folder for each call and writes one JSON-RPC 2.0 request on its standard input; the plugin writes its answer on standard output, one JSON message per line, and exits. Settings marked `secret` are kept in the vault and passed in `params.settings`.

| Method | Params | Result |
| --- | --- | --- |
| `inventory.list` | `settings` | `{"hosts": [{"key", "name", "hostname", "port", "user", "group": [], "tags": [], "notes"}]}` |
| `panel.render` | `settings`, `target` | `{"title", "tables": [{"title", "columns", "rows", "actions"}], "text"}` |
| `actions.list` | `settings`, `target` | `{"actions": [{"id", "label", "confirm"}]}` |
| `actions.run` | `settings`, `target`, `action`, `item` | `{"output"}`, or `{"command"}` to type in a new terminal on the host |
| `credentials.get` | `settings`, `target` | `{"password"}`, or `{"privateKey", "certificate"}` |

While it works, a panel or an action can ask Shellvane to run a command on the host over the existing connection: it writes `{"jsonrpc":"2.0","id":"x","method":"host.exec","params":{"command":"df -h"}}` and reads `{"id":"x","result":{"output":"…","code":0}}`. Plugins never get SSH access themselves. A call that takes more than two minutes is stopped; an error answer (`{"error":{"code":1,"message":"…"}}`) is shown to the user.

The repository has a complete example in `examples/plugins/disk-usage`, and `scripts/plugin-index.sh` packages plugins and writes a store index (`{"plugins":[{"id","name","version","description","url","sha256",…}]}`) to publish on any HTTPS server.
