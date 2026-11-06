import { Injectable } from '@angular/core';

import { KeyService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { AgentStatus, KeyUsage, RotationResult, TokenSupport } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Key as TokenKey, Token } from '../../../../bindings/github.com/csturiale/shellvane/core/pkcs11';
import type { Key } from '../../../../bindings/github.com/csturiale/shellvane/core/keys';

export type { AgentStatus, KeyUsage, RotationResult, Token, TokenKey, TokenSupport };

export type SshKey = Key;

@Injectable({ providedIn: 'root' })
export class KeyApi {
  async list(): Promise<Key[]> {
    return (await KeyService.List()) ?? [];
  }
  async tokenSupport(): Promise<TokenSupport> {
    const s = await KeyService.TokenSupport();
    return { ...s, modules: s.modules ?? [] };
  }
  async tokens(module: string): Promise<Token[]> {
    return (await KeyService.Tokens(module)) ?? [];
  }
  async tokenKeys(module: string, serial: string): Promise<TokenKey[]> {
    return (await KeyService.TokenKeys(module, serial)) ?? [];
  }
  addTokenKey(name: string, module: string, token: Token, key: TokenKey): Promise<Key> {
    return KeyService.AddTokenKey(name, module, token, key);
  }
  generate(name: string, kind: string, bits: number, comment: string): Promise<Key> {
    return KeyService.Generate(name, kind, bits, comment);
  }
  import(name: string, data: string, passphrase: string): Promise<Key> {
    return KeyService.Import(name, data, passphrase);
  }
  rename(id: string, name: string): Promise<void> {
    return KeyService.Rename(id, name);
  }
  delete(id: string): Promise<void> {
    return KeyService.Delete(id);
  }
  setCertificate(id: string, certLine: string): Promise<void> {
    return KeyService.SetCertificate(id, certLine);
  }
  deploy(keyId: string, hostId: string): Promise<void> {
    return KeyService.Deploy(keyId, hostId);
  }
  startAgent(): Promise<AgentStatus> {
    return KeyService.StartAgent();
  }
  stopAgent(): Promise<void> {
    return KeyService.StopAgent();
  }
  agentStatus(): Promise<AgentStatus> {
    return KeyService.AgentStatus();
  }
  async usage(keyId: string): Promise<KeyUsage[]> {
    return (await KeyService.Usage(keyId)) ?? [];
  }
  export(keyId: string, format: 'openssh' | 'ppk', passphrase: string): Promise<string> {
    return KeyService.Export(keyId, format, passphrase);
  }
  async rotate(keyId: string, removeOld: boolean): Promise<RotationResult> {
    const r = await KeyService.Rotate(keyId, removeOld);
    return { ...r, hosts: r.hosts ?? [] };
  }
}
