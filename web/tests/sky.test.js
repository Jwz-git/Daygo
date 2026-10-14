import test from 'node:test';
import assert from 'node:assert/strict';
import { createSkyPainter, skyBlend } from '../src/sky.js';

const palette = [
  [300, '#111111', '#222222', '#333333'],
  [600, '#444444', '#555555', '#666666'],
  [900, '#777777', '#888888', '#999999'],
];
function elements() {
  const writes = [];
  const node = name => ({ style: new Proxy({}, { set(style, property, value) { writes.push({ name, property, value }); style[property] = value; return true; } }) });
  return { writes, layers: [node('base'), node('overlay')], sun: node('sun'), glow: node('glow'), disc: node('disc'), haze: node('haze'), moon: node('moon'), stars: node('stars') };
}

test('palette blend clamps endpoints and remains continuous through forward and reverse boundaries', () => {
  assert.deepEqual(skyBlend(0, palette), { index: 0, mix: 0 });
  assert.deepEqual(skyBlend(450, palette), { index: 0, mix: 0.5 });
  assert.deepEqual(skyBlend(600, palette), { index: 1, mix: 0 });
  assert.deepEqual(skyBlend(750, palette), { index: 1, mix: 0.5 });
  assert.deepEqual(skyBlend(1200, palette), { index: 1, mix: 1 });
  assert.deepEqual(skyBlend(599.99, palette), { index: 0, mix: (599.99 - 300) / 300 });
});

test('scrolling within a palette interval only composites cached gradients', () => {
  const nodes = elements();
  const paint = createSkyPainter(nodes, palette);
  paint(400);
  assert.equal(nodes.writes.filter(write => write.property === 'backgroundImage').length, 2);
  nodes.writes.length = 0;
  for (let m = 401; m < 500; m++) paint(m);
  assert.equal(nodes.writes.filter(write => write.property === 'backgroundImage').length, 0);
  assert.ok(nodes.writes.every(write => write.property === 'opacity' || write.property === 'transform'));
  nodes.writes.length = 0;
  paint(499);
  assert.equal(nodes.writes.length, 0, 'painting the same minute must not rewrite styles');
});

test('palette changes repurpose exactly two layers in either scroll direction', () => {
  const nodes = elements();
  const paint = createSkyPainter(nodes, palette);
  paint(450);
  const firstBase = nodes.layers[0].style.backgroundImage;
  paint(750);
  assert.ok(nodes.layers[0].style.backgroundImage.includes('#444444'));
  assert.equal(nodes.layers[1].style.opacity, '0.5');
  paint(450);
  assert.equal(nodes.layers[0].style.backgroundImage, firstBase);
  assert.equal(nodes.layers[1].style.opacity, '0.5');
  assert.equal(nodes.layers.length, 2);
});

test('CSSOM unit normalization cannot make an unchanged transform write again', () => {
  const nodes = elements();
  let value = '', writes = 0;
  nodes.sun.style = {
    get transform() { return value.replace(', 0)', ', 0px)'); },
    set transform(next) { value = next; writes++; },
  };
  const paint = createSkyPainter(nodes, palette);
  paint(360);
  assert.equal(nodes.sun.style.transform, 'translate3d(26vw, 96vh, 0px)');
  paint(360);
  assert.equal(writes, 1);
});

test('sun and moon use transforms; dawn and midnight keep their visibility states', () => {
  const nodes = elements();
  const paint = createSkyPainter(nodes, palette);
  paint(360);
  assert.equal(nodes.sun.style.transform, 'translate3d(26vw, 96vh, 0)');
  assert.equal(nodes.glow.style.opacity, '1');
  assert.equal(nodes.disc.style.opacity, '1');
  assert.equal(nodes.moon.style.opacity, '0');
  assert.equal(nodes.stars.style.opacity, '0');
  paint(1440);
  assert.equal(nodes.glow.style.opacity, '0');
  assert.equal(nodes.disc.style.opacity, '0');
  assert.equal(nodes.moon.style.opacity, '1');
  assert.equal(nodes.stars.style.opacity, '1');
  assert.equal(nodes.moon.style.transform, 'translate3d(0, 14vh, 0)');
  assert.ok(nodes.writes.every(write => !write.property.startsWith('--')));
});
