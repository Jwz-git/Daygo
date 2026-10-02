import { screen } from './screens.js';

const $ = (s, r = document) => r.querySelector(s);
const $$ = (s, r = document) => [...r.querySelectorAll(s)];
const clamp = (v, a = 0, b = 1) => Math.min(b, Math.max(a, v));
const toMin = (s) => { const [h, m] = s.split(':').map(Number); return h * 60 + m; };
const fmt = (m) => `${String(Math.floor(m / 60)).padStart(2, '0')}:${String(Math.round(m % 60)).padStart(2, '0')}`;
const dur = (m) => (m >= 60 ? `${Math.floor(m / 60)} 小时${m % 60 ? ` ${m % 60} 分` : ''}` : `${m} 分钟`);

export const CATS = [
  { id: 'build', name: '构建', c: '#3ed6b5' },
  { id: 'meet', name: '会议', c: '#ffb03b' },
  { id: 'research', name: '研究', c: '#8c7cff' },
  { id: 'talk', name: '沟通', c: '#ff5b36' },
  { id: 'design', name: '设计', c: '#ff8fb8' },
  { id: 'system', name: 'System', c: '#8e8c99' },
];
const catOf = (id) => CATS.find((c) => c.id === id) || CATS[0];

let ctx;
let toastTimer;
function toast(msg, ms = 2600, action) {
  const t = $('.toast');
  if (t.parentElement !== document.body) document.body.appendChild(t);
  t.textContent = msg;
  if (action) { const b = document.createElement('button'); b.type = 'button'; b.textContent = action; t.appendChild(b); }
  t.classList.add('on');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.remove('on'), ms);
  return t;
}
function inView(el, cb, opts = { threshold: 0.15 }) {
  const io = new IntersectionObserver((es) => es.forEach((e) => cb(e.isIntersecting)), opts);
  io.observe(el);
}

export function initSections(c) {
  ctx = c;
  howSection();
  timelineSection();
  reviewSection();
  privacySection();
  aiSection();
  alwaysSection();
  visitSection();
}

/* ======================================================================
   HOW · capture → understand → organize
   ====================================================================== */
function howSection() {
  const { gsap, ScrollTrigger, setIntervalSec } = ctx;
  const layers = $$('.stage__layer');
  const prog = $$('.stage__progress span');
  const steps = $$('.step');
  // narrow screens: each step carries its own visual inline
  if (matchMedia('(max-width: 1080px)').matches) {
    layers.forEach((l, i) => { const w = document.createElement('div'); w.className = 'inline-stage'; w.appendChild(l); steps[i].appendChild(w); });
    document.querySelector('.how').classList.add('is-inline');
  }
  let active = -1;
  const setStep = (i) => {
    if (i === active) return;
    active = i;
    layers.forEach((l, k) => l.classList.toggle('is-on', k === i));
    prog.forEach((p, k) => p.classList.toggle('on', k === i));
    steps.forEach((s, k) => s.classList.toggle('is-on', k === i));
    if (i === 1) typeThought();
  };
  steps.forEach((s, i) => ScrollTrigger.create({ trigger: s, start: 'top 55%', end: 'bottom 55%', onToggle: (st) => st.isActive && setStep(i) }));
  setStep(0);

  // --- Capture demo
  const monitor = $('.monitor'), scr = $('.monitor__screen'), flash = $('.monitor__flash'), strip = $('.filmstrip');
  const work = [['code', '#3ed6b5'], ['terminal', '#3ed6b5'], ['browser', '#8c7cff'], ['chat', '#ff5b36'], ['code', '#3ed6b5'], ['design', '#ff8fb8'], ['meeting', '#ffb03b'], ['doc', '#8c7cff']];
  let wi = 0, seed = 1, demoTimer = null, running = false;
  const SPEED = { 1: 0.45, 5: 0.8, 10: 1.25, 20: 1.8, 30: 2.4, 60: 3.4 };
  let speed = SPEED[10];
  const paint = () => { const [k, a] = work[wi % work.length]; scr.innerHTML = screen(k, seed, a); };
  paint();
  for (let i = 0; i < 7; i++) { const d = document.createElement('div'); d.className = 'fs'; const [k, a] = work[(i + 3) % work.length]; d.innerHTML = screen(k, 50 + i, a); strip.appendChild(d); }
  function shot() {
    flash.classList.remove('go'); void flash.offsetWidth; flash.classList.add('go');
    const d = document.createElement('div'); d.className = 'fs'; d.innerHTML = scr.innerHTML;
    strip.prepend(d);
    gsap.fromTo(d, { marginLeft: -110, opacity: 0 }, { marginLeft: 0, opacity: 1, duration: 0.7, ease: 'expo.out' });
    while (strip.children.length > 9) strip.lastChild.remove();
    // the "work" changes every couple of shots
    seed++;
    if (Math.random() < 0.45) wi++;
    setTimeout(paint, 220);
  }
  const loop = () => { if (!running) return; shot(); demoTimer = setTimeout(loop, speed * 1000); };
  inView($('.how'), (v) => { running = v; clearTimeout(demoTimer); if (v) demoTimer = setTimeout(loop, 600); });

  $$('[data-interval] button').forEach((b) => b.addEventListener('click', () => {
    $$('[data-interval] button').forEach((x) => { x.classList.toggle('on', x === b); x.setAttribute('aria-pressed', x === b); });
    const v = +b.dataset.v;
    speed = SPEED[v];
    $('.monitor__iv').textContent = v;
    setIntervalSec(v);
    clearTimeout(demoTimer); if (running) loop();
  }));
  $$('[data-res] button').forEach((b) => b.addEventListener('click', () => {
    $$('[data-res] button').forEach((x) => { x.classList.toggle('on', x === b); x.setAttribute('aria-pressed', x === b); });
    monitor.classList.toggle('lowres', b.dataset.v === '720');
    $('.monitor__res').textContent = `${b.dataset.v}p`;
  }));

  // --- Understand
  $('.lens__frame').innerHTML = screen('code', 7, '#3ed6b5');
  $$('.lens__boxes span').forEach((s) => { const t = s.textContent; s.textContent = ''; const b = document.createElement('b'); b.className = 'lb'; b.textContent = t; s.appendChild(b); });
  const thought = $('.thought__t');
  const lines = '你在修复登录过期的问题，随后回复了代码评审。→ 归类为「构建」';
  let typing;
  function typeThought() {
    clearInterval(typing);
    let i = 0; thought.textContent = '';
    typing = setInterval(() => { thought.textContent = lines.slice(0, ++i); if (i >= lines.length) clearInterval(typing); }, 28);
    gsap.fromTo('.lens__boxes span', { opacity: 0, scale: 1.15 }, { opacity: 1, scale: 1, duration: 0.6, stagger: 0.45, delay: 0.4, ease: 'expo.out' });
  }

  // --- Organize
  const cf = $('.card--hero .card__frames');
  ['code', 'terminal', 'code', 'browser', 'code', 'chat'].forEach((k, i) => { const d = document.createElement('div'); d.innerHTML = screen(k, 20 + i, '#3ed6b5'); cf.appendChild(d); });

}

/* ======================================================================
   TIMELINE · interactive day
   ====================================================================== */
const DAY = [
  { s: '09:02', e: '09:24', cat: 'talk', t: '清理收件箱，准备站会', d: '回复了 6 封邮件，在站会文档里列出今天要推进的三件事。', k: ['chat', 'doc', 'browser'] },
  { s: '09:24', e: '10:41', cat: 'build', t: '修复登录态过期的竞态条件', d: '在 session.go 中发现刷新 token 与并发请求的时序问题，加入单飞锁，并补充回归测试。', k: ['code', 'terminal', 'code', 'browser'] },
  { s: '10:41', e: '11:15', cat: 'meet', t: '支付重构评审会', d: '和后端同学过了一遍幂等键方案，决定把回调重试移入队列。', k: ['meeting', 'doc', 'meeting'] },
  { s: '11:15', e: '12:08', cat: 'research', t: '排查 WebSocket 断线重连', d: '对比心跳间隔与代理超时设置，确认断线来自网关 60 秒空闲断开。', k: ['browser', 'terminal', 'doc', 'browser'] },
  { s: '12:08', e: '13:10', cat: 'system', t: '离开 · 午休', d: '屏幕锁定期间不会截图，这段时间只标记为离开。', k: ['away'] },
  { s: '13:10', e: '14:32', cat: 'build', t: '实现离线缓存层', d: '为时间线接口加上本地缓存与失效策略，接入存储适配器。', k: ['code', 'code', 'terminal'] },
  { s: '14:32', e: '15:05', cat: 'design', t: '和设计讨论空状态插画', d: '在设计稿上留言，对齐了时间线为空时的引导文案与插画方向。', k: ['design', 'chat', 'design'] },
  { s: '15:05', e: '15:40', cat: 'talk', t: '回复用户反馈 #482', d: '复现了导出时的时区错误，回复用户并建了修复任务。', k: ['chat', 'browser'] },
  { s: '15:40', e: '17:20', cat: 'build', t: '为时间线写端到端测试', d: '新增 12 条端到端用例，覆盖编辑、删除与重新处理。', k: ['code', 'terminal', 'code', 'browser'] },
  { s: '17:20', e: '17:55', cat: 'research', t: '阅读 SQLite WAL 文档', d: '整理了 WAL 模式下检查点的注意事项，记进团队文档。', k: ['doc', 'browser'] },
  { s: '17:55', e: '18:20', cat: 'talk', t: '写每日回顾，发给团队', d: '把今天的亮点、完成项和阻塞项整理成站会文本。', k: ['doc', 'chat'] },
];
const REFINE = {
  build: [['梳理缓存失效策略', '按读写频率给接口分组，为每组设定失效时间，并写下取舍。'], ['实现本地存储适配器', '完成适配层并接入时间线接口，补了 4 条单元测试。'], ['重构 token 刷新流程', '把刷新逻辑收拢到一个模块，去掉了两处重复调用。']],
  meet: [['对齐支付回调重试方案', '会上确定由队列负责重试，接口只保证幂等。']],
  research: [['对比网关与代理超时配置', '列出三层超时设置，定位到 60 秒空闲断开。']],
  talk: [['复现并回复时区导出问题', '确认是导出时没有带上用户时区，已回复并建任务。'], ['同步今日进展到团队频道', '发出当日回顾，并 @ 了需要确认网关配置的同学。']],
  design: [['评审空状态插画两版方向', '更偏向第二版：更克制，也和新的视觉语言一致。']],
  system: [['离开 · 午休', '屏幕锁定期间不会截图。']],
};

function timelineSection() {
  const band = $('.tl__band'), wrap = $('.tl__band-wrap'), sel = $('.tl__sel'), rep = $('.tl__reprocess');
  const D0 = toMin('09:00'), D1 = toMin('18:30'), SPAN = D1 - D0;
  let day = DAY.map((x, i) => ({ ...x, id: i, del: false }));
  let nextId = day.length;
  let cur = 1, fidx = 0, range = null, autoplay = null;
  const card = $('.tl__card');

  // hours + legend
  const hours = $('.tl__hours');
  for (let h = 9; h <= 18; h++) { const s = document.createElement('span'); s.textContent = `${String(h).padStart(2, '0')}:00`; s.style.left = `${((h * 60 - D0) / SPAN) * 100}%`; hours.appendChild(s); }
  const legend = $('.tl__legend');
  CATS.forEach((c) => { const s = document.createElement('span'); s.style.setProperty('--c', c.c); s.innerHTML = `<i></i>${c.name}`; legend.appendChild(s); });
  $('.tl__now').style.left = `${((toMin('18:20') - D0) / SPAN) * 100}%`;

  const pct = (m) => ((m - D0) / SPAN) * 100;
  function renderBand(newIds = []) {
    band.innerHTML = '';
    day.forEach((x, i) => {
      const b = document.createElement('div');
      b.className = 'seg-b' + (i === cur ? ' is-sel' : '') + (x.del ? ' is-del' : '') + (newIds.includes(x.id) ? ' is-new' : '');
      b.style.left = `calc(${pct(toMin(x.s))}% + 1px)`;
      b.style.width = `calc(${pct(toMin(x.e)) - pct(toMin(x.s))}% - 2px)`;
      b.style.setProperty('--c', catOf(x.cat).c);
      b.style.animationDelay = `${newIds.indexOf(x.id) * 0.08}s`;
      b.dataset.i = i;
      b.setAttribute('role', 'option');
      b.setAttribute('aria-selected', i === cur);
      b.tabIndex = i === cur ? 0 : -1;
      b.setAttribute('aria-label', `${x.s}–${x.e} ${x.t}`);
      band.appendChild(b);
    });
  }

  const cats = $('.tl__cats');
  CATS.forEach((c) => { const b = document.createElement('button'); b.type = 'button'; b.dataset.id = c.id; b.style.setProperty('--c', c.c); b.innerHTML = `<i></i>${c.name}`; cats.appendChild(b); });
  cats.addEventListener('click', (e) => {
    const b = e.target.closest('button'); if (!b) return;
    day[cur].cat = b.dataset.id; renderBand(); renderCard(false);
  });

  const frameEl = $('.tl__frame'), strip = $('.tl__strip');
  function framesOf(x) { return Array.from({ length: 8 }, (_, k) => x.k[k % x.k.length]); }
  function renderFrames() {
    const x = day[cur], a = catOf(x.cat).c, ks = framesOf(x);
    strip.innerHTML = ks.map((k, j) => `<div class="${j === fidx ? 'on' : ''}">${screen(k, x.id * 13 + j, a)}</div>`).join('');
    showFrame(fidx);
  }
  function showFrame(j) {
    fidx = j;
    const x = day[cur], a = catOf(x.cat).c, ks = framesOf(x);
    frameEl.innerHTML = screen(ks[j], x.id * 13 + j, a);
    $$('div', strip).forEach((d, k) => d.classList.toggle('on', k === j));
    $('.tl__fidx').textContent = `帧 ${String(j + 1).padStart(2, '0')} / 08`;
    const s = toMin(x.s), e = toMin(x.e);
    $('.tl__ftime').textContent = fmt(s + ((e - s) * j) / 8) + ':' + String((j * 17) % 60).padStart(2, '0');
  }
  function renderCard(anim = true) {
    const x = day[cur];
    const c = catOf(x.cat);
    card.style.setProperty('--c', c.c);
    $('.tl__range').textContent = `${x.s} — ${x.e}`;
    $('.tl__dur').textContent = dur(toMin(x.e) - toMin(x.s));
    $('.tl__title').textContent = x.t;
    $('.tl__summary').textContent = x.d;
    $$('button', cats).forEach((b) => b.classList.toggle('on', b.dataset.id === x.cat));
    card.classList.toggle('is-del', x.del);
    $('.tl__del').textContent = x.del ? '恢复卡片' : '删除卡片';
    renderFrames();
    if (anim) ctx.gsap.fromTo([$('.tl__title'), $('.tl__summary'), frameEl], { y: 14, opacity: 0 }, { y: 0, opacity: 1, duration: 0.7, stagger: 0.05, ease: 'expo.out' });
  }
  function select(i, focus = false) { cur = i; fidx = 0; renderBand(); renderCard(); if (focus) band.children[i]?.focus(); }
  band.addEventListener('keydown', (e) => {
    const k = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
    if (k) { e.preventDefault(); select(clamp(cur + k, 0, day.length - 1), true); }
    else if (e.key === 'Home' || e.key === 'End') { e.preventDefault(); select(e.key === 'Home' ? 0 : day.length - 1, true); }
  });

  $('.tl__title').addEventListener('input', (e) => { day[cur].t = e.target.textContent; });
  $('.tl__summary').addEventListener('input', (e) => { day[cur].d = e.target.textContent; });
  $$('.tl__title, .tl__summary').forEach((el) => el.addEventListener('keydown', (e) => { if (e.key === 'Enter' && !e.isComposing && e.keyCode !== 229) { e.preventDefault(); el.blur(); } }));
  $('.tl__del').addEventListener('click', () => {
    const x = day[cur]; x.del = !x.del; renderBand(); renderCard(false);
    if (x.del) {
      const t = toast(`已删除「${x.t}」`, 4000, '撤销');
      $('button', t).onclick = () => { x.del = false; renderBand(); renderCard(false); t.classList.remove('on'); };
    }
  });

  // strip scrubbing
  strip.addEventListener('pointermove', (e) => {
    const r = strip.getBoundingClientRect();
    const n = $$('div', strip).filter((d) => d.offsetParent).length;
    const j = clamp(Math.floor(((e.clientX - r.left) / r.width) * n), 0, n - 1);
    if (j !== fidx) showFrame(j);
  });
  let touched = 0;
  strip.addEventListener('pointerdown', () => (touched = performance.now()));
  strip.addEventListener('click', (e) => { const d = e.target.closest('.tl__strip > div'); if (d) showFrame($$('div', strip).indexOf(d)); });

  // band: click to select, drag to range-select
  let down = null;
  const xToMin = (cx) => { const r = band.getBoundingClientRect(); return D0 + clamp((cx - r.left) / r.width) * SPAN; };
  let pending = false;
  const repLabel = () => $('span', rep) || rep.appendChild(document.createElement('span'));
  const cancelDrag = () => { if (!down) return; down = null; if (!pending) { sel.classList.remove('on'); range = null; } };
  wrap.addEventListener('pointercancel', cancelDrag);
  wrap.addEventListener('lostpointercapture', () => { if (down && !down.drag) down = null; });
  wrap.addEventListener('pointerdown', (e) => {
    if (pending || e.target.closest('.tl__reprocess')) return;
    down = { x: e.clientX, m: xToMin(e.clientX), drag: false };
    try { wrap.setPointerCapture(e.pointerId); } catch {}
  });
  wrap.addEventListener('pointermove', (e) => {
    if (!down) return;
    if (!down.drag && Math.abs(e.clientX - down.x) > 6) { down.drag = true; sel.classList.add('on'); rep.style.display = 'none'; }
    if (down.drag) {
      const a = Math.min(down.m, xToMin(e.clientX)), b = Math.max(down.m, xToMin(e.clientX));
      sel.style.left = `${pct(a)}%`; sel.style.width = `${pct(b) - pct(a)}%`;
      range = [a, b];
    }
  });
  wrap.addEventListener('pointerup', (e) => {
    if (!down) return;
    if (down.drag && range) {
      // snap to the cards the range touches
      const hit = day.map((x, i) => [x, i]).filter(([x]) => toMin(x.e) > range[0] && toMin(x.s) < range[1]);
      if (hit.length) {
        const a = toMin(hit[0][0].s), b = toMin(hit[hit.length - 1][0].e);
        range = [a, b];
        sel.style.left = `${pct(a)}%`; sel.style.width = `${pct(b) - pct(a)}%`;
        repLabel().textContent = `${fmt(a)} – ${fmt(b)}`;
        rep.style.display = '';
      } else { sel.classList.remove('on'); range = null; }
    } else {
      const m = down.m;
      const i = day.findIndex((x) => toMin(x.s) <= m && m < toMin(x.e));
      sel.classList.remove('on'); range = null; rep.style.display = 'none';
      if (i >= 0) select(i);
    }
    down = null;
  });
  rep.addEventListener('click', () => {
    if (!range || pending) return;
    pending = true; rep.disabled = true;
    const [a, b] = range;
    sel.classList.add('shimmer');
    rep.textContent = '分析中…';
    setTimeout(() => {
      const before = day.filter((x) => toMin(x.s) >= a && toMin(x.e) <= b);
      const replaced = [];
      const used = {};
      before.forEach((x) => {
        const pool = REFINE[x.cat] || REFINE.build;
        const pick = () => { used[x.cat] = (used[x.cat] || 0) + 1; return pool[(used[x.cat] - 1) % pool.length]; };
        const s = toMin(x.s), e = toMin(x.e);
        if (e - s > 60 && x.cat !== 'system') {
          const mid = s + Math.round((e - s) * 0.45);
          const p1 = pick(), p2 = pick();
          replaced.push({ ...x, id: nextId++, e: fmt(mid), t: p1[0], d: p1[1], del: false });
          replaced.push({ ...x, id: nextId++, s: fmt(mid), t: p2[0], d: p2[1], k: [...x.k].reverse(), del: false });
        } else {
          const p = pick();
          replaced.push({ ...x, id: nextId++, t: p[0], d: p[1], del: false });
        }
      });
      const first = day.findIndex((x) => x === before[0]);
      day = [...day.slice(0, first), ...replaced, ...day.slice(first + before.length)];
      cur = first;
      fidx = 0;
      renderBand(replaced.map((x) => x.id));
      renderCard();
      sel.classList.remove('on', 'shimmer');
      rep.innerHTML = '↻ 重新处理 <span></span>';
      rep.style.display = 'none';
      rep.disabled = false; pending = false;
      range = null;
      toast(`已重新处理 ${fmt(a)} – ${fmt(b)} · ${before.length} 张卡片替换为 ${replaced.length} 张，无重复`);
    }, 1500);
  });

  renderBand();
  renderCard(false);
  inView($('.tl'), (v) => {
    clearInterval(autoplay);
    if (v) autoplay = setInterval(() => { if (!strip.matches(':hover') && performance.now() - touched > 6000) showFrame((fidx + 1) % 8); }, 1400);
  });
}

/* ======================================================================
   REVIEW · daily + weekly sundial
   ====================================================================== */
function reviewSection() {
  const { gsap, ScrollTrigger } = ctx;
  // typewriter on daily items
  const items = $$('.daily li');
  const texts = items.map((li) => li.textContent);
  items.forEach((li) => (li.textContent = ''));
  ScrollTrigger.create({
    trigger: '.daily', start: 'top 75%', once: true,
    onEnter: () => {
      let k = 0;
      const next = () => {
        if (k >= items.length) return;
        const li = items[k], t = texts[k]; let i = 0;
        li.classList.add('typed');
        const iv = setInterval(() => { li.textContent = t.slice(0, ++i); if (i >= t.length) { clearInterval(iv); k++; setTimeout(next, 60); } }, 18);
      };
      next();
    },
  });
  $('.daily__copy').addEventListener('click', async (e) => {
    const b = e.currentTarget;
    const txt = `【10 月 2 日 站会】\n亮点：\n- ${texts.slice(0, 2).join('\n- ')}\n完成：\n- ${texts.slice(2, 6).join('\n- ')}\n阻塞：\n- ${texts[6]}`;
    try { await navigator.clipboard.writeText(txt); b.querySelector('span').textContent = '已复制 ✓'; } catch { b.querySelector('span').textContent = '复制失败'; }
    setTimeout(() => (b.querySelector('span').textContent = '复制为站会文本'), 2200);
  });

  // sundial
  const svg = $('.sundial__svg');
  const NS = 'http://www.w3.org/2000/svg';
  const W = [
    { id: 'build', m: 870 }, { id: 'research', m: 426 }, { id: 'talk', m: 378 }, { id: 'meet', m: 372 }, { id: 'design', m: 254 },
  ];
  const total = W.reduce((a, b) => a + b.m, 0);
  const cx = 200, cy = 200;
  const pol = (r, a) => [cx + r * Math.cos(a - Math.PI / 2), cy + r * Math.sin(a - Math.PI / 2)];
  const el = (tag, attrs) => { const n = document.createElementNS(NS, tag); for (const k in attrs) n.setAttribute(k, attrs[k]); svg.appendChild(n); return n; };
  // clock ticks
  for (let i = 0; i < 96; i++) {
    const a = (i / 96) * Math.PI * 2, big = i % 4 === 0;
    const [x1, y1] = pol(194, a), [x2, y2] = pol(big ? 186 : 190, a);
    el('line', { x1, y1, x2, y2, stroke: 'currentColor', 'stroke-opacity': big ? 0.35 : 0.15, 'stroke-width': 1 });
  }
  // category arcs
  const R = 166, SW = 22, gap = 0.025;
  let a0 = 0;
  const arcs = W.map((w) => {
    const sweep = (w.m / total) * Math.PI * 2;
    const s = a0 + gap / 2, e = a0 + sweep - gap / 2;
    a0 += sweep;
    const [x1, y1] = pol(R, s), [x2, y2] = pol(R, e);
    const p = el('path', { d: `M${x1} ${y1} A${R} ${R} 0 ${e - s > Math.PI ? 1 : 0} 1 ${x2} ${y2}`, class: 'arc', stroke: catOf(w.id).c, 'stroke-width': SW });
    const len = p.getTotalLength();
    p.style.strokeDasharray = `${len} ${len}`; p.style.strokeDashoffset = len;
    p.dataset.id = w.id; p.dataset.m = w.m;
    return p;
  });
  // daily spokes
  const days = [7.2, 8.1, 6.8, 7.6, 7.4, 0.9, 0.3], dn = ['一', '二', '三', '四', '五', '六', '日'];
  const spokes = days.map((h, i) => {
    const a = (i / 7) * Math.PI * 2 + Math.PI / 7;
    const [x1, y1] = pol(126, a), [x2, y2] = pol(126 + h * 2.6, a);
    const ln = el('line', { x1, y1, x2, y2, stroke: 'currentColor', 'stroke-width': 6, 'stroke-linecap': 'round', 'stroke-opacity': 0.8 });
    const [tx, ty] = pol(116, a);
    const t = el('text', { x: tx, y: ty + 4, 'text-anchor': 'middle', 'font-size': 11, 'font-family': 'Geist Mono, monospace', fill: 'currentColor', 'fill-opacity': 0.45 });
    t.textContent = dn[i];
    return ln;
  });
  spokes.forEach((s) => { const L = Math.hypot(s.x2.baseVal.value - s.x1.baseVal.value, s.y2.baseVal.value - s.y1.baseVal.value) + 1; s.style.strokeDasharray = `${L} ${L}`; s.style.strokeDashoffset = L; });
  ScrollTrigger.create({
    trigger: '.weekly', start: 'top 75%', once: true,
    onEnter: () => {
      gsap.to(arcs, { strokeDashoffset: 0, duration: 1.6, stagger: 0.12, ease: 'expo.inOut' });
      gsap.to(spokes, { strokeDashoffset: 0, duration: 1, stagger: 0.06, delay: 0.6, ease: 'expo.out' });
    },
  });
  const big = $('.sundial__big'), sub = $('.sundial__sub'), orig = big.innerHTML;
  arcs.forEach((p) => {
    p.setAttribute('tabindex', '0');
    p.setAttribute('role', 'button');
    p.setAttribute('aria-label', `${catOf(p.dataset.id).name} ${Math.round((p.dataset.m / total) * 100)}%`);
    const show = () => {
      const m = +p.dataset.m;
      arcs.forEach((q) => q.classList.toggle('dim', q !== p));
      p.setAttribute('stroke-width', SW + 8);
      big.innerHTML = `${Math.floor(m / 60)}<small>h</small> ${m % 60}<small>m</small>`;
      sub.textContent = `${catOf(p.dataset.id).name} · ${Math.round((m / total) * 100)}%`;
    };
    const hide = () => { arcs.forEach((q) => q.classList.remove('dim')); p.setAttribute('stroke-width', SW); big.innerHTML = orig; sub.textContent = '跟踪时长'; };
    p.addEventListener('pointerenter', show); p.addEventListener('focus', show); p.addEventListener('click', show);
    p.addEventListener('pointerleave', hide); p.addEventListener('blur', hide);
  });
  const lg = $('.weekly__legend');
  [...W, { id: 'system', m: 312 }].forEach((w) => {
    const s = document.createElement('span'); s.style.setProperty('--c', catOf(w.id).c);
    s.innerHTML = `<i></i>${catOf(w.id).name}<b>${w.id === 'system' ? '不计入' : `${Math.round((w.m / total) * 100)}%`}</b>`;
    lg.appendChild(s);
  });
}

/* ======================================================================
   PRIVACY · the vault
   ====================================================================== */
function privacySection() {
  const { gsap, ScrollTrigger } = ctx;
  const svg = $('.vault__svg');
  svg.innerHTML = `
    <defs>
      <radialGradient id="vg" cx="50%" cy="50%" r="50%"><stop offset="0" stop-color="#ff8a4c" stop-opacity=".22"/><stop offset=".7" stop-color="#8c7cff" stop-opacity=".06"/><stop offset="1" stop-color="#8c7cff" stop-opacity="0"/></radialGradient>
      <linearGradient id="beam" gradientUnits="userSpaceOnUse" x1="490" y1="0" x2="640" y2="0"><stop offset="0" stop-color="#ffb03b"/><stop offset="1" stop-color="#ff5b36"/></linearGradient>
    </defs>
    <circle cx="300" cy="230" r="200" fill="url(#vg)"/>
    <circle class="v-ring" cx="300" cy="230" r="190" fill="none" stroke="rgba(243,239,231,.28)" stroke-dasharray="2 6"/>
    <circle cx="300" cy="230" r="120" fill="none" stroke="rgba(243,239,231,.08)"/>
    <text x="300" y="28" text-anchor="middle">这台电脑</text>
    <g class="v-orbit"></g>
    <g transform="translate(272 202) scale(.875)" style="color:#ffb03b"><use href="#mark" width="64" height="64"/></g>
    <g class="v-cloud">
      <path class="v-beam" d="M490 230 C 560 230, 580 230, 640 230" stroke="url(#beam)" stroke-width="2" fill="none" stroke-dasharray="6 8"/>
      <rect x="640" y="196" width="150" height="68" rx="16" fill="rgba(255,255,255,.05)" stroke="rgba(255,176,59,.6)"/>
      <text x="715" y="226" text-anchor="middle" style="fill:#f3efe7">你配置的 AI</text>
      <text x="715" y="246" text-anchor="middle" style="font-size:10px">唯一出网路径</text>
    </g>
    <g class="v-local" opacity="0">
      <path class="v-beam" d="M330 230 C 360 230, 370 230, 392 230" stroke="#3ed6b5" stroke-width="2" fill="none" stroke-dasharray="4 6"/>
      <rect x="392" y="208" width="84" height="44" rx="12" fill="rgba(62,214,181,.12)" stroke="#3ed6b5"/>
      <text x="434" y="234" text-anchor="middle" style="fill:#3ed6b5;font-size:11px">本地模型</text>
      <text x="300" y="448" text-anchor="middle" style="fill:#3ed6b5">分析全程留在设备上</text>
    </g>`;
  const orbit = $('.v-orbit', svg);
  const items = ['截图', '时间线', '日志', '数据库', '钥匙串'];
  const nodes = items.map((t) => {
    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.innerHTML = `<rect x="-34" y="-15" width="68" height="30" rx="15" fill="rgba(8,8,11,.6)" stroke="rgba(243,239,231,.22)"/><text x="0" y="4" text-anchor="middle" style="fill:#f3efe7;font-size:11px">${t}</text>`;
    orbit.appendChild(g);
    return g;
  });
  let ang = 0, on = false;
  ctx.gsap.ticker.add(() => {
    if (!on) return;
    ang += 0.0022;
    nodes.forEach((g, i) => {
      const a = ang + (i / nodes.length) * Math.PI * 2;
      g.setAttribute('transform', `translate(${300 + Math.cos(a) * 150} ${230 + Math.sin(a) * 150 * 0.92})`);
    });
  });
  inView($('.vault'), (v) => (on = v));
  ang = 0.3; on = true; ctx.gsap.ticker.tick?.();
  gsap.to($$('.v-beam', svg), { strokeDashoffset: -140, duration: 3, repeat: -1, ease: 'none' });

  $$('.vault__toggle button').forEach((b) => b.addEventListener('click', () => {
    $$('.vault__toggle button').forEach((x) => { x.classList.toggle('on', x === b); x.setAttribute('aria-checked', x === b); });
    const local = b.dataset.mode === 'local';
    gsap.to($('.v-cloud', svg), { opacity: local ? 0.12 : 1, duration: 0.6 });
    gsap.to($('.v-local', svg), { opacity: local ? 1 : 0, duration: 0.6 });
  }));

  ScrollTrigger.create({ trigger: '.nos', start: 'top 80%', once: true, onEnter: () => $('.nos').classList.add('on') });

  // blocked apps → redacted placeholder frames
  const seq = [['code', 'IDE'], ['vault', null], ['browser', '浏览器'], ['chat', null], ['code', 'IDE'], ['bank', null], ['doc', '文档'], ['vault', null]];
  const strip = $('.redact-strip');
  const KM = { vault: ['doc', '#ffb03b', '密码管理器'], chat: ['chat', '#ff6a45', '私人聊天'], bank: ['browser', '#8c7cff', '网上银行'] };
  seq.forEach(([k, lbl], i) => {
    const d = document.createElement('div'); d.className = 'rf';
    const m = KM[k];
    d.dataset.app = m ? k : '';
    d.innerHTML = (m ? screen(m[0], 90 + i, m[1]) : screen(k, 70 + i, '#3ed6b5')) + `<div class="rx">已屏蔽</div><span class="lbl">${m ? m[2] : lbl}</span>`;
    strip.appendChild(d);
  });
  const blocked = new Set(['vault']);
  const sync = () => {
    $$('.app').forEach((a) => { a.classList.toggle('on', blocked.has(a.dataset.app)); a.setAttribute('aria-pressed', blocked.has(a.dataset.app)); });
    $$('.rf', strip).forEach((f) => f.classList.toggle('red', blocked.has(f.dataset.app)));
  };
  $$('.app').forEach((a) => a.addEventListener('click', () => { const k = a.dataset.app; blocked.has(k) ? blocked.delete(k) : blocked.add(k); sync(); }));
  sync();
}

/* ======================================================================
   AI · fallback chain
   ====================================================================== */
function aiSection() {
  const { gsap } = ctx;
  const chain = $('.chain'), svg = $('.chain__svg'), src = $('.chain__src'), out = $('.chain__out');
  const provs = $$('.prov');
  const failed = new Set();
  const NS = 'http://www.w3.org/2000/svg';
  let pIn = [], pOut = [], dot;

  function build() {
    svg.innerHTML = '<defs><linearGradient id="chainGrad" x1="0" x2="1"><stop offset="0" stop-color="#3ed6b5"/><stop offset="1" stop-color="#8c7cff"/></linearGradient></defs>';
    const off = (el) => { let x = 0, y = 0; for (let n = el; n && n !== chain; n = n.offsetParent) { x += n.offsetLeft; y += n.offsetTop; } return [x, y]; };
    const rel = (el, side) => { const [x, y] = off(el); return [side === 'r' ? x + el.offsetWidth : x, y + el.offsetHeight / 2]; };
    const curve = ([x1, y1], [x2, y2]) => { const mx = (x1 + x2) / 2; return `M${x1} ${y1} C${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`; };
    const s = rel(src, 'r'), o = rel(out, 'l');
    pIn = provs.map((p) => { const e = document.createElementNS(NS, 'path'); e.setAttribute('d', curve(s, rel(p, 'l'))); svg.appendChild(e); return e; });
    pOut = provs.map((p) => { const e = document.createElementNS(NS, 'path'); e.setAttribute('d', curve(rel(p, 'r'), o)); svg.appendChild(e); return e; });
    dot = document.createElementNS(NS, 'circle'); dot.setAttribute('r', 5); dot.setAttribute('class', 'pulse'); dot.setAttribute('cx', -50); svg.appendChild(dot);
    paint();
  }
  const active = () => provs.findIndex((_, i) => !failed.has(i));
  function paint() {
    const a = active();
    provs.forEach((p, i) => {
      p.classList.toggle('is-fail', failed.has(i));
      p.classList.toggle('is-active', i === a);
      $('.prov__st', p).textContent = failed.has(i) ? '超时 ✕' : i === a ? '处理中' : '待命';
      pIn[i]?.setAttribute('class', failed.has(i) ? 'fail' : i === a ? 'live' : '');
      pOut[i]?.setAttribute('class', i === a ? 'live' : '');
    });
    $('.chain__via').textContent = a < 0 ? '全部失败 · 稍后重试' : `经由 ${$('.prov__body b', provs[a]).textContent}`;
  }
  const along = (path, d = 0.9, bad = false) => new Promise((res) => {
    const L = path.getTotalLength(); const o = { t: 0 };
    dot.classList.toggle('bad', false);
    gsap.to(o, { t: 1, duration: d, ease: 'power2.inOut', onUpdate: () => { const pt = path.getPointAtLength(o.t * L); dot.setAttribute('cx', pt.x); dot.setAttribute('cy', pt.y); }, onComplete: () => { if (bad) dot.classList.add('bad'); res(); } });
  });
  let running = false, alive = true;
  async function run() {
    if (running) return; running = true;
    while (alive) {
      if (!chain.dataset.vis) { await new Promise((r) => setTimeout(r, 500)); continue; }
      for (let i = 0; i < provs.length; i++) {
        if (failed.has(i)) { await along(pIn[i], 0.8, true); await new Promise((r) => setTimeout(r, 350)); continue; }
        await along(pIn[i]);
        provs[i].animate([{ transform: 'scale(1)' }, { transform: 'scale(1.02)' }, { transform: 'scale(1)' }], { duration: 400 });
        await along(pOut[i], 0.8);
        out.animate([{ boxShadow: '0 0 0 0 rgba(62,214,181,.6)' }, { boxShadow: '0 0 0 18px rgba(62,214,181,0)' }], { duration: 800 });
        break;
      }
      dot.setAttribute('cx', -50);
      await new Promise((r) => setTimeout(r, 700));
    }
  }
  provs.forEach((p, i) => p.addEventListener('click', () => { failed.has(i) ? failed.delete(i) : failed.add(i); paint(); }));
  inView(chain, (v) => { if (v) { chain.dataset.vis = '1'; setTimeout(build, 1600); } else delete chain.dataset.vis; });
  const mq = matchMedia('(max-width: 1080px)');
  let rt;
  window.addEventListener('resize', () => { clearTimeout(rt); rt = setTimeout(build, 200); });
  document.fonts?.ready.then(build);
  build();
  if (!mq.matches) run(); else mq.addEventListener('change', () => !mq.matches && run());
}

/* ======================================================================
   ALWAYS · bento
   ====================================================================== */
function alwaysSection() {
  const { gsap, setPaused } = ctx;
  const tile = $('.tile--menubar'), menu = $('.menu'), icon = $('.menubar__icon');
  const setMenu = (o) => { menu.classList.toggle('open', o); icon.classList.toggle('open', o); icon.setAttribute('aria-expanded', o); };
  icon.setAttribute('aria-haspopup', 'true');
  setMenu(true);
  icon.addEventListener('click', (e) => { e.stopPropagation(); setMenu(!menu.classList.contains('open')); });
  const ringP = $('.pause-ring__p'), ringT = $('.pause-ring__t'), ringL = $('.pause-ring__l');
  let tween = null;
  const resume = (msg) => {
    tween?.kill(); tween = null;
    tile.classList.remove('paused'); setPaused(false);
    $('.menu__state span').textContent = '正在记录';
    ringT.textContent = '--:--'; ringP.style.strokeDashoffset = 327;
    if (msg) toast(msg);
  };
  $$('[data-pause]', menu).forEach((b) => b.addEventListener('click', () => {
    const v = b.dataset.pause;
    if (v === '0') return resume('已恢复记录');
    tween?.kill();
    tile.classList.add('paused'); setPaused(true);
    if (v === 'inf') {
      $('.menu__state span').textContent = '已暂停';
      ringT.textContent = '∞'; ringL.textContent = '直到你恢复';
      ringP.style.strokeDashoffset = 0;
      return;
    }
    const mins = +v;
    $('.menu__state span').textContent = `已暂停 ${mins} 分钟`;
    ringL.textContent = '后自动恢复 · 演示加速';
    const o = { s: mins * 60 };
    tween = gsap.to(o, {
      s: 0, duration: mins * 0.5, ease: 'none',
      onUpdate: () => { ringT.textContent = `${String(Math.floor(o.s / 60)).padStart(2, '0')}:${String(Math.floor(o.s % 60)).padStart(2, '0')}`; ringP.style.strokeDashoffset = 327 * (1 - o.s / (mins * 60)); },
      onComplete: () => resume('定时暂停结束，已自动恢复记录'),
    });
  }));

  // system events
  const sstrip = $('.sys__strip');
  const bars = Array.from({ length: 32 }, () => { const i = document.createElement('i'); sstrip.appendChild(i); return i; });
  const evs = new Set();
  $$('.sys button').forEach((b) => b.addEventListener('click', () => { const k = b.dataset.ev; evs.has(k) ? evs.delete(k) : evs.add(k); b.classList.toggle('on', evs.has(k)); b.setAttribute('aria-pressed', evs.has(k)); }));
  let hist = bars.map(() => true);
  let sysOn = false;
  inView($('.tile--sys'), (v) => (sysOn = v));
  setInterval(() => {
    if (!sysOn) return;
    hist = [...hist.slice(1), evs.size === 0];
    bars.forEach((b, i) => b.classList.toggle('off', !hist[i]));
  }, 380);

  // disk
  const range = $('.range'), dv = $('.disk__v');
  const upd = () => { dv.textContent = range.value; range.style.setProperty('--p', `${((range.value - 5) / 95) * 100}%`); };
  range.addEventListener('input', upd); upd();

  // categories
  const swatches = ['#3ed6b5', '#ffb03b', '#8c7cff', '#ff5b36', '#ff8fb8', '#7ab8ff', '#c8f560', '#8e8c99'];
  const cc = $('.catchips');
  const list = CATS.slice(0, 5).map((c) => ({ name: c.name, ci: swatches.indexOf(c.c) }));
  let added = 0;
  const draw = () => {
    cc.innerHTML = list.map((c, i) => `<button type="button" data-i="${i}" style="--c:${swatches[c.ci]}"><i></i>${c.name}</button>`).join('') + (list.length < 8 ? '<button type="button" class="add">+ 新分类</button>' : '');
  };
  cc.addEventListener('click', (e) => {
    const b = e.target.closest('button'); if (!b) return;
    if (b.classList.contains('add')) { list.push({ name: ['阅读', '写作', '运营'][added++ % 3], ci: (5 + added) % swatches.length }); }
    else { const c = list[+b.dataset.i]; c.ci = (c.ci + 1) % swatches.length; }
    draw();
  });
  draw();

  // languages
  const words = ['一天结束', '一天結束', 'The day ends', '一日の終わり', '하루의 끝', 'Tagesende', 'Fin du jour', 'Fin del día', 'Fim do dia'];
  const w = $('.lang__w'); let li = 0;
  setInterval(() => {
    if (document.hidden) return;
    li = (li + 1) % words.length;
    gsap.to(w, { yPercent: -110, opacity: 0, duration: 0.45, ease: 'power3.in', onComplete: () => { w.textContent = words[li]; gsap.fromTo(w, { yPercent: 110, opacity: 0 }, { yPercent: 0, opacity: 1, duration: 0.7, ease: 'expo.out' }); } });
  }, 2200);

  // theme
  const tt = $('.tile--theme');
  $$('[data-t]', tt).forEach((b) => b.addEventListener('click', () => {
    $$('[data-t]', tt).forEach((x) => x.classList.toggle('on', x === b));
    const t = b.dataset.t === 'system' ? (matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark') : b.dataset.t;
    tt.dataset.mode = t;
  }));

  document.addEventListener('click', (e) => { if (!e.target.closest('.tile--menubar')) setMenu(false); });
}

/* ======================================================================
   VISIT · your own timeline of this page
   ====================================================================== */
function visitSection() {
  const { state, secs } = ctx;
  const colors = ['#ff5b36', '#ffb03b', '#7aa2ff', '#f3efe7', '#ffdcae', '#ff6b3d', '#8c7cff', '#5a5a9a', '#3e3e70', '#2e2e55'];
  const band = $('.visit__band'), labels = $('.visit__labels');
  band.innerHTML = secs.map((_, i) => `<span style="--c:${colors[i]};flex-grow:0"></span>`).join('');
  labels.innerHTML = secs.map((s, i) => `<span style="--c:${colors[i]}"><i></i>${s.dataset.name} <b></b></span>`).join('');
  const spans = $$('span', band), lbs = $$('b', labels);
  const tEl = $('.visit__time'), fEl = $('.visit__frames');
  let on = false;
  inView($('.visit'), (v) => (on = v));
  const tick = () => {
    if (!on) return;
    const total = (performance.now() - state.start) / 1000;
    tEl.textContent = `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(Math.floor(total % 60)).padStart(2, '0')}`;
    fEl.textContent = state.frames;
    const sum = state.dwell.reduce((a, b) => a + b, 0) || 1;
    spans.forEach((s, i) => (s.style.flexGrow = ((state.dwell[i] / sum) * 100).toFixed(2)));
    lbs.forEach((b, i) => { const d = Math.round(state.dwell[i]); b.textContent = `${Math.floor(d / 60)}:${String(d % 60).padStart(2, '0')}`; });
  };
  setInterval(tick, 1000);
  window.addEventListener('daygo:capture', tick);
}
