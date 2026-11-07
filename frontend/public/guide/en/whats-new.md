# What's new {#top}

## This version {#current}

**Connections**
- Every key is tried in turn (the host's, the agent's, those in `~/.ssh`); choose the authentication methods and their order per host or group, or offer only the host's key: [Authentication](shellvane:topic/auth)
- **Telnet** (`telnet://host`) and **serial consoles** (`serial://COM3?baud=115200`) from quick connect and the palette: [Connecting](shellvane:topic/connecting)
- **Mosh**, **remote desktop (RDP)** and **VNC** with the system's clients, **X11 forwarding**, host reachability, **Wake-on-LAN** and **port knocking**: [Connecting](shellvane:topic/connecting)
- Initial directory, environment variables and remote command per host: [Connecting](shellvane:topic/connecting)
- A **command before connecting**, with presets for **AWS EC2 Instance Connect** and **Google Cloud OS Login**: [Jump hosts and proxies](shellvane:topic/jump-proxy#preconnect)
- Connection diagnostics with the handshake of the last attempts, and an anonymized support bundle: [Privacy and security](shellvane:topic/security)

**Hosts**
- **Export a group**, with or without its hosts, and import it on another computer: [Hosts](shellvane:topic/hosts)
- Tags, favorites, recent hosts, duplicate, connect to a whole group, Markdown notes: [Hosts](shellvane:topic/hosts)
- Import from MobaXterm, SecureCRT, WinSCP, FileZilla, Tabby, mRemoteNG and Termius, besides OpenSSH, PuTTY and KiTTY: [Import](shellvane:topic/import)
- **End-to-end encrypted sync** between computers through a shared folder, a WebDAV server, a Git repository or an S3 bucket: [Sync](shellvane:topic/sync)

**Terminal**
- **Files through the terminal** with ZMODEM: `sz` saves in Downloads, `rz` opens the file chooser: [Terminal](shellvane:topic/terminal#zmodem)
- **Move a tab to its own window**, sessions still connected: [Terminal](shellvane:topic/terminal#detach)
- The side panel can be **hidden and resized** for more room for terminals: [Terminal](shellvane:topic/terminal#panel)
- A **+** after the tabs opens a new tab on a local shell, a host (with search) or a typed address: [Terminal](shellvane:topic/terminal#splits)
- **Context menus** on tabs, panes, hosts and groups (close others, to the left, to the right, all…), and detached tabs can **move back** to the main window: [Terminal](shellvane:topic/terminal#menus)
- An **interface size** setting for the whole app, separate from the terminal font; shortcuts are shown as ⌘ on macOS: [Terminal](shellvane:topic/terminal#appearance)
- Custom **color schemes** with an editor and importers (iTerm2, Windows Terminal, Base16, Alacritty, kitty, Ghostty), matched app colors: [Terminal](shellvane:topic/terminal)
- Desktop **notifications** from programs (OSC 9/777) and when long commands end, **inline images** (Sixel), the **server's load** on its pane: [Terminal](shellvane:topic/terminal)
- Session **logs** and **recordings** with a player, search, maximize, per-pane zoom: [Terminal](shellvane:topic/terminal)
- A **broadcast bar** and pane selection by group or tag: [Broadcast](shellvane:topic/broadcast)
- **Snippets** imported from shell scripts or lists of commands, and exported as a file: [Snippets](shellvane:topic/snippets)
- **Runbooks**: steps on many hosts with conditions, confirmations and a results grid: [Runbooks](shellvane:topic/runbooks)

**Team**
- A **team vault**: logins and notes shared end-to-end encrypted through a shared folder, WebDAV, Git or S3, used by hosts directly, and **shared host groups** kept up to date on every member's computer (in the app from shellvane.com)

**Plugins**
- **Hosts from AWS, Google Cloud, Azure, Kubernetes, Ansible, Terraform, Proxmox and NetBox**, refreshed into a group, tokens in the vault; **Docker, systemd and logs** panels on connected hosts; install only what you need, from the **store** or a file, or write your own; logins from **HashiCorp Vault, CyberArk, Conjur, Teleport and Boundary**: [Plugins](shellvane:topic/plugins)

**Files and tunnels**
- **Preview** files, **compare and sync folders**, resume transfers, edit on this computer: [Files](shellvane:topic/files)
- Ports listening on the server, forwarded in one click, and tunnels to **Unix sockets**: [Tunnels](shellvane:topic/tunnels)

**Keys and vault**
- Keys on **hardware tokens** (YubiKey, smart cards, HSMs) through PKCS#11, with the PIN asked at login: [SSH keys](shellvane:topic/keys#token)
- Where each key is used, export (OpenSSH, PuTTY), **rotation** on every host, **Shellvane's SSH agent** for other programs: [SSH keys](shellvane:topic/keys)
- Vault **recovery key**, **auto-lock** and **unlock with the system keychain**, confirmed with **Touch ID** or **Windows Hello**: [Vault](shellvane:topic/vault)

**Getting started**
- A **first-run tour** sets up the vault and imports your hosts: [Getting started](shellvane:topic/getting-started)

**Integration**
- `shellvane-cli`, `ssh://` and `sftp://` links, **portable mode**, a **tray** icon: [Integration](shellvane:topic/integration)
- **Signed updates** installed in place, also on macOS; installers for Windows, macOS and Linux (.deb, .rpm, AppImage): [Integration](shellvane:topic/integration)
