import { TransferHost, ZmodemTransfer } from './zmodem-transfer';

function host(written: Uint8Array[]): TransferHost {
  return {
    toTerminal: (d) => written.push(d),
    send: () => undefined,
    begin: async () => 'id',
    append: async () => undefined,
    finish: async () => '/tmp/x',
    cancelFile: async () => undefined,
    pick: async () => [],
    state: () => undefined,
    result: () => undefined,
  };
}

describe('ZmodemTransfer', () => {
  it('passes ordinary output through untouched', () => {
    const written: Uint8Array[] = [];
    const z = new ZmodemTransfer(host(written));
    const data = new TextEncoder().encode('ls -l\r\n\x1b[1mtotal 0\x1b[0m\r\n');
    z.consume(data);
    expect(written).toEqual([data]);
    expect(written[0]).toBe(data); // not copied: no work for plain output
    expect(z.active).toBe(false);
  });

  it('keeps output with a control byte that is not ZMODEM', () => {
    const written: Uint8Array[] = [];
    const z = new ZmodemTransfer(host(written));
    z.consume(new Uint8Array([0x61, 0x18, 0x62, 0x0d, 0x0a]));
    const all = written.flatMap((w) => Array.from(w));
    expect(all).toEqual([0x61, 0x18, 0x62, 0x0d, 0x0a]);
    expect(z.active).toBe(false);
  });

  it('starts a receive session when sz announces itself', () => {
    const written: Uint8Array[] = [];
    const z = new ZmodemTransfer(host(written));
    // ZRQINIT as sent by lrzsz's sz.
    const header = [...new TextEncoder().encode('rz\r**\x18B00000000000000\r'), 0x8a, 0x11];
    z.consume(new Uint8Array(header));
    expect(z.active).toBe(true);
    z.cancel();
  });
});
