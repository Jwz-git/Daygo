// Procedural "screenshots" — abstract UI miniatures in one unified style.
// Every frame on the site is drawn here, so captures look like one family.

const INK = '#101016';
const PANEL = '#17171f';
const LINE = 'rgba(243,239,231,.10)';
const TXT = 'rgba(243,239,231,.55)';
const TXT2 = 'rgba(243,239,231,.22)';

export function rng(seed) {
  let a = seed >>> 0 || 1;
  return () => {
    a |= 0; a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const r2 = (n) => Math.round(n * 10) / 10;
const rect = (x, y, w, h, fill, rx = 1.5, extra = '') =>
  `<rect x="${r2(x)}" y="${r2(y)}" width="${r2(w)}" height="${r2(h)}" rx="${rx}" fill="${fill}" ${extra}/>`;

function chrome(title) {
  return (
    rect(0, 0, 160, 100, INK, 0) +
    rect(0, 0, 160, 9, PANEL, 0) +
    `<circle cx="5" cy="4.5" r="1.4" fill="#ff5b36" opacity=".8"/><circle cx="9.5" cy="4.5" r="1.4" fill="#ffb03b" opacity=".8"/><circle cx="14" cy="4.5" r="1.4" fill="#3ed6b5" opacity=".8"/>` +
    (title ? rect(60, 3, 40, 3, TXT2, 1.5) : '')
  );
}

const kinds = {
  code(R, a) {
    let s = chrome(true) + rect(0, 9, 26, 91, PANEL, 0);
    for (let i = 0; i < 9; i++) s += rect(4, 14 + i * 7, 8 + R() * 12, 2.4, i === 2 ? a : TXT2, 1.2);
    let y = 14, indent = 0;
    for (let i = 0; i < 12; i++) {
      indent = Math.max(0, Math.min(3, indent + (R() < 0.3 ? 1 : R() < 0.3 ? -1 : 0)));
      let x = 32 + indent * 7;
      const segs = 1 + Math.floor(R() * 3);
      for (let k = 0; k < segs; k++) {
        const w = 6 + R() * 22;
        const c = R() < 0.25 ? a : R() < 0.5 ? TXT : TXT2;
        s += rect(x, y, w, 2.6, c, 1.3);
        x += w + 3;
        if (x > 150) break;
      }
      y += 6.6;
    }
    s += rect(26, 86, 134, 14, '#0c0c11', 0) + rect(30, 90, 50, 2.4, '#ff5b36', 1.2, 'opacity=".7"') + rect(30, 94.5, 34, 2.4, TXT2, 1.2);
    return s;
  },
  browser(R, a) {
    let s = chrome(false) + rect(20, 2.5, 120, 4.5, '#22222c', 2.2);
    s += rect(10, 16, 70, 5, TXT, 2) + rect(10, 25, 40, 3, TXT2, 1.5);
    for (let i = 0; i < 5; i++) {
      const y = 36 + i * 12;
      s += rect(10, y, 2.5, 8, i === 1 ? a : TXT2, 1) + rect(16, y, 40 + R() * 40, 3, i === 1 ? a : TXT, 1.5) + rect(16, y + 5, 60 + R() * 30, 2.2, TXT2, 1.1);
    }
    s += rect(112, 16, 40, 60, PANEL, 3) + rect(116, 22, 30, 18, a, 2, 'opacity=".35"') + rect(116, 46, 26, 2.4, TXT2, 1.2) + rect(116, 51, 20, 2.4, TXT2, 1.2);
    return s;
  },
  chat(R, a) {
    let s = chrome(true) + rect(0, 9, 34, 91, PANEL, 0);
    for (let i = 0; i < 6; i++) s += `<circle cx="8" cy="${17 + i * 12}" r="3.4" fill="${i === 1 ? a : TXT2}"/>` + rect(14, 15 + i * 12, 14, 2.2, TXT2, 1.1);
    let y = 16;
    for (let i = 0; i < 6; i++) {
      const me = R() < 0.45;
      const w = 30 + R() * 50;
      const h = 6 + Math.floor(R() * 2) * 4;
      s += me ? rect(152 - w, y, w, h, a, 3, 'opacity=".55"') : rect(40, y, w, h, '#24242e', 3);
      y += h + 4;
      if (y > 82) break;
    }
    s += rect(40, 88, 112, 7, '#1d1d26', 3.5);
    return s;
  },
  doc(R, a) {
    let s = chrome(true) + rect(30, 14, 100, 86, '#f3efe7', 2);
    s += rect(40, 22, 54, 5, '#08080b', 2) + rect(40, 31, 30, 2.4, a, 1.2);
    let y = 40;
    for (let i = 0; i < 9; i++) {
      s += rect(40, y, i % 4 === 3 ? 40 + R() * 20 : 76 + R() * 4, 2.2, 'rgba(8,8,11,.28)', 1.1);
      y += i % 4 === 3 ? 8 : 5;
    }
    return s;
  },
  design(R, a) {
    let s = chrome(true) + rect(0, 9, 22, 91, PANEL, 0) + rect(138, 9, 22, 91, PANEL, 0);
    s += rect(34, 20, 92, 64, '#f3efe7', 3);
    s += `<circle cx="62" cy="50" r="15" fill="${a}" opacity=".85"/>` + rect(82, 38, 34, 4, '#08080b', 2) + rect(82, 46, 26, 2.6, 'rgba(8,8,11,.35)', 1.3) + rect(82, 58, 22, 7, '#08080b', 3.5);
    s += `<rect x="45.5" y="33.5" width="33" height="33" fill="none" stroke="#3ed6b5" stroke-width=".8"/>`;
    for (let i = 0; i < 6; i++) s += rect(4, 14 + i * 7, 14, 2.4, TXT2, 1.2) + rect(142, 14 + i * 9, 14, 2.4, TXT2, 1.2);
    return s;
  },
  meeting(R, a) {
    let s = chrome(true);
    const cols = ['#2a2a36', '#24303a', '#33282e', '#2b2840'];
    for (let i = 0; i < 4; i++) {
      const x = 6 + (i % 2) * 75, y = 13 + Math.floor(i / 2) * 38;
      s += rect(x, y, 73, 36, cols[i], 3) + `<circle cx="${x + 36.5}" cy="${y + 15}" r="7" fill="${i === 0 ? a : TXT2}" opacity="${i === 0 ? 0.8 : 1}"/>` + rect(x + 26.5, y + 25, 20, 3, TXT2, 1.5);
      if (i === 0) s += `<rect x="${x + 0.5}" y="${y + 0.5}" width="72" height="35" rx="3" fill="none" stroke="${a}" stroke-width="1"/>`;
    }
    s += rect(56, 91, 48, 6, '#22222c', 3) + `<circle cx="80" cy="94" r="2" fill="#ff5b36"/>`;
    return s;
  },
  terminal(R, a) {
    let s = rect(0, 0, 160, 100, '#0b0b0f', 0) + rect(0, 0, 160, 9, PANEL, 0);
    s += `<circle cx="5" cy="4.5" r="1.4" fill="#ff5b36" opacity=".8"/><circle cx="9.5" cy="4.5" r="1.4" fill="#ffb03b" opacity=".8"/><circle cx="14" cy="4.5" r="1.4" fill="#3ed6b5" opacity=".8"/>`;
    let y = 15;
    for (let i = 0; i < 12; i++) {
      s += rect(6, y, 3, 2.4, a, 1.2) + rect(12, y, 20 + R() * 90, 2.4, R() < 0.2 ? '#ff5b36' : TXT, 1.2);
      y += 6.6;
    }
    return s;
  },
  away() {
    return rect(0, 0, 160, 100, '#0d0d12', 0) + `<circle cx="80" cy="54" r="14" fill="none" stroke="${TXT2}" stroke-width="1"/>` + rect(66, 76, 28, 2.4, TXT2, 1.2);
  },
};

export const KINDS = Object.keys(kinds).filter((k) => k !== 'away');

export function screen(kind = 'code', seed = 1, accent = '#3ed6b5') {
  const R = rng(seed * 9973 + kind.length * 31);
  const body = (kinds[kind] || kinds.code)(R, accent);
  return `<svg viewBox="0 0 160 100" preserveAspectRatio="xMidYMid slice" xmlns="http://www.w3.org/2000/svg">${body}<rect x=".5" y=".5" width="159" height="99" fill="none" stroke="${LINE}"/></svg>`;
}
