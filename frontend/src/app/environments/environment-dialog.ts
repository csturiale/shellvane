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
import { Literals } from '../core/i18n/literals';

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
  protected readonly saving = signal(false);
  protected readonly error = signal<TranslatableError | null>(null);

  ngOnInit(): void {
    void this.tunnels.load().catch(() => undefined);
    void this.workspaces.load().catch(() => undefined);
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

  protected toggleTunnel(id: string, on: boolean): void {
    this.tunnelIds = on ? [...this.tunnelIds, id] : this.tunnelIds.filter((t) => t !== id);
  }

  /** Presets for the usual cloud logins. */
  protected preset(kind: 'gcloud' | 'aws' | 'azure'): void {
    const presets = {
      gcloud: ['gcloud auth print-access-token', 'gcloud auth login'],
      aws: ['aws sts get-caller-identity --profile {{profile}}', 'aws sso login --profile {{profile}}'],
      azure: ['az account show', 'az login'],
    };
    [this.check, this.login] = presets[kind];
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
      login: { check: this.check.trim(), login: this.login.trim(), prepare: this.prepare.trim() },
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
