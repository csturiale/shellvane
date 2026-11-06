# Hosts and groups {#top}

A **host** is a server you connect to. A **group** is a folder of hosts and subgroups that share settings.

## Create, edit, move {#manage}

- **New host** / **New group** are in the header of the Hosts panel; the folder icon on a group creates a host inside it.
- Drag hosts and groups to move them. The buttons on each row edit or delete it; deleting a group tells you how many subgroups and hosts it contains, which are deleted with it.
- The filter box searches names, addresses and notes.
- A host's **label** is optional: without it the hostname is shown.

[New host](shellvane:action/newHost) · [New group](shellvane:action/newGroup)

## Context menus {#menus}

Right-click a host to connect, open it with Mosh or remote desktop, browse its files, wake it, copy its address, mark it favorite, read its notes, duplicate, edit or delete it. Right-click a group to connect to all its hosts, add a host or a subgroup in it, copy it as a template, edit or delete it.

## Inherited settings {#inheritance}

Settings left empty on a host come from its group, then from the parent groups, then from the defaults. The placeholder of each field tells you the inherited value and which group set it.

Inheritable settings: user, port, SSH key, startup script, terminal colors, jump hosts, proxy, ProxyCommand, password and certificate commands, agent forwarding, tmux/screen, automatic reconnection, Kubernetes context.

Example: put `user: deploy`, the `prod` key and the bastion as jump host on the group **Production**; every host inside only needs its address.

## Export and share groups {#templates}

To give colleagues a set of connections (say an `AWS/prod` and `AWS/staging` tree with the right user, port and proxy command), use the clipboard button on a group: **Export the group** copies the group, its subgroups and their settings as text. With **Include the hosts** (on by default) the hosts go with them: name, address, tags, settings and jump hosts, which are linked again by name. Leave it off to share just the structure, as a model to fill with your own hosts. Vault keys, passwords, notes, the local terminal theme, Wake-on-LAN addresses and knock sequences are never included.

On the other computer, the clipboard button at the top of the panel, **Import groups**, creates them at the top level or inside a group you choose. A group that already exists with the same name is kept as it is and only receives what it lacks; a host already there with the same name and address is skipped, so importing twice adds nothing. Keys are then assigned with **SSH key** as usual.

For safety, commands in an imported export are **not** imported — ProxyCommand, password, certificate and pre-connect commands (they would run on your computer), startup and remote commands, and X11 forwarding. The message after the import lists them: add them by hand if you trust where the export comes from.

## Colors {#colors}

A group color is shown on every pane, tab and status bar entry of its hosts, so a production terminal is recognizable at a glance. Subgroups inherit the nearest color.

## Notes {#notes}

Each host has free-text notes, also searched by the filter.

## Recognizing panes {#labels}

Every pane shows its label, `user@host:port`, the group path with its color, the jump hosts ("via bastion") and its state. Hover a tab for the same details.

## Tags, favorites, recent {#tags}

- Give hosts **tags** (comma separated) in the host dialog; they appear under the name. Click a tag, or type `#tag` in the filter, to show the hosts that have it. The filter also searches notes.
- The star marks a **favorite**; favorites and the last hosts you connected to are listed at the top of the panel.
- **Duplicate** copies a host with its settings, tags and notes.
- **Connect to all**: the grid button on a group, or the button under the filter, opens every host in one tab, in a grid, with broadcast on.
- **Notes** are Markdown: the notes button on a host shows them formatted.
- The quick connect bar remembers the last addresses you typed.
