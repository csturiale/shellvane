/** The parts of zmodem.js (CommonJS, untyped) that Shellvane uses. */
declare module 'zmodem.js/src/zmodem_browser' {
  export type Octets = number[];

  export interface Details {
    name: string;
    size?: number;
  }

  export interface Offer {
    get_details(): Details;
    accept(): Promise<unknown>;
    skip(): void;
    on(event: 'input', handler: (payload: Octets) => void): void;
  }

  export interface Session {
    type: 'send' | 'receive';
    start(): void;
    close(): Promise<void>;
    abort(): void;
    has_ended(): boolean;
    on(event: 'offer', handler: (offer: Offer) => void): void;
    on(event: 'session_end', handler: () => void): void;
  }

  export interface Detection {
    confirm(): Session;
    deny(): void;
  }

  export interface SentryOptions {
    to_terminal(octets: Octets): void;
    sender(octets: Octets): void;
    on_detect(detection: Detection): void;
    on_retract(): void;
  }

  class Sentry {
    constructor(options: SentryOptions);
    consume(input: Octets | ArrayBuffer | Uint8Array): void;
    get_confirmed_session(): Session | null;
  }

  export interface SendOptions {
    on_progress?: (file: File, xfer: unknown, piece: Uint8Array) => void;
    on_file_complete?: (file: File) => void;
  }

  const Zmodem: {
    Sentry: typeof Sentry;
    Browser: { send_files(session: Session, files: FileList | File[], options?: SendOptions): Promise<void> };
  };
  export default Zmodem;
}
