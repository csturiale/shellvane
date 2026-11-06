import { isDangerous, render, systemValues, toTerminalInput, userVariables } from './variables';

describe('snippet variables', () => {
  it('finds user variables once, with choices, skipping system ones', () => {
    expect(userVariables('docker logs -f {{container}} --tail {{lines:100|1000}} on {{host}} {{container}}')).toEqual([
      { name: 'container', choices: [] },
      { name: 'lines', choices: ['100', '1000'] },
    ]);
  });

  it('renders values and leaves unknown variables alone', () => {
    expect(render('ssh {{user}}@{{host}} {{missing}}', { user: 'me', host: 'h' })).toBe('ssh me@h {{missing}}');
  });

  it('provides system values from the target', () => {
    const v = systemValues({ host: 'web', user: 'deploy', port: 22 }, new Date(2026, 9, 30, 9, 5, 7));
    expect(v).toEqual({ host: 'web', user: 'deploy', port: '22', date: '2026-10-30', time: '09:05:07' });
  });

  it('converts newlines to Enter and presses Enter when running', () => {
    expect(toTerminalInput('cd /srv\nls\n', true)).toBe('cd /srv\rls\r');
    expect(toTerminalInput('git commit -m "', false)).toBe('git commit -m "');
  });

  it('flags dangerous commands', () => {
    expect(isDangerous('sudo rm -rf /var/www')).toBe(true);
    expect(isDangerous('rm -fr build')).toBe(true);
    expect(isDangerous('sudo reboot')).toBe(true);
    expect(isDangerous('dd if=/dev/zero of=/dev/sda')).toBe(true);
    expect(isDangerous('ls -la; df -h')).toBe(false);
    expect(isDangerous('rm file.txt')).toBe(false);
  });
});
