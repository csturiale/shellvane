import {
  Component,
  DestroyRef,
  ElementRef,
  NgZone,
  afterNextRender,
  computed,
  effect,
  signal,
  inject,
  input,
  untracked,
  viewChild,
} from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';
import { LucideCircleDot, LucideLayers, LucideFolder, LucideFolderOpen, LucideMaximize2, LucideScrollText, LucideMinimize2, LucideRadioTower, LucidePuzzle, LucideSearch, LucideX } from '@lucide/angular';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { ImageAddon } from '@xterm/addon-image';
import { WebglAddon } from '@xterm/addon-webgl';
import { Terminal } from '@xterm/xterm';

import { SessionApi, TerminalSocket } from '../core/backend/session-api';
import { ThemeService } from '../core/theme/theme.service';
import { colorVar } from '../hosts/colors';
import { BUILTIN_HIGHLIGHTS, Highlighter } from '../terminal/highlighter';
import { findScheme } from '../terminal/schemes';
import { CommandTracker, OscNotification, parseOsc777, parseOsc9 } from '../terminal/osc';
import { CommandHistory, LineTracker } from '../terminal/suggest';
import { SnippetsStore } from '../snippets/snippets.store';
import { TerminalPrefs } from '../terminal/terminal-prefs';
import { Navigation } from '../layout/navigation';
import { KubePicker } from './kube-picker';
import { panesOf } from './layout';
import { clipboardAction, combo, isAppShortcut, shortcutAction } from './shortcuts';
import { readClipboard, writeClipboard } from '../core/clipboard';
import { ConfirmDialog } from '../shared/confirm-dialog';
import { FormsModule } from '@angular/forms';
import { DiagnosticsApi, Handshake } from '../core/backend/diagnostics-api';
import { HandshakeView } from '../diagnostics/handshake-view';
import { DetectedPort, detectPorts } from '../tunnels/ports';
import { TunnelsStore } from '../tunnels/tunnels.store';
import { EnvState, EnvironmentsStore } from '../environments/environments.store';
import { PortsMenu } from './ports-menu';
import { MonitorMenu } from './monitor-menu';
import { openUrl } from '../core/browser';
import { Modal } from '../shared/modal';
import { Pane, RETRY_MAX_ATTEMPTS, SessionsStore, addressOf, labelOf } from './sessions.store';
import { DownloadsApi } from '../core/backend/downloads-api';
import { FilesApi } from '../core/backend/files-api';
import { TransferResult, TransferState, ZmodemTransfer } from './zmodem-transfer';
import { TransferBar } from './transfer-bar';
import { PluginPanel } from '../plugins/plugin-panel';
import { ContextMenuService } from '../shared/context-menu.service';
import { EXTENSIONS } from '../extensions/extensions';

const encoder = new TextEncoder();
/** Resize events are coalesced: dragging a splitter fires dozens. */
const RESIZE_DEBOUNCE_MS = 50;
/**
 * Flow control: above HIGH unrendered bytes Go stops reading the SSH channel,
 * below LOW it resumes. Keeps memory bounded when a command floods output.
 */
const FLOW_HIGH = 1024 * 1024;
const FLOW_LOW = 128 * 1024;

/** A dropped stream is opened again this many times, this long apart. */
const REATTACH_TRIES = 5;
const REATTACH_DELAY_MS = 1000;

/** One xterm.js terminal bound to one SSH session through a Wails stream. */
@Component({
  selector: 'app-terminal-view',
  imports: [TransferBar, TranslocoDirective, FormsModule, ConfirmDialog, Modal, HandshakeView, PortsMenu, MonitorMenu, LucideFolder, LucideX, LucideRadioTower, LucideSearch, LucideMaximize2, LucideMinimize2, LucideScrollText, LucideCircleDot, LucideFolderOpen, LucidePuzzle, LucideLayers, KubePicker, PluginPanel],
  host: {
    '[class.narrow]': 'width() > 0 && width() < 460',
    '[class.tiny]': 'width() > 0 && width() < 300',
  },
  templateUrl: './terminal-view.html',
  styleUrl: './terminal-view.scss',
})
export class TerminalView {
  /** The pane's width, which hides secondary header items when narrow. */
  protected readonly width = signal(0);
  /** The plugins' panels (Docker, systemd, logs) for this host. */
  protected readonly pluginsOpen = signal(false);
  readonly pane = input.required<Pane>();
  /** The pane's tab is the active one. */
  readonly visible = input(true);
  /** The pane has focus within its tab. */
  readonly focused = input(true);
  /** The tab is in broadcast mode. */
  readonly broadcastOn = input(false);
  /** The pane receives broadcast input. */
  readonly broadcasting = input(false);
  /** The pane is shown alone in its tab. */
  readonly maximized = input(false);

  protected readonly store = inject(SessionsStore);
  private readonly menu = inject(ContextMenuService);
  protected readonly searchKeys = combo('F');
  protected readonly maximizeKeys = combo('Enter');
  private readonly diagnostics = inject(DiagnosticsApi);
  /** The last connection attempt to this pane's target, when shown. */
  protected readonly handshake = signal<Handshake | null | undefined>(undefined);

  private readonly tunnels = inject(TunnelsStore);
  private readonly environments = inject(EnvironmentsStore);
  /** Extensions that add entries to the context menu. */
  private readonly menuExtensions = EXTENSIONS.flatMap((e) => (e.terminalMenu ? [inject(e.terminalMenu)] : []));
  /** The state of the environment that opened the pane, if any. */
  protected readonly envState = computed<EnvState | null>(() => {
    const env = this.pane().env;
    return env ? this.environments.stateOf(env.id) : null;
  });
  /** A port a program printed on the server, offered for forwarding. */
  protected readonly proposal = signal<DetectedPort | null>(null);
  /** Address of the forward opened from the proposal. */
  protected readonly forwardedTo = signal<string | null>(null);
  private readonly seenPorts = new Set<number>();
  private readonly decoder = new TextDecoder();

  /** Offers to forward local URLs printed by programs on the server. */
  private watchPorts(data: Uint8Array): void {
    const src = this.pane().source;
    if (src.kind !== 'host') {
      return;
    }
    const text = this.decoder.decode(data, { stream: true });
    for (const p of detectPorts(text)) {
      if (!this.seenPorts.has(p.port)) {
        this.seenPorts.add(p.port);
        if (!this.tunnels.forwardOf(src.hostId, p.port)) {
          this.forwardedTo.set(null);
          this.proposal.set(p);
        }
      }
    }
  }

  protected async forwardProposal(): Promise<void> {
    const p = this.proposal();
    const src = this.pane().source;
    if (!p || src.kind !== 'host') {
      return;
    }
    try {
      await this.tunnels.forward(src.hostId, { port: p.port, addr: p.addr, process: '' });
      this.proposal.set(null);
      // The local address arrives with the tunnel's state.
      const started = Date.now();
      const check = () => {
        const t = this.tunnels.forwardOf(src.hostId, p.port);
        if (t) {
          this.forwardedTo.set(t.status.addr);
        } else if (Date.now() - started < 10000) {
          setTimeout(check, 200);
        }
      };
      check();
    } catch {
      this.proposal.set(null);
    }
  }

  protected browse(addr: string): void {
    void openUrl(`http://${addr}`);
  }

  /** Shows how the last connection to this pane's server went. */
  protected async showHandshake(): Promise<void> {
    const t = this.pane().target;
    const address = t.host.includes(':') ? `[${t.host}]:${t.port}` : `${t.host}:${t.port}`;
    try {
      const list = await this.diagnostics.handshakes();
      this.handshake.set(list.find((h) => h.address === address && h.user === t.user) ?? null);
    } catch {
      this.handshake.set(null);
    }
  }
  private readonly prefs = inject(TerminalPrefs);
  private readonly history = inject(CommandHistory);
  private readonly snippetsStore = inject(SnippetsStore);
  /** The command line being typed, for suggestions and history. */
  private readonly line = new LineTracker();
  /** The suggested rest of the command, drawn after the cursor. */
  private ghost: { el: HTMLElement; suffix: string } | null = null;
  private ghostFrame = 0;
  /** Per-pane zoom, in points added to the font size. */
  protected readonly zoom = signal(0);
  protected readonly appearance = computed(() => ({
    scheme: findScheme(this.pane().terminalTheme || this.prefs.scheme(), this.theme.isDark()),
    // The interface scale zooms the whole page: the terminal compensates, so
    // its font size stays the one chosen for terminals.
    fontSize: Math.max(6, (this.pane().fontSize || this.prefs.fontSize()) + this.zoom()) / (this.theme.scale() / 100),
  }));
  /** Terminal behaviour from the preferences. */
  private readonly behaviour = computed(() => ({
    cursorStyle: this.prefs.cursorStyle(),
    cursorBlink: this.prefs.cursorBlink(),
    scrollback: this.prefs.scrollback(),
    macOptionIsMeta: this.prefs.altIsMeta(),
    screenReaderMode: this.prefs.screenReader(),
  }));

  // Scrollback search.
  protected readonly searching = signal(false);
  protected readonly searchCase = signal(false);
  protected readonly searchRegex = signal(false);
  protected readonly searchResult = signal<{ index: number; count: number } | null>(null);
  protected searchText = '';
  private search?: SearchAddon;
  private readonly searchInput = viewChild<ElementRef<HTMLInputElement>>('searchInput');

  /** Path of a log or recording just started, shown briefly. */
  protected readonly notice = signal<string | null>(null);
  /** Multi-line text waiting for confirmation before it is pasted. */
  protected readonly pendingPaste = signal<string | null>(null);
  protected readonly pasteLines = computed(() => (this.pendingPaste() ?? '').split(/\r?\n/).length);
  protected readonly address = computed(() => addressOf(this.pane()));
  protected readonly label = computed(() => labelOf(this.pane()));
  protected readonly color = computed(() => (this.pane().color ? colorVar(this.pane().color) : null));
  protected readonly maxAttempts = RETRY_MAX_ATTEMPTS;
  /** Ticks while a reconnection is pending, for the countdown. */
  private readonly now = signal(Date.now());
  protected readonly secondsLeft = computed(() => Math.max(0, Math.ceil(((this.pane().retry?.at ?? 0) - this.now()) / 1000)));
  private readonly api = inject(SessionApi);
  private readonly downloads = inject(DownloadsApi);
  private readonly files = inject(FilesApi);
  /** ZMODEM (rz / sz on the server): the running transfer and the last outcome. */
  protected readonly transfer = signal<TransferState | null>(null);
  protected readonly transferResult = signal<TransferResult | null>(null);
  private readonly uploadInput = viewChild<ElementRef<HTMLInputElement>>('uploadInput');
  private pickResolve?: (files: File[]) => void;
  private readonly zmodem = new ZmodemTransfer({
    toTerminal: (data) => this.write(this.socketSession, data),
    send: (data) => this.send(data),
    begin: (name) => this.downloads.begin(name),
    append: (id, data) => this.downloads.append(id, data),
    finish: (id) => this.downloads.finish(id),
    cancelFile: (id) => this.downloads.cancel(id),
    pick: () => this.pickFiles(),
    state: (s) => this.zone.run(() => this.transfer.set(s)),
    result: (r) => this.zone.run(() => this.transferResult.set(r)),
  });
  private readonly nav = inject(Navigation);
  private readonly transloco = inject(TranslocoService);
  private readonly theme = inject(ThemeService);
  private readonly zone = inject(NgZone);
  private readonly hostEl = viewChild.required<ElementRef<HTMLElement>>('host');

  private term?: Terminal;
  private highlighter?: Highlighter;
  /** Host rules first, so they win over the built-in set. */
  private readonly highlightRules = computed(() => [...this.pane().highlights, ...(this.prefs.builtinHighlights() ? BUILTIN_HIGHLIGHTS : [])]);
  private fit?: FitAddon;
  private socket?: TerminalSocket;
  private socketSession = '';
  private destroyed = false;
  private reattachTimer?: ReturnType<typeof setTimeout>;
  private reattachTries = 0;
  private resizeTimer?: ReturnType<typeof setTimeout>;
  private pending = 0;
  private unregister?: () => void;
  private paused = false;

  constructor() {
    const destroyRef = inject(DestroyRef);

    afterNextRender(() => {
      // xterm.js handlers fire per keystroke and per frame: keep them away
      // from change detection.
      this.zone.runOutsideAngular(() => this.init());
    });

    // A new session ID (reconnect) needs a new stream; the screen is kept.
    effect(() => {
      const { sessionId, state } = this.pane();
      if (!this.term) {
        return;
      }
      untracked(() => {
        if (sessionId !== this.socketSession) {
          // The old output stays visible above a dim separator.
          const time = new Intl.DateTimeFormat(this.transloco.getActiveLang(), { timeStyle: 'short' }).format(new Date());
          this.term!.write(`\r\n\x1b[2m── ${this.transloco.translate('sessions.reconnectedAt', { time })} ──\x1b[0m\r\n`);
          this.attach(sessionId);
        }
        if (state === 'connected') {
          this.sendSize();
        }
      });
    });

    effect(() => {
      const visible = this.visible();
      const focused = this.focused();
      if (visible && this.term) {
        untracked(() => {
          this.scheduleFit();
          if (focused) {
            this.term!.focus();
          }
        });
      }
    });

    // Colors and font size follow the host's settings, then the app preference.
    effect(() => {
      const appearance = this.appearance();
      untracked(() => {
        if (this.term) {
          this.term.options.theme = appearance.scheme.colors;
          this.term.options.fontSize = appearance.fontSize;
          this.scheduleFit();
        }
      });
    });

    effect((onCleanup) => {
      if (this.pane().retry) {
        untracked(() => this.now.set(Date.now()));
        const timer = setInterval(() => this.now.set(Date.now()), 250);
        onCleanup(() => clearInterval(timer));
      }
    });

    effect(() => {
      const options = this.behaviour();
      untracked(() => {
        if (this.term) {
          Object.assign(this.term.options, options);
        }
      });
    });

    effect(() => {
      const rules = this.highlightRules();
      untracked(() => this.highlighter?.setRules(rules));
    });

    destroyRef.onDestroy(() => {
      this.destroyed = true;
      cancelAnimationFrame(this.ghostFrame);
      clearTimeout(this.reattachTimer);
      this.highlighter?.dispose();
      this.unregister?.();
      clearTimeout(this.resizeTimer);
      this.socket?.close();
      this.term?.dispose();
    });
  }

  private init(): void {
    const term = new Terminal({
      allowProposedApi: true, // required by the unicode11 and search addons
      // OSC 8 hyperlinks printed by the server: web pages only, in the
      // system browser (never in the app's window, never file: or others).
      linkHandler: { activate: (_e, uri) => void openUrl(uri), allowNonHttpProtocols: false },
      fontFamily: '"JetBrains Mono", "Cascadia Code", Menlo, Consolas, "DejaVu Sans Mono", monospace',
      fontSize: this.appearance().fontSize,
      theme: this.appearance().scheme.colors,
      ...this.behaviour(),
    });
    this.search = new SearchAddon();
    term.loadAddon(this.search);
    this.search.onDidChangeResults((r) =>
      this.zone.run(() => this.searchResult.set(r.resultCount > 0 ? { index: r.resultIndex + 1, count: r.resultCount } : null)),
    );
    this.fit = new FitAddon();
    term.loadAddon(this.fit);
    term.loadAddon(new WebLinksAddon((_e, uri) => void openUrl(uri)));
    // Inline images: Sixel and the iTerm2 image protocol (imgcat).
    term.loadAddon(new ImageAddon({ sixelSizeLimit: 25_000_000, storageLimit: 64 }));
    const unicode = new Unicode11Addon();
    term.loadAddon(unicode);
    term.unicode.activeVersion = '11';
    term.open(this.hostEl().nativeElement);
    try {
      const webgl = new WebglAddon();
      webgl.onContextLoss(() => {
        webgl.dispose(); // falls back to the DOM renderer
        this.renderer = 'dom';
      });
      term.loadAddon(webgl);
      this.renderer = 'webgl';
    } catch {
      // No WebGL in this webview: the DOM renderer is used.
    }

    // App shortcuts (split, focus moves) must not reach the remote shell.
    term.attachCustomKeyEventHandler((ev) => {
      if (ev.type === 'keydown' && (ev.key === 'ArrowRight' || ev.key === 'Tab') && !ev.shiftKey && !ev.ctrlKey && !ev.altKey && !ev.metaKey && this.ghost?.suffix) {
        ev.preventDefault(); // Tab must not move the focus out of the terminal
        this.acceptSuggestion();
        return false;
      }
      if (ev.type === 'keydown' && this.paneShortcut(ev)) {
        ev.preventDefault();
        return false;
      }
      if (isAppShortcut(ev)) {
        return false;
      }
      if (ev.type === 'keydown' && ev.key === 'Backspace' && this.prefs.backspaceCtrlH() && !ev.ctrlKey && !ev.altKey && !ev.metaKey) {
        this.send(encoder.encode('\x08'));
        this.store.typed(this.pane().key, '\x08');
        return false;
      }
      const clip = clipboardAction(ev);
      if (clip) {
        ev.preventDefault();
        void this.clipboard(clip);
        return false;
      }
      return true;
    });
    term.onData((data) => {
      this.send(encoder.encode(data));
      this.store.typed(this.pane().key, data);
      this.followLine(data);
    });
    // The suggestion follows the echo of what was typed.
    term.onCursorMove(() => this.scheduleGhost());
    term.onScroll(() => this.scheduleGhost());
    term.buffer.onBufferChange(() => {
      this.line.reset();
      this.scheduleGhost();
    });
    this.unregister = this.store.registerSender(this.pane().key, (data) => this.send(encoder.encode(data)));
    term.onBinary((data) => this.send(Uint8Array.from(data, (c) => c.charCodeAt(0))));
    new ResizeObserver(() => {
      this.scheduleFit();
      const w = this.hostEl().nativeElement.clientWidth;
      if (Math.abs(w - this.width()) >= 8) {
        this.zone.run(() => this.width.set(w));
      }
    }).observe(this.hostEl().nativeElement);

    this.highlighter = new Highlighter(term);
    this.highlighter.setRules(this.highlightRules());
    term.onSelectionChange(() => {
      const text = term.getSelection();
      if (text && this.prefs.copyOnSelect()) {
        void writeClipboard(text).catch(() => undefined);
      }
    });
    // OSC 7 (file://host/path): the shell's current folder, for the Files view.
    term.parser.registerOscHandler(7, (data) => {
      const cwd = cwdOf(data);
      if (cwd) {
        this.zone.run(() => this.store.setCwd(this.pane().key, cwd));
      }
      return true;
    });
    term.onBell(() => this.zone.run(() => this.store.bell(this.pane().key, !this.visible())));
    // Notifications asked by programs (OSC 9, OSC 777) and the end of long
    // commands (shell integration, OSC 133).
    const notifyFromOsc = (n: OscNotification | null) => {
      if (n) {
        this.zone.run(() => this.store.notify(this.pane().key, n.title || this.pane().title, n.body, !this.visible()));
      }
      return true;
    };
    term.parser.registerOscHandler(9, (data) => notifyFromOsc(parseOsc9(data)));
    term.parser.registerOscHandler(777, (data) => notifyFromOsc(parseOsc777(data)));
    const commands = new CommandTracker();
    term.parser.registerOscHandler(133, (data) => {
      if (data.startsWith('A')) {
        this.line.reset(); // a new prompt
      }
      const done = commands.feed(data);
      const after = this.prefs.commandNotifySeconds() * 1000;
      if (done && after > 0 && done.ms >= after) {
        const body = this.transloco.translate(done.exit ? 'terminal.commandFailed' : 'terminal.commandDone', {
          seconds: Math.round(done.ms / 1000),
          code: done.exit ?? 0,
        });
        this.zone.run(() => this.store.notify(this.pane().key, this.pane().title, body, !this.visible()));
      }
      return false; // let other handlers see the marks too
    });
    // Pastes through the webview (menu, middle click) are checked too.
    this.hostEl().nativeElement.addEventListener(
      'paste',
      (ev: ClipboardEvent) => {
        const text = ev.clipboardData?.getData('text/plain') ?? '';
        if (this.needsConfirm(text)) {
          ev.preventDefault();
          ev.stopPropagation();
          this.zone.run(() => this.pendingPaste.set(text));
        }
      },
      true,
    );

    this.term = term;
    this.scheduleFit();
    this.attach(this.pane().sessionId);
    if (this.focused()) {
      term.focus();
    }
  }

  /** The pane's context menu (header and terminal area). */
  /** Where this pane's commands are remembered, or null (no history). */
  private historyScope(): string | null {
    const s = this.pane().source;
    switch (s.kind) {
      case 'host':
        return `host:${s.hostId}`;
      case 'mosh':
        return `host:${s.hostId}`;
      case 'target':
        return `target:${s.spec}`;
      case 'local':
        return 'local';
      default:
        return null;
    }
  }

  /** Follows typed data: remembers finished commands, updates the suggestion. */
  private followLine(data: string): void {
    const term = this.term;
    const scope = this.historyScope();
    if (!term || !scope || !this.prefs.suggestions()) {
      return;
    }
    // The line on screen before Enter: a command is kept only if it was
    // echoed there (never what is typed at a password prompt), and only in
    // the shell's normal screen (not in vim, less or top).
    const buf = term.buffer.active;
    const shown = buf.getLine(buf.baseY + buf.cursorY)?.translateToString(true) ?? '';
    const normal = buf.type === 'normal';
    for (const cmd of this.line.feed(data)) {
      if (normal && !this.broadcasting() && shown.includes(cmd)) {
        this.history.remember(scope, cmd);
      }
    }
    this.scheduleGhost();
  }

  private scheduleGhost(): void {
    if (!this.ghostFrame) {
      this.ghostFrame = requestAnimationFrame(() => {
        this.ghostFrame = 0;
        this.drawGhost();
      });
    }
  }

  /** Shows the rest of the best matching command after the cursor, or nothing. */
  private drawGhost(): void {
    const term = this.term;
    const scope = this.historyScope();
    const typed = this.line.line;
    const buf = term?.buffer.active;
    let suffix = '';
    if (term && buf && scope && this.prefs.suggestions() && !this.broadcasting() && this.line.clean && typed.trim() && buf.type === 'normal' && buf.viewportY === buf.baseY) {
      const before = (buf.getLine(buf.baseY + buf.cursorY)?.translateToString(false) ?? '').slice(0, buf.cursorX);
      if (before.endsWith(typed)) {
        if (!this.snippetsStore.snippets().length) {
          void this.snippetsStore.load().catch(() => undefined);
        }
        const snippets = this.snippetsStore.snippets().map((s) => s.body).filter((b) => !b.includes('{{') && !b.includes('\n'));
        const full = this.history.suggest(scope, typed, snippets);
        suffix = full.slice(typed.length, typed.length + Math.max(0, term.cols - buf.cursorX - 1));
      }
    }
    if (!suffix) {
      if (this.ghost) {
        this.ghost.el.style.display = 'none';
        this.ghost.suffix = '';
      }
      return;
    }
    const screen = term!.element?.querySelector<HTMLElement>('.xterm-screen');
    if (!screen) {
      return;
    }
    if (!this.ghost) {
      const el = document.createElement('span');
      el.setAttribute('aria-hidden', 'true');
      Object.assign(el.style, { position: 'absolute', pointerEvents: 'none', whiteSpace: 'pre', zIndex: '5', opacity: '0.45' });
      screen.parentElement!.appendChild(el);
      this.ghost = { el, suffix: '' };
    }
    const core = (term as unknown as { _core?: { _renderService?: { dimensions?: { css?: { cell?: { width: number; height: number } } } } } })._core;
    const cell = core?._renderService?.dimensions?.css?.cell ?? { width: screen.clientWidth / term!.cols, height: screen.clientHeight / term!.rows };
    const el = this.ghost.el;
    Object.assign(el.style, {
      display: 'block',
      left: `${screen.offsetLeft + buf!.cursorX * cell.width}px`,
      top: `${screen.offsetTop + buf!.cursorY * cell.height}px`,
      height: `${cell.height}px`,
      lineHeight: `${cell.height}px`,
      fontFamily: term!.options.fontFamily ?? 'monospace',
      fontSize: `${term!.options.fontSize}px`,
      letterSpacing: '0px',
      color: this.appearance().scheme.colors.foreground ?? '#888',
    });
    el.textContent = suffix;
    this.ghost.suffix = suffix;
  }

  /** → or Tab takes the suggestion (Tab reaches the shell only when none is shown). */
  private acceptSuggestion(): void {
    const suffix = this.ghost?.suffix ?? '';
    if (!suffix) {
      return;
    }
    this.send(encoder.encode(suffix));
    this.store.typed(this.pane().key, suffix);
    this.line.feed(suffix);
    this.scheduleGhost();
  }

  /** The environment badge: starts it again when it is not ready, else opens one more terminal. */
  protected envAction(): void {
    const env = this.pane().env;
    if (!env) {
      return;
    }
    if (this.envState() === 'ready') {
      void this.environments.openTerminal(env.id);
    } else if (this.envState() !== 'starting') {
      void this.environments.resume(env.id);
    }
  }

  protected paneMenu(ev: MouseEvent): void {
    const p = this.pane();
    const connected = p.state === 'connected';
    const hasSelection = !!this.term?.hasSelection();
    const tab = this.store.tabs().find((t) => panesOf(t.layout).includes(p.key));
    const alone = !tab || panesOf(tab.layout).length < 2;
    this.menu.open(ev, [
      { label: 'menu.copy', disabled: !hasSelection, action: () => void this.clipboard('copy') },
      { label: 'menu.paste', disabled: !connected, action: () => void this.clipboard('paste') },
      { label: 'menu.selectAll', action: () => this.term?.selectAll() },
      { label: 'menu.clear', action: () => this.term?.clear() },
      null,
      { label: 'sessions.splitRight', action: () => void this.store.split('row') },
      { label: 'sessions.splitDown', action: () => void this.store.split('column') },
      { label: this.maximized() ? 'sessions.restore' : 'sessions.maximize', action: () => this.store.toggleMaximized(p.key) },
      { label: 'menu.moveToNewTab', disabled: alone, action: () => this.store.movePaneToNewTab(p.key) },
      null,
      p.env && this.envState() === 'ready' ? { label: 'environments.newTerminal', action: () => void this.environments.openTerminal(p.env!.id) } : null,
      p.env && this.envState() !== 'ready' && this.envState() !== 'starting' ? { label: 'environments.resume', action: () => void this.environments.resume(p.env!.id) } : null,
      ...this.menuExtensions.flatMap((c) => c.items({ selection: hasSelection ? (this.term?.getSelection() ?? '') : '', connected })),
      { label: 'sessions.search', action: () => this.openSearch() },
      p.source.kind === 'host' && connected ? { label: 'plugins.panel.open', action: () => this.pluginsOpen.set(true) } : null,
      p.cwd && p.source.kind !== 'target' ? { label: 'sessions.openFolder', action: () => this.openFolder() } : null,
      { label: p.logging ? 'logs.stopLog' : 'logs.startLog', disabled: !connected, action: () => this.toggleLog() },
      { label: p.recording ? 'logs.stopRecording' : 'logs.startRecording', disabled: !connected, action: () => this.toggleRecord() },
      null,
      connected ? null : { label: 'sessions.reconnect', action: () => void this.store.reconnect(p.key) },
      { label: 'sessions.closePane', danger: true, action: () => void this.store.closePane(p.key) },
    ]);
  }

  /** Copies the selection, or pastes (bracketed when the shell asks). */
  private async clipboard(action: 'copy' | 'paste'): Promise<void> {
    try {
      if (action === 'copy') {
        const text = this.term?.getSelection();
        if (text) {
          await writeClipboard(text);
        }
      } else {
        const text = await readClipboard();
        if (text && this.needsConfirm(text)) {
          this.zone.run(() => this.pendingPaste.set(text));
        } else if (text) {
          this.term?.paste(text);
        }
      }
    } catch {
      // Clipboard not available.
    }
  }

  /** Shows the pane's current folder in the Files view. */
  protected openFolder(): void {
    const p = this.pane();
    this.nav.openFiles(p.source.kind === 'host' || p.source.kind === 'mosh' ? p.source.hostId : 'local', p.cwd);
  }

  protected openRecordings(): void {
    this.notice.set(null);
    this.nav.view.set('recordings');
  }

  protected toggleLog(): void {
    void this.store.toggleLogging(this.pane().key, this.prefs.logFormat()).then(
      (path) => path && this.notice.set(path),
      () => undefined,
    );
  }

  protected toggleRecord(): void {
    void this.store.toggleRecording(this.pane().key).then(
      (path) => path && this.notice.set(path),
      () => undefined,
    );
  }

  private needsConfirm(text: string): boolean {
    return this.prefs.confirmPaste() && /\r|\n/.test(text.replace(/\r?\n$/, ''));
  }

  protected confirmPaste(): void {
    const text = this.pendingPaste();
    this.pendingPaste.set(null);
    if (text) {
      this.term?.paste(text);
    }
    this.term?.focus();
  }

  protected cancelPaste(): void {
    this.pendingPaste.set(null);
    this.term?.focus();
  }

  /** Shortcuts that act on this pane: search, zoom, maximize. */
  private paneShortcut(ev: KeyboardEvent): boolean {
    switch (shortcutAction(ev)) {
      case 'search':
        this.zone.run(() => this.openSearch());
        return true;
      case 'zoomIn':
        this.zone.run(() => this.zoom.update((z) => Math.min(z + 1, 24)));
        return true;
      case 'zoomOut':
        this.zone.run(() => this.zoom.update((z) => z - 1));
        return true;
      case 'zoomReset':
        this.zone.run(() => this.zoom.set(0));
        return true;
      case 'maximize':
        this.zone.run(() => this.store.toggleMaximized(this.pane().key));
        return true;
    }
    return false;
  }

  protected openSearch(): void {
    this.searching.set(true);
    const selected = this.term?.getSelection();
    if (selected && !selected.includes('\n')) {
      this.searchText = selected;
    }
    setTimeout(() => this.searchInput()?.nativeElement.select());
  }

  protected closeSearch(): void {
    this.searching.set(false);
    this.searchResult.set(null);
    this.search?.clearDecorations();
    this.term?.focus();
  }

  protected find(back = false): void {
    if (!this.search || !this.searchText) {
      this.searchResult.set(null);
      this.search?.clearDecorations();
      return;
    }
    const options = {
      caseSensitive: this.searchCase(),
      regex: this.searchRegex(),
      decorations: {
        matchBackground: '#eab30866',
        matchOverviewRuler: '#eab308',
        activeMatchBackground: '#f97316',
        activeMatchColorOverviewRuler: '#f97316',
      },
    };
    if (back) {
      this.search.findPrevious(this.searchText, options);
    } else {
      this.search.findNext(this.searchText, options);
    }
  }

  protected onSearchKey(ev: KeyboardEvent): void {
    if (ev.key === 'Enter') {
      ev.preventDefault();
      this.find(ev.shiftKey);
    } else if (ev.key === 'Escape') {
      this.closeSearch();
    }
  }

  private attach(sessionId: string): void {
    // A terminal moved from another window first gets its recent output.
    const replay = !this.socketSession && !!this.pane().replay;
    clearTimeout(this.reattachTimer);
    const old = this.socket;
    this.socket = undefined;
    old?.close();
    this.socketSession = sessionId;
    const socket = this.api.connect();
    socket.binaryType = 'arraybuffer';
    socket.onopen = () => socket.send(encoder.encode(JSON.stringify({ sessionId, replay })));
    this.pending = 0;
    this.paused = false;
    socket.onmessage = (ev) => {
      this.reattachTries = 0;
      if (sessionId === this.socketSession) {
        this.zmodem.consume(new Uint8Array(ev.data as ArrayBuffer));
      }
    };
    // A stream that drops while the session runs is opened again: the
    // backend keeps the session, and the output in between, for a while.
    socket.onclose = () => {
      if (this.bench && !this.benchResult()) {
        this.bench.ended = true;
        if (this.pending === 0) {
          this.finishBench();
        }
      }
      if (this.destroyed || this.socket !== socket || this.pane().sessionId !== sessionId || this.pane().state !== 'connected') {
        return;
      }
      if (this.reattachTries++ < REATTACH_TRIES) {
        this.reattachTimer = setTimeout(() => {
          if (!this.destroyed && this.socket === socket && this.pane().state === 'connected') {
            this.attach(sessionId); // socketSession is set: no replay
          }
        }, REATTACH_DELAY_MS);
      }
    };
    this.socket = socket;
  }

  /** rz on the server asks for files: the file chooser opens. */
  private pickFiles(): Promise<File[]> {
    return new Promise((resolve) => {
      const input = this.uploadInput()?.nativeElement;
      if (!input) {
        resolve([]);
        return;
      }
      this.pickResolve = resolve;
      input.value = '';
      // A dialog dismissed without a choice fires "cancel" (or nothing, then focus returns).
      input.oncancel = () => this.picked([]);
      this.zone.run(() => input.click());
    });
  }

  protected picked(files: File[]): void {
    const resolve = this.pickResolve;
    this.pickResolve = undefined;
    resolve?.(files);
  }

  protected onUploadChosen(ev: Event): void {
    this.picked(Array.from((ev.target as HTMLInputElement).files ?? []));
  }

  protected cancelTransfer(): void {
    this.zmodem.cancel();
  }

  protected revealDownload(path: string): void {
    void this.files.revealLocal(path).catch(() => undefined);
  }

  protected toggleReceive(): void {
    const key = this.pane().key;
    const tab = this.store.tabs().find((t) => panesOf(t.layout).includes(key));
    if (tab) {
      this.store.toggleExcluded(tab.key, key);
    }
  }

  /** The renderer xterm.js uses: WebGL, or the DOM when there is none. */
  private renderer: 'webgl' | 'dom' = 'dom';
  /** A running measurement (benchmark panes). */
  private bench?: { start: number; bytes: number; ended: boolean };
  /** The measurement's result, shown on the pane. */
  protected readonly benchResult = signal<string | null>(null);

  /** All output drawn: the time from the first byte to the last one shown. */
  private finishBench(): void {
    const b = this.bench;
    if (!b || this.benchResult()) {
      return;
    }
    const seconds = (performance.now() - b.start) / 1000;
    const mb = b.bytes / (1024 * 1024);
    const ua = navigator.userAgent;
    const engine = /Edg\//.test(ua) ? 'WebView2' : /AppleWebKit/.test(ua) && !/Chrome/.test(ua) ? 'WebKit' : /Chrome/.test(ua) ? 'Chromium' : 'other';
    this.zone.run(() =>
      this.benchResult.set(
        this.transloco.translate('benchmark.result', {
          mb: mb.toFixed(1),
          seconds: seconds.toFixed(2),
          rate: (mb / seconds).toFixed(1),
          renderer: this.renderer === 'webgl' ? 'WebGL' : 'DOM',
          engine,
          os: navigator.platform,
        }),
      ),
    );
  }

  protected copyBench(): void {
    const r = this.benchResult();
    if (r) {
      void writeClipboard(r);
    }
  }

  /** Writes output and pauses Go while xterm.js has too much queued. */
  private write(sessionId: string, data: Uint8Array): void {
    if (!this.term) {
      return;
    }
    this.pending += data.length;
    this.watchPorts(data);
    if (this.pane().source.kind === 'benchmark') {
      this.bench ??= { start: performance.now(), bytes: 0, ended: false };
      this.bench.bytes += data.length;
    }
    this.term.write(data, () => {
      this.pending -= data.length;
      if (this.bench?.ended && this.pending === 0) {
        this.finishBench();
      }
      if (this.paused && this.pending < FLOW_LOW && sessionId === this.socketSession) {
        this.paused = false;
        void this.api.resume(sessionId).catch(() => undefined);
      }
    });
    if (!this.paused && this.pending > FLOW_HIGH) {
      this.paused = true;
      void this.api.pause(sessionId).catch(() => undefined);
    }
  }

  private send(data: Uint8Array): void {
    if (this.socket?.readyState === 1 && this.pane().state === 'connected') {
      this.socket.send(data);
    }
  }

  private scheduleFit(): void {
    clearTimeout(this.resizeTimer);
    this.resizeTimer = setTimeout(() => {
      const el = this.hostEl().nativeElement;
      if (!this.term || !this.fit || el.clientWidth === 0 || el.clientHeight === 0) {
        return; // hidden tab: fit when it becomes visible
      }
      this.fit.fit();
      this.sendSize();
    }, RESIZE_DEBOUNCE_MS);
  }

  private sendSize(): void {
    if (this.term) {
      void this.api.resize(this.pane().sessionId, this.term.cols, this.term.rows).catch(() => undefined);
    }
  }
}

/** The path of an OSC 7 report ("file://host/path"), or ''. */
export function cwdOf(data: string): string {
  const m = /^file:\/\/[^/]*(\/.*)$/.exec(data);
  if (!m) {
    return '';
  }
  try {
    return decodeURIComponent(m[1]);
  } catch {
    return m[1];
  }
}
