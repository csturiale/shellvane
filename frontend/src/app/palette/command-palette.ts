import { Component, ElementRef, computed, inject, output, signal, viewChild, afterNextRender } from '@angular/core';
import { TranslocoDirective, TranslocoService } from '@jsverse/transloco';

import { HostsStore } from '../hosts/hosts.store';
import { LanguageService } from '../core/i18n/language.service';
import { AVAILABLE_LANGUAGES, Language } from '../core/i18n/languages';
import { THEME_MODES, ThemeMode, ThemeService } from '../core/theme/theme.service';
import { Navigation } from '../layout/navigation';
import { EXTENSION_VIEWS, FULL_VIEWS, SECTIONS, View } from '../layout/views';
import { Sharing } from '../extensions/sharing';
import { SessionApi } from '../core/backend/session-api';
import type { Program } from '../core/backend/session-api';
import { SessionsStore } from '../sessions/sessions.store';
import { serialSpec } from '../sessions/serial';
import { WorkspacesStore } from '../sessions/workspaces.store';
import { search as guideSearch } from '../guide/guide';
import { GuideStore } from '../guide/guide.store';
import { EnvironmentsStore } from '../environments/environments.store';
import { SnippetsStore } from '../snippets/snippets.store';
import { systemValues, render, toTerminalInput, userVariables } from '../snippets/variables';
import { VaultStore } from '../vault/vault.store';
import { fuzzyFilter } from './fuzzy';
import { DangerousCommands } from '../broadcast/dangerous-commands';

type Group = 'host' | 'snippet' | 'action' | 'guide';

interface Command {
  group: Group;
  label: string;
  detail: string;
  run: () => void;
}

/** Prefixes restrict the search to one group. */
const PREFIXES: Record<string, Group> = { '@': 'host', '#': 'snippet', '>': 'action', '?': 'guide' };
const MAX_RESULTS = 50;

@Component({
  selector: 'app-command-palette',
  imports: [TranslocoDirective],
  templateUrl: './command-palette.html',
  styleUrl: './command-palette.scss',
})
export class CommandPalette {
  readonly closed = output<void>();
  readonly cheatsheet = output<void>();

  private readonly hosts = inject(HostsStore);
  private readonly snippets = inject(SnippetsStore);
  private readonly sessions = inject(SessionsStore);
  private readonly workspaces = inject(WorkspacesStore);
  private readonly guide = inject(GuideStore);
  private readonly environments = inject(EnvironmentsStore);
  private readonly vault = inject(VaultStore);
  private readonly sharing = inject(Sharing);
  private readonly theme = inject(ThemeService);
  private readonly language = inject(LanguageService);
  private readonly nav = inject(Navigation);
  private readonly danger = inject(DangerousCommands);
  private readonly transloco = inject(TranslocoService);

  private readonly input = viewChild.required<ElementRef<HTMLInputElement>>('input');
  protected readonly query = signal('');
  protected readonly index = signal(0);

  protected readonly results = computed(() => {
    const raw = this.query();
    const group = PREFIXES[raw[0]];
    const q = (group ? raw.slice(1) : raw).trim();
    if (group === 'guide') {
      // The guide is searched by its own index (titles and text).
      return guideSearch(this.guide.index(), q, MAX_RESULTS).map((s) => ({
        group: 'guide' as const,
        label: s.title,
        detail: this.transloco.translate('guide.topics.' + s.topic),
        run: () => this.nav.openGuide(s.topic, s.anchor === 'top' ? '' : s.anchor),
      }));
    }
    const all = this.commands().filter((c) => !group || c.group === group);
    const found = fuzzyFilter(q, all, (c) => `${c.label} ${c.detail}`).slice(0, MAX_RESULTS);
    // A typed address can always be connected to directly.
    if (!group && /^[^\s@]+@[^\s@]+$|^[\w.-]+:\d+$|^(telnet|serial):\/\/\S+$/.test(q)) {
      found.unshift({
        group: 'action',
        label: this.transloco.translate('palette.connectTo', { target: q }),
        detail: '',
        run: () => void this.sessions.openTarget(q),
      });
    }
    return found;
  });

  constructor() {
    afterNextRender(() => this.input().nativeElement.focus());
    if (!this.hosts.loaded()) {
      void this.hosts.load().catch(() => undefined);
    }
    void this.snippets.load().catch(() => undefined);
    void this.workspaces.load().catch(() => undefined);
    void this.guide.loadIndex();
    void this.environments.load().catch(() => undefined);
    void this.sessionApi
      .localShells()
      .then((list) => this.shells.set(list))
      .catch(() => undefined);
    void this.sessionApi
      .serialPorts()
      .then((list) => this.serialPorts.set(list))
      .catch(() => undefined);
  }

  private readonly serialPorts = signal<string[]>([]);

  private readonly sessionApi = inject(SessionApi);
  private readonly shells = signal<Program[]>([]);

  private commands(): Command[] {
    const t = (k: string, p?: object) => this.transloco.translate(k, p);
    const go = (view: View) => () => this.nav.view.set(view);
    const hostCommands: Command[] = this.hosts.hosts().map((h) => ({
      group: 'host',
      label: h.name,
      detail: h.hostname,
      run: () => {
        if (FULL_VIEWS.includes(this.nav.view())) {
          this.nav.view.set('hosts');
        }
        void this.sessions.openHost(h.id, h.name);
      },
    }));
    const snippetCommands: Command[] = this.snippets.snippets().map((s) => ({
      group: 'snippet',
      label: s.name,
      detail: s.body,
      run: () => {
        const focused = this.sessions.focused();
        const text = render(s.body, systemValues(focused?.target ?? null));
        const needsDialog =
          !focused ||
          userVariables(s.body).length > 0 ||
          (this.sessions.receivers(focused.key).length > 1 && this.danger.test(text));
        if (needsDialog) {
          this.nav.view.set('snippets'); // variables and confirmations live there
        } else {
          this.sessions.sendToFocused(toTerminalInput(text, s.run));
        }
      },
    }));
    const workspaceCommands: Command[] = this.workspaces.list().map((w) => ({
      group: 'action',
      label: t('workspaces.open', { name: w.name }),
      detail: 'workspace',
      run: () => {
        if (FULL_VIEWS.includes(this.nav.view())) {
          this.nav.view.set('hosts');
        }
        void this.workspaces.open(w);
      },
    }));
    const leave = () => {
      if (FULL_VIEWS.includes(this.nav.view())) {
        this.nav.view.set('hosts');
      }
    };
    const localCommands: Command[] = this.shells().map((p) => ({
      group: 'action',
      label: t('local.newWith', { shell: p.name }),
      detail: 'local terminal ' + p.path,
      run: () => {
        leave();
        void this.sessions.openLocal(p.id);
      },
    }));
    const serialCommands: Command[] = this.serialPorts().map((port) => ({
      group: 'action',
      label: t('palette.serialConsole', { port }),
      detail: 'serial console 115200',
      run: () => {
        leave();
        void this.sessions.openTarget(serialSpec(port));
      },
    }));
    // Environments with variables ask for them in their panel.
    const environmentCommands: Command[] = this.environments.list().map((e) => ({
      group: 'action',
      label: t('environments.startNamed', { name: e.name }),
      detail: 'environment',
      run: () => {
        if ((e.variables ?? []).length > 0) {
          this.nav.view.set('environments');
        } else {
          leave();
          void this.environments.start(e, {});
        }
      },
    }));
    const tab = this.sessions.active();
    const actions: Command[] = [
      { group: 'action', label: t('hosts.actions.newHost'), detail: '', run: () => this.nav.ask('hosts', 'newHost') },
      { group: 'action', label: t('hosts.actions.newGroup'), detail: '', run: () => this.nav.ask('hosts', 'newGroup') },
      { group: 'action', label: t('import.title'), detail: 'OpenSSH PuTTY KiTTY', run: () => this.nav.ask('hosts', 'import') },
      ...SECTIONS.map((s) => ({ group: 'action' as const, label: t('palette.goTo', { view: t('nav.' + s) }), detail: '', run: go(s) })),
      ...EXTENSION_VIEWS.map((v) => ({ group: 'action' as const, label: t('palette.goTo', { view: t(v.label) }), detail: '', run: go(v.id) })),
      { group: 'action', label: t('palette.goTo', { view: t('nav.settings') }), detail: '', run: go('settings') },
      ...THEME_MODES.map((m: ThemeMode) => ({
        group: 'action' as const,
        label: t('palette.theme', { theme: t('settings.themes.' + m) }),
        detail: '',
        run: () => this.theme.set(m),
      })),
      ...AVAILABLE_LANGUAGES.map((l: Language) => ({
        group: 'action' as const,
        label: t('palette.language', { language: t('settings.languageNames.' + l) }),
        detail: 'language lingua',
        run: () => this.language.set(l),
      })),
      { group: 'action', label: t('palette.shortcuts'), detail: '', run: () => this.cheatsheet.emit() },
      { group: 'action', label: t('gallery.open'), detail: 'gallery snippets themes', run: () => this.nav.gallery.set('snippets') },
      { group: 'action', label: t('logs.title'), detail: 'recordings asciinema logs', run: go('recordings') },
    ];
    if (this.vault.state() === 'unlocked') {
      actions.push({ group: 'action', label: t('vault.lockNow'), detail: 'vault', run: () => void this.vault.lock() });
      for (const c of this.sharing.paletteCommands()) {
        actions.push({ group: 'action', label: t(c.label), detail: c.detail, run: c.run });
      }
    }
    if (tab) {
      actions.push(
        { group: 'action', label: t('sessions.splitRight'), detail: '', run: () => void this.sessions.split('row') },
        { group: 'action', label: t('sessions.splitDown'), detail: '', run: () => void this.sessions.split('column') },
        { group: 'action', label: t('broadcast.toggle'), detail: 'broadcast', run: () => this.sessions.toggleBroadcast(tab.key) },
        { group: 'action', label: t('sessions.closeTab'), detail: '', run: () => void this.sessions.closeTab(tab.key) },
      );
    }
    return [...hostCommands, ...snippetCommands, ...environmentCommands, ...localCommands, ...serialCommands, ...workspaceCommands, ...actions];
  }

  protected onInput(ev: Event): void {
    this.query.set((ev.target as HTMLInputElement).value);
    this.index.set(0);
  }

  protected onKeydown(ev: KeyboardEvent): void {
    const n = this.results().length;
    switch (ev.key) {
      case 'ArrowDown':
        ev.preventDefault();
        this.index.set(n ? (this.index() + 1) % n : 0);
        break;
      case 'ArrowUp':
        ev.preventDefault();
        this.index.set(n ? (this.index() - 1 + n) % n : 0);
        break;
      case 'Enter':
        ev.preventDefault();
        this.choose(this.results()[this.index()]);
        break;
      case 'Escape':
        ev.preventDefault();
        this.closed.emit();
        break;
    }
  }

  protected choose(c: Command | undefined): void {
    if (c) {
      this.closed.emit();
      c.run();
    }
  }
}
