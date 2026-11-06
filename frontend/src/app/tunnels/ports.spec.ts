import { detectPorts } from './ports';

describe('detectPorts', () => {
  it('finds local URLs and addresses', () => {
    expect(detectPorts('  ➜  Local:   http://localhost:5173/\n  Listening on 0.0.0.0:8080, [::1]:9000')).toEqual([
      { port: 5173, addr: '127.0.0.1' },
      { port: 8080, addr: '0.0.0.0' },
      { port: 9000, addr: '::1' },
    ]);
  });

  it('ignores other hosts, versions and impossible ports', () => {
    expect(detectPorts('http://example.com:8080 v127.0.0.1:1.2 localhost:99999')).toEqual([]);
  });
});
