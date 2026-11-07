# Terminal, tabs and panes {#top}

## Tabs and splits {#splits}

- The **+** after the last tab opens a new tab: a local terminal, a favorite or recent host, any saved host by searching its name, address, group or tag, or an address typed in (`user@host:port`, `telnet://…`, `serial://…`). Arrows and Enter work too.
- **Split right** ([[D]]) and **Split down** ([[E]]) open another session to the same host next to the focused pane.
- Drag the divider to resize. **Alt+arrows** move the focus between panes.
- **Close pane**: [[W]].

## Command suggestions {#suggestions}

Turn on **Settings › Terminal › Suggest commands as you type** and, while you type a command, the rest of one you already ran appears in grey after the cursor: press **→** to take it, or keep typing to ignore it. Suggestions come first from the commands run on the same connection, then on the others, then from your snippets without variables. **Tab** stays the shell's own completion, which knows the files, branches and services on the server now, and the shell's history (**↑**, **Ctrl+R**) works as usual. If you prefer, tick **Also with Tab** next to the setting: Tab then takes a suggestion while one is shown, and reaches the shell otherwise.

Only what you type at the shell's prompt is remembered, and only if it appeared on screen: what you type at a password prompt, in full-screen programs (vim, less, top) or in broadcast is never kept, nor commands that start with a space or look like they contain a password, token or key. The history stays on this computer; **Clear history** in the same settings deletes it. Suggestions follow a line only while you type it character by character: after Tab completion, arrow keys or Ctrl shortcuts they wait for the next command.

## Send and receive files in the terminal (ZMODEM) {#zmodem}

With `lrzsz` installed on the server, files move through the terminal itself, also across jump hosts and inside tmux or screen:

- `sz file1 file2` sends files to this computer: they are saved in your **Downloads** folder (never replacing a file already there: `name (1).ext`). Click a path in the panel that appears to show it in its folder.
- `rz` opens the file chooser here, and the chosen files go to the server's current folder.

A bar shows the progress, with **Cancel**. Closing the file chooser without choosing cancels `rz`.

## Faster transfers with trzsz {#trzsz}

With [trzsz](https://trzsz.github.io) on the server (`pip install trzsz`, or the `trzsz-go` packages), transfers are faster than ZMODEM and also work for folders:

- `tsz file1 file2` (or `tsz -d folder`) sends to this computer: a folder dialog of the system asks where to save.
- `trz` (or `trz -d` for folders) opens the system's file dialog, and the chosen files go to the server's current folder.

A progress bar appears in the terminal; Ctrl+C stops the transfer. On Linux the dialogs need `zenity` installed. SSH sessions only.

## Move a tab to its own window {#detach}

The **Move to a new window** button on a tab opens it in a separate window, for a second screen or to keep one server in view. The sessions stay connected while they move: the new window shows the recent output and scrollback (the last 512 KB), the splits and the broadcast settings of the tab, and nothing else. Closing its last tab closes the window. In the separate window the same button becomes **Move back to the main window**, which returns the tab, still connected, next to the others. Tabs in separate windows are not reopened at the next start.

## More room for terminals {#panel}

The side panel (hosts, snippets, tunnels…) can be hidden: the button at the bottom of the activity bar, a click on the icon of the section already open, or **[[L]]**. Clicking any section shows it again. Drag the panel's right edge to make it wider or narrower; dragging it almost closed, or double-clicking the edge, hides it. The width and the choice are remembered.

## Search, maximize, zoom {#search}

- **Search** the scrollback with [[F]] or the magnifier in the pane header: Enter and Shift+Enter move between matches, with options for case and regular expressions.
- **Maximize** a pane with [[Enter]] or the header button: it fills the tab and the side panel steps aside; again (or showing the panel) restores the layout.
- **Zoom** one pane with [[=]] and [[-]], [[0]] to reset.
- **Rename** a tab by double-clicking its title.
- A tab that rings the **bell** while hidden shows 🔔 (and a system notification when Shellvane is in the background).

## Context menus {#menus}

Right-click for the actions of what is under the pointer:

- a **tab**: rename, move to a new window (or back to the main one), close, close the other tabs, those to the left or to the right, or all of them;
- a **pane** (its header or the terminal): copy, paste, select all, clear the scrollback, split, maximize, move to a new tab, search, open the current folder, log, recording, reconnect, close;
- a **host** or a **group** in the hosts panel: see [Hosts](shellvane:topic/hosts#menus).

In narrow panes the header keeps the connection's name, address, group and state (shortened, in full in the tooltip) and the main buttons; the others are in the pane's menu.

## Moving panes {#drag}

Drag a pane by its header:

- onto a side of another pane to split there;
- onto its center to swap the two;
- onto a tab to move it into that tab;
- onto the empty part of the tab bar to give it a tab of its own.

The session is never interrupted.

## Appearance {#appearance}

Settings › Terminal sets the color scheme, font size, cursor (block, bar, underline, blinking), scrollback length, copy on select, confirmation before multi-line pastes and before closing tabs with open sessions, Option/Alt as Meta, Backspace as ^H and screen reader support. Groups and hosts can override colors and font size, e.g. a red-tinted scheme for production.

Settings › Appearance › **Interface size** makes the text and controls of the whole app larger or smaller (80–150%); terminals keep the font size chosen for them.

## Highlights {#highlights}

Log levels, IP addresses, HTTP status codes and UUIDs are colored (the bytes are not changed). Turn the built-in set off in Settings › Terminal, and add your own with [highlight rules](shellvane:topic/automation#rules). When a command prints more than a thousand lines at once (the `cat` of a big file), only the last thousand are colored, so highlighting never slows the output down.

## Copy and paste {#clipboard}

Select text with the mouse, then **[[C]]** copies it and **[[V]]** pastes; plain Ctrl+C stays the interrupt key of the shell. Pasted text reaches the other panes too while broadcasting. Links are clickable.

## Flow control {#flow}

When a command floods the terminal, Shellvane pauses reading from the server until the screen catches up, so the app stays responsive and memory bounded.

## Logs and recordings {#logs}

- The **log** button in a pane header saves its output to a file: plain text, text with timestamps, or raw (with colors), chosen in Settings › Terminal.
- The **record** button records the session in the asciinema v2 format (a red REC badge shows it); recordings can be played in Shellvane or with `asciinema play`.
- Hosts and groups can log or record every session (Advanced › Log and recording).
- **Logs and recordings** (in the command palette, or from the notice shown when one starts) lists the files with a viewer and a player: pause, seek, speed from 0.5× to 8×, long pauses shortened.

## Encoding and terminal type {#encoding}

Old servers that do not use UTF-8 (Latin-1, Windows-1252, Shift_JIS, KOI8-R…) can be set under Advanced › Encoding: output is converted to UTF-8 and what you type to the server's encoding. TERM (default `xterm-256color`) can be changed there too.

## Color schemes {#themes}

**Settings → Terminal** offers built-in schemes and your own. **New** and **Duplicate** open the editor: every color (background, text, cursor, selection and the 16 ANSI colors) with a color picker or a hex value, and a live preview. Empty colors take sensible defaults.

**Import…** reads schemes from other terminals: iTerm2 (`.itermcolors`), Windows Terminal (a scheme or a whole `settings.json`), Base16 and Base24 (YAML), Alacritty (TOML or YAML), Kitty and Ghostty theme files. Several files can be chosen at once. **Export** copies a scheme in Windows Terminal JSON, which most terminals can import.

A host or group can use its own scheme (in its dialog), for example red tones for production. **App colors follow the terminal scheme** tints the whole window with the selected scheme's colors.

## Notifications and images {#notifications}

Programs can ask for a notification with the escape sequences OSC 9 (`printf '\e]9;Build done\a'`) or OSC 777 (`printf '\e]777;notify;Deploy;finished\a'`). If the pane is hidden its tab gets a mark, and when Shellvane is not in front a system notification appears.

With **shell integration** (OSC 133) Shellvane also notifies when a command that ran longer than the time set in **Settings → Terminal** ends, with its exit code. Many prompts emit these marks already (Starship, oh-my-posh, fish 4, the shell integration scripts of VS Code or WezTerm); for plain bash, add to `~/.bashrc` on the server:

```bash
PS0='\[\e]133;C\a\]'
PROMPT_COMMAND='printf "\e]133;D;%s\a\e]133;A\a" "$?"'
```

Images are shown inline with Sixel and the iTerm2 image protocol, e.g. `img2sixel photo.png`, `chafa -f sixel`, or `imgcat`.

## Server load {#monitor}

The pulse icon in the header of a host's pane shows the server's CPU use, load average, memory, swap, disk usage of `/` and uptime, refreshed every three seconds while open. The figures are read with one short command on a separate channel (Linux `/proc`, or `sysctl`/`vm_stat` on macOS and BSD): nothing is installed on the server.

## Measure performance {#benchmark}

**Settings › Diagnostics › Terminal performance › Measure** opens a tab that prints 8 MB of numbered lines and times them from the first byte to the last one drawn, then shows the speed with the renderer (WebGL or DOM), the web engine and the system. It also says when the last byte was received and how many times the app paused the output for the terminal to catch up. **Transport only** sends the same 8 MB without drawing them: if it is much faster than **Measure**, the limit is drawing, not the stream. Use **Copy** to report the results: the target is 20 MB/s or more.
