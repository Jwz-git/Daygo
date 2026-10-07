// Chart geometry ported from Daygo/frontend/src/lib/chartLayout.ts and
// lib/sankeyLayout.ts (themselves after Dayflow, MIT), plus the app colour rule
// from stores/weeklyCharts.ts.

function worstRatio(row, side) {
  const sum = row.reduce((a, b) => a + b, 0);
  if (sum <= 0 || side <= 0) return Infinity;
  const max = Math.max(...row), min = Math.min(...row);
  return Math.max((side * side * max) / (sum * sum), (sum * sum) / (side * side * min));
}

/** Squarified treemap (Bruls, Huizing, van Wijk), biggest first. */
export function squarify(items, value, bounds, gap = 0) {
  const entries = items.map((item) => ({ item, value: Math.max(0, value(item)) })).filter((e) => e.value > 0);
  const total = entries.reduce((s, e) => s + e.value, 0);
  if (total <= 0 || bounds.width <= 0 || bounds.height <= 0) return [];
  const scale = (bounds.width * bounds.height) / total;
  const areas = entries.map((e) => e.value * scale);
  const out = [];
  let rem = { ...bounds }, i = 0;
  while (i < entries.length) {
    const side = Math.min(rem.width, rem.height);
    const row = [areas[i]];
    let next = i + 1;
    while (next < entries.length && worstRatio([...row, areas[next]], side) <= worstRatio(row, side)) { row.push(areas[next]); next++; }
    const rowArea = row.reduce((s, a) => s + a, 0);
    const horizontal = rem.width >= rem.height;
    const thick = horizontal ? rowArea / rem.height : rowArea / rem.width;
    let cursor = horizontal ? rem.y : rem.x;
    row.forEach((area, k) => {
      const len = area / thick;
      const r = horizontal ? { x: rem.x, y: cursor, width: thick, height: len } : { x: cursor, y: rem.y, width: len, height: thick };
      out.push({ item: entries[i + k].item, rect: { x: r.x, y: r.y, width: Math.max(0, r.width - gap), height: Math.max(0, r.height - gap) } });
      cursor += len;
    });
    rem = horizontal ? { x: rem.x + thick, y: rem.y, width: rem.width - thick, height: rem.height } : { x: rem.x, y: rem.y + thick, width: rem.width, height: rem.height - thick };
    i = next;
  }
  const right = bounds.x + bounds.width, bottom = bounds.y + bounds.height;
  for (const p of out) {
    if (Math.abs(p.rect.x + p.rect.width + gap - right) < 0.5) p.rect.width += gap;
    if (Math.abs(p.rect.y + p.rect.height + gap - bottom) < 0.5) p.rect.height += gap;
  }
  return out;
}

/* ───────── Sankey ───────── */
export const SANKEY_WIDTH = 1748, SANKEY_HEIGHT = 933, SANKEY_BAR = 12;
const SOURCE_COLOR = '#D9CBC0';
export const SANKEY_COLUMNS = {
  source: { x: 72, top: 273, bottom: 706, gap: 0, minHeight: 0, labelX: 105, labelHeight: 52 },
  categories: { x: 760, top: 126, bottom: 828, gap: 20, minHeight: 40, labelX: 802, labelTop: 64, labelBottom: 874, labelHeight: 54, labelSpacing: 12 },
  apps: { x: 1334, top: 54, bottom: 928, gap: 20, minHeight: 28, labelX: 1372, labelTop: 38, labelBottom: 923, labelHeight: 56, labelSpacing: 10 },
};

function allocate(items, spec) {
  if (!items.length) return [];
  const available = Math.max(items.length * spec.minHeight, spec.bottom - spec.top - spec.gap * (items.length - 1));
  const total = items.reduce((s, i) => s + i.minutes, 0);
  const flexible = Math.max(0, available - spec.minHeight * items.length);
  let cursor = spec.top;
  return items.map((item) => {
    const height = spec.minHeight + (total > 0 ? (flexible * item.minutes) / total : flexible / items.length);
    const band = { ...item, x: spec.x, y: cursor, height, labelY: 0 };
    cursor += height + spec.gap;
    return band;
  });
}
function stack(items, top, height) {
  const total = items.reduce((s, i) => s + i.minutes, 0);
  let cursor = top;
  return items.map((item) => {
    const size = total > 0 ? (item.minutes / total) * height : height / items.length;
    const seg = { ...item, top: cursor, bottom: cursor + size };
    cursor += size;
    return seg;
  });
}
function placeLabels(bands, spec) {
  const sorted = [...bands].sort((a, b) => a.y + a.height / 2 - (b.y + b.height / 2));
  let cursor = spec.labelTop;
  for (const b of sorted) { b.labelY = Math.max(b.y + b.height / 2 - spec.labelHeight / 2, cursor); cursor = b.labelY + spec.labelHeight + spec.labelSpacing; }
  const last = sorted.length - 1;
  if (last < 0) return;
  const overflow = sorted[last].labelY + spec.labelHeight - spec.labelBottom;
  if (overflow <= 0) return;
  sorted[last].labelY -= overflow;
  for (let i = last - 1; i >= 0; i--) sorted[i].labelY = Math.min(sorted[i].labelY, sorted[i + 1].labelY - spec.labelHeight - spec.labelSpacing);
}

export function sankeyLayout(categories, apps, links) {
  const { source: S, categories: C, apps: A } = SANKEY_COLUMNS;
  const total = categories.reduce((s, n) => s + n.minutes, 0);
  const cats = allocate(categories, C);
  const catBy = new Map(cats.map((b) => [b.key, b]));
  const center = (key) => {
    let w = 0, n = 0;
    for (const l of links) { if (l.to !== key) continue; const c = catBy.get(l.from); if (!c) continue; w += (c.y + c.height / 2) * l.minutes; n += l.minutes; }
    return n ? w / n : 999;
  };
  const ordered = apps.map((app, index) => ({ app, index, c: center(app.key) }))
    .sort((a, b) => (a.app.key === 'other' ? 1 : b.app.key === 'other' ? -1 : a.c - b.c || a.index - b.index)).map((e) => e.app);
  const appBands = allocate(ordered, A);
  const appBy = new Map(appBands.map((b) => [b.key, b]));
  const source = { key: 'source', minutes: total, color: SOURCE_COLOR, x: S.x, y: S.top, height: S.bottom - S.top, labelY: (S.top + S.bottom) / 2 - S.labelHeight / 2 };
  const flows = [];
  for (const seg of stack(cats, source.y, source.height)) {
    flows.push({ id: `f${flows.length}`, kind: 'source', from: 'source', to: seg.key, minutes: seg.minutes, fromColor: SOURCE_COLOR, toColor: seg.color,
      x0: source.x + SANKEY_BAR, y0t: seg.top, y0b: seg.bottom, x1: seg.x, y1t: seg.y, y1b: seg.y + seg.height, tension: 0.15,
      opacity: 0.14 + 0.08 * Math.sqrt(seg.minutes / Math.max(total, 1)) });
  }
  const vis = links.filter((l) => catBy.has(l.from) && appBy.has(l.to));
  const out = new Map(), inn = new Map();
  for (const c of cats) for (const s of stack(vis.filter((l) => l.from === c.key).sort((a, b) => appBy.get(a.to).y - appBy.get(b.to).y), c.y, c.height)) out.set(`${s.from}|${s.to}`, s);
  for (const a of appBands) for (const s of stack(vis.filter((l) => l.to === a.key).sort((x, y) => catBy.get(x.from).y - catBy.get(y.from).y), a.y, a.height)) inn.set(`${s.from}|${s.to}`, s);
  const maxL = Math.max(1, ...vis.map((l) => l.minutes));
  for (const l of vis) {
    const c = catBy.get(l.from), a = appBy.get(l.to), o = out.get(`${l.from}|${l.to}`), i = inn.get(`${l.from}|${l.to}`);
    flows.push({ id: `f${flows.length}`, kind: 'link', from: l.from, to: l.to, minutes: l.minutes, fromColor: c.color, toColor: a.color,
      x0: c.x + SANKEY_BAR, y0t: o.top, y0b: o.bottom, x1: a.x, y1t: i.top, y1b: i.bottom, tension: 0.42, opacity: 0.08 + 0.18 * Math.sqrt(l.minutes / maxL) });
  }
  placeLabels(cats, C);
  placeLabels(appBands, A);
  return { source, categories: cats, apps: appBands, flows };
}

export function ribbon(x0, y0t, y0b, x1, y1t, y1b, tension) {
  const c = Math.max(90, (x1 - x0) * tension), f = (v) => v.toFixed(2);
  return `M${f(x0)} ${f(y0t)} C${f(x0 + c)} ${f(y0t)} ${f(x1 - c)} ${f(y1t)} ${f(x1)} ${f(y1t)} L${f(x1)} ${f(y1b)} C${f(x1 - c)} ${f(y1b)} ${f(x0 + c)} ${f(y0b)} ${f(x0)} ${f(y0b)} Z`;
}
const tint = (hex) => {
  const n = hex.replace('#', '').toUpperCase();
  if (n === '000000' || n === '333333' || n === '111111' || n === '24292F') return '#CAC2BA';
  if (n === 'D9D9D9' || n === 'BFB6AE') return '#CFC8C1';
  return `#${n}`;
};
export function gradientStops(f) {
  const s = Math.max(0.08, Math.min(f.opacity, 0.36));
  const from = tint(f.fromColor), to = tint(f.toColor);
  if (f.kind === 'source') return [[0, '#E3D8CF', 0.18], [0.24, '#ECE3DC', 0.16], [0.58, to, Math.min(0.12, s * 0.42)], [0.82, to, Math.min(0.2, s * 0.72)], [1, to, Math.min(0.32, s * 1.08)]];
  return [[0, from, Math.min(0.2, s * 0.68)], [0.24, from, Math.min(0.11, s * 0.4)], [0.54, to, Math.min(0.05, s * 0.2)], [0.78, to, Math.min(0.12, s * 0.42)], [1, to, Math.min(0.27, s * 0.9)]];
}

// Dayflow's appColorHex: a brand colour by name, else a palette slot from djb2.
const NEEDLES = [[/chatgpt/, '#333333'], [/claude/, '#D97757'], [/xcode/, '#4085FD'], [/dayflow|daygo/, '#FF7A2F'], [/figma/, '#FF7262'], [/slack/, '#36C5F0'], [/zoom/, '#4085FD'], [/meet/, '#34A853'], [/youtube/, '#FF0000'], [/notion/, '#111111'], [/linear/, '#5E6AD2'], [/github/, '#24292F'], [/safari/, '#2E8BFF'], [/chrome/, '#4285F4'], [/mail/, '#4F8EF7'], [/messages|信息/, '#38D06E'], [/other|其他/, '#D9D9D9']];
const PALETTE = ['#93BCFF', '#DE9DFC', '#6CDACD', '#FFA189', '#FFC6B7', '#D9D9D9'];
export function appColor(name) {
  const low = name.toLowerCase();
  const m = NEEDLES.find(([re]) => re.test(low));
  if (m) return m[1];
  let h = 5381n;
  for (const b of new TextEncoder().encode(name)) h = BigInt.asIntN(64, (h << 5n) + h + BigInt(b));
  return PALETTE[Number((h < 0n ? -h : h) % BigInt(PALETTE.length))];
}
