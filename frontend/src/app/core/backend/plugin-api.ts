import { SITE_URL } from '../site';
import { Injectable } from '@angular/core';

import { PluginService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { PluginConfig, PluginInfo, StoreItem } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Action, Config, Panel, Result, SyncResult, Table } from '../../../../bindings/github.com/csturiale/shellvane/core/plugins';

export type { Action, Config, Panel, PluginConfig, PluginInfo, Result, StoreItem, SyncResult, Table };

/** The store index used unless the user sets another one. */
export const DEFAULT_STORE_URL = `${SITE_URL}/plugins/index.json`;

/** Plugins and their configurations. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class PluginApi {
  async plugins(): Promise<PluginInfo[]> {
    return (await PluginService.Plugins()) ?? [];
  }
  async configs(): Promise<PluginConfig[]> {
    return (await PluginService.Configs()) ?? [];
  }
  save(config: Config, secrets: Record<string, string>): Promise<PluginConfig> {
    return PluginService.SaveConfig(config, secrets);
  }
  delete(id: string): Promise<void> {
    return PluginService.DeleteConfig(id);
  }
  sync(id: string): Promise<SyncResult> {
    return PluginService.Sync(id);
  }
  async panelPlugins(): Promise<PluginInfo[]> {
    return (await PluginService.PanelPlugins()) ?? [];
  }
  panel(pluginId: string, hostId: string): Promise<Panel> {
    return PluginService.Panel(pluginId, hostId);
  }
  async hostActions(pluginId: string, hostId: string): Promise<Action[]> {
    return (await PluginService.HostActions(pluginId, hostId)) ?? [];
  }
  run(pluginId: string, hostId: string, action: string, item: string): Promise<Result> {
    return PluginService.RunAction(pluginId, hostId, action, item);
  }
  install(id: string): Promise<void> {
    return PluginService.Install(id);
  }
  uninstall(id: string): Promise<void> {
    return PluginService.Uninstall(id);
  }
  /** The system's file chooser for a plugin's .zip; "" when cancelled. */
  chooseFile(title: string): Promise<string> {
    return PluginService.ChooseFile(title);
  }
  installFile(path: string): Promise<PluginInfo> {
    return PluginService.InstallFile(path);
  }
  async store(url: string): Promise<StoreItem[]> {
    return (await PluginService.Store(url)) ?? [];
  }
  installFromStore(url: string, id: string): Promise<PluginInfo> {
    return PluginService.InstallFromStore(url, id);
  }
}
