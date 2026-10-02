import Lenis from 'lenis';
import gsap from 'gsap';
import { ScrollTrigger } from 'gsap/ScrollTrigger';
import { createGL, SKIES, mixSky } from './gl.js';
import { initSections } from './sections.js';

gsap.registerPlugin(ScrollTrigger);

const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const clamp = (v, a = 0, b = 1) => Math.min(b, Math.max(a, v));
const smooth = (e0, e1, x) => { const t = clamp((x - e0) / (e1 - e0)); return t * t * (3 - 2 * t); };
const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const mobile = matchMedia('(max-width: 720px), (pointer: coarse)').matches;
const toMin = (s) => { const [h, m] = s.split(':').map(Number); return h * 60 + m; };
const fmt = (m) => { m = Math.min(1439, Math.max(0, Math.round(m))); return `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(m % 60).padStart(2, '0')}`; };

document.body.classList.add('is-loading');
if (!mobile) document.body.classList.add('has-cursor');

/* ---------------- WebGL ---------------- */
let gl = null;
try { gl = createGL($('.gl'), { mobile }); } catch (e) {
  console.warn('WebGL unavailable', e);
  $('.gl').style.background = 'linear-gradient(#06071a, #1a1840 60%, #ff6a3d)';
}

/* ---------------- Smooth scroll ---------------- */
const lenis = reduced ? null : new Lenis({ lerp: 0.085, wheelMultiplier: 0.95 });
if (lenis) {
  lenis.on('scroll', ScrollTrigger.update);
  lenis.stop();
}
const getY = () => (lenis ? lenis.scroll : window.scrollY);
gsap.ticker.add((t) => { lenis?.raf(t * 1000); frame(t); gl?.render(reduced ? 8 : t % 3600); });
gsap.ticker.lagSmoothing(0);
if (import.meta.env.DEV) window.__daygo = { gl, lenis };

$$('a[href^="#"]').forEach((a) => a.addEventListener('click', (e) => {
  const id = a.getAttribute('href');
  const el = id === '#top' ? 0 : $(id);
  if (el === null) return;
  e.preventDefault();
  if (lenis) lenis.scrollTo(el, { duration: 1.8, easing: (x) => 1 - Math.pow(1 - x, 4) });
  else window.scrollTo({ top: el === 0 ? 0 : el.getBoundingClientRect().top + window.scrollY, behavior: reduced ? 'auto' : 'smooth' });
}));

/* ---------------- Day mapping (sections → sky, clock, theme) ---------------- */
const secs = $$('main > section');
let M = [];
let docMax = 1;
function measure() {
  const y = window.scrollY;
  M = secs.map((el) => {
    const r = el.getBoundingClientRect();
    return { el, top: r.top + y, h: r.height, sky: SKIES[el.dataset.sky], time: toMin(el.dataset.time), theme: el.dataset.theme, label: el.dataset.label, name: el.dataset.name };
  });
  docMax = Math.max(1, document.documentElement.scrollHeight - window.innerHeight);
}
measure();
window.addEventListener('resize', measure);
ScrollTrigger.addEventListener('refresh', measure);
// content heights change after load (typing, re-processed cards…): keep triggers + day map honest
let roT;
new ResizeObserver(() => { clearTimeout(roT); roT = setTimeout(() => ScrollTrigger.refresh(), 250); }).observe($('main'));

const hud = { time: $('.hud__time'), label: $('.hud__label'), frames: $('.hud__frames b'), rec: $('.hud__rec'), root: $('.hud') };
const rail = $('.day-rail__fill');
const ticks = $('.day-rail__ticks');
['06', '12', '18', '24'].forEach((h, i) => { const s = document.createElement('span'); s.textContent = h; s.style.top = `${(i + 1) * 25}%`; ticks.appendChild(s); });

const hero = { el: $('.hero'), title: $$('.hero__title .line > span'), en: $('.hero__en'), foot: $('.hero__foot'), meta: $('.hero__meta'), after: $('.hero__after'), scroll: $('.hero__scroll') };
const dl = $('.dl');

export const state = { section: 0, interval: 10, paused: false, frames: 0, dwell: [], framesBySec: [], start: performance.now(), loaded: false };
state.dwell = secs.map(() => 0);
state.framesBySec = secs.map(() => 0);

let lastT = 0, lastLabel = '', lastNav = '', lastClock = '';
function frame(t) {
  const dt = Math.min(0.1, t - lastT); lastT = t;
  const y = getY(), vh = window.innerHeight, c = y + vh * 0.5;

  let i = M.findIndex((m) => c < m.top + m.h);
  if (i < 0) i = M.length - 1;
  const m = M[i], next = M[i + 1], prev = M[i - 1];
  const w = vh * 0.5;
  let sky = m.sky;
  const dEnd = m.top + m.h - c, dStart = c - m.top;
  if (next && dEnd < w) sky = mixSky(m.sky, next.sky, smooth(0, 1, 0.5 - dEnd / (2 * w)));
  else if (prev && dStart < w) sky = mixSky(prev.sky, m.sky, smooth(0, 1, 0.5 + dStart / (2 * w)));
  gl?.setSky(sky);

  // clock
  const endT = next ? next.time : 23 * 60 + 59;
  const clock = fmt(m.time + (endT - m.time) * clamp((c - m.top) / m.h));
  if (clock !== lastClock) { hud.time.textContent = clock; lastClock = clock; $('.menubar__clock') && ($('.menubar__clock').textContent = clock); }
  if (m.label !== lastLabel) { hud.label.textContent = m.label; lastLabel = m.label; }
  state.section = i;
  if (state.loaded && !document.hidden) state.dwell[i] += dt;

  // nav theme = section under the nav
  const navY = y + 40;
  const under = M.find((s) => navY < s.top + s.h) || m;
  const nt = under.theme;
  if (nt !== lastNav) { document.documentElement.dataset.navTheme = nt; lastNav = nt; }

  rail.style.transform = `scaleY(${clamp(y / docMax)})`;

  // Hero choreography
  const H = M[0];
  const heroP = clamp(y / Math.max(1, H.h - vh));
  const morph = smooth(0.06, 0.72, heroP);
  const exit = clamp((y - (H.h - vh)) / (vh * 0.9));
  const stars = dl ? smooth(0, 1, (y + vh - (M[M.length - 1].top)) / (vh * 1.3)) : 0;
  gl?.setHero(morph, exit);
  gl?.setStars(stars);
  if (state.loaded && !reduced && y < H.h + vh) {
    const out = smooth(0.0, 0.32, heroP);
    hero.title.forEach((s, k) => { s.style.transform = `translate3d(0, ${-out * (60 + k * 30)}%, 0)`; s.style.opacity = 1 - smooth(0.05 + k * 0.03, 0.28 + k * 0.03, heroP); });
    hero.en.style.opacity = hero.foot.style.opacity = hero.meta.style.opacity = 1 - out * 1.6;
    hero.foot.style.transform = `translateY(${out * 40}px)`;
    hero.foot.inert = out > 0.3;
    hero.scroll.style.opacity = 1 - smooth(0, 0.08, heroP);
    const a = smooth(0.62, 0.8, heroP) * (1 - smooth(0.93, 1.0, heroP + exit * 0.2));
    hero.after.style.opacity = a;
    hero.after.style.transform = `translateY(${(1 - smooth(0.62, 0.85, heroP)) * 40 - exit * 80}px)`;
  }
}

/* ---------------- Capture ticks (the site records itself) ---------------- */
const shutter = $('.shutter');
let capTimer = null;
function scheduleCapture() {
  clearTimeout(capTimer);
  capTimer = setTimeout(() => { capture(); scheduleCapture(); }, state.interval * 1000);
}
export function capture(silent = false) {
  if (state.paused || document.hidden) return;
  state.frames++;
  state.framesBySec[state.section]++;
  hud.frames.textContent = String(state.frames).padStart(3, '0');
  hud.rec.classList.remove('tick'); void hud.rec.offsetWidth; hud.rec.classList.add('tick');
  if (!silent) {
    shutter.classList.remove('snap'); void shutter.offsetWidth; shutter.classList.add('snap');
    gl?.pulse();
  }
  window.dispatchEvent(new CustomEvent('daygo:capture'));
}
export function setIntervalSec(s) { state.interval = s; scheduleCapture(); }
export function setPaused(p) { state.paused = p; hud.root.classList.toggle('paused', p); }

/* ---------------- Cursor ---------------- */
if (!mobile) {
  const cur = $('.cursor'), label = $('.cursor__label');
  const ring = $('.cursor__ring'), dot = $('.cursor__dot');
  let x = innerWidth / 2, y = innerHeight / 2, rx = x, ry = y;
  window.addEventListener('pointermove', (e) => { x = e.clientX; y = e.clientY; }, { passive: true });
  window.addEventListener('pointerdown', () => cur.classList.add('is-down'));
  window.addEventListener('pointerup', () => cur.classList.remove('is-down'));
  gsap.ticker.add(() => {
    rx += (x - rx) * 0.2; ry += (y - ry) * 0.2;
    dot.style.transform = `translate3d(${x}px, ${y}px, 0)`;
    ring.style.transform = `translate3d(${rx}px, ${ry}px, 0)`;
    label.style.transform = `translate3d(${rx}px, ${ry}px, 0)`;
  });
  document.addEventListener('pointerover', (e) => {
    const t = e.target.closest('a, button, [data-cursor], input, [contenteditable]');
    cur.classList.toggle('is-hover', !!t);
    const l = t?.closest('[data-cursor]')?.dataset.cursor;
    cur.classList.toggle('has-label', !!l);
    if (l) label.textContent = l;
  });
  // magnetic buttons
  if (!reduced) $$('.magnetic').forEach((b) => {
    const qx = gsap.quickTo(b, 'x', { duration: 0.6, ease: 'power3.out' }), qy = gsap.quickTo(b, 'y', { duration: 0.6, ease: 'power3.out' });
    b.addEventListener('pointermove', (e) => {
      const r = b.getBoundingClientRect();
      qx((e.clientX - r.left - r.width / 2) * 0.25); qy((e.clientY - r.top - r.height / 2) * 0.3);
    });
    b.addEventListener('pointerleave', () => { qx(0); qy(0); });
  });
}

/* ---------------- Text splitting & reveals ---------------- */
function splitLines(el) {
  const parts = el.innerHTML.split(/<br\s*\/?>/i);
  el.innerHTML = parts.map((p) => `<span class="line"><span>${p.trim()}</span></span>`).join('');
  el.classList.add('split');
  return $$('.line > span', el);
}
$$('.section .h2, .dl__title').forEach((h) => {
  const lines = splitLines(h);
  gsap.from(lines, { yPercent: 110, rotate: 2, duration: 1.3, ease: 'expo.out', stagger: 0.09, scrollTrigger: { trigger: h, start: 'top 85%' } });
});
$$('.kicker').forEach((k) => gsap.from(k.children, { y: 14, opacity: 0, duration: 0.9, stagger: 0.07, ease: 'power3.out', scrollTrigger: { trigger: k, start: 'top 90%' } }));
const REVEAL = '.block, .tile, .daily, .weekly, .dlbtn, .node, .prov, .firstrun li, .tl__detail, .vault__viz, .vault__copy, .ai__head .body, .tl__tips, .privacy__en, .visit';
gsap.set(REVEAL, { opacity: 0, y: 50 });
const revealIO = new IntersectionObserver((entries) => {
  const els = entries.filter((e) => e.isIntersecting).map((e) => e.target);
  if (!els.length) return;
  els.forEach((el) => revealIO.unobserve(el));
  gsap.to(els, { opacity: 1, y: 0, duration: 1.2, ease: 'expo.out', stagger: 0.08 });
}, { rootMargin: '0px 0px -8% 0px' });
$$(REVEAL).forEach((el) => revealIO.observe(el));

// Manifesto: characters light up as you read
const man = $('.manifesto');
man.setAttribute('aria-label', man.textContent);
(function splitChars(root) {
  const walk = (node) => {
    [...node.childNodes].forEach((n) => {
      if (n.nodeType === 3) {
        const frag = document.createDocumentFragment();
        [...n.textContent].forEach((ch) => { const s = document.createElement('span'); s.className = 'w'; s.setAttribute('aria-hidden', 'true'); s.textContent = ch; frag.appendChild(s); });
        n.replaceWith(frag);
      } else walk(n);
    });
  };
  walk(root);
})(man);
const chars = $$('.w', man);
ScrollTrigger.create({
  trigger: man, start: 'top 80%', end: 'bottom 40%', scrub: true,
  onUpdate: (st) => {
    const lit = st.progress * chars.length * 1.05;
    chars.forEach((c, k) => { const o = (0.14 + clamp(lit - k, 0, 1) * 0.86).toFixed(2); if (c._o !== o) { c._o = o; c.style.opacity = o; } });
  },
});

// Truth bar: three hours split into what really happened
const segs = $$('.truth__seg');
ScrollTrigger.create({
  trigger: '.truth', start: 'top 75%', end: 'top 20%', scrub: 0.6,
  onUpdate: (st) => segs.forEach((s, k) => s.style.setProperty('--p', smooth(k * 0.18, k * 0.18 + 0.4, st.progress))),
});
gsap.from('.mini-card', { y: 40, opacity: 0, duration: 1, stagger: 0.1, ease: 'expo.out', scrollTrigger: { trigger: '.truth__cards', start: 'top 88%' } });

/* ---------------- Sections ---------------- */
initSections({ state, setIntervalSec, setPaused, capture, secs, gsap, ScrollTrigger, mobile, reduced });

/* ---------------- Loader ---------------- */
function intro() {
  const clock = $('.loader__clock');
  const o = { m: 0 };
  const tl = gsap.timeline({ defaults: { ease: 'expo.inOut' } });
  if (reduced) tl.timeScale(12);
  hero.title.forEach((s) => (s.style.transform = 'translate3d(0,110%,0)'));
  gsap.set([hero.en, hero.foot, hero.meta, '.nav'], { opacity: 0 });
  tl.to('.loader__mark', { clipPath: 'inset(0 0 0% 0)', duration: 1.0 }, 0)
    .to('.loader__line span', { scaleX: 1, duration: 1.4 }, 0)
    .to(o, { m: 330, duration: 1.4, onUpdate: () => (clock.textContent = fmt(o.m)) }, 0)
    .to('.loader', { clipPath: 'inset(0 0 100% 0)', duration: 1.1, ease: 'expo.inOut' }, 1.55)
    .add(() => { $('.loader').remove(); document.body.classList.remove('is-loading'); }, 2.65)
    .add(() => {
      lenis?.start();
      state.loaded = true;
      measure();
      ScrollTrigger.refresh();
      scheduleCapture();
    }, 3.25);
  tl.fromTo(hero.title, { y: '110%' }, { y: '0%', duration: 1.3, stagger: 0.08, ease: 'expo.out', clearProps: 'transform' }, 1.75)
    .to([hero.meta, hero.en, hero.foot, '.nav'], { opacity: 1, duration: 0.8, stagger: 0.04, ease: 'power2.out' }, 2.3);
}
if (document.fonts?.ready) Promise.race([document.fonts.ready, new Promise((r) => setTimeout(r, 2500))]).then(intro);
else intro();
