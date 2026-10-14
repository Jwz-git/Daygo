import test from 'node:test';
import assert from 'node:assert/strict';
import { createFrameLoop, createPausableTimers, createScrollActivity } from '../src/motion.js';

function frames() {
  const pending = new Map();
  let id = 0;
  return {
    pending,
    requestFrame(callback) { pending.set(++id, callback); return id; },
    cancelFrame(id) { pending.delete(id); },
    flush(time = 16) { const callbacks = [...pending.values()]; pending.clear(); callbacks.forEach(callback => callback(time)); },
  };
}

test('coalesces invalidations and sleeps when work has settled', () => {
  const env = frames();
  let calls = 0;
  const loop = createFrameLoop(() => { calls++; return calls < 3; }, env);
  loop.request(); loop.request(); loop.request();
  assert.equal(env.pending.size, 1);
  env.flush(); env.flush(); env.flush(); env.flush();
  assert.equal(calls, 3);
  assert.equal(env.pending.size, 0);
  loop.request(); env.flush();
  assert.equal(calls, 4);
});

test('suspends hidden frames, resumes once, and cannot restart after destruction', () => {
  const env = frames();
  let calls = 0;
  const loop = createFrameLoop(() => { calls++; return true; }, env);
  loop.request(); loop.setEnabled(false); loop.request(); env.flush();
  assert.equal(calls, 0);
  loop.setEnabled(true); env.flush();
  assert.equal(calls, 1);
  loop.destroy(); loop.request(); loop.setEnabled(true); env.flush();
  assert.equal(calls, 1);
  assert.equal(env.pending.size, 0);
});

test('rebuilding owners leaves only the current frame loop', () => {
  const env = frames();
  let calls = 0;
  let loop;
  for (let i = 0; i < 10; i++) {
    loop?.destroy();
    loop = createFrameLoop(() => { calls++; return true; }, env);
    loop.request();
  }
  env.flush();
  assert.equal(calls, 1);
  assert.equal(env.pending.size, 1);
  loop.destroy();
});

test('limits rendering frequency and paints the latest state instead of dropping it', () => {
  const env = frames();
  let state = 0;
  const painted = [];
  const loop = createFrameLoop(() => { painted.push(state); return false; }, { ...env, minInterval: 1000 / 30 });
  loop.request(); env.flush(0);
  state = 1; loop.request(); env.flush(8);
  state = 2; loop.request(); env.flush(16);
  assert.deepEqual(painted, [0]);
  env.flush(34);
  assert.deepEqual(painted, [0, 2]);
  assert.equal(env.pending.size, 0);
});

test('a continuous animation respects the frame budget and resumes immediately after hiding', () => {
  const env = frames();
  let calls = 0;
  const loop = createFrameLoop(() => { calls++; return true; }, { ...env, minInterval: 1000 / 30 });
  loop.request(); env.flush(0); env.flush(8); env.flush(16); env.flush(34);
  assert.equal(calls, 2);
  loop.setEnabled(false); env.flush(40);
  loop.setEnabled(true); env.flush(41);
  assert.equal(calls, 3);
  loop.destroy();
  assert.equal(env.pending.size, 0);
});

function timers() {
  let time = 0, id = 0;
  const pending = new Map();
  return {
    pending, now: () => time,
    setTimer(callback, delay) { pending.set(++id, { callback, at: time + delay }); return id; },
    clearTimer(id) { pending.delete(id); },
    advance(ms) {
      time += ms;
      for (const [key, task] of [...pending]) if (task.at <= time) { pending.delete(key); task.callback(); }
    },
  };
}

test('offscreen timers preserve their remaining delay and resume without duplication', () => {
  const env = timers();
  const scope = createPausableTimers(env);
  let calls = 0;
  scope.schedule(() => calls++, 700);
  env.advance(200); scope.setEnabled(false); env.advance(10000);
  assert.equal(calls, 0);
  assert.equal(env.pending.size, 0);
  scope.setEnabled(true); scope.setEnabled(true); env.advance(499);
  assert.equal(calls, 0);
  env.advance(1);
  assert.equal(calls, 1);
  assert.equal(env.pending.size, 0);
});

test('timers created while paused wait, and destroyed owners never mutate again', () => {
  const env = timers();
  const scope = createPausableTimers(env);
  let calls = 0;
  scope.setEnabled(false); scope.schedule(() => calls++, 100);
  env.advance(1000);
  assert.equal(calls, 0);
  scope.setEnabled(true); env.advance(100);
  assert.equal(calls, 1);
  scope.schedule(() => calls++, 100);
  scope.destroy(); scope.setEnabled(true); scope.schedule(() => calls++, 0); env.advance(1000);
  assert.equal(calls, 1);
  assert.equal(env.pending.size, 0);
});

test('continuous scroll pauses decorations once and restores them only after input settles', () => {
  const env = timers();
  const states = [];
  const activity = createScrollActivity(value => states.push(value), { ...env, delay: 160 });
  activity.update(); env.advance(100); activity.update(); env.advance(159);
  assert.deepEqual(states, [true]);
  assert.equal(env.pending.size, 1);
  env.advance(1);
  assert.deepEqual(states, [true, false]);
  assert.equal(env.pending.size, 0);
});

test('hiding or destroying scroll activity cancels delayed restoration', () => {
  const env = timers();
  const states = [];
  const activity = createScrollActivity(value => states.push(value), env);
  activity.update(); activity.reset(); env.advance(1000);
  assert.deepEqual(states, [true, false]);
  activity.update(); activity.destroy(); activity.update(); env.advance(1000);
  assert.deepEqual(states, [true, false, true, false]);
  assert.equal(env.pending.size, 0);
});
