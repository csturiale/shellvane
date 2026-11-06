# Getting started {#top}

On a new installation Shellvane opens a short tour: language, vault, importing your hosts. **Start the tour again** in the checklist below reopens it.

Shellvane keeps your servers, credentials and terminals in one place. This page takes you from an empty app to your first connection in a few minutes; every step links to the screen where you do it.

## 1. Create the vault {#vault}

The vault stores passwords, key passphrases, private keys and TOTP secrets, encrypted with a master password that never leaves your computer. You can use Shellvane without it, but then nothing secret is remembered.

[Open the vault settings](shellvane:view/settings)

## 2. Add or import your hosts {#hosts}

- **Import** what you already have: your OpenSSH `~/.ssh/config`, PuTTY or KiTTY sessions. Nothing is changed in the source, and a backup is taken first.
- Or create hosts by hand, inside **groups** that share settings (user, port, key, jump host…).

[Import hosts](shellvane:action/import) · [New host](shellvane:action/newHost)

## 3. Connect {#connect}

Double-click a host, or type `user@host:port` in the quick connect bar at the top. The first time, Shellvane shows the server's fingerprint and asks whether to trust it.

You can also open a terminal on this computer: [New local terminal](shellvane:action/newLocal) ([[T]]).

## 4. Use a key instead of a password {#key}

Generate a key in **Keys**, then use **Install on a host** to add it to the server's `authorized_keys`. Assign it to a host or a whole group.

[Open Keys](shellvane:view/keys)

## 5. Go further {#next}

- Split panes and drag them around: [Terminal](shellvane:topic/terminal)
- Type in many servers at once: [Broadcast](shellvane:topic/broadcast)
- Save commands with variables: [Snippets](shellvane:topic/snippets)
- Forward ports: [Tunnels](shellvane:topic/tunnels)
- Copy files, even server to server: [Files (SFTP)](shellvane:topic/files)
- Never lose a session: [Reconnection and workspaces](shellvane:topic/continuity)
- React to output, script logins: [Automation](shellvane:topic/automation)
- Open Telnet sessions and serial consoles: [Connecting](shellvane:topic/connecting#telnet-serial)
- Share a group of hosts with a colleague: [Hosts](shellvane:topic/hosts#templates)
- Run the same steps on many hosts: [Runbooks](shellvane:topic/runbooks)
- Keep two computers in step: [Sync](shellvane:topic/sync)

Press **[[K]]** anywhere to search hosts, snippets and actions, or type `?` there to search this guide.
