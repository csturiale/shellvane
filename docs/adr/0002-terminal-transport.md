# 0002. Terminal I/O transport

- Status: accepted
- Date: 2026-10-30

## Context

Terminal output must reach xterm.js fast, in order, without blocking the UI,
and with backpressure so that a runaway command cannot flood the webview.
The project plan (§4.1) compared Wails events (JSON), a local binary
WebSocket, and Wails v3 streams.

## Decision

Use one Wails stream per terminal, named `terminal`:

- The first frame is JSON `{"sessionId": "..."}`; every later frame is raw
  bytes, in both directions.
- Go reads the SSH channel in 32 KiB chunks and calls the blocking
  `StreamConn.Send`, which stops reading from SSH while frames are undelivered.
- Control (open, resize, close, prompts) uses ordinary bindings and events,
  not the stream.
- The frontend talks to a `TerminalSocket` interface with the shape of a
  WebSocket. Server builds of Wails already serve streams as real WebSockets,
  so the Electron fallback (plan B) can reuse the frontend unchanged.

## Measurements (spike)

| Environment | Test | Result |
| --- | --- | --- |
| Desktop app, Linux, WebKitGTK 6.0, Xvfb, software rendering | `seq 1 1000000` (6.9 MB) through SSH to a local sshd | 424 ms, about 16 MB/s |
| Server build in headless Chromium, DOM renderer | `seq 1 200000` (1.3 MB) | about 250 ms end to end |
| Server build in headless Chromium, WebGL in software (no GPU, VM) | Settings › Diagnostics › Terminal performance: 8 MB generated in Go, timed to the last byte drawn | 1.10 s, 7.2 MB/s |
| Same, 2026-11 | Terminal performance › Measure, then › Transport only (counted, not drawn) | Measure 1.2–1.6 s (5–6.7 MB/s), last byte received after 0.7 s; transport only 0.15–0.18 s (45–54 MB/s) |
| Desktop app, macOS (Apple silicon), WKWebView, WebGL | Terminal performance › Measure | 1.00 s, 8.0 MB/s |
| Same Mac, 2026-11 | Measure, then Transport only | Measure 0.63 s (12.7 MB/s, received in 0.62 s, 3 pauses); transport only 0.08 s (105 MB/s) |
| VM, xterm.js 6.0 alone (no app, 160×45) | 8 MB written in 32 KB chunks | DOM 14–15 MB/s, WebGL in software 7–10 MB/s; 6.1 beta the same |
| VM, the app after the 2026-11 changes | Measure | DOM 10 → 12.8 MB/s, WebGL 5.5 → 8.5 MB/s: at xterm.js's own speed |

All runs but the macOS one had no GPU. The 2026-11 runs show the stream
is not the limit (about 50 MB/s without drawing): the time goes to xterm.js
parsing and drawing, which in a VM is mostly the software GPU (a CPU
profile shows about 50–80% of the run in native rendering, the rest in
xterm.js's parser and buffer). Longer lines are not faster: 80-character
lines took 2.8 s in the same VM. The 20 MB/s budget of §13.2 is not met yet. Windows (WebView2)
and macOS (WKWebView) still need the same measurement: Settings › Diagnostics ›
Terminal performance runs it on any system, without a shell or server, and
its result can be copied here.

## Consequences

- No local port is opened, so there is no socket to protect.
- xterm.js buffers writes without limit: Go-side backpressure bounds the stream,
  not the terminal's parser queue. Next step: track pending bytes with the
  `write` callback and send pause/resume to Go above a high watermark.
- The stream ends when the window reloads; the session is then closed. Session
  restore across reloads belongs to the reconnect work (v1).

## Drawing (2026-11)

With the stream at 50–100 MB/s, the limit is xterm.js parsing and drawing.
Measured against xterm.js alone, the app lost about a third to its own
work around the terminal; four changes recovered it:

- frames that arrive in one delivery are joined (up to 512 KB) before
  `term.write`, since each write has a fixed cost;
- the command suggestion is repositioned once per drawn frame (`onRender`)
  instead of on every `onScroll`, which fires for each line of a flood;
- highlighting scans at most the last 1000 new lines per write: during a
  flood only those are colored;
- flow control pauses Go above 4 MiB not yet drawn and resumes below 1 MiB,
  so xterm.js still has work while new output is on its way.

What is left is inside xterm.js (buffer line recycling, events per scrolled
line, the viewport's scrollbar) and the same in its 6.1 beta. Further gains
would need changes upstream; the 20 MB/s target is within reach on Apple
silicon, not in software-rendered VMs.
