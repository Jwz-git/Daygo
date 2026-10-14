import test from 'node:test';
import assert from 'node:assert/strict';
import { createChartTooltip } from '../src/interaction.js';

function fixture() {
  const frames = new Map();
  let id = 0, html = '', textWrites = 0;
  const tip = { hidden: true, style: {}, get innerHTML() { return html; }, set innerHTML(value) { html = value; textWrites++; } };
  const host = { getBoundingClientRect: () => ({ left: 100, top: 50 }) };
  const options = { requestFrame(callback) { frames.set(++id, callback); return id; }, cancelFrame(id) { frames.delete(id); } };
  return { tip, host, options, frames, writes: () => textWrites, flush(time) { const pending = [...frames.values()]; frames.clear(); pending.forEach(callback => callback(time)); } };
}

test('pointer bursts coalesce to the latest tooltip position and content', () => {
  const f = fixture();
  const tooltip = createChartTooltip(f.tip, f.host, f.options);
  tooltip.show({ clientX: 110, clientY: 60 }, 'first');
  tooltip.show({ clientX: 140, clientY: 90 }, 'latest');
  assert.equal(f.frames.size, 1);
  f.flush(0);
  assert.equal(f.tip.innerHTML, 'latest');
  assert.equal(f.writes(), 1);
  assert.equal(f.tip.style.transform, 'translate3d(calc(40px - 50%), calc(40px - 100% - 14px), 0)');
  assert.equal(f.tip.style.left, undefined);
  assert.equal(f.tip.style.top, undefined);
});

test('moving over the same tile does not recreate tooltip content', () => {
  const f = fixture();
  const tooltip = createChartTooltip(f.tip, f.host, f.options);
  tooltip.show({ clientX: 110, clientY: 60 }, 'same'); f.flush(0);
  tooltip.show({ clientX: 120, clientY: 70 }, 'same'); f.flush(34);
  assert.equal(f.writes(), 1);
  assert.equal(f.tip.hidden, false);
});

test('leaving or destroying a tooltip cancels queued pointer work', () => {
  const f = fixture();
  const tooltip = createChartTooltip(f.tip, f.host, f.options);
  tooltip.show({ clientX: 110, clientY: 60 }, 'old'); tooltip.hide(); f.flush(0);
  assert.equal(f.writes(), 0);
  assert.equal(f.tip.hidden, true);
  tooltip.show({ clientX: 120, clientY: 70 }, 'new'); f.flush(1);
  assert.equal(f.tip.innerHTML, 'new');
  tooltip.destroy(); tooltip.show({ clientX: 130, clientY: 80 }, 'after destroy'); f.flush(50);
  assert.equal(f.tip.innerHTML, 'new');
  assert.equal(f.tip.hidden, true);
});
