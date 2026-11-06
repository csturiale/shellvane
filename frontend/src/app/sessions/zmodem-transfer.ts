import Zmodem from 'zmodem.js/src/zmodem_browser';
import type { Offer, Session } from 'zmodem.js/src/zmodem_browser';

/** What the terminal shows while files move. */
export interface TransferState {
  direction: 'download' | 'upload';
  name: string;
  done: number;
  total: number;
  files: number;
}

/** The outcome, shown once the session ends. */
export interface TransferResult {
  direction: 'download' | 'upload';
  saved: string[];
  sent: number;
  cancelled: boolean;
  error: string | null;
}

export interface TransferHost {
  /** Terminal output that is not part of a transfer. */
  toTerminal(data: Uint8Array): void;
  /** Bytes for the server. */
  send(data: Uint8Array): void;
  /** Saving a received file: begin, append chunks, finish (returns the path). */
  begin(name: string): Promise<string>;
  append(id: string, data: Uint8Array): Promise<void>;
  finish(id: string): Promise<string>;
  cancelFile(id: string): Promise<void>;
  /** Asks for the files to upload; empty when the user cancels. */
  pick(): Promise<File[]>;
  state(s: TransferState | null): void;
  result(r: TransferResult): void;
}

/** Bytes buffered before a chunk goes to the backend. */
const CHUNK = 1 << 20;
/** ZDLE / CAN: every ZMODEM header contains it. */
const ZDLE = 0x18;

/**
 * ZMODEM in a terminal (rz and sz on the server). Output only goes through
 * zmodem.js when it may hold a ZMODEM header or a transfer is running, so
 * ordinary output costs nothing.
 */
export class ZmodemTransfer {
  private readonly sentry: InstanceType<typeof Zmodem.Sentry>;
  private session: Session | null = null;
  private cancelled = false;

  constructor(private readonly host: TransferHost) {
    this.sentry = new Zmodem.Sentry({
      to_terminal: (octets) => host.toTerminal(new Uint8Array(octets)),
      sender: (octets) => host.send(new Uint8Array(octets)),
      on_retract: () => undefined,
      on_detect: (detection) => {
        const session = detection.confirm();
        this.session = session;
        this.cancelled = false;
        if (session.type === 'receive') {
          this.receive(session);
        } else {
          void this.upload(session);
        }
      },
    });
  }

  /** Feeds server output: a transfer takes its part, the rest goes to the terminal. */
  consume(data: Uint8Array): void {
    if (!this.session && !data.includes(ZDLE)) {
      this.host.toTerminal(data);
      return;
    }
    try {
      this.sentry.consume(data);
    } catch (err) {
      // A broken transfer must not swallow the terminal.
      this.end({ direction: 'download', saved: [], sent: 0, cancelled: false, error: String(err) });
      this.host.toTerminal(data);
    }
  }

  get active(): boolean {
    return this.session !== null;
  }

  /** Stops the transfer; the server's rz/sz gets the cancel sequence. */
  cancel(): void {
    this.cancelled = true;
    try {
      this.session?.abort();
    } catch {
      // Already over.
    }
  }

  /** sz on the server: files come to this computer. */
  private receive(session: Session): void {
    const saved: string[] = [];
    let files = 0;
    let failure: string | null = null;
    let queue: Promise<unknown> = Promise.resolve();
    session.on('offer', (offer: Offer) => {
      const { name, size } = offer.get_details();
      files++;
      let done = 0;
      let buffer: Uint8Array[] = [];
      let buffered = 0;
      let id: Promise<string> | null = this.host.begin(name);
      const flush = () => {
        if (!buffered || !id) {
          return;
        }
        const data = join(buffer, buffered);
        buffer = [];
        buffered = 0;
        const target = id;
        queue = queue.then(async () => this.host.append(await target, data));
      };
      this.host.state({ direction: 'download', name, done, total: size ?? 0, files });
      offer.on('input', (payload) => {
        buffer.push(new Uint8Array(payload));
        buffered += payload.length;
        done += payload.length;
        if (buffered >= CHUNK) {
          flush();
        }
        this.host.state({ direction: 'download', name, done, total: size ?? 0, files });
      });
      offer.accept().then(
        () => {
          flush();
          const target = id;
          id = null;
          queue = queue.then(async () => saved.push(await this.host.finish(await target!)));
        },
        () => {
          const target = id;
          id = null;
          queue = queue.then(async () => this.host.cancelFile(await target!));
        },
      );
      queue = queue.catch((err) => (failure = String(err)));
    });
    session.on('session_end', () => {
      void queue.finally(() =>
        this.end({ direction: 'download', saved, sent: 0, cancelled: this.cancelled, error: failure }),
      );
    });
    session.start();
  }

  /** rz on the server: files go from this computer. */
  private async upload(session: Session): Promise<void> {
    const files = await this.host.pick();
    if (files.length === 0) {
      this.cancel();
      this.end({ direction: 'upload', saved: [], sent: 0, cancelled: true, error: null });
      return;
    }
    let sent = 0;
    let index = 0;
    try {
      await Zmodem.Browser.send_files(session, files, {
        on_progress: (file, _xfer, piece) => {
          this.progress(file, piece.length, files.length, index);
        },
        on_file_complete: () => {
          sent++;
          index++;
        },
      });
      await session.close();
      this.end({ direction: 'upload', saved: [], sent, cancelled: false, error: null });
    } catch (err) {
      this.end({ direction: 'upload', saved: [], sent, cancelled: this.cancelled, error: this.cancelled ? null : String(err) });
    }
  }

  private uploaded = new Map<File, number>();

  private progress(file: File, bytes: number, count: number, index: number): void {
    const done = (this.uploaded.get(file) ?? 0) + bytes;
    this.uploaded.set(file, done);
    this.host.state({ direction: 'upload', name: file.name, done, total: file.size, files: Math.min(index + 1, count) });
  }

  private end(result: TransferResult): void {
    this.session = null;
    this.uploaded.clear();
    this.host.state(null);
    this.host.result(result);
  }
}

function join(parts: Uint8Array[], length: number): Uint8Array {
  const out = new Uint8Array(length);
  let at = 0;
  for (const p of parts) {
    out.set(p, at);
    at += p.length;
  }
  return out;
}
