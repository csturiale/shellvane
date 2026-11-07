# Roadmap

Shellvane is a cross-platform desktop manager for SSH, SFTP, tunnels and
multi-host work. This page summarizes what is done and what comes next; the
in-app guide documents every feature in detail.

## Principles

- **Local-first**: works offline, no account; optional end-to-end encrypted
  sync.
- **Secure by default**: secrets encrypted at rest, host keys verified, secrets
  never exposed to the UI without an explicit action.
- **Keyboard-first**: command palette and shortcuts everywhere.
- **Compatible**: reads your `~/.ssh/config` and `known_hosts` without breaking
  them; everything can be exported in open formats.
- **Multilingual**: English and Italian, with every string translatable.

## Done

- **Connections**: groups with inherited settings, quick connect, fuzzy search,
  tags, favorites, notes; multiple jump hosts, SOCKS/HTTP proxies,
  `ProxyCommand`; local shells (PTY/ConPTY, WSL), Telnet, serial, Mosh, RDP/VNC
  launchers; host status, Wake-on-LAN, port knocking.
- **Authentication**: password, keys, agents (OpenSSH, Pageant, 1Password,
  Bitwarden), keyboard-interactive/2FA with TOTP, OpenSSH certificates,
  ephemeral certificates (Vault SSH, step-ca), PKCS#11 tokens,
  Kerberos/GSSAPI, AWS SSM, Azure Bastion and Google IAP transports, credential
  plugins (Vault, Teleport, Boundary).
- **Keys and vault**: generation, import/export (OpenSSH, PPK), deploy,
  assisted rotation, built-in agent; Argon2id + XChaCha20-Poly1305 vault, system
  keychain, Touch ID/Windows Hello, recovery key, auto-lock.
- **Terminal**: tabs, splits, drag & drop, search, logging, asciinema recording
  and player, Kubernetes context per pane, tmux/screen-aware reconnection,
  notifications, Sixel images, ZMODEM and trzsz transfers, detachable windows,
  themes with editor and importers.
- **Automation**: startup scripts with expect, on-output rules (highlighting,
  notifications, automatic answers), snippets with variables, runbooks across
  hosts, broadcast with dangerous-command checks, environments (cloud login,
  jump host, tunnels and kubectl in one click).
- **Files and tunnels**: dual-pane SFTP (also server to server), queue,
  conflicts, resume, remote editing, folder sync; L/R/D and Unix socket
  tunnels, X11 forwarding, detection of remote listening ports.
- **Import**: OpenSSH, PuTTY, KiTTY, MobaXterm, SecureCRT, WinSCP, FileZilla,
  Tabby, mRemoteNG, Termius.
- **Integration**: CLI companion, `ssh://`/`sftp://` links, tray, portable mode,
  signed auto-update, sync through a folder, WebDAV, Git or S3, plugins
  (inventories for clouds, Kubernetes, Ansible, Terraform, Proxmox, NetBox;
  panels for Docker, systemd, logs; external JSON-RPC plugins), community
  gallery of snippets and themes.
- **The app on shellvane.com** also includes, free like everything else, an
  assistant that explains commands and errors and a team vault with shared
  groups, environments, snippets, workspaces and rules. Shellvane is funded by
  donations: [shellvane.com/support](https://shellvane.com/support/).

## Next

- Code signing and notarization of the official packages (Windows, macOS).
- FIDO2 keys without an external agent.
- More importers (Remmina, Xshell, Royal TS, Ásbrú).
- Performance measurements on Windows and macOS (ADR 0002).
- Translations open to contributors.
