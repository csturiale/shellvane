import { NgComponentOutlet } from '@angular/common';
import { Sharing } from '../extensions/sharing';
import { Component, computed, inject, input, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { Environment } from '../core/backend/environment-api';
import { toTranslatableError, TranslatableError } from '../core/backend/errors';
import { HelpLink } from '../guide/help-link';
import { parseLines } from '../hosts/form-utils';
import { HostsStore } from '../hosts/hosts.store';
import { WorkspacesStore } from '../sessions/workspaces.store';
import { Modal } from '../shared/modal';
import { describeTunnel } from '../tunnels/describe';
import { TunnelsStore } from '../tunnels/tunnels.store';
import { EnvironmentsStore } from './environments.store';
import { BROWSERS, BrowserPrefs } from '../sessions/browser-prefs';
import { Literals } from '../core/i18n/literals';
import { EnvironmentApi } from '../core/backend/environment-api';
import { PluginApi } from '../core/backend/plugin-api';
import { CredentialSource } from '../extensions/extension';

interface VariableRow {
  name: string;
  default: string;
  choices: string;
}

/** Creates or edits an environment, one section per step. */
@Component({
  selector: 'app-environment-dialog',
  imports: [NgComponentOutlet, FormsModule, TranslocoDirective, Modal, HelpLink],
  templateUrl: './environment-dialog.html',
  styleUrls: ['../shared/form.scss', './environment-dialog.scss'],
})
export class EnvironmentDialog {
  protected readonly sharing = inject(Sharing);
  /** Literal placeholder shown in the hint. */
  protected readonly hint = new Literals({ example: '{{name}}' });
  /** null creates one. */
  readonly environment = input<Environment | null>(null);
  readonly closed = output<void>();

  private readonly store = inject(EnvironmentsStore);
  private readonly hostsStore = inject(HostsStore);
  protected readonly tunnels = inject(TunnelsStore);
  protected readonly workspaces = inject(WorkspacesStore);
  protected readonly describe = describeTunnel;
  protected readonly hosts = computed(() =>
    [...this.hostsStore.hosts()].sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  );

  protected name = '';
  protected description = '';
  protected variables: VariableRow[] = [];
  protected check = '';
  protected login = '';
  protected prepare = '';
  protected readonly prepareExample = 'gcloud container clusters get-credentials {{cluster}} --region {{region}}';
  protected hostId = '';
  protected terminal = true;
  protected script = '';
  protected kubeContext = '';
  protected kubeNamespace = '';
  protected tunnelIds: string[] = [];
  protected localContext = '';
  protected localNamespace = '';
  protected localTunnel = '';
  protected localTerminals = 0;
  protected workspaceId = '';
  /** gcloud profile: its own gcloud configuration for this environment. */
  protected gcloud = '';
  /** The login's sign-in page opens in a private window. */
  protected privateBrowser = false;
  /** Browser of the sign-in page; '' uses the one of the settings. */
  protected browser = '';
  /** Sign-in links printed in the login terminal open in that browser. */
  protected openLinks = false;
  protected readonly browsers = BROWSERS;
  protected readonly browserPrefs = inject(BrowserPrefs);
  /** Login secret: a credential plugin configuration (or a team login) and its account. */
  protected secretSource = '';
  protected secretAddress = '';
  protected secretUser = '';
  protected secretSafe = '';
  protected readonly gcloudProfiles = signal<string[]>([]);
  protected readonly brokers = signal<{ id: string; name: string }[]>([]);
  protected readonly sharedLogins = signal<CredentialSource[]>([]);
  private readonly envApi = inject(EnvironmentApi);
  private readonly pluginApi = inject(PluginApi);
  protected readonly saving = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);

  ngOnInit(): void {
    void this.tunnels.load().catch(() => undefined);
    void this.workspaces.load().catch(() => undefined);
    void this.loadSources();
    const e = this.environment();
    if (!e) {
      return;
    }
    this.name = e.name;
    this.description = e.description;
    this.variables = (e.variables ?? []).map((v) => ({ name: v.name, default: v.default, choices: (v.choices ?? []).join(', ') }));
    this.check = e.login.check;
    this.login = e.login.login;
    this.prepare = e.login.prepare ?? '';
    this.gcloud = e.gcloud ?? '';
    this.privateBrowser = !!e.login.privateBrowser;
    this.browser = e.login.browser ?? '';
    this.openLinks = !!e.login.openLinks;
    const s = e.login.secret;
    [this.secretSource, this.secretAddress, this.secretUser, this.secretSafe] = [s?.source ?? '', s?.address ?? '', s?.user ?? '', s?.safe ?? ''];
    this.hostId = e.remote.hostId;
    this.terminal = e.remote.terminal;
    this.script = (e.remote.script ?? []).join('\n');
    this.kubeContext = e.remote.kubeContext;
    this.kubeNamespace = e.remote.kubeNamespace;
    this.tunnelIds = [...(e.tunnelIds ?? [])];
    this.localContext = e.localKube.context;
    this.localNamespace = e.localKube.namespace;
    this.localTunnel = e.localKube.tunnelId;
    this.localTerminals = e.localKube.terminals;
    this.workspaceId = e.workspaceId;
  }

  /** Credential plugins and team logins that can give the login's password; gcloud profiles in use. */
  private async loadSources(): Promise<void> {
    try {
      const [plugins, configs] = await Promise.all([this.pluginApi.plugins(), this.pluginApi.configs()]);
      const brokering = new Set(plugins.filter((p) => p.installed && p.capabilities?.includes('credentials')).map((p) => p.id));
      this.brokers.set(configs.filter((c) => brokering.has(c.pluginId)).map((c) => ({ id: c.id, name: c.name })));
    } catch {
      // Plugins unavailable (tests, browser without backend).
    }
    this.sharing.credentialSources().then((l) => this.sharedLogins.set(l), () => undefined);
    this.envApi.gcloudProfiles().then((p) => this.gcloudProfiles.set(p), () => undefined);
  }

  protected toggleTunnel(id: string, on: boolean): void {
    this.tunnelIds = on ? [...this.tunnelIds, id] : this.tunnelIds.filter((t) => t !== id);
  }

  /** Presets for the usual cloud logins. */
  protected preset(kind: 'gcloud' | 'aws' | 'azure'): void {
    const presets = {
      gcloud: ['gcloud auth print-access-token', 'gcloud auth login --update-adc'],
      aws: ['aws sts get-caller-identity --profile {{profile}}', 'aws sso login --profile {{profile}}'],
      azure: ['az account show', 'az login'],
    };
    [this.check, this.login] = presets[kind];
    // Browser sign-ins: a session already open in the browser could pick the wrong account.
    this.privateBrowser = kind !== 'aws' || this.privateBrowser;
  }

  /**
   * A private GKE cluster reached through a jump host: log in with gcloud,
   * write the context here (get-credentials) and open local terminals whose
   * kubectl goes through the tunnel chosen in step 4.
   */
  protected presetGke(): void {
    this.preset('gcloud');
    for (const name of ['project', 'region', 'cluster']) {
      if (!this.variables.some((v) => v.name === name)) {
        this.variables.push({ name, default: '', choices: '' });
      }
    }
    this.prepare = 'gcloud container clusters get-credentials {{cluster}} --region {{region}} --project {{project}}';
    this.localContext = 'gke_{{project}}_{{region}}_{{cluster}}';
    this.localTerminals = Math.max(1, this.localTerminals);
    this.terminal = false;
  }

  protected async save(): Promise<void> {
    const e: Environment = {
      id: this.environment()?.id ?? '',
      name: this.name,
      description: this.description,
      variables: this.variables
        .filter((v) => v.name.trim())
        .map((v) => ({
          name: v.name.trim(),
          default: v.default,
          choices: v.choices
            .split(',')
            .map((c) => c.trim())
            .filter(Boolean),
        })),
      login: {
        check: this.check.trim(),
        login: this.login.trim(),
        prepare: this.prepare.trim(),
        secret: this.secretSource
          ? { source: this.secretSource, address: this.secretAddress.trim(), user: this.secretUser.trim(), safe: this.secretSafe.trim() }
          : { source: '', address: '', user: '', safe: '' },
        privateBrowser: this.privateBrowser,
        browser: this.browser,
        openLinks: this.openLinks,
      },
      gcloud: this.gcloud.trim(),
      remote: {
        hostId: this.hostId,
        terminal: this.terminal,
        script: parseLines(this.script) ?? [],
        kubeContext: this.kubeContext.trim(),
        kubeNamespace: this.kubeNamespace.trim(),
      },
      tunnelIds: this.tunnelIds,
      localKube: {
        context: this.localContext.trim(),
        namespace: this.localNamespace.trim(),
        tunnelId: this.localTunnel,
        terminals: Number(this.localTerminals) || 0,
      },
      workspaceId: this.workspaceId,
    };
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.store.save(e);
      this.closed.emit();
    } catch (err) {
      this.error.set(toTranslatableError(err));
    } finally {
      this.saving.set(false);
    }
  }
}
