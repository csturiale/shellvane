import { Component, computed, inject, input, signal } from '@angular/core';
import { ControlContainer, FormsModule, NgForm } from '@angular/forms';
import { TranslocoDirective } from '@jsverse/transloco';

import { Settings } from '../core/backend/host-api';
import { optionalText, parseLines } from './form-utils';
import { HostsStore } from './hosts.store';
import { HelpLink } from '../guide/help-link';
import { PluginApi } from '../core/backend/plugin-api';
import { CredentialSource } from '../extensions/extension';
import { Sharing } from '../extensions/sharing';

/** Editable copy of the connection settings shared by hosts and groups. */
export interface ConnectionModel {
  jumps: string[];
  /** Custom authentication order; when false the methods are inherited. */
  authCustom: boolean;
  /** All methods, in order; "on" ones are used. */
  authMethods: { id: AuthMethod; on: boolean }[];
  identitiesOnly: '' | 'on' | 'off';
  /** Kerberos (GSSAPI) single sign-on: '' inherits. */
  kerberos: '' | 'on' | 'off';
  kerberosRealm: string;
  kerberosPrincipal: string;
  proxy: string;
  proxyCommand: string;
  /** Cloud access service: '' direct, 'awsSsm', 'azureBastion', 'gcpIap'. */
  transport: string;
  cloudTarget: string;
  cloudRegion: string;
  cloudAccount: string;
  cloudBastion: string;
  cloudGroup: string;
  passwordCommand: string;
  certificateCommand: string;
  preConnectCommand: string;
  /** Plugin configuration brokering the login; '' inherits. */
  credentialPlugin: string;
  /** '' inherits, 'on' and 'off' override. */
  forwardAgent: '' | 'on' | 'off';
  forwardX11: '' | 'on' | 'off';
  /** '' inherits, 'none' turns it off. */
  multiplexer: '' | 'none' | 'tmux' | 'screen';
  autoReconnect: '' | 'on' | 'off';
  kubeContext: string;
  kubeNamespace: string;
  /** '' inherits. */
  encoding: string;
  termType: string;
  logSession: '' | 'on' | 'off';
  logFormat: string;
  recordSession: '' | 'on' | 'off';
  initialDirectory: string;
  /** NAME=value, one per line. */
  environment: string;
  remoteCommand: string;
  macAddress: string;
  wakeBroadcast: string;
  wakeOnConnect: '' | 'on' | 'off';
  knockSequence: string;
  /** '' inherits, 'none' turns it off. */
  remoteDesktop: '' | 'none' | 'rdp' | 'vnc';
  remoteDesktopPort: string;
  remoteDesktopViaSsh: '' | 'on' | 'off';
}

export type AuthMethod = 'publickey' | 'keyboard-interactive' | 'password';
/** OpenSSH's order. */
export const AUTH_METHODS: AuthMethod[] = ['publickey', 'keyboard-interactive', 'password'];

function authList(order: string[] | null | undefined): { id: AuthMethod; on: boolean }[] {
  const chosen = (order ?? []).filter((m): m is AuthMethod => AUTH_METHODS.includes(m as AuthMethod));
  const list = chosen.map((id) => ({ id, on: true }));
  for (const id of AUTH_METHODS) {
    if (!chosen.includes(id)) {
      list.push({ id, on: !order?.length });
    }
  }
  return list;
}

/** Encodings offered for old servers; UTF-8 is the default. */
export const ENCODINGS = ['UTF-8', 'ISO-8859-1', 'ISO-8859-15', 'windows-1252', 'ISO-8859-2', 'KOI8-R', 'Shift_JIS', 'EUC-JP', 'GBK', 'Big5', 'EUC-KR'];

export function connectionModel(s: Settings | undefined): ConnectionModel {
  return {
    jumps: [...(s?.jumpChain ?? [])],
    authCustom: !!s?.authMethods?.length,
    authMethods: authList(s?.authMethods),
    identitiesOnly: tristate(s?.identitiesOnly),
    kerberos: tristate(s?.kerberos),
    kerberosRealm: s?.kerberosRealm ?? '',
    kerberosPrincipal: s?.kerberosPrincipal ?? '',
    proxy: s?.proxy ?? '',
    proxyCommand: s?.proxyCommand ?? '',
    transport: s?.transport ?? '',
    cloudTarget: s?.cloudTarget ?? '',
    cloudRegion: s?.cloudRegion ?? '',
    cloudAccount: s?.cloudAccount ?? '',
    cloudBastion: s?.cloudBastion ?? '',
    cloudGroup: s?.cloudGroup ?? '',
    passwordCommand: s?.passwordCommand ?? '',
    certificateCommand: s?.certificateCommand ?? '',
    preConnectCommand: s?.preConnectCommand ?? '',
    credentialPlugin: s?.credentialPlugin ?? '',
    forwardAgent: tristate(s?.forwardAgent),
    forwardX11: tristate(s?.forwardX11),
    multiplexer: s?.multiplexer === undefined || s?.multiplexer === null ? '' : ((s.multiplexer || 'none') as ConnectionModel['multiplexer']),
    autoReconnect: tristate(s?.autoReconnect),
    kubeContext: s?.kubeContext ?? '',
    kubeNamespace: s?.kubeNamespace ?? '',
    encoding: s?.encoding ?? '',
    termType: s?.termType ?? '',
    logSession: tristate(s?.logSession),
    logFormat: s?.logFormat ?? '',
    recordSession: tristate(s?.recordSession),
    initialDirectory: s?.initialDirectory ?? '',
    environment: (s?.environment ?? []).join('\n'),
    remoteCommand: s?.remoteCommand ?? '',
    macAddress: s?.macAddress ?? '',
    wakeBroadcast: s?.wakeBroadcast ?? '',
    wakeOnConnect: tristate(s?.wakeOnConnect),
    knockSequence: s?.knockSequence ?? '',
    remoteDesktop: s?.remoteDesktop === undefined || s?.remoteDesktop === null ? '' : ((s.remoteDesktop || 'none') as ConnectionModel['remoteDesktop']),
    remoteDesktopPort: s?.remoteDesktopPort ? String(s.remoteDesktopPort) : '',
    remoteDesktopViaSsh: tristate(s?.remoteDesktopViaSsh),
  };
}

function tristate(v: boolean | null | undefined): '' | 'on' | 'off' {
  return v === true ? 'on' : v === false ? 'off' : '';
}

export function connectionSettings(m: ConnectionModel): Partial<Settings> {
  return {
    jumpChain: m.jumps.length ? m.jumps.filter(Boolean) : null,
    authMethods: m.authCustom && m.authMethods.some((a) => a.on) ? m.authMethods.filter((a) => a.on).map((a) => a.id) : null,
    identitiesOnly: m.identitiesOnly === '' ? null : m.identitiesOnly === 'on',
    kerberos: m.kerberos === '' ? null : m.kerberos === 'on',
    kerberosRealm: m.kerberos === 'on' ? optionalText(m.kerberosRealm) : null,
    kerberosPrincipal: m.kerberos === 'on' ? optionalText(m.kerberosPrincipal) : null,
    proxy: optionalText(m.proxy),
    proxyCommand: optionalText(m.proxyCommand),
    transport: optionalText(m.transport),
    cloudTarget: optionalText(m.cloudTarget),
    cloudRegion: optionalText(m.cloudRegion),
    cloudAccount: optionalText(m.cloudAccount),
    cloudBastion: optionalText(m.cloudBastion),
    cloudGroup: optionalText(m.cloudGroup),
    passwordCommand: optionalText(m.passwordCommand),
    certificateCommand: optionalText(m.certificateCommand),
    preConnectCommand: optionalText(m.preConnectCommand),
    credentialPlugin: optionalText(m.credentialPlugin),
    forwardAgent: m.forwardAgent === '' ? null : m.forwardAgent === 'on',
    forwardX11: m.forwardX11 === '' ? null : m.forwardX11 === 'on',
    multiplexer: m.multiplexer === '' ? null : m.multiplexer === 'none' ? '' : m.multiplexer,
    autoReconnect: m.autoReconnect === '' ? null : m.autoReconnect === 'on',
    kubeContext: optionalText(m.kubeContext),
    kubeNamespace: optionalText(m.kubeNamespace),
    encoding: optionalText(m.encoding),
    termType: optionalText(m.termType),
    logSession: m.logSession === '' ? null : m.logSession === 'on',
    logFormat: optionalText(m.logFormat),
    recordSession: m.recordSession === '' ? null : m.recordSession === 'on',
    initialDirectory: optionalText(m.initialDirectory),
    environment: parseLines(m.environment),
    remoteCommand: optionalText(m.remoteCommand),
    macAddress: optionalText(m.macAddress),
    wakeBroadcast: optionalText(m.wakeBroadcast),
    wakeOnConnect: m.wakeOnConnect === '' ? null : m.wakeOnConnect === 'on',
    knockSequence: optionalText(m.knockSequence),
    remoteDesktop: m.remoteDesktop === '' ? null : m.remoteDesktop === 'none' ? '' : m.remoteDesktop,
    remoteDesktopPort: Number(m.remoteDesktopPort) > 0 ? Number(m.remoteDesktopPort) : null,
    remoteDesktopViaSsh: m.remoteDesktopViaSsh === '' ? null : m.remoteDesktopViaSsh === 'on',
  };
}

/** Ready-made commands for common cloud access brokers (%h %p %r expanded). */
export const PROXY_PRESETS: { id: string; command: string }[] = [
  { id: 'awsSsm', command: 'aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p' },
  { id: 'gcpIap', command: 'gcloud compute start-iap-tunnel %h %p --listen-on-stdin' },
  { id: 'cloudflare', command: 'cloudflared access ssh --hostname %h' },
  { id: 'teleport', command: 'tsh proxy ssh %r@%h:%p' },
  { id: 'netcat', command: 'nc %h %p' },
];

/** Ready-made password and certificate commands. */
export const PASSWORD_PRESETS: { id: string; command: string }[] = [
  { id: 'pass', command: 'pass show ssh/$SHELLVANE_HOST' },
  { id: 'onePassword', command: 'op read "op://Private/$SHELLVANE_HOST/password"' },
  { id: 'bitwarden', command: 'bw get password $SHELLVANE_HOST' },
  { id: 'vaultKv', command: 'vault kv get -field=password secret/ssh/$SHELLVANE_HOST' },
];
export const CERTIFICATE_PRESETS: { id: string; command: string }[] = [
  { id: 'vaultSsh', command: 'vault write -field=signed_key ssh-client-signer/sign/default public_key=@$SHELLVANE_PUBLIC_KEY_FILE' },
  {
    id: 'stepCa',
    command: 'step ssh certificate --sign --principal $SHELLVANE_USER $SHELLVANE_USER $SHELLVANE_PUBLIC_KEY_FILE -f >/dev/null && cat "${SHELLVANE_PUBLIC_KEY_FILE%.pub}-cert.pub"',
  },
];

/** Commands run before connecting. */
export const PRECONNECT_PRESETS: { id: string; command: string }[] = [
  {
    id: 'ec2InstanceConnect',
    command: 'aws ec2-instance-connect send-ssh-public-key --instance-id $SHELLVANE_HOST --instance-os-user $SHELLVANE_USER --ssh-public-key file://$SHELLVANE_PUBLIC_KEY_FILE',
  },
  { id: 'gcpOsLogin', command: 'gcloud compute os-login ssh-keys add --key-file=$SHELLVANE_PUBLIC_KEY_FILE --ttl=1h' },
];

/** The "Advanced" section of the host and group dialogs. */
@Component({
  selector: 'app-connection-options',
  imports: [HelpLink, FormsModule, TranslocoDirective],
  templateUrl: './connection-options.html',
  styleUrls: ['../shared/form.scss', './connection-options.scss'],
  // Inputs join the parent's template-driven form.
  viewProviders: [{ provide: ControlContainer, useExisting: NgForm }],
})
export class ConnectionOptions {
  readonly model = input.required<ConnectionModel>();
  /** The edited host, which cannot be its own jump host. */
  readonly selfId = input('');

  private readonly store = inject(HostsStore);
  private readonly pluginApi = inject(PluginApi);
  private readonly sharing = inject(Sharing);
  /** Logins shared with this person (for example by a team). */
  protected readonly sharedLogins = signal<CredentialSource[]>([]);
  /** Plugin configurations that broker logins (Vault, Teleport, Boundary…). */
  protected readonly brokers = signal<{ id: string; name: string }[]>([]);
  protected readonly proxyPresets = PROXY_PRESETS;
  protected readonly encodings = ENCODINGS;
  protected readonly passwordPresets = PASSWORD_PRESETS;
  protected readonly certificatePresets = CERTIFICATE_PRESETS;
  protected readonly preConnectPresets = PRECONNECT_PRESETS;

  constructor() {
    void this.loadBrokers();
  }

  protected hasBroker(id: string): boolean {
    return this.brokers().some((b) => b.id === id) || this.sharedLogins().some((l) => l.ref === id);
  }

  private async loadBrokers(): Promise<void> {
    try {
      const [plugins, configs] = await Promise.all([this.pluginApi.plugins(), this.pluginApi.configs()]);
      const brokering = new Set(plugins.filter((p) => p.installed && p.capabilities?.includes('credentials')).map((p) => p.id));
      this.brokers.set(configs.filter((c) => brokering.has(c.pluginId)).map((c) => ({ id: c.id, name: c.name })));
    } catch {
      // Plugins unavailable (tests, browser without backend).
    }
    try {
      this.sharedLogins.set(await this.sharing.credentialSources());
    } catch {
      // Nothing shared, or the vault is locked.
    }
  }

  protected readonly jumpCandidates = computed(() =>
    [...this.store.hosts()]
      .filter((h) => h.id !== this.selfId())
      .sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })),
  );

  protected hasAuth(): boolean {
    return this.model().authMethods.some((a) => a.on);
  }

  protected open(): boolean {
    const m = this.model();
    return m.jumps.length > 0 || m.authCustom || !!(m.identitiesOnly || m.kerberos || m.proxy || m.proxyCommand || m.transport || m.passwordCommand || m.certificateCommand || m.preConnectCommand || m.credentialPlugin || m.forwardAgent || m.forwardX11 || m.multiplexer || m.autoReconnect || m.kubeContext || m.encoding || m.termType || m.logSession || m.recordSession || m.initialDirectory || m.environment || m.remoteCommand || m.macAddress || m.knockSequence || m.remoteDesktop);
  }

  protected addJump(): void {
    const first = this.jumpCandidates()[0];
    if (first) {
      this.model().jumps.push(first.id);
    }
  }

  /** Moves an authentication method up (-1) or down (+1). */
  protected moveAuth(i: number, by: -1 | 1): void {
    const list = this.model().authMethods;
    const j = i + by;
    if (j >= 0 && j < list.length) {
      [list[i], list[j]] = [list[j], list[i]];
    }
  }

  /** Applies a preset chosen from a select, then resets the select. */
  protected applyPreset(event: Event, field: 'proxyCommand' | 'passwordCommand' | 'certificateCommand' | 'preConnectCommand', presets: { id: string; command: string }[]): void {
    const select = event.target as HTMLSelectElement;
    const preset = presets.find((p) => p.id === select.value);
    if (preset) {
      this.model()[field] = preset.command;
    }
    select.value = '';
  }
}
