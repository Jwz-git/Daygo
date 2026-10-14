const clamp = (value, min = 0, max = 1) => Math.min(max, Math.max(min, value));
const lerp = (a, b, mix) => a + (b - a) * mix;
const smooth = (a, b, value) => { const mix = clamp((value - a) / (b - a)); return mix * mix * (3 - 2 * mix); };

export function skyBlend(minute, palette) {
  let index = 0;
  while (index < palette.length - 2 && minute >= palette[index + 1][0]) index++;
  return { index, mix: clamp((minute - palette[index][0]) / (palette[index + 1][0] - palette[index][0])) };
}

// Keep two rasterized gradients, regardless of the number of palette stops.
// Scroll only changes leaf transforms/opacity, never inherited root variables.
export function createSkyPainter({ layers, sun, glow, disc, haze, moon, stars }, palette) {
  let lastIndex = -1;
  const values = new WeakMap();
  const write = (node, property, value) => {
    const text = String(value);
    let previous = values.get(node);
    if (!previous) { previous = new Map(); values.set(node, previous); }
    // CSSOM normalizes units (0 -> 0px); compare our inputs instead of serialized styles.
    if (previous.get(property) === text) return;
    node.style[property] = text;
    previous.set(property, text);
  };
  const gradient = stop => `linear-gradient(180deg, ${stop[1]} 0%, ${stop[2]} 58%, ${stop[3]} 100%)`;
  return minute => {
    const { index, mix } = skyBlend(minute, palette);
    if (index !== lastIndex) {
      write(layers[0], 'backgroundImage', gradient(palette[index]));
      write(layers[1], 'backgroundImage', gradient(palette[index + 1]));
      lastIndex = index;
    }
    write(layers[1], 'opacity', mix);
    const f = (minute - 360) / (1150 - 360);
    const alt = Math.sin(Math.PI * clamp(f, -0.08, 1.08));
    const x = lerp(26, 82, clamp(f));
    write(sun, 'transform', `translate3d(${x}vw, ${96 - alt * 74}vh, 0)`);
    write(glow, 'opacity', 1 - smooth(1140, 1200, minute));
    write(disc, 'opacity', 1 - smooth(1130, 1175, minute));
    write(haze, 'transform', `translate3d(calc(${x}vw - 50%), 0, 0)`);
    write(haze, 'opacity', (0.25 + 0.6 * (1 - clamp(alt * 1.6))) * (1 - smooth(1170, 1260, minute)));
    write(stars, 'opacity', smooth(1120, 1240, minute));
    write(moon, 'opacity', smooth(1190, 1290, minute));
    write(moon, 'transform', `translate3d(0, ${lerp(34, 14, smooth(1190, 1440, minute))}vh, 0)`);
  };
}
