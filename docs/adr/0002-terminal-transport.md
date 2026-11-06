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

All runs had no GPU; the 20 MB/s budget of §13.2 is close. Windows (WebView2)
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
