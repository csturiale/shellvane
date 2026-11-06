# Broadcast {#top}

Broadcast sends what you type in one pane to every pane of the tab, for example to run the same command on several servers.

## Turn it on {#use}

Click the antenna icon in the tab bar or press [[B]]. The tab and its panes get an orange border and a **BROADCAST** badge.

## Choose the panes {#select}

Each pane header has a **Receive** checkbox: untick it to leave a pane out.

The broadcast bar under the tabs also chooses the panes at once: **All panes**, or the panes of one **group**, or the panes of hosts with one **tag**. Hosts → **Connect to all** opens a group or a filtered list in a grid tab with broadcast already on.

## Broadcast bar {#bar}

When broadcast is on, a bar appears under the tabs. Type a command there and press Enter: it goes to every receiving pane, whichever pane has the focus. Untick **Enter** to send text without running it. ↑ and ↓ browse the commands sent before; **Ctrl+C** interrupts the command in every receiving pane.

Typing directly in a pane still works and is mirrored to the others.

## Safety {#safety}

- Commands that look dangerous (`rm -rf`, `shutdown`, `DROP TABLE`…) ask for confirmation when they would reach several panes, from the broadcast bar, snippets and the palette. The list is a set of regular expressions (case-insensitive) in **Settings → Broadcast**: add your own, e.g. `kubectl\s+delete`, or restore the defaults.
- [Automatic responses](shellvane:topic/automation#respond) never fire in panes that receive broadcast input.
