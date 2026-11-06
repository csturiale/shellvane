import { Component, Injector, computed, effect, inject, signal, untracked, Type } from '@angular/core';
import { TranslocoDirective } from '@jsverse/transloco';
import { LucideLock, LucideLockOpen, LucideSearch, LucideSquareTerminal } from '@lucide/angular';

import { BackendService } from './core/backend/backend.service';
import { HostsPanel } from './hosts/hosts-panel';
import { SnippetsPanel } from './snippets/snippets-panel';
import { TunnelsPanel } from './tunnels/tunnels-panel';
import { KeysPanel } from './keys/keys-panel';
import { TunnelsStore } from './tunnels/tunnels.store';
import { ActivityBar } from './layout/activity-bar';
import { Navigation } from './layout/navigation';
import { EXTENSION_VIEWS, FULL_VIEWS, Section } from './layout/views';
import { FilesView } from './files/files-view';
import { AutomationView } from './automation/automation-view';
import { PluginsView } from './plugins/plugins-view';
import { Sharing } from './extensions/sharing';
import { GuideView } from './guide/guide-view';
import { RecordingsView } from './recordings/recordings-view';
import { EnvironmentsPanel } from './environments/environments-panel';
import { EnvironmentsStore } from './environments/environments.store';
import { EnvironmentNotice } from './environments/environment-notice';
import { readPreference, writePreference } from './core/preferences';
import { View } from './layout/views';
import { TriggerToasts } from './automation/trigger-toasts';
import { PromptDialog } from './sessions/prompt-dialog';
import { SessionsStore, labelOf } from './sessions/sessions.store';
import { serialSpec } from './sessions/serial';
import { SessionApi } from './core/backend/session-api';
import { ContextMenu } from './shared/context-menu';
import { Workspace } from './sessions/workspace';
import { WorkspacesStore } from './sessions/workspaces.store';
import { WorkspaceMenu } from './sessions/workspace-menu';
import { SettingsPage } from './settings/settings-page';
import { Cheatsheet } from './palette/cheatsheet';
import { CommandPalette } from './palette/command-palette';
import { combo, shortcutAction } from './sessions/shortcuts';
import { VaultStore } from './vault/vault.store';
import { HostsStore } from './hosts/hosts.store';
import { closeWindow, detachedToken } from './sessions/detached';
import { WelcomeTour } from './onboarding/welcome-tour';
import type { Info } from '../../bindings/github.com/csturiale/shellvane/app';
import type { UpdatesStore } from './updates/updates.store';
import { guardLinks } from './core/browser';
import { NgComponentOutlet } from '@angular/common';
import { EXTENSIONS } from './extensions/extensions';

@Component({
  selector: 'app-root',
  imports: [
    NgComponentOutlet,
    TranslocoDirective,
    ContextMenu,
    LucideSearch,
    LucideLock,
    LucideLockOpen,
    LucideSquareTerminal,
    ActivityBar,
    HostsPanel,
    SnippetsPanel,
    TunnelsPanel,
    KeysPanel,
    SettingsPage,
    Workspace,
    WorkspaceMenu,
    PromptDialog,
    FilesView,
    AutomationView,
    PluginsView,
    GuideView,
    RecordingsView,
    EnvironmentsPanel,
    TriggerToasts,
    EnvironmentNotice,
    CommandPalette,
    Cheatsheet,
    WelcomeTour,
  ],
  templateUrl: './app.html',
  styleUrl: './app.scss',
  host: {
    '[class.no-panel]': 'panelHidden()',
    '[class.detached]': '!!detached',
    '[style.--panel-width.px]': 'panelWidth()',
    '(document:keydown)': 'onKeydown($event)',
  },
})
export class App {
  /** Components extensions render once at the root. */
  protected readonly extensionRoots = EXTENSIONS.flatMap((e) => e.root ?? []);
  /** The extension view on screen, if any, once its chunk is loaded. */
  protected readonly extensionView = signal<{ id: string; component: Type<unknown> } | null>(null);
  protected readonly backend = inject(BackendService);
  protected readonly sessions = inject(SessionsStore);
  protected readonly paletteKeys = combo('K');
  private readonly sessionApi = inject(SessionApi);
  protected readonly label = labelOf;
  protected readonly vault = inject(VaultStore);
  protected readonly tunnels = inject(TunnelsStore);
  private readonly injector = inject(Injector);
  /** Loaded after startup; shows a notice when an update is available. */
  protected readonly updates = signal<UpdatesStore | null>(null);
  private readonly workspaces = inject(WorkspacesStore);
  private readonly environments = inject(EnvironmentsStore);

  private readonly nav = inject(Navigation);
  protected readonly view = this.nav.view;
  protected readonly fullView = computed(() => FULL_VIEWS.includes(this.view()));
  protected readonly section = computed<Section | null>(() => (this.fullView() ? null : (this.view() as Section)));
  protected readonly info = signal<Info | null>(null);
  /** The guide's "What's new" was not opened since this version. */
  protected readonly news = computed(() => {
    const v = this.info()?.version;
    return !!v && readPreference('guide.seenVersion') !== v;
  });

  /** The side panel can be hidden and resized, for more room for terminals. */
  protected readonly panelCollapsed = signal(readPreference('layout.panelCollapsed') === 'true');
  protected readonly panelWidth = signal(clampPanel(Number(readPreference('layout.panelWidth')) || DEFAULT_PANEL));
  /** A maximized pane also hides the side panel, to give it all the room. */
  protected readonly paneMaximized = computed(() => !this.fullView() && (this.sessions.active()?.maximized ?? null) !== null);
  protected readonly panelHidden = computed(() => this.fullView() || !!this.detached || this.panelCollapsed() || this.paneMaximized());

  protected togglePanel(): void {
    const max = this.sessions.active()?.maximized ?? null;
    if (this.paneMaximized() && max !== null) {
      this.sessions.toggleMaximized(max); // showing the panel restores the pane
      this.setCollapsed(false);
      return;
    }
    this.setCollapsed(!this.panelCollapsed());
  }

  private setCollapsed(on: boolean): void {
    this.panelCollapsed.set(on);
    writePreference('layout.panelCollapsed', String(on));
  }

  /** Dragging the panel's edge resizes it; dropping it very narrow hides it. */
  protected startResize(ev: PointerEvent): void {
    ev.preventDefault();
    const handle = ev.target as HTMLElement;
    handle.setPointerCapture(ev.pointerId);
    const left = handle.parentElement?.getBoundingClientRect().left ?? 0;
    const move = (e: PointerEvent) => {
      const width = e.clientX - left - ACTIVITY_WIDTH;
      if (width < MIN_PANEL / 2) {
        this.setCollapsed(true);
      } else {
        this.setCollapsed(false);
        this.panelWidth.set(clampPanel(width));
      }
    };
    const up = () => {
      handle.removeEventListener('pointermove', move);
      handle.removeEventListener('pointerup', up);
      writePreference('layout.panelWidth', String(this.panelWidth()));
    };
    handle.addEventListener('pointermove', move);
    handle.addEventListener('pointerup', up);
  }

  protected onSelect(view: View): void {
    // Clicking the open section again hides its panel; any section shows it.
    if (!FULL_VIEWS.includes(view)) {
      this.setCollapsed(view === this.view() && !this.panelCollapsed());
    }
    if (view === 'guide' && this.news()) {
      writePreference('guide.seenVersion', this.info()!.version);
      this.info.update((i) => (i ? { ...i } : i)); // recompute news
      this.nav.openGuide('whats-new');
      return;
    }
    if (view === 'guide') {
      this.nav.openGuide(this.nav.guideTopic().topic, '');
      return;
    }
    this.view.set(view);
  }
  /** Set in a window that shows a tab moved out of the main window. */
  protected readonly detached = detachedToken();

  /** A fresh install (no vault, no hosts) starts with the tour, once. */
  private readonly hostsStore = inject(HostsStore);
  private readonly tourDismissed = signal(readPreference('tour.done') === 'true');
  protected readonly showTour = computed(
    () =>
      this.nav.tour() ||
      (!this.detached && !this.tourDismissed() && this.vault.state() === 'notSetUp' && this.hostsStore.loaded() && this.hostsStore.hosts().length === 0),
  );

  protected closeTour(): void {
    writePreference('tour.done', 'true');
    this.nav.tour.set(false);
    this.tourDismissed.set(true);
  }
  protected readonly paletteOpen = signal(false);
  protected readonly cheatsheetOpen = signal(false);

  private readonly sharing = inject(Sharing);

  constructor() {
    guardLinks(document);
    // An extension's view loads when it is opened.
    effect(() => {
      const v = EXTENSION_VIEWS.find((x) => x.id === this.view());
      if (v && this.extensionView()?.id !== v.id) {
        void v.load().then((component) => this.extensionView.set({ id: v.id, component }));
      }
    });
    // Shared objects (for example a team's) follow once the vault is open.
    effect(() => {
      if (!this.detached && this.vault.state() === 'unlocked') {
        untracked(() => this.sharing.start());
      }
    });
    // A window holding moved tabs closes with its last tab.
    let adopted = false;
    effect(() => {
      const count = this.sessions.tabs().length;
      if (this.detached && count > 0) {
        adopted = true;
      } else if (this.detached && adopted && count === 0) {
        closeWindow();
      }
    });
    // A panel asked to do something (new host, import...) shows itself.
    effect(() => {
      if (this.nav.request() && !this.fullView()) {
        untracked(() => this.setCollapsed(false));
      }
    });
    void this.backend.info().then((info) => {
      this.info.set(info);
      if (info && this.detached) {
        // A window holding a moved tab: only its terminals, no restore.
        void this.sessions.adopt(this.detached).catch(() => closeWindow());
        return;
      }
      if (info) {
        // Tabs that detached windows give back.
        this.sessionApi.onReturn((token) => void this.sessions.adopt(token).catch(() => undefined));
        void this.tunnels.init().catch(() => undefined);
        // Loaded after startup to keep the first bundle small.
        void import('./themes/themes.store').then((m) => this.injector.get(m.ThemesStore).load()).catch(() => undefined);
        void import('./keys/keys.store').then((m) => this.injector.get(m.KeysStore).initAgent()).catch(() => undefined);
        void import('./core/open-requests').then((m) => this.injector.get(m.OpenRequests).start());
        void import('./sync/sync-reload').then((m) => this.injector.get(m.SyncReload).start());
        void import('./updates/updates.store').then((m) => {
          const store = this.injector.get(m.UpdatesStore);
          this.updates.set(store);
          return store.checkDaily();
        });
        // Environments' panes come back as plain shells: start their environments again.
        void this.workspaces.start().then(() => this.environments.resumeRestored()).catch(() => undefined);
      }
    });
  }

  protected onKeydown(ev: KeyboardEvent): void {
    const action = shortcutAction(ev);
    if (action === 'palette') {
      ev.preventDefault();
      this.paletteOpen.update((open) => !open);
    } else if (action === 'newLocal') {
      ev.preventDefault();
      if (FULL_VIEWS.includes(this.view())) {
        this.view.set('hosts');
      }
      void this.sessions.openLocal();
    } else if (action === 'cheatsheet') {
      ev.preventDefault();
      this.cheatsheetOpen.set(true);
    } else if (action === 'togglePanel' && !this.fullView()) {
      ev.preventDefault();
      this.togglePanel();
    }
  }

  /** Last quick connections, newest first. */
  protected readonly quickHistory = signal<string[]>(readHistory());
  /** Serial ports offered in the quick connect list, read when it gets focus. */
  protected readonly serialSuggestions = signal<string[]>([]);

  protected async loadSerialPorts(): Promise<void> {
    const ports = await this.sessionApi.serialPorts().catch(() => [] as string[]);
    this.serialSuggestions.set(ports.map(serialSpec).filter((s) => !this.quickHistory().includes(s)));
  }

  protected async quickConnect(input: HTMLInputElement): Promise<void> {
    const spec = input.value.trim();
    if (!spec) {
      return;
    }
    await this.sessions.openTarget(spec);
    input.value = '';
    this.quickHistory.update((list) => [spec, ...list.filter((s) => s !== spec)].slice(0, QUICK_HISTORY));
    writePreference('quick.history', JSON.stringify(this.quickHistory()));
    if (this.view() === 'settings') {
      this.view.set('hosts');
    }
  }
}

const QUICK_HISTORY = 15;

function readHistory(): string[] {
  try {
    return JSON.parse(readPreference('quick.history') ?? '[]') as string[];
  } catch {
    return [];
  }
}

/** Widths of the side panel, in pixels. */
const DEFAULT_PANEL = 260;
const MIN_PANEL = 180;
const MAX_PANEL = 560;
const ACTIVITY_WIDTH = 48;

function clampPanel(width: number): number {
  return Math.round(Math.min(MAX_PANEL, Math.max(MIN_PANEL, width)));
}
