import { connectionModel, connectionSettings } from './connection-options';

describe('connection options', () => {
  it('round-trips settings', () => {
    const s = {
      jumpChain: ['a', 'b'],
      authMethods: ['password', 'publickey'],
      identitiesOnly: true,
      kerberos: true,
      kerberosRealm: 'CORP.EXAMPLE.COM',
      kerberosPrincipal: 'mario@CORP.EXAMPLE.COM',
      proxy: 'socks5://p:1080',
      proxyCommand: 'nc %h %p',
      transport: 'awsSsm',
      cloudTarget: 'i-0abc',
      cloudRegion: 'eu-south-1',
      cloudAccount: 'prod',
      cloudBastion: 'bastion',
      cloudGroup: 'rg',
      passwordCommand: 'pass show x',
      certificateCommand: 'sign',
      preConnectCommand: 'push-key',
      credentialPlugin: 'cfg-vault',
      forwardAgent: true,
      forwardX11: true,
      multiplexer: 'tmux',
      autoReconnect: false,
      kubeContext: 'prod-eu',
      kubeNamespace: 'web',
      encoding: 'ISO-8859-1',
      termType: 'vt100',
      logSession: true,
      logFormat: 'timestamped',
      recordSession: false,
      initialDirectory: '~/app',
      environment: ['A=1', 'B=two words'],
      remoteCommand: 'htop',
      macAddress: 'aa:bb:cc:dd:ee:ff',
      wakeBroadcast: '192.168.1.255',
      wakeOnConnect: true,
      knockSequence: '7000,8000/udp',
      remoteDesktop: 'vnc',
      remoteDesktopPort: 5901,
      remoteDesktopViaSsh: true,
    };
    expect(connectionSettings(connectionModel(s))).toEqual(s);
  });

  it('maps empty fields to inherit', () => {
    const m = connectionModel(undefined);
    expect(m.forwardAgent).toBe('');
    expect(connectionSettings(m)).toEqual({
      jumpChain: null,
      authMethods: null,
      identitiesOnly: null,
      kerberos: null,
      kerberosRealm: null,
      kerberosPrincipal: null,
      proxy: null,
      proxyCommand: null,
      transport: null,
      cloudTarget: null,
      cloudRegion: null,
      cloudAccount: null,
      cloudBastion: null,
      cloudGroup: null,
      passwordCommand: null,
      certificateCommand: null,
      preConnectCommand: null,
      credentialPlugin: null,
      forwardAgent: null,
      forwardX11: null,
      multiplexer: null,
      autoReconnect: null,
      kubeContext: null,
      kubeNamespace: null,
      encoding: null,
      termType: null,
      logSession: null,
      logFormat: null,
      recordSession: null,
      initialDirectory: null,
      environment: null,
      remoteCommand: null,
      macAddress: null,
      wakeBroadcast: null,
      wakeOnConnect: null,
      knockSequence: null,
      remoteDesktop: null,
      remoteDesktopPort: null,
      remoteDesktopViaSsh: null,
    });
  });

  it('keeps the unchecked methods after the chosen ones', () => {
    const m = connectionModel({ authMethods: ['password'] });
    expect(m.authCustom).toBe(true);
    expect(m.authMethods).toEqual([
      { id: 'password', on: true },
      { id: 'publickey', on: false },
      { id: 'keyboard-interactive', on: false },
    ]);
    m.authMethods[1].on = true;
    expect(connectionSettings(m).authMethods).toEqual(['password', 'publickey']);
    m.authCustom = false;
    expect(connectionSettings(m).authMethods).toBeNull();
  });

  it('keeps an explicit "no" for agent forwarding', () => {
    const m = connectionModel({ forwardAgent: false });
    expect(m.forwardAgent).toBe('off');
    expect(connectionSettings(m).forwardAgent).toBe(false);
  });
});

it('keeps an explicit "no multiplexer" distinct from inherit', () => {
  const m = connectionModel({ multiplexer: '' });
  expect(m.multiplexer).toBe('none');
  expect(connectionSettings(m).multiplexer).toBe('');
});
