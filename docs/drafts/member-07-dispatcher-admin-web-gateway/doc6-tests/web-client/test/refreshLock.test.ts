import { RefreshLock } from '../src/refreshLock';

describe('RefreshLock (UT-FE-05..06, race condition)', () => {
  it('5 parallel callers cause exactly 1 refresh call', async () => {
    let calls = 0;
    let release!: (v: string) => void;
    const lock = new RefreshLock(() => { calls++; return new Promise<string>((r) => (release = r)); });
    const results = Promise.all([lock.run(), lock.run(), lock.run(), lock.run(), lock.run()]);
    release('new-token');
    expect(await results).toEqual(Array(5).fill('new-token'));
    expect(calls).toBe(1);
  });
  it('a later 401 after the first refresh finished starts a new refresh', async () => {
    let calls = 0;
    const lock = new RefreshLock(async () => `t${++calls}`);
    expect(await lock.run()).toBe('t1');
    expect(await lock.run()).toBe('t2');
  });
  it('a failed refresh does not block the next attempt', async () => {
    let calls = 0;
    const lock = new RefreshLock(async () => { calls++; if (calls === 1) throw new Error('down'); return 'ok'; });
    await expect(lock.run()).rejects.toThrow('down');
    expect(await lock.run()).toBe('ok');
  });
});
