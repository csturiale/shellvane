# Reconnection, tmux and workspaces {#top}

## Automatic reconnection {#reconnect}

When a connection drops (network change, sleep, server restart), the pane says **Connection lost** and tries again after 1, 2, 4… seconds, up to 30, at most 8 times, and immediately when the network comes back. **Reconnect now** and **Cancel** are in the pane. Failures that another attempt cannot fix (wrong password, changed host key) stop the attempts.

Turn it off in Settings › Sessions, or per host/group under Advanced. The old output stays on screen with a "reconnected at" line.

## Persistent sessions with tmux or screen {#tmux}

Set **Persistent session** (Advanced) to tmux or GNU screen: the shell then runs in a named session on the server, so after a disconnection — or a restart of Shellvane — you find the same programs still running, with their output. Each pane has its own session name. If the multiplexer is not installed, the normal shell opens. A badge "tmux" marks these panes.

## Workspaces {#workspaces}

The layout icon in the tab bar saves the open tabs, splits and connections under a name; open it again later from the same menu, from the welcome screen or from the command palette. Saving with an existing name replaces it.

## Restore at startup {#restore}

With Settings › Sessions › **Restore tabs at startup**, the tabs open when you quit come back at the next start (tmux/screen sessions are resumed).

[Open the settings](shellvane:view/settings)
