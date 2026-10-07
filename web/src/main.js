import Lenis from 'lenis';
import './app.css';
import './styles.css';
import { createApp, mountDaily, mountWeekly, mountChat } from './app.js';
import { CARDS, CATS } from './data.js';
import { t, L, getLang, setLang, onLang, fmt12 } from './i18n.js';

const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const clamp = (v, a = 0, b = 1) => Math.min(b, Math.max(a, v));
const lerp = (a, b, t) => a + (b - a) * t;
const smooth = (a, b, x) => { const t = clamp((x - a) / (b - a)); return t * t * (3 - 2 * t); };
const easeInOut = (t) => (t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2);
const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const root = document.documentElement;

/* ───────── Smooth scroll ───────── */
const lenis = reduced ? null : new Lenis({ lerp: 0.09, wheelMultiplier: 0.9 });
const scrollY = () => (lenis ? lenis.scroll : window.scrollY);
const scrollTo = (y) => (lenis ? lenis.scrollTo(y, { duration: 1.4, easing: (x) => 1 - Math.pow(1 - x, 4) }) : window.scrollTo({ top: y, behavior: reduced ? 'auto' : 'smooth' }));
$$('a[href^="#"]').forEach((a) => a.addEventListener('click', (e) => {
  const id = a.getAttribute('href');
  const t = id === '#top' ? 0 : id === '#product' ? M.chapterStart[0] + 2 : $(id) && $(id).getBoundingClientRect().top + window.scrollY;
  if (t === null || t === undefined) return;
  e.preventDefault();
  scrollTo(t);
}));

/* ───────── Timeline window (hero) ───────── */
const compactQ = matchMedia('(max-width: 760px)');
let app = null;
const mount = $('.win__mount');
const NAV = { daily: '#daily', weekly: '#weekly', chat: '#chat', settings: '#privacy' };
function buildApp() {
  mount.innerHTML = '';
  app = createApp({ compact: compactQ.matches });
  mount.appendChild(app.el);
  app.start();
  // the rail jumps to the section that shows that page on its own
  app.onNavigate((name) => {
    hintSeen = true;
    const t = name === 'timeline' ? M.chapterStart[0] + 2 : $(NAV[name]).getBoundingClientRect().top + window.scrollY;
    scrollTo(t);
  });
}
let hintSeen = false;
mount.addEventListener('pointerdown', () => { hintSeen = true; });

/* ───────── Feature components ───────── */
const FEATS = { daily: mountDaily, weekly: mountWeekly, chat: mountChat };
const feats = new Map();
const played = new Set();
const featIO = new IntersectionObserver((entries) => entries.forEach((en) => {
  if (!en.isIntersecting) return;
  featIO.unobserve(en.target);
  played.add(en.target);
  setTimeout(() => feats.get(en.target)?.play(), 350);
}), { threshold: 0.3 });
function buildFeatures() {
  $$('[data-mount]').forEach((host) => {
    const f = FEATS[host.dataset.mount];
    if (!f) return;
    feats.set(host, f(host));
    // after a language switch, sections already seen show their finished state
    if (played.has(host)) feats.get(host).play(); else featIO.observe(host);
  });
}

/* ───────── Language ───────── */
const langEl = $('.lang'), langThumb = $('.lang__thumb');
function placeLangThumb() {
  const on = langEl.querySelector(`[data-lang="${getLang()}"]`);
  langEl.querySelectorAll('button').forEach((b) => b.classList.toggle('is-on', b === on));
  langThumb.style.width = `${on.offsetWidth}px`;
  langThumb.style.transform = `translateX(${on.offsetLeft - 3}px)`;
}
function applyStatic() {
  const zh = getLang() === 'zh';
  root.lang = zh ? 'zh-CN' : 'en';
  document.title = t('meta.title');
  $$('[data-i18n]').forEach((n) => { n.textContent = t(n.dataset.i18n); });
  $$('[data-i18n-html]').forEach((n) => { n.innerHTML = t(n.dataset.i18nHtml); });
  placeLangThumb();
}
langEl.querySelectorAll('button').forEach((b) => b.addEventListener('click', () => setLang(b.dataset.lang)));
onLang(() => {
  document.body.classList.add('is-switching');
  setTimeout(() => {
    applyStatic();
    buildApp();
    buildFeatures();
    pcards.innerHTML = '';
    cardI = 0;
    measure();
    document.body.classList.remove('is-switching');
  }, 180);
});

/* ───────── Layout measurement ───────── */
// stage = intro + the timeline chapter (lengths in viewport heights)
const INTRO = 0.9, CH = [2.8], TAIL = 0.15;
const stage = $('.stage'), win = $('[data-win]'), hero = $('.hero');
const caps = $$('.cap');
const M = { vh: 0, vw: 0, stageTop: 0, introLen: 0, chapterStart: [], chapterLen: [], stageEnd: 0, s: 1, top: 0, heroDy: 0, W: 1240, H: 780 };

function measure() {
  const vh = window.innerHeight, vw = window.innerWidth;
  M.vh = vh; M.vw = vw;
  const total = INTRO + CH.reduce((a, b) => a + b, 0) + TAIL;
  stage.style.setProperty('--stage-h', `${(total + 1) * vh}px`);
  M.stageTop = stage.offsetTop;
  M.introLen = INTRO * vh;
  let y = M.stageTop + M.introLen;
  M.chapterStart = CH.map((c) => { const s = y; y += c * vh; return s; });
  M.chapterLen = CH.map((c) => c * vh);
  M.stageEnd = y;

  const small = vw <= 760;
  M.W = small ? 820 : 1240; M.H = 780;
  const capTop = small ? 76 : 86, capH = small ? 92 : 112, bottom = small ? 24 : 40;
  root.style.setProperty('--cap-top', `${capTop}px`);
  root.style.setProperty('--cap-h', `${capH}px`);
  const avH = vh - capTop - capH - bottom, avW = vw - (small ? 24 : 64);
  // phones get a taller window that fills the space instead of a letterboxed one
  if (small) M.H = clamp(avH / (avW / M.W), 780, 1250);
  M.s = Math.min(avW / M.W, avH / M.H, 1.08);
  M.top = capTop + capH + Math.max(0, (avH - M.H * M.s) / 2) * 0.4;
  win.style.setProperty('--win-w', `${M.W}px`);
  win.style.setProperty('--win-h', `${M.H}px`);
  win.style.setProperty('--s', M.s);
  // in the hero the window waits low on the page, under the headline
  const heroTop = Math.max(small ? 92 : 104, vh * 0.12);
  root.style.setProperty('--hero-top', `${heroTop}px`);
  const heroBottom = heroTop + hero.offsetHeight;
  M.heroDy = Math.max(heroBottom + 36, vh * 0.55) - M.top;

  // sky anchors: scroll position → minutes of the day
  const sec = (id) => { const el = $(id); return el.offsetTop; };
  const docMax = Math.max(1, document.documentElement.scrollHeight - vh);
  M.anchors = [
    [0, 6 * 60 + 5],
    [M.chapterStart[0], 9 * 60],
    [M.stageEnd, 10 * 60 + 40],
    [sec('#daily') + vh * 0.3, 11 * 60 + 30],
    [sec('#weekly') + vh * 0.3, 14 * 60],
    [sec('#chat') + vh * 0.3, 16 * 60],
    [sec('#how') + vh * 0.2, 18 * 60 + 40],
    [sec('#origin') + vh * 0.2, 19 * 60 + 30],
    [sec('#privacy') + vh * 0.1, 20 * 60 + 30],
    [sec('#download'), 22 * 60 + 40],
    [docMax, 23 * 60 + 50],
  ];
}

/* ───────── Sky ───────── */
const SKY = [
  // minute, top, mid, bottom, sun
  [300, '#2a2f5c', '#7d6f9e', '#f0a48a', '#ff8a5c'],
  [365, '#a3b5e6', '#f1d4d8', '#ffcfac', '#ffae78'],
  [450, '#9fc0f0', '#d8e3f7', '#ffe3cf', '#ffd2a0'],
  [600, '#88b6f0', '#c9ddf8', '#f4eef0', '#fff1d8'],
  [720, '#76adf0', '#bcd8f7', '#eef4fb', '#ffffff'],
  [870, '#86aee8', '#cad9f2', '#f8eadf', '#fff0cf'],
  [990, '#8f9fd8', '#e4c7cf', '#ffd4b0', '#ffc48a'],
  [1065, '#5a5f9e', '#c98aa2', '#ffa877', '#ff9a5a'],
  [1120, '#2d3170', '#8f5a8a', '#f2875a', '#ff7a45'],
  [1180, '#151a44', '#352c62', '#7a3f6c', '#ff6a3d'],
  [1260, '#0b0f2b', '#151a40', '#272552', '#ff6a3d'],
  [1440, '#04050f', '#090c22', '#12163a', '#ff6a3d'],
];
const hex = (c) => [1, 3, 5].map((i) => parseInt(c.slice(i, i + 2), 16));
const SKYN = SKY.map(([m, ...cs]) => [m, ...cs.map(hex)]);
const mix = (a, b, t) => `rgb(${a.map((v, i) => Math.round(lerp(v, b[i], t))).join(',')})`;
const skyAt = (m) => {
  let i = 0;
  while (i < SKYN.length - 2 && m > SKYN[i + 1][0]) i++;
  const a = SKYN[i], b = SKYN[i + 1], t = clamp((m - a[0]) / (b[0] - a[0]));
  return [1, 2, 3, 4].map((k) => mix(a[k], b[k], t));
};
function minuteAt(y) {
  const A = M.anchors;
  if (y <= A[0][0]) return A[0][1];
  for (let i = 0; i < A.length - 1; i++) {
    if (y <= A[i + 1][0]) return lerp(A[i][1], A[i + 1][1], clamp((y - A[i][0]) / Math.max(1, A[i + 1][0] - A[i][0])));
  }
  return A[A.length - 1][1];
}

// stars
const stars = $('.sky__stars');
const sctx = stars.getContext('2d');
let starList = [];
function sizeStars() {
  const dpr = Math.min(2, window.devicePixelRatio || 1);
  stars.width = innerWidth * dpr; stars.height = innerHeight * dpr;
  sctx.setTransform(dpr, 0, 0, dpr, 0, 0);
  starList = Array.from({ length: Math.round((innerWidth * innerHeight) / 5200) }, () => ({ x: Math.random() * innerWidth, y: Math.random() * innerHeight * 0.85, r: Math.random() * 1.2 + 0.25, p: Math.random() * 6.28, s: 0.6 + Math.random() * 1.6 }));
}
function drawStars(t, a) {
  sctx.clearRect(0, 0, innerWidth, innerHeight);
  if (a <= 0.01) return;
  for (const s of starList) {
    sctx.globalAlpha = a * (0.45 + 0.55 * Math.sin(t * s.s + s.p) ** 2);
    sctx.fillStyle = '#fff';
    sctx.beginPath(); sctx.arc(s.x, s.y, s.r, 0, 6.283); sctx.fill();
  }
}

const clock = $('.nav__time');
const meta = $('meta[name="theme-color"]');
let lastNight = null, lastClock = '';
function paintSky(m, t) {
  const [top, mid, bot, sun] = skyAt(m);
  const st = root.style;
  st.setProperty('--sky-top', top);
  st.setProperty('--sky-mid', mid);
  st.setProperty('--sky-bot', bot);
  st.setProperty('--sun', sun);
  // the sun arcs from sunrise (06:00) to sunset (19:10)
  const f = (m - 360) / (1150 - 360);
  const alt = Math.sin(Math.PI * clamp(f, -0.08, 1.08));
  st.setProperty('--sun-x', `${lerp(26, 82, clamp(f, 0, 1))}%`);
  st.setProperty('--sun-y', `${96 - alt * 74}%`);
  const lowSun = 1 - clamp(alt * 1.6);
  st.setProperty('--haze', (0.25 + 0.6 * lowSun) * (1 - smooth(1170, 1260, m)));
  st.setProperty('--sun-glow', 1 - smooth(1140, 1200, m));
  st.setProperty('--sun-disc', 1 - smooth(1130, 1175, m));
  const night = smooth(1120, 1240, m);
  st.setProperty('--moon', smooth(1190, 1290, m));
  st.setProperty('--moon-y', `${lerp(34, 14, smooth(1190, 1440, m))}vh`);
  stars.style.opacity = night;
  drawStars(t, night);

  const isNight = m >= 1070;
  if (isNight !== lastNight) {
    lastNight = isNight;
    root.classList.toggle('is-night', isNight);
    root.dataset.dgAppearance = isNight ? 'dark' : 'light';
  }
  const mm = Math.round(m);
  const label = `${String(Math.floor(mm / 60) % 24).padStart(2, '0')}:${String(mm % 60).padStart(2, '0')}`;
  if (label !== lastClock) { lastClock = label; clock.textContent = label; meta.content = top; }
}

/* ───────── Stage choreography ───────── */
let chapter = null;

function updateStage(y) {
  const k = clamp((y - M.stageTop) / M.introLen);
  const e = easeInOut(k);
  // hero copy lifts away
  hero.style.opacity = 1 - smooth(0, 0.5, k);
  hero.style.transform = `translateY(${-k * 90}px) scale(${1 - k * 0.05})`;
  hero.style.visibility = k > 0.6 ? 'hidden' : 'visible';
  // window rises and flattens
  const dy = lerp(M.heroDy, 0, e);
  const rx = lerp(26, 0, e);
  const sc = M.s * lerp(0.9, 1, e);
  win.style.transform = `translateX(-50%) translateY(${M.top + dy}px) rotateX(${rx}deg) scale(${sc})`;

  // the timeline chapter: caption in, demo a card halfway through
  const on = k > 0.82;
  if (on !== chapter) { chapter = on; caps.forEach((cap) => cap.classList.toggle('is-on', on)); }
  win.classList.toggle('show-hint', on && !hintSeen);
  app.progress(on ? clamp((y - M.chapterStart[0]) / M.chapterLen[0]) : 0);
}

/* ───────── Pipeline (dusk section) ───────── */
const frames = $('[data-frames]'), pcards = $('[data-cards]'), lens = $('.pipe__lens');
const SHOT_APPS = [['vscode', CATS.focus.color, '#1f2330'], ['claude', CATS.focus.color, '#faf9f5'], ['messages', CATS.comm.color, '#ffffff'], ['notes', CATS.personal.color, '#fffaf0'], ['safari', CATS.research.color, '#ffffff'], ['youtube', CATS.distraction.color, '#141418'], ['terminal', CATS.focus.color, '#1b1b1f']];
let pipeOn = false, shotI = 0, cardI = 0, pipeTimer = null;
function spawnShot() {
  const [app, c, bg] = SHOT_APPS[shotI++ % SHOT_APPS.length];
  const s = document.createElement('div');
  s.className = 'shot';
  s.style.cssText = `--c:${c};--shot-bg:${bg};--dy:${(Math.random() * 120 - 60).toFixed(0)}px;--r:${(Math.random() * 10 - 5).toFixed(1)}deg;--w:${frames.clientWidth}px`;
  s.innerHTML = `<div class="shot__bar"><i></i><i></i><i></i></div><div class="shot__body"><img src="apps/${app}.png" alt="" /><div class="shot__lines"><b></b><b></b><b></b><b></b></div></div>`;
  frames.appendChild(s);
  s.addEventListener('animationend', () => { s.remove(); lens.classList.add('is-gulp'); setTimeout(() => lens.classList.remove('is-gulp'), 220); });
}
function spawnCard() {
  const pool = CARDS.filter((x) => x.app);
  const c = pool[cardI++ % pool.length];
  const card = document.createElement('div');
  card.className = 'pcard is-enter';
  card.style.setProperty('--c', CATS[c.cat].color);
  card.innerHTML = `<img src="apps/${c.app}.png" alt="" /><b>${L(c.title)}</b><span>${fmt12(c.start)}</span>`;
  pcards.prepend(card);
  layoutCards();
  requestAnimationFrame(() => requestAnimationFrame(() => card.classList.remove('is-enter')));
  if (pcards.children.length > 5) {
    const last = pcards.lastElementChild;
    last.style.opacity = 0;
    setTimeout(() => last.remove(), 600);
  }
}
function layoutCards() {
  const H = pcards.clientHeight, ch = innerWidth <= 860 ? 46 : 54;
  [...pcards.children].forEach((c, i) => { c.style.top = `${H / 2 - ch * 1.5 + i * ch}px`; c.style.opacity = i > 3 ? 0 : 1 - i * 0.16; });
}
function pipeLoop(on) {
  if (on === pipeOn) return;
  pipeOn = on;
  clearInterval(pipeTimer);
  if (!on) return;
  spawnShot();
  let n = 0;
  pipeTimer = setInterval(() => { spawnShot(); if (++n % 2 === 0) spawnCard(); }, 1100);
  if (!pcards.children.length) { spawnCard(); spawnCard(); spawnCard(); }
}

/* ───────── Reveals, magnetic buttons ───────── */
$$('.tile-g').forEach((tile) => tile.addEventListener('pointermove', (e) => {
  const r = tile.getBoundingClientRect();
  tile.style.setProperty('--mx', `${e.clientX - r.left}px`);
  tile.style.setProperty('--my', `${e.clientY - r.top}px`);
}));
const io = new IntersectionObserver((entries) => entries.forEach((en) => {
  if (en.target.classList.contains('pipe')) pipeLoop(en.isIntersecting && !reduced);
  if (en.isIntersecting) en.target.classList.add('is-in');
}), { threshold: 0.18 });
$$('[data-anim]').forEach((el) => io.observe(el));

if (!reduced && matchMedia('(pointer: fine)').matches) {
  $$('.magnetic').forEach((b) => {
    b.addEventListener('pointermove', (e) => {
      const r = b.getBoundingClientRect();
      b.style.transform = `translate(${(e.clientX - r.left - r.width / 2) * 0.18}px, ${(e.clientY - r.top - r.height / 2) * 0.28}px)`;
    });
    b.addEventListener('pointerleave', () => { b.style.transform = ''; });
  });
}

/* ───────── Boot ───────── */
applyStatic();
buildApp();
buildFeatures();
measure();
sizeStars();
compactQ.addEventListener('change', () => { buildApp(); measure(); });
let rT;
window.addEventListener('resize', () => { clearTimeout(rT); rT = setTimeout(() => { measure(); sizeStars(); layoutCards(); placeLangThumb(); }, 120); });
document.fonts?.ready.then(placeLangThumb);
document.fonts?.ready.then(() => { measure(); });
// section heights settle after fonts and charts lay out; keep the sky anchors honest
let roT;
new ResizeObserver(() => { clearTimeout(roT); roT = setTimeout(measure, 150); }).observe($('main'));

function frame(t) {
  lenis?.raf(t);
  const y = scrollY();
  updateStage(y);
  paintSky(minuteAt(y), t / 1000);
  requestAnimationFrame(frame);
}
requestAnimationFrame(frame);
if (import.meta.env.DEV) window.__daygo = { lenis, M, get app() { return app; } };
