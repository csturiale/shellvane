# Connecting {#top}

## Open a session {#open}

- Double-click a host, press Enter on it, or use the command palette ([[K]], then `@` to search hosts only).
- **Quick connect**: type `user@host:port` in the bar at the top. The port and the user are optional.
- Several sessions to the same server share one connection: splitting a pane opens a new channel instead of logging in again.

## Host keys {#hostkey}

The first time you connect, Shellvane shows the server's key fingerprint. Compare it with the one the administrator gives you (`ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the server):

- **Trust and remember** saves it in Shellvane's known hosts.
- **Trust once** connects without saving.
- **Reject** stops.

If a known server presents a **different** key, the connection is refused: it may be a reinstall, or someone in the middle. Remove the old key only when you know why it changed.

## States {#states}

A pane is *connecting*, *connected*, *closed* (the shell ended, with its exit code) or *failed* (with the reason). Closed and failed panes offer **Reconnect**; dropped connections reconnect by themselves, see [Reconnection](shellvane:topic/continuity). Terminals keep running while you are on another page (settings, files, the guide): they end only when their shell or connection ends, or when you close them.

## Local terminal {#local}

**New local terminal** ([[T]], or the terminal icon in the tab bar) opens a shell of this computer: your login shell, or the one chosen in Settings › Local terminal (bash, zsh, fish, PowerShell, cmd, WSL, Git Bash). Local panes work like SSH ones: split, drag, broadcast, rules, workspaces.

[New local terminal](shellvane:action/newLocal)

## Telnet and serial consoles {#telnet-serial}

For network equipment and boards without SSH, type in **Quick connect** (or the command palette):

- `telnet://host` or `telnet://host:2323`: a Telnet session, which sends the terminal type and the window size. Telnet is not encrypted: use it only on networks you trust.
- `serial://COM3?baud=115200` on Windows, `serial:///dev/ttyUSB0?baud=115200` on macOS and Linux: a serial console. Optional settings are `data` (5–8, default 8), `parity` (`none`, `odd`, `even`, `mark`, `space`) and `stop` (`1`, `1.5`, `2`); the default is 9600 8N1. The serial ports of this computer are suggested in the quick connect list and in the palette at 115200. On Linux your user needs access to the port (usually the `dialout` group).

The pane shows a **Telnet** or **Serial** badge with the address or the port and its settings. These panes are kept in workspaces like the others.

## Directory, environment and command {#session}

In a host's or group's **Advanced** options:

- **Initial directory**: the shell starts there (`~/projects` is under your home);
- **Environment variables**: `NAME=value` lines exported before the shell starts. Shellvane sets them itself, so they work even when the server's `AcceptEnv` would refuse OpenSSH's `SetEnv`;
- **Remote command**: runs instead of the interactive shell, like OpenSSH's `RemoteCommand` (e.g. `htop`, `sudo -i`, `docker exec -it app bash`); the pane ends when it exits. With tmux or screen it runs inside the persistent session.

Groups pass them on to their hosts like the other settings. Importing `~/.ssh/config` brings `SetEnv` and `RemoteCommand` along.

## Reachability, Wake-on-LAN and port knocking {#wake}

A dot on a host's icon shows whether its SSH port answers (green, with the latency in the tooltip) or not (red). Hosts reached through a jump host or a proxy are not probed. The check runs every minute while the hosts panel is open and can be turned off in **Settings**.

With a **MAC address** in the **Advanced** options, the power button of the host sends a Wake-on-LAN packet (to `255.255.255.255:9` or the broadcast address you set). **Wake before connecting** sends it at every connection and waits up to two minutes for the host to answer.

**Port knocking** touches a sequence of ports before connecting, e.g. `7000,8000/udp,9000`, for servers whose firewall opens SSH only after the right knock.

## Mosh, remote desktop and VNC {#launchers}

When `mosh` is installed, the Wi-Fi button of a host opens it with Mosh in a pane: the session survives sleep, roaming between networks and bad connections. Mosh's SSH login uses the host's port, user, jump hosts and ProxyCommand, and Shellvane's agent when it runs; the server needs `mosh-server`.

**Remote desktop** in a host's **Advanced** options adds a screen button that opens the host's desktop with the system's client: RDP (Remote Desktop on Windows and macOS, FreeRDP or Remmina on Linux) or VNC (a VNC viewer, or the built-in viewer of macOS). **Through SSH** first opens a temporary forward to the port on the host, so the desktop needs no open port and is encrypted by SSH.

## X11 forwarding {#x11}

**X11 forwarding** in a host's **Advanced** options shows graphical programs started on the server (`xclock`, `gitk`, `virt-manager`…) on this computer, like `ssh -X`. A local X server is needed: the desktop on Linux, XQuartz on macOS, VcXsrv or X410 on Windows. The server needs `X11Forwarding yes` and `xauth`.

The server receives a random cookie; each X connection it opens must present it, and Shellvane replaces it with your display's real cookie, which never leaves this computer.
