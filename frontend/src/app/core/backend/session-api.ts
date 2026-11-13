import { Injectable } from '@angular/core';
import { Events, Stream } from '@wailsio/runtime';

import { SessionService } from '../../../../bindings/github.com/csturiale/shellvane/app';
import type {
  BrowserChoice,
  Highlight,
  KubeInfo,
  LocalOptions,
  OpenOptions,
  Prompt,
  PromptAnswer,
  SessionInfo,
  SessionState,
} from '../../../../bindings/github.com/csturiale/shellvane/app';
import type { Selection as KubeSelection } from '../../../../bindings/github.com/csturiale/shellvane/core/kube';
import type { Program } from '../../../../bindings/github.com/csturiale/shellvane/core/localshell';
import type { File as SessionFile } from '../../../../bindings/github.com/csturiale/shellvane/core/sessionlog';
import { SessionPrompt, SessionReturn, SessionState as SessionStateEvent } from '../../../../bindings/events';

export type { BrowserChoice, KubeSelection, Program, SessionFile };
export type { Highlight, KubeInfo, LocalOptions, OpenOptions, Prompt, PromptAnswer, SessionInfo, SessionState };

/** The subset of WebSocket a terminal needs; Wails streams implement it. */
export interface TerminalSocket {
  binaryType: string;
  readyState: number;
  onopen: ((ev: Event) => void) | null;
  onmessage: ((ev: MessageEvent) => void) | null;
  onclose: ((ev: Event) => void) | null;
  send(data: ArrayBufferLike | ArrayBufferView | string): void;
  close(): void;
}

/** SSH sessions. Tests replace this class with a fake. */
@Injectable({ providedIn: 'root' })
export class SessionApi {
  openHost(hostId: string, cols: number, rows: number, opts: OpenOptions): Promise<SessionInfo> {
    return SessionService.OpenHost(hostId, cols, rows, opts);
  }
  openLocal(cols: number, rows: number, opts: LocalOptions): Promise<SessionInfo> {
    return SessionService.OpenLocal(cols, rows, opts);
  }
  async localShells(): Promise<Program[]> {
    return (await SessionService.LocalShells()) ?? [];
  }
  async kubeContexts(id: string): Promise<string[]> {
    return (await SessionService.KubeContexts(id)) ?? [];
  }
  /** Asks the cluster of a local pane for its version, through the pane's kubeconfig. */
  checkKube(id: string): Promise<void> {
    return SessionService.CheckKube(id);
  }
  setKube(id: string, sel: KubeSelection): Promise<void> {
    return SessionService.SetKube(id, sel);
  }
  async serialPorts(): Promise<string[]> {
    return (await SessionService.SerialPorts()) ?? [];
  }
  openBenchmark(cols: number, rows: number, megabytes: number): Promise<SessionInfo> {
    return SessionService.OpenBenchmark(cols, rows, megabytes);
  }
  openTarget(spec: string, cols: number, rows: number): Promise<SessionInfo> {
    return SessionService.OpenTarget(spec, cols, rows);
  }
  answer(promptId: string, answer: PromptAnswer): Promise<void> {
    return SessionService.Answer(promptId, answer);
  }
  pause(id: string): Promise<void> {
    return SessionService.Pause(id);
  }
  resume(id: string): Promise<void> {
    return SessionService.Resume(id);
  }
  resize(id: string, cols: number, rows: number): Promise<void> {
    return SessionService.Resize(id, cols, rows);
  }
  startLogging(id: string, format: string): Promise<string> {
    return SessionService.StartLogging(id, format);
  }
  stopLogging(id: string): Promise<void> {
    return SessionService.StopLogging(id);
  }
  startRecording(id: string): Promise<string> {
    return SessionService.StartRecording(id);
  }
  stopRecording(id: string): Promise<void> {
    return SessionService.StopRecording(id);
  }
  async sessionFiles(): Promise<SessionFile[]> {
    return (await SessionService.SessionFiles()) ?? [];
  }
  readSessionFile(path: string): Promise<string> {
    return SessionService.ReadSessionFile(path);
  }
  deleteSessionFile(path: string): Promise<void> {
    return SessionService.DeleteSessionFile(path);
  }
  sessionFilesDir(): Promise<string> {
    return SessionService.SessionFilesDir();
  }
  setBroadcast(ids: string[], on: boolean): Promise<void> {
    return SessionService.SetBroadcast(ids, on);
  }
  /** Moves sessions to a new window; payload comes back from takeDetached there. */
  detach(payload: string, sessionIds: string[]): Promise<void> {
    return SessionService.Detach(payload, sessionIds);
  }
  /** Gives a detached window's sessions back to the main window. */
  reattach(payload: string, sessionIds: string[]): Promise<void> {
    return SessionService.Reattach(payload, sessionIds);
  }
  /** Tabs given back by detached windows (their takeDetached token). */
  onReturn(handler: (token: string) => void): () => void {
    return Events.On(SessionReturn, (ev) => handler(ev.data));
  }
  takeDetached(token: string): Promise<string> {
    return SessionService.TakeDetached(token);
  }
  close(id: string): Promise<void> {
    return SessionService.Close(id);
  }
  /** Subscribes to state changes; returns the unsubscribe function. */
  onState(handler: (state: SessionState) => void): () => void {
    return Events.On(SessionStateEvent, (ev) => handler(ev.data));
  }
  onPrompt(handler: (prompt: Prompt) => void): () => void {
    return Events.On(SessionPrompt, (ev) => handler(ev.data));
  }
  /** Opens the byte stream of one terminal. */
  connect(): TerminalSocket {
    return Stream('terminal') as unknown as TerminalSocket;
  }
}
