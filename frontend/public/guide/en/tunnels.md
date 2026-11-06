# Tunnels {#top}

Tunnels forward network ports through an SSH connection.

## Types {#types}

- **Local** (`-L`): a port on this computer reaches a host and port seen from the server. Example: local `5432` → `db.internal:5432`, then connect your SQL client to `localhost:5432`.
- **Remote** (`-R`): a port on the server reaches a host and port seen from this computer.
- **Dynamic** (`-D`): a SOCKS5 proxy on this computer; point your browser to it to browse as the server.

## Use {#use}

Start and stop tunnels from the Tunnels panel; **Start with the app** opens them at launch. Each tunnel shows its state, connections and transferred bytes. The status bar counts active tunnels.

Tunnels share the connection with terminals to the same host. When it drops, the tunnel shows *Reconnecting* and opens again after 1, 2, 4… seconds (at most 30) until the server is reachable; failures another attempt cannot fix (wrong password, port already in use) stop it.

[Open Tunnels](shellvane:view/tunnels)

## Ports on the server {#ports}

The port icon in the header of a host's pane lists the TCP ports listening on that server (read with `ss`, `netstat` or `/proc`, on a separate channel). **Forward** opens a local forward on the same port when it is free, any free port otherwise, and shows the local address: click it to open it in the browser.

When a program in the pane prints a local address such as `http://localhost:5173`, Shellvane offers to forward that port.

These forwards are **temporary**: they appear in Tunnels marked "temporary" and end when stopped. **Save as tunnel** turns one into a saved profile.

## Unix sockets {#sockets}

A destination that starts with `/` is a Unix socket, without a port: a local tunnel to `/var/run/docker.sock` makes the server's Docker reachable at `127.0.0.1:<port>` (`DOCKER_HOST=tcp://127.0.0.1:2375 docker ps`), and the same works for PostgreSQL or MySQL sockets.
