import { CommandTracker, parseOsc777, parseOsc9 } from './osc';

describe('OSC notifications', () => {
  it('reads OSC 9 and 777, ignoring progress reports', () => {
    expect(parseOsc9('Build done')).toEqual({ title: '', body: 'Build done' });
    expect(parseOsc9('4;1;50')).toBeNull();
    expect(parseOsc777('notify;Deploy;finished; all green')).toEqual({ title: 'Deploy', body: 'finished; all green' });
    expect(parseOsc777('preexec')).toBeNull();
  });

  it('times commands marked by OSC 133', () => {
    const t = new CommandTracker();
    expect(t.feed('D;0', 1000)).toBeNull(); // no command started
    t.feed('C', 1000);
    expect(t.feed('D;2', 13_500)).toEqual({ ms: 12_500, exit: 2 });
    t.feed('C', 20_000);
    t.feed('A', 21_000);
    expect(t.feed('D', 22_000)).toBeNull();
    t.feed('C', 30_000);
    expect(t.feed('D', 31_000)).toEqual({ ms: 1000, exit: null });
  });
});
