# Privacy and security {#top}

## What stays on this computer {#local}

Hosts, settings, known hosts, rules and the audit log are in a local database; secrets are in the encrypted [vault](shellvane:topic/vault). Shellvane sends nothing anywhere except the connections you open, the update check (once a day unless you turn it off, a download of the signed release manifest from the project's public GitHub releases that tells only Shellvane's version and your system, such as `Shellvane/1.2.0 (darwin/arm64)`, and nothing about you; see [Updates](shellvane:topic/integration#updates)) the encrypted files sent to your [sync](shellvane:topic/sync) folder, WebDAV server, Git repository or S3 bucket if you set it up, and the weekly [usage statistics](#usage) only if you agreed to them. Telnet is not encrypted: use it only on networks you trust. An exported group never holds keys, passwords or notes.

## Confirmations {#confirm}

Shellvane asks before: trusting a new host key (and refuses a changed one), every signature with a forwarded agent, dangerous snippets in broadcast, deleting hosts, groups, keys and rules.

## Automation limits {#automation}

Automatic responses are opt-in per group or host, rate-limited, off in broadcast and audited. See [Automation](shellvane:topic/automation#respond).

## Commands you configure {#commands}

Password, certificate, ProxyCommand and before-connecting commands run on this computer with your permissions; only use commands you trust.

## Diagnostics and support bundle {#diagnostics}

When a connection fails, **Connection details** on the pane shows each step of the handshake: the address reached, the server's version, the negotiated algorithms (key exchange, host key, cipher, MAC), the host key's fingerprint, the authentication methods the server allows and those that failed, and the exact error. **Settings → Diagnostics** lists the last 50 attempts since Shellvane started.

**Copy report** copies an attempt as text for a colleague or a bug report, with host names, users and addresses replaced by pseudonyms (`host-01`, `user-01`, `10.255.0.x`).

The **support bundle** is a ZIP file with the version and system, your settings, the structure of groups and hosts, and the recent connection attempts. Before saving, **Preview** shows every file exactly as it will be written. The same pseudonyms are used in all files, so they stay correlated; the table of correspondences is never saved. The vault, private keys, passwords, TOTP seeds, notes and commands (password, certificate, proxy, before-connecting and startup commands) are never included: settings that may hold them only say "(set)". The bundle is saved in the application's folder and sent nowhere.

## Anonymous usage statistics {#usage}

Shellvane asks once whether to share anonymous usage statistics; until you say yes nothing is counted or sent. Change your mind any time in **Settings › Privacy**, where **Show what is sent** displays the exact report.

- **What is counted**, on this computer: how many times each feature was used (sessions by kind, splits, broadcast, SFTP, tunnels, environments, runbooks, snippets, accepted suggestions, the command palette, gallery and plugin installs, imports, sync, team sharing, workspaces, recordings, the assistant, views opened), and how many times well-known command-line programs ran (`kubectl`, `docker`, `git`, `terraform`, `systemctl`… from a fixed public list): only the program's name, after `sudo` or variable assignments. Anything else you type is not looked at.
- **What is sent**, once a week: those counts, the number of days they cover, Shellvane's version and the system (for example `darwin`, `arm64`). No identifier of you or this computer, no host, address, user, path, argument, file name or output.
- **Where**: to shellvane.com, which stores the report without your IP address and keeps it to see which features matter and which tools deserve an integration. It is never shared or sold.

Turning the statistics off deletes the counts not sent yet.
