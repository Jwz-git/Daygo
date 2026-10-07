// Daygo UI built from the app's own components: the timeline window used in the
// hero, plus standalone pieces (daily, weekly, chat, privacy) for the feature
// sections. Markup, sizes and behaviour follow Daygo/frontend/src.
import { CATS, CARDS, PENDING, NOW, PLANS, STANDUP, WEEK, CHAT } from './data.js';
import { L, t, getLang, dur, durCompact, fmt12, fmt24, hourLabel, hourTick } from './i18n.js';
import { squarify, sankeyLayout, ribbon, gradientStops, appColor, SANKEY_WIDTH, SANKEY_HEIGHT, SANKEY_BAR, SANKEY_COLUMNS } from './charts.js';

const ICONS = new Set(['messages', 'notes', 'vscode', 'github', 'chrome', 'safari', 'terminal', 'youtube', 'discord', 'finder', 'claude', 'gemini', 'daygo', 'xcode']);
const el = (html) => { const tp = document.createElement('template'); tp.innerHTML = html.trim(); return tp.content.firstElementChild; };
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;
const esc = (s) => String(s).replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));

export const appIcon = (app, name = '?', color = '#8E8C99') =>
  app && ICONS.has(app) ? `<img src="apps/${app}.png" alt="" loading="lazy" />` : `<span class="letter" style="--c:${color}">${esc(L(name)).charAt(0).toUpperCase()}</span>`;

const I = {
  chevL: '<svg class="gl" viewBox="0 0 24 24"><path d="M15 5 8 12l7 7"/></svg>',
  chevR: '<svg class="gl" viewBox="0 0 24 24"><path d="M9 5l7 7-7 7"/></svg>',
  close: '<svg class="gl" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6 6 18"/></svg>',
  check: '<svg class="gl" viewBox="0 0 24 24"><path d="M4.5 12.5 9.5 17.5 19.5 6.5"/></svg>',
  play: '<svg class="gl gl--fill" viewBox="0 0 24 24"><path d="M7.5 5.7v12.6a1.4 1.4 0 0 0 2.1 1.2l10-6.3a1.4 1.4 0 0 0 0-2.4l-10-6.3a1.4 1.4 0 0 0-2.1 1.2Z"/></svg>',
  pause: '<svg class="gl gl--fill" viewBox="0 0 24 24"><path d="M7 5h2a1.5 1.5 0 0 1 1.5 1.5v11A1.5 1.5 0 0 1 9 19H7a1.5 1.5 0 0 1-1.5-1.5v-11A1.5 1.5 0 0 1 7 5ZM15 5h2a1.5 1.5 0 0 1 1.5 1.5v11A1.5 1.5 0 0 1 17 19h-2a1.5 1.5 0 0 1-1.5-1.5v-11A1.5 1.5 0 0 1 15 5Z"/></svg>',
  gear: '<svg class="gl gl--fill" viewBox="0 0 24 24"><path fill-rule="evenodd" d="M12 4.6 13.04 4.67 13.95 2.6 15.6 3.1 15.2 5.33 16.11 5.85 17.23 6.77 17.92 7.56 20.02 6.73 20.84 8.26 18.98 9.55 19.26 10.56 19.4 12 19.33 13.04 21.4 13.95 20.9 15.6 18.67 15.2 18.15 16.11 17.23 17.23 16.44 17.92 17.27 20.02 15.74 20.84 14.45 18.98 13.44 19.26 12 19.4 10.96 19.33 10.05 21.4 8.4 20.9 8.8 18.67 7.89 18.15 6.77 17.23 6.08 16.44 3.98 17.27 3.16 15.74 5.02 14.45 4.74 13.44 4.6 12 4.67 10.96 2.6 10.05 3.1 8.4 5.33 8.8 5.85 7.89 6.77 6.77 7.56 6.08 6.73 3.98 8.26 3.16 9.55 5.02 10.56 4.74ZM15.2 12a3.2 3.2 0 1 0-6.4 0 3.2 3.2 0 1 0 6.4 0Z"/></svg>',
  sparkle: '<svg class="gl gl--fill" viewBox="0 0 24 24"><path d="M10 3c.7 4.4 2.6 6.3 7 7-4.4.7-6.3 2.6-7 7-.7-4.4-2.6-6.3-7-7 4.4-.7 6.3-2.6 7-7ZM18 14.5c.3 1.9 1.1 2.7 3 3-1.9.3-2.7 1.1-3 3-.3-1.9-1.1-2.7-3-3 1.9-.3 2.7-1.1 3-3Z"/></svg>',
  send: '<svg class="gl gl--fill" viewBox="0 0 24 24"><path d="M3.4 4.3a1 1 0 0 1 1.3-1.2l15.9 7.9a1.1 1.1 0 0 1 0 2L4.7 20.9a1 1 0 0 1-1.3-1.2L5.5 12Zm2.9 6.8h6.5a.9.9 0 0 1 0 1.8H6.3Z"/></svg>',
  menu: '<svg class="gl" viewBox="0 0 24 24"><path d="M4 6.5h16M4 12h16M4 17.5h16"/></svg>',
  undo: '<svg class="gl" viewBox="0 0 24 24"><path d="M8.5 14.5 3.5 9.5l5-5"/><path d="M3.5 9.5H15a5.5 5.5 0 0 1 0 11h-3"/></svg>',
  left: '<svg class="gl" viewBox="0 0 24 24"><path d="M20 12H4.5M10.5 5.5 4 12l6.5 6.5"/></svg>',
  up: '<svg class="gl" viewBox="0 0 24 24"><path d="M12 20V4.5M5.5 10.5 12 4l6.5 6.5"/></svg>',
  right: '<svg class="gl" viewBox="0 0 24 24"><path d="M4 12h15.5M13.5 5.5 20 12l-6.5 6.5"/></svg>',
  lock: '<svg class="gl gl--fill" viewBox="0 0 24 24"><path d="M7 10h10a2.5 2.5 0 0 1 2.5 2.5V18a2.5 2.5 0 0 1-2.5 2.5H7A2.5 2.5 0 0 1 4.5 18v-5.5A2.5 2.5 0 0 1 7 10Z"/><path d="M8 10V7.5a4 4 0 0 1 8 0V10" fill="none" stroke="currentColor" stroke-width="2.4"/></svg>',
};

const catName = (k) => L(CATS[k].name);
const catColor = (k) => CATS[k].color;
const spinner = () => `<span class="gen-card__spinner" aria-hidden="true">${Array.from({ length: 9 }, (_, i) => `<i style="animation-delay:${((i % 3) + Math.floor(i / 3)) * 0.12}s"></i>`).join('')}</span>`;

/* ───────── Donut ───────── */
function donut(slices, center, cls = '') {
  const R = 87.75, C = 2 * Math.PI * R, gap = slices.length > 1 ? 2.2 : 0;
  const total = slices.reduce((s, x) => s + x.v, 0);
  let acc = 0;
  const arcs = slices.map((s, i) => {
    const len = Math.max(0, (s.v / total) * C - gap), off = -acc;
    acc += (s.v / total) * C;
    return `<circle class="donut__sector" data-i="${i}" cx="102.5" cy="102.5" r="${R}" style="stroke:${s.c}" stroke-dasharray="${len} ${C}" data-off="${off}" stroke-dashoffset="${off + len}"/>`;
  }).join('');
  return `<div class="donut ${cls}"><svg viewBox="0 0 205 205"><circle class="donut__base" cx="102.5" cy="102.5" r="102.5"/><g transform="rotate(-90 102.5 102.5)">${arcs}</g><circle class="donut__disk" cx="102.5" cy="102.5" r="73"/></svg><div class="donut__center">${center}</div></div>`;
}
function playDonut(dn) {
  dn.querySelectorAll('.donut__sector').forEach((s) => {
    s.style.transition = 'none';
    s.setAttribute('stroke-dashoffset', +s.dataset.off + parseFloat(s.getAttribute('stroke-dasharray')));
    s.getBoundingClientRect();
    s.style.transition = '';
    requestAnimationFrame(() => s.setAttribute('stroke-dashoffset', s.dataset.off));
  });
}
function bindDonut(dn, legendItems) {
  const set = (i) => {
    dn.classList.toggle('is-focus', i !== null);
    dn.querySelectorAll('.donut__sector').forEach((s) => s.classList.toggle('is-hot', +s.dataset.i === i));
    legendItems.forEach((l, k) => l.classList.toggle('is-hot', k === i));
  };
  dn.querySelectorAll('.donut__sector').forEach((s) => { s.addEventListener('mouseenter', () => set(+s.dataset.i)); s.addEventListener('mouseleave', () => set(null)); });
  legendItems.forEach((l, k) => { l.addEventListener('mouseenter', () => set(k)); l.addEventListener('mouseleave', () => set(null)); });
}

/* ───────── Mock frames for the card player ───────── */
function screenHTML(kind) {
  const lines = (n, cls = '') => Array.from({ length: n }, (_, i) => `<i class="${cls}" style="--w:${30 + ((i * 37) % 55)}%;--x:${(i % 4) * 6}%"></i>`).join('');
  switch (kind) {
    case 'editor': return `<div class="scr scr--editor"><aside>${lines(9)}</aside><main><div class="scr__tabs"><b></b><b></b></div><div class="scr__code">${lines(16, 'tok')}</div></main></div>`;
    case 'terminal': return `<div class="scr scr--term"><div class="scr__code">${lines(14, 'tok')}</div></div>`;
    case 'chat': return `<div class="scr scr--chat"><aside>${'<p><span></span><i></i></p>'.repeat(6)}</aside><main><div class="b b--l"></div><div class="b b--r"></div><div class="b b--l b--s"></div><div class="b b--r b--s"></div><div class="scr__input"></div></main></div>`;
    case 'doc': return `<div class="scr scr--doc">${[3, 2, 3].map((n) => `<div class="col"><b></b>${'<p></p>'.repeat(n)}</div>`).join('')}</div>`;
    case 'meet': return `<div class="scr scr--meet">${['#7a8cf2', '#f0a17e', '#6fc3df', '#a77cf0'].map((c) => `<div><span style="background:${c}"></span></div>`).join('')}</div>`;
    case 'video': return '<div class="scr scr--video"><div class="scr__vid"></div><div class="scr__side"><p></p><p></p><p></p></div></div>';
    default: return `<div class="scr scr--browser"><div class="scr__url"><i></i><i></i><i></i><b></b></div><div class="scr__page"><h6></h6>${lines(5)}<div class="scr__img"></div>${lines(4)}</div></div>`;
  }
}
function player(card) {
  return `<div class="player"><div class="player__stage">${screenHTML(card.screen)}</div>
    <button class="player__big-play" aria-label="${t('player.play')}">${I.play}</button>
    <span class="player__clock">${fmt24(card.start)}</span><div class="player__bar"><i></i></div></div>`;
}
function bindPlayer(p, card, onTouch = () => {}) {
  if (!p) return { stop() {} };
  const btn = p.querySelector('.player__big-play'), bar = p.querySelector('.player__bar i'), clock = p.querySelector('.player__clock');
  let raf = 0, t0 = 0, pos = 0;
  const tick = (now) => {
    pos = Math.min(1, pos + (now - t0) / 6000);
    t0 = now;
    bar.style.width = `${pos * 100}%`;
    clock.textContent = fmt24(card.start + (card.end - card.start) * pos);
    p.style.setProperty('--pos', pos);
    if (pos < 1) raf = requestAnimationFrame(tick); else stop(true);
  };
  function stop(ended = false) { cancelAnimationFrame(raf); p.classList.remove('is-playing'); btn.innerHTML = I.play; if (ended) pos = 0; }
  function start() { p.classList.add('is-playing'); btn.innerHTML = I.pause; t0 = performance.now(); raf = requestAnimationFrame(tick); }
  btn.addEventListener('click', (e) => { e.stopPropagation(); onTouch(); if (p.classList.contains('is-playing')) stop(); else start(); });
  return { stop, start };
}

/* ───────── Timeline window ───────── */
const DAY_START = 8 * 60, DAY_END = 23 * 60, PPM = 2.6; // PIXELS_PER_MINUTE in layout.ts
const yOf = (m) => (m - DAY_START) * PPM;

export function createApp({ compact = false } = {}) {
  const zh = getLang() === 'zh';
  const cards = [...CARDS];
  const usedCats = Object.keys(CATS).filter((k) => cards.some((c) => c.cat === k) || PENDING.cat === k);
  const root = el(`
  <div class="dg ${compact ? 'is-compact' : ''}">
    <div class="dg__lights" aria-hidden="true"><i></i><i></i><i></i></div>
    <div class="dg__shell">
      <nav class="rail" aria-label="Daygo">
        <div class="rail__list">
          ${['timeline', 'daily', 'weekly', 'chat'].map((k) => `<button class="rail-item ${k === 'timeline' ? 'is-active' : ''}" data-nav="${k}"><span class="rail-item__glyph"><i class="ic ic--${k}"></i></span><span class="rail-item__label">${t(`rail.${k}`)}</span></button>`).join('')}
        </div>
        <div class="rail__utility">
          <div class="rec"><span class="rec__dot"></span>${t('rail.rec')}</div>
          <button class="rail-item" data-nav="settings"><span class="rail-item__glyph">${I.gear}</span><span class="rail-item__label">${t('rail.settings')}</span></button>
        </div>
      </nav>
      <main class="panel">
        <section class="page is-active">
          <header class="page-header">
            <div class="page-header__lead">
              <div class="period-nav"><button class="period-nav__arrow" aria-label="previous">${I.chevL}</button><button class="period-nav__arrow" aria-label="next">${I.chevR}</button><button class="period-nav__current">${t('common.today')}</button></div>
              <button class="icon-btn icon-btn--cal" aria-label="calendar"><i class="ic ic--calendar"></i></button>
              <div class="seg"><span class="seg__thumb"></span><button class="is-on">${t('common.day')}</button><button>${t('common.week')}</button></div>
              <h1 class="dg-page-date">${zh ? '9月30日周三' : 'Wed, Sep 30'}</h1>
            </div>
            <div class="page-header__trail"><span class="muted-note">${t('common.tz')}</span></div>
          </header>
          <div class="chips" data-chips>
            <button class="chip is-on" data-cat="all">${t('common.all')}</button>
            ${usedCats.map((k) => `<button class="chip" data-cat="${k}" style="--c:${catColor(k)}"><i></i>${catName(k)}</button>`).join('')}
            <button class="icon-btn chips__edit" aria-label="edit"><i class="ic ic--edit" style="width:13px;height:13px"></i></button>
          </div>
          <div class="tl-body">
            <div class="tl-track-cell">
              <section class="timeline-track" data-track><div class="timeline-track__canvas" style="height:${(DAY_END - DAY_START) * PPM}px"></div></section>
              <div class="timeline-footer">
                <button class="review-badge" type="button" tabindex="-1"><span class="review-badge__stack" aria-hidden="true"><i class="review-badge__back"></i><i class="review-badge__front">${cards.length}</i></span><span>${t('tl.review')}</span></button>
                <button class="copy-button" data-copy><span class="copy-button__content"><i class="ic ic--copy"></i>${t('tl.copy')}</span></button>
              </div>
            </div>
            <aside class="inspector" data-lenis-prevent>
              <div class="insp-view is-active" data-view="overview"></div>
              <div class="insp-view" data-view="detail"></div>
            </aside>
          </div>
        </section>
      </main>
    </div>
  </div>`);
  const $ = (s) => root.querySelector(s);
  const $$ = (s) => [...root.querySelectorAll(s)];
  let navCb = () => {};
  let touched = false;
  const touch = () => { touched = true; };

  // segmented control
  const seg = $('.seg');
  const placeThumb = () => {
    const on = seg.querySelector('button.is-on'), th = seg.querySelector('.seg__thumb');
    if (!on.offsetWidth) return;
    th.style.width = `${on.offsetWidth}px`;
    th.style.transform = `translateX(${on.offsetLeft - 2}px)`;
  };
  seg.querySelectorAll('button').forEach((b) => b.addEventListener('click', () => { seg.querySelectorAll('button').forEach((x) => x.classList.toggle('is-on', x === b)); placeThumb(); }));

  /* track: hour marks, plan markers, cards, generating + recording cards */
  const scroller = $('[data-track]');
  const canvas = scroller.firstElementChild;
  for (let h = DAY_START / 60; h < DAY_END / 60; h++) canvas.appendChild(el(`<div class="hour-mark" style="top:${yOf(h * 60)}px"><time>${hourLabel(h)}</time><span></span></div>`));
  PLANS.forEach((p) => {
    const m = el(`<button class="plan-marker is-${p.status}" style="top:${yOf(p.start)}px;height:${(p.end - p.start) * PPM}px;--plan-color:${catColor(p.cat)};--plan-lane:0" aria-label="${fmt24(p.start)}–${fmt24(p.end)} ${esc(L(p.title))}"></button>`);
    m.addEventListener('click', (e) => { e.stopPropagation(); touch(); openPlan(p, m); });
    canvas.appendChild(m);
  });
  const events = el('<div class="timeline-track__events"></div>');
  canvas.appendChild(events);
  const planPop = el('<div class="plan-pop dg-popover" hidden></div>');
  canvas.appendChild(planPop);
  function openPlan(p, marker) {
    if (!planPop.hidden && planPop.dataset.id === p.id) { planPop.hidden = true; return; }
    planPop.dataset.id = p.id;
    planPop.innerHTML = `<span class="plan-pop__cat" style="--c:${catColor(p.cat)}"><i></i>${catName(p.cat)}</span><b>${esc(L(p.title))}</b><small>${fmt24(p.start)} – ${fmt24(p.end)} · ${p.status === 'done' ? t('plan.done') : t('plan.upcoming')}</small>`;
    planPop.style.top = `${parseFloat(marker.style.top)}px`;
    planPop.hidden = false;
  }

  const cardEl = (c) => {
    const minutes = c.end - c.start;
    const height = Math.max(10, minutes * PPM - 2);
    const b = el(`<button class="activity-card ${minutes < 13 ? 'is-compact' : ''} ${height >= 96 ? 'is-detailed' : ''}" style="--c:${catColor(c.cat)};top:${yOf(c.start) + 1}px;height:${height}px" data-id="${c.id}" data-cat="${c.cat}" aria-label="${esc(L(c.title))}, ${fmt12(c.start)} – ${fmt12(c.end)}">
      <span class="activity-card__rail"></span>
      ${minutes >= 10 ? `<span class="activity-card__icon">${c.app ? appIcon(c.app, L(c.title), catColor(c.cat)) : appIcon(null, c.apps[0]?.[1] ?? '?', catColor(c.cat))}</span>
      <span class="activity-card__copy"><span class="activity-card__title">${esc(L(c.title))}</span></span>
      <span class="activity-card__time">${fmt12(c.start)} – ${fmt12(c.end)}</span>` : ''}</button>`);
    b.addEventListener('click', (e) => { e.stopPropagation(); touch(); if (selected === c.id) closeDetail(); else selectCard(c.id); });
    return b;
  };
  const cardEls = cards.map(cardEl);
  cardEls.forEach((c) => events.appendChild(c));
  const processing = el(`<div class="range range--processing" style="top:${yOf(PENDING.start)}px;height:${Math.max(34, (PENDING.end - PENDING.start) * PPM)}px"><div class="gen-card range__status">${spinner()}<span class="gen-card__text">${t('tl.generating')}</span></div></div>`);
  events.appendChild(processing);
  const recording = el(`<div class="gen-card gen-card--now" style="top:${yOf(NOW) + 6}px">${spinner()}<span class="gen-card__text">${t('tl.recordingNow')}</span></div>`);
  events.appendChild(recording);
  scroller.addEventListener('click', () => { touch(); planPop.hidden = true; if (selected !== null) closeDetail(); });

  // the pending window turns into its card while the visitor watches
  let resolved = false;
  function resolvePending() {
    if (resolved) return;
    resolved = true;
    processing.classList.add('is-out');
    setTimeout(() => {
      processing.remove();
      cards.push(PENDING);
      const c = cardEl(PENDING);
      c.classList.add('is-new');
      cardEls.push(c);
      events.appendChild(c);
      applyFilter();
      renderOverview();
    }, reduced ? 0 : 380);
  }

  // category chips
  let filter = 'all';
  function applyFilter() { cardEls.forEach((c) => c.classList.toggle('is-dim', filter !== 'all' && c.dataset.cat !== filter)); }
  $$('[data-chips] .chip').forEach((b) => b.addEventListener('click', () => {
    touch();
    filter = b.dataset.cat;
    $$('[data-chips] .chip').forEach((x) => x.classList.toggle('is-on', x === b));
    applyFilter();
  }));

  /* inspector: overview (InspectorDayOverview) */
  const overview = $('[data-view="overview"]');
  function renderOverview() {
    const totals = usedCats.map((k) => ({ k, v: cards.filter((c) => c.cat === k).reduce((s, c) => s + c.end - c.start, 0) })).filter((x) => x.v > 0);
    const all = totals.reduce((s, x) => s + x.v, 0);
    const center = zh ? `<small>${t('ov.total')}</small><b>${Math.floor(all / 60)}小时<br/>${all % 60}分钟</b>` : `<small>${t('ov.total')}</small><b>${Math.floor(all / 60)} hr<br/>${all % 60} min</b>`;
    overview.innerHTML = `<span class="eyebrow">${t('ov.eyebrow')}</span><h3 class="insp-overview__title">${t('ov.title')}</h3><div class="insp-rule"></div>
      ${donut(totals.map((x) => ({ v: x.v, c: catColor(x.k) })), center, 'tl-donut')}
      <div class="legend">${totals.map((x) => `<div style="--c:${catColor(x.k)}"><span><i></i>${catName(x.k)}</span><b>${dur(x.v)}</b></div>`).join('')}</div>
      <div class="tile"><h4>${t('plan.title')}</h4>${PLANS.map((p) => `<div class="plan-row ${p.status === 'done' ? 'is-done' : ''}" style="--c:${catColor(p.cat)}"><i></i><span>${esc(L(p.title))}</span><em>${fmt24(p.start)} – ${fmt24(p.end)}</em></div>`).join('')}</div>`;
    bindDonut(overview.querySelector('.donut'), [...overview.querySelectorAll('.legend > div')]);
    if (overview.classList.contains('is-active')) playDonut(overview.querySelector('.donut'));
  }
  renderOverview();

  /* inspector: card detail (InspectorCardDetail) */
  const detail = $('[data-view="detail"]');
  let selected = null, pl = null;
  function selectCard(id) {
    const c = cards.find((x) => x.id === id);
    selected = id;
    cardEls.forEach((e) => e.classList.toggle('is-selected', +e.dataset.id === id));
    pl?.stop();
    detail.innerHTML = `<div class="card-swap">
      <header class="insp__header"><div class="insp__heading"><p class="insp__eyebrow">${catName(c.cat)}</p><h2 class="insp__title">${esc(L(c.title))}</h2></div>
        <button class="insp__close" aria-label="${t('insp.close')}">${I.close}</button></header>
      <div class="card-time"><span style="background:${catColor(c.cat)}"></span>${fmt24(c.start)} – ${fmt24(c.end)} · ${dur(c.end - c.start)}</div>
      ${player(c)}
      <section class="insp__section"><h3>${t('insp.summary')}</h3><p>${esc(L(c.summary))}</p></section>
      ${c.detail ? `<section class="insp__section"><h3>${t('insp.detailed')}</h3><p>${esc(L(c.detail))}</p></section>` : ''}
      ${c.apps.length ? `<section class="insp__section"><h3>${t('insp.apps')}</h3><ul class="app-sites">${c.apps.map(([a, n]) => `<li>${appIcon(a, n, catColor(c.cat))}<span>${esc(L(n))}</span></li>`).join('')}</ul></section>` : ''}
      ${c.distractions?.length ? `<section class="insp__section"><h3>${t('insp.distractions')}</h3>${c.distractions.map((d) => `<div class="distraction"><span>${fmt24(d.at)} – ${fmt24(d.at + d.len)}</span><strong>${esc(L(d.text))}</strong></div>`).join('')}</section>` : ''}
      <section class="insp__section"><h3>${t('insp.verdict')}</h3><div class="verdict">
        ${[['distraction', 'var(--cat-red)'], ['neutral', '#a3a1aa'], ['focus', '#2fb57a']].map(([k, v]) => `<button style="--verdict:${v}" data-v="${k}" class="${verdicts.get(c.id) === k ? 'is-active' : ''}"><i></i>${t(`v.${k}`)}</button>`).join('')}
      </div></section></div>`;
    detail.querySelector('.insp__close').addEventListener('click', () => { touch(); closeDetail(); });
    detail.querySelectorAll('.verdict button').forEach((b) => b.addEventListener('click', () => {
      touch();
      const on = verdicts.get(c.id) !== b.dataset.v;
      if (on) verdicts.set(c.id, b.dataset.v); else verdicts.delete(c.id);
      detail.querySelectorAll('.verdict button').forEach((x) => x.classList.toggle('is-active', on && x === b));
    }));
    pl = bindPlayer(detail.querySelector('.player'), c, touch);
    showView('detail');
    $('.inspector').scrollTop = 0;
  }
  function closeDetail() {
    selected = null;
    pl?.stop();
    cardEls.forEach((e) => e.classList.remove('is-selected'));
    showView('overview');
  }
  function showView(v) {
    $$('.insp-view').forEach((x) => {
      const on = x.dataset.view === v;
      x.classList.toggle('is-active', on);
      if (on) { x.style.animation = 'none'; x.getBoundingClientRect(); x.style.animation = ''; }
    });
    root.classList.toggle('has-detail', v === 'detail');
    if (v === 'overview') playDonut(overview.querySelector('.donut'));
  }

  const verdicts = new Map();

  const copy = $('[data-copy]');
  copy.addEventListener('click', () => {
    const content = copy.firstElementChild;
    content.innerHTML = `${I.check}${t('tl.copied')}`;
    copy.classList.add('is-copied');
    setTimeout(() => { content.innerHTML = `<i class="ic ic--copy"></i>${t('tl.copy')}`; copy.classList.remove('is-copied'); }, 1400);
  });
  $$('[data-nav]').forEach((b) => b.addEventListener('click', () => navCb(b.dataset.nav)));

  /* track scroll is driven by the page (one scroll = one day) */
  let target = 0, current = 0;
  const startTop = () => yOf(9 * 60) - 14;
  const endTop = () => Math.max(startTop(), yOf(NOW) + 120 - scroller.clientHeight);
  target = current = startTop();
  scroller.scrollTop = current;
  function tick() {
    current += (target - current) * 0.14;
    if (Math.abs(target - current) < 0.3) current = target;
    scroller.scrollTop = current;
    requestAnimationFrame(tick);
  }
  requestAnimationFrame(tick);

  return {
    el: root,
    start() { placeThumb(); playDonut(overview.querySelector('.donut')); },
    onNavigate(cb) { navCb = cb; },
    progress(p) {
      const k = Math.min(1, Math.max(0, (p - 0.08) / 0.8));
      target = startTop() + (endTop() - startTop()) * (k * k * (3 - 2 * k));
      if (p > 0.86) resolvePending();
      if (touched) return;
      // demo: open whichever card sits in the middle of the view, once per pass
      if (p > 0.3 && p < 0.72 && selected === null) {
        const mid = DAY_START + (target + scroller.clientHeight / 2) / PPM;
        const pick = cards.filter((c) => c.cat !== 'distraction').sort((a, b) => Math.abs((a.start + a.end) / 2 - mid) - Math.abs((b.start + b.end) / 2 - mid))[0];
        if (pick) selectCard(pick.id);
      } else if ((p <= 0.22 || p >= 0.8) && selected !== null) closeDetail();
    },
  };
}

/* ───────── Daily: DailyWorkflowOverview + standup ───────── */
export function mountDaily(host) {
  const zh = getLang() === 'zh';
  const all = [...CARDS, PENDING];
  const W0 = 9 * 60, W1 = 22 * 60, SLOT = 15, slots = (W1 - W0) / SLOT;
  const rowKeys = ['focus', 'comm', 'learn', 'research', 'personal'];
  const rows = rowKeys.map((k) => ({
    k,
    cells: Array.from({ length: slots }, (_, i) => {
      const s = W0 + i * SLOT, e = s + SLOT;
      let m = 0, title = '';
      for (const c of all) { if (c.cat !== k) continue; const o = Math.max(0, Math.min(e, c.end) - Math.max(s, c.start)); if (o > m) title = L(c.title); m += o; }
      return { m, occ: m / SLOT, title, s };
    }),
  }));
  const markers = [...all.filter((c) => c.cat === 'distraction').map((c) => ({ s: c.start, e: c.end })), ...all.flatMap((c) => (c.distractions || []).map((d) => ({ s: d.at, e: d.at + d.len })))];
  const totals = [...rowKeys, 'distraction'].map((k) => ({ k, v: all.filter((c) => c.cat === k).reduce((s, c) => s + c.end - c.start, 0) })).filter((x) => x.v > 0);
  host.innerHTML = `
  <section class="daily-section">
    <header class="section-heading"><div><h3>${t('wf.title')}</h3><p>${t('wf.desc')}</p></div><span class="slot-note">${t('wf.note')}</span></header>
    <div class="workflow-card dg-card" data-wf>
      <div class="workflow-scroll"><div class="workflow-grid">
        <div class="workflow-axis-label"></div>
        <div class="workflow-axis">${Array.from({ length: (W1 - W0) / 60 + 1 }, (_, i) => `<span style="--i:${i * 4}" class="${i === 0 ? 'is-first' : i === (W1 - W0) / 60 ? 'is-last' : ''}">${hourTick(W0 / 60 + i)}</span>`).join('')}</div>
        ${rows.map((r, ri) => `<div class="workflow-label"><i style="background:${catColor(r.k)}"></i><span>${catName(r.k)}</span></div>
          <div class="workflow-cells">${r.cells.map((c, i) => `<i class="workflow-cell ${c.occ > 0 ? 'is-occupied' : ''}" style="--daily-cell-color:${catColor(r.k)};--daily-cell-alpha:${0.3 + Math.min(1, c.occ) * 0.7};--d:${i + ri * 3}" data-i="${i}" data-r="${ri}"></i>`).join('')}</div>`).join('')}
        <div class="workflow-label workflow-label--d"><span>${t('wf.distractions')}</span></div>
        <div class="workflow-distraction-track">${markers.map((m) => `<span style="left:${((m.s - W0) / (W1 - W0)) * 100}%;width:max(4px, ${((m.e - m.s) / (W1 - W0)) * 100}%)"></span>`).join('')}</div>
      </div><div class="workflow-tip" data-tip><strong></strong><span></span></div></div>
      <div class="workflow-totals"><span class="workflow-totals__title">${t('wf.total')}</span>${totals.map((x) => `<span class="workflow-total"><i style="background:${catColor(x.k)}"></i>${catName(x.k)} <strong>${dur(x.v)}</strong></span>`).join('')}</div>
    </div>
  </section>
  <section class="daily-section">
    <header class="section-heading"><div><h3>${t('su.title')}</h3><p>${t('su.desc')}</p></div>
      <div class="btn-row"><button class="dbtn dbtn--secondary" data-regen>${t('su.regen')}</button><button class="dbtn dbtn--primary" data-copy>${t('su.copy')}</button></div></header>
    <div class="dg-card standup"><section><small>01</small><h4>${t('su.done')}</h4><ul data-list="done"></ul></section><section><small>02</small><h4>${t('su.next')}</h4><ul data-list="next"></ul></section></div>
  </section>`;
  const wf = host.querySelector('[data-wf]'), grid = host.querySelector('.workflow-grid'), tip = host.querySelector('[data-tip]');
  // DailyWorkflowOverview's sizing: a 20–38px step that fills the card, 9:1 cell-to-gap
  function size() {
    const width = wf.clientWidth;
    const label = 112;
    const step = Math.min(38, Math.max(12, (width - 40 - label - 13) / slots));
    const scale = Math.min(1.2, Math.max(0.85, step / 20));
    grid.style.setProperty('--daily-cell', `${step * 0.9}px`);
    grid.style.setProperty('--daily-gap', `${step * 0.1}px`);
    grid.style.setProperty('--daily-step', `${step}px`);
    grid.style.setProperty('--daily-text-scale', scale);
    grid.style.setProperty('--grid-w', `${slots * step - step * 0.1}px`);
    grid.style.gridTemplateColumns = `${label}px max-content`;
  }
  new ResizeObserver(size).observe(wf);
  let hideT = 0;
  host.querySelectorAll('.workflow-cell.is-occupied').forEach((cell) => {
    cell.addEventListener('mouseenter', () => {
      clearTimeout(hideT);
      const c = rows[+cell.dataset.r].cells[+cell.dataset.i];
      const r = cell.getBoundingClientRect(), b = wf.getBoundingClientRect();
      tip.querySelector('strong').textContent = zh ? `${Math.round(c.m)} 分钟` : `${Math.round(c.m)} min`;
      tip.querySelector('strong').style.color = catColor(rows[+cell.dataset.r].k);
      tip.querySelector('span').textContent = c.title;
      tip.style.left = `${r.left + r.width / 2 - b.left}px`;
      tip.style.top = `${r.top - b.top}px`;
      tip.classList.add('is-visible');
    });
    cell.addEventListener('mouseleave', () => { hideT = setTimeout(() => tip.classList.remove('is-visible'), 80); });
  });
  let run = 0;
  async function typeStandup() {
    const me = ++run;
    const lists = [...host.querySelectorAll('[data-list]')];
    lists.forEach((l) => { l.innerHTML = ''; });
    for (const l of lists) for (const item of STANDUP[l.dataset.list]) {
      const line = L(item), li = el('<li class="caret"></li>');
      l.appendChild(li);
      for (let i = 1; i <= line.length; i += zh ? 1 : 2) { if (me !== run) return; li.textContent = line.slice(0, i); if (!reduced) await sleep(24); }
      li.textContent = line;
      li.classList.remove('caret');
    }
  }
  host.querySelector('[data-regen]').addEventListener('click', typeStandup);
  const copy = host.querySelector('[data-copy]');
  copy.addEventListener('click', () => { copy.textContent = `✓ ${t('su.copied')}`; setTimeout(() => { copy.textContent = t('su.copy'); }, 1400); });
  return { play() { wf.classList.add('is-in'); typeStandup(); } };
}

/* ───────── Weekly: treemap + sankey ───────── */
function weekModel() {
  const cats = WEEK.map((c) => ({ key: c.key, name: catName(c.key), color: catColor(c.key), apps: c.apps.map(([n, icon, m, d]) => ({ key: L(n), name: L(n), icon, minutes: m, change: d })), minutes: c.apps.reduce((s, a) => s + a[2], 0) }))
    .sort((a, b) => b.minutes - a.minutes);
  cats.forEach((c) => c.apps.sort((a, b) => b.minutes - a.minutes));
  return cats;
}
function chartTip(container) {
  const tip = el('<div class="wk-tip" hidden></div>');
  container.appendChild(tip);
  return {
    show(e, html) {
      const r = container.getBoundingClientRect();
      tip.innerHTML = html;
      tip.style.left = `${e.clientX - r.left}px`;
      tip.style.top = `${e.clientY - r.top}px`;
      tip.hidden = false;
    },
    hide() { tip.hidden = true; },
  };
}

export function mountWeekly(host) {
  const cats = weekModel();
  const total = cats.reduce((s, c) => s + c.minutes, 0);
  host.innerHTML = `
    <section class="wk-card"><h3 class="wk-card__title">${t('wk.treemap')}</h3><div class="wk-card__body"><div class="tm" data-tm></div></div></section>
    <section class="wk-card"><h3 class="wk-card__title">${t('wk.sankey')}</h3><div class="wk-card__body"><div class="sk" data-sk></div></div></section>
    <p class="wk-foot">${t('wk.foot')}</p>`;

  /* treemap — WeeklyTreemapCard in an 800×400 design space */
  const tm = host.querySelector('[data-tm]');
  const WIDTH = 800, HEIGHT = 400;
  const TYPES = { large: { name: 22, detail: 12, delta: 10.5, gap: 4, padding: 12, paddingX: 22 }, medium: { name: 18, detail: 11.5, delta: 10, gap: 3, padding: 10, paddingX: 16 }, compact: { name: 14.5, detail: 10.5, delta: 9.5, gap: 2, padding: 6, paddingX: 10 } };
  const pct = (r) => `left:${(r.x / WIDTH) * 100}%;top:${(r.y / HEIGHT) * 100}%;width:${(r.width / WIDTH) * 100}%;height:${(r.height / HEIGHT) * 100}%`;
  const badge = (m) => (m === null || m === 0 ? null : `${m > 0 ? '+' : '-'} ${Math.abs(m)}m`);
  function buildTreemap() {
    const scale = tm.clientWidth / WIDTH || 1;
    const typeOf = (r) => { const w = r.width * scale, h = r.height * scale; return w >= 160 && h >= 110 ? TYPES.large : w >= 90 && h >= 54 ? TYPES.medium : TYPES.compact; };
    const modeOf = (r, app, ty) => {
      const w = r.width * scale, h = r.height * scale, nameRow = (s) => Math.max(13, s * 1.1) * 1.15, time = ty.detail * 1.35;
      const b = badge(app.change) ? ty.gap + ty.delta * 1.3 + 2 : 0;
      if (w >= 96 && h >= ty.padding * 2 + nameRow(ty.name) + ty.gap + time + b) return 'full';
      if (w >= 64 && h >= ty.padding * 2 + nameRow(Math.max(ty.name - 2, 12)) + ty.gap + time) return 'compact';
      return 'labelOnly';
    };
    let html = '';
    for (const { item: c, rect } of squarify(cats, (x) => x.minutes, { x: 0, y: 0, width: WIDTH, height: HEIGHT }, 6)) {
      html += `<section class="tm__shell" data-cat="${c.key}" style="${pct(rect)};--tm-color:${c.color}"><header class="tm__header"><span>${c.name}</span>${rect.width * scale >= 170 ? `<b>${durCompact(c.minutes)}</b>` : ''}</header></section>`;
      const inner = { x: rect.x + 6, y: rect.y + 26, width: Math.max(0, rect.width - 12), height: Math.max(0, rect.height - 32) };
      for (const { item: a, rect: r } of squarify(c.apps, (x) => x.minutes, inner, 4)) {
        const ty = typeOf(r), mode = modeOf(r, a, ty);
        const ns = mode === 'full' ? ty.name : mode === 'compact' ? Math.max(ty.name - 2, 12) : Math.max(ty.name - 3, 11);
        const ds = mode === 'full' ? ty.detail : Math.max(ty.detail - 1, 11);
        const b = badge(a.change);
        html += `<div class="tm__tile" data-cat="${c.key}" data-app="${esc(a.name)}" data-min="${a.minutes}" data-share="${Math.round((a.minutes / c.minutes) * 100)}" data-change="${a.change ?? ''}" style="${pct(r)};--tm-color:${c.color};--tm-name:${ns}px;--tm-detail:${ds}px;--tm-delta:${ty.delta}px;gap:${ty.gap}px;padding:${ty.padding}px ${ty.paddingX}px">
          <span class="tm__name-row">${a.icon || mode !== 'labelOnly' ? `<span class="tm__icon" style="--s:${Math.max(13, Math.round(ns * 1.1))}px">${appIcon(a.icon, a.name, c.color)}</span>` : ''}<span class="tm__name">${esc(a.name)}</span></span>
          ${mode !== 'labelOnly' ? `<span class="tm__time">${durCompact(a.minutes)}</span>` : ''}
          ${mode === 'full' && b ? `<span class="tm__change ${a.change > 0 ? 'is-up' : 'is-down'}">${b}</span>` : ''}</div>`;
      }
    }
    tm.innerHTML = html;
    const tip = chartTip(tm);
    tm.querySelectorAll('.tm__tile').forEach((tile) => {
      tile.addEventListener('pointermove', (e) => {
        tm.classList.add('has-hover');
        tm.querySelectorAll('.tm__tile.is-hovered').forEach((x) => x !== tile && x.classList.remove('is-hovered'));
        tile.classList.add('is-hovered');
        const c = cats.find((x) => x.key === tile.dataset.cat), ch = tile.dataset.change;
        const shareTxt = getLang() === 'zh' ? `占${c.name}的 ${tile.dataset.share}%` : `${tile.dataset.share}% of ${c.name}`;
        const chTxt = ch === '' ? (getLang() === 'zh' ? '上周未使用' : 'Not used last week') : (getLang() === 'zh' ? `较上周 ${+ch > 0 ? '+' : '−'}${dur(Math.abs(+ch))}` : `${+ch > 0 ? '+' : '−'}${dur(Math.abs(+ch))} vs last week`);
        tip.show(e, `<b>${tile.dataset.app}</b><span>${c.name} · ${dur(+tile.dataset.min)}</span><span>${shareTxt}</span><span>${chTxt}</span>`);
      });
      tile.addEventListener('pointerleave', () => { tm.classList.remove('has-hover'); tile.classList.remove('is-hovered'); tip.hide(); });
    });
  }
  new ResizeObserver(() => buildTreemap()).observe(tm);

  /* sankey — WeeklySankeyCard, Dayflow geometry in 1748×933 */
  const sk = host.querySelector('[data-sk]');
  const appNodes = [];
  cats.forEach((c) => c.apps.forEach((a) => { const n = appNodes.find((x) => x.key === a.key); if (n) n.minutes += a.minutes; else appNodes.push({ key: a.key, name: a.name, icon: a.icon, minutes: a.minutes }); }));
  appNodes.sort((a, b) => b.minutes - a.minutes);
  const kept = appNodes.slice(0, 9), restKeys = new Set(appNodes.slice(9).map((a) => a.key));
  const apps = [...kept.map((a) => ({ ...a, color: appColor(a.name) })), { key: 'other', name: t('wk.other'), icon: null, minutes: appNodes.slice(9).reduce((s, a) => s + a.minutes, 0), color: '#D9D9D9' }];
  const links = [];
  cats.forEach((c) => c.apps.forEach((a) => {
    const to = restKeys.has(a.key) ? 'other' : a.key;
    const l = links.find((x) => x.from === c.key && x.to === to);
    if (l) l.minutes += a.minutes; else links.push({ from: c.key, to, minutes: a.minutes });
  }));
  const layout = sankeyLayout(cats.map((c) => ({ key: c.key, minutes: c.minutes, color: c.color })), apps.map((a) => ({ key: a.key, minutes: a.minutes, color: a.color })), links);
  const share = (m) => `${Math.round((m / total) * 100)}%`;
  const C = SANKEY_COLUMNS;
  const ul = ribbon(layout.source.x + SANKEY_BAR, layout.source.y, layout.source.y + layout.source.height, C.categories.x, Math.min(...layout.categories.map((b) => b.y)), Math.max(...layout.categories.map((b) => b.y + b.height)), 0.15);
  const ur = ribbon(C.categories.x + SANKEY_BAR, Math.min(...layout.categories.map((b) => b.y)), Math.max(...layout.categories.map((b) => b.y + b.height)), C.apps.x, Math.min(...layout.apps.map((b) => b.y)), Math.max(...layout.apps.map((b) => b.y + b.height)), 0.22);
  const slot = (x, y, h, w) => `left:${(x / SANKEY_WIDTH) * 100}%;top:${((y + h / 2) / SANKEY_HEIGHT) * 100}%;max-width:${(w / SANKEY_WIDTH) * 100}%`;
  const node = (id) => `data-node="${id}"`;
  sk.innerHTML = `<svg viewBox="0 0 ${SANKEY_WIDTH} ${SANKEY_HEIGHT}" preserveAspectRatio="none" aria-hidden="true">
    <defs>
      <linearGradient id="sk-ul" x1="0" x2="1" y1="0" y2="0"><stop offset="0" stop-color="#E6DBD1" stop-opacity="0.48"/><stop offset="0.42" stop-color="#EFE9E3" stop-opacity="0.34"/><stop offset="0.76" stop-color="#F4EEE9" stop-opacity="0.2"/><stop offset="1" stop-color="#F7F2ED" stop-opacity="0.08"/></linearGradient>
      <linearGradient id="sk-ur" x1="0" x2="1" y1="0" y2="0"><stop offset="0" stop-color="#EFE7E0" stop-opacity="0.08"/><stop offset="0.46" stop-color="#F4EEE9" stop-opacity="0.11"/><stop offset="1" stop-color="#EFE7E0" stop-opacity="0.07"/></linearGradient>
      ${layout.flows.map((f) => `<linearGradient id="sk-${f.id}" gradientUnits="userSpaceOnUse" x1="${f.x0}" x2="${f.x1}" y1="0" y2="0">${gradientStops(f).map(([o, c, a]) => `<stop offset="${o}" stop-color="${c}" stop-opacity="${a.toFixed(3)}"/>`).join('')}</linearGradient>`).join('')}
    </defs>
    <g class="sk__underlay"><path d="${ul}" fill="url(#sk-ul)"/><path d="${ur}" fill="url(#sk-ur)" opacity="0.72"/></g>
    ${layout.flows.map((f) => `<path class="sk__ribbon sk__ribbon--in" data-from="${f.from}" data-to="${f.to}" d="${ribbon(f.x0, f.y0t, f.y0b, f.x1, f.y1t, f.y1b, f.tension)}" fill="url(#sk-${f.id})" style="--i:${f.kind === 'source' ? 0 : 1}"/>`).join('')}
    <rect class="sk__bar" ${node('source')} x="${layout.source.x}" y="${layout.source.y}" width="${SANKEY_BAR}" height="${layout.source.height}" fill="${layout.source.color}"/>
    ${layout.categories.map((b) => `<rect class="sk__bar" ${node(`c:${b.key}`)} x="${b.x}" y="${b.y}" width="${SANKEY_BAR}" height="${b.height}" fill="${b.color}"/>`).join('')}
    ${layout.apps.map((b) => `<rect class="sk__bar" ${node(`a:${b.key}`)} x="${b.x}" y="${b.y}" width="${SANKEY_BAR}" height="${b.height}" fill="${b.color}"/>`).join('')}
  </svg>
  <div class="sk__label" ${node('source')} style="${slot(C.source.labelX, layout.source.labelY, C.source.labelHeight, 220)}"><b>${t('wk.week')}</b><span class="sk__meta">${dur(total)}<i></i>100%</span></div>
  ${layout.categories.map((b) => `<div class="sk__label" ${node(`c:${b.key}`)} style="${slot(C.categories.labelX, b.labelY, C.categories.labelHeight, 260)}"><b>${catName(b.key)}</b><span class="sk__meta">${dur(b.minutes)}<i></i>${share(b.minutes)}</span></div>`).join('')}
  ${layout.apps.map((b) => { const a = apps.find((x) => x.key === b.key); return `<div class="sk__label sk__label--app" ${node(`a:${b.key}`)} style="${slot(C.apps.labelX, b.labelY, C.apps.labelHeight, SANKEY_WIDTH - C.apps.labelX)}">${a.key !== 'other' ? `<span class="sk__icon">${appIcon(a.icon, a.name, a.color)}</span>` : ''}<b>${esc(a.name)}</b><span class="sk__meta">${dur(b.minutes)}<i></i>${share(b.minutes)}</span></div>`; }).join('')}`;
  // Dayflow's activeNodeID: hover (or click to pin) a node, everything unrelated dims
  let pinned = null;
  const related = (id) => {
    if (!id) return () => true;
    const [kind, key] = id === 'source' ? ['source', 'source'] : [id[0], id.slice(2)];
    return (f) => (kind === 'source' ? f.from === 'source' : kind === 'c' ? f.from === key || f.to === key : f.to === key || (f.from === 'source' && links.some((l) => l.to === key && l.from === f.to)));
  };
  function focus(id) {
    sk.classList.toggle('has-focus', !!id);
    const rel = related(id);
    const flows = [...sk.querySelectorAll('.sk__ribbon')];
    const live = new Set(['source']);
    flows.forEach((p) => {
      const ok = rel({ from: p.dataset.from, to: p.dataset.to });
      p.classList.toggle('is-dim', !ok);
      if (ok) { live.add(p.dataset.from === 'source' ? `c:${p.dataset.to}` : `c:${p.dataset.from}`); if (p.dataset.from !== 'source') live.add(`a:${p.dataset.to}`); }
    });
    if (id) live.add(id);
    sk.querySelectorAll('[data-node]').forEach((n) => n.classList.toggle('is-dim', !!id && !live.has(n.dataset.node) && !(id === 'source')));
  }
  sk.querySelectorAll('[data-node]').forEach((n) => {
    n.addEventListener('pointerenter', () => !pinned && focus(n.dataset.node));
    n.addEventListener('pointerleave', () => !pinned && focus(null));
    n.addEventListener('click', (e) => { e.stopPropagation(); pinned = pinned === n.dataset.node ? null : n.dataset.node; focus(pinned); });
  });
  sk.querySelectorAll('.sk__ribbon').forEach((p) => {
    const id = p.dataset.from === 'source' ? `c:${p.dataset.to}` : `a:${p.dataset.to}`;
    p.addEventListener('pointerenter', () => !pinned && focus(id));
    p.addEventListener('pointerleave', () => !pinned && focus(null));
  });
  sk.addEventListener('click', () => { pinned = null; focus(null); });
  return { play() { host.classList.add('is-in'); } };
}

/* ───────── Chat ───────── */
export function mountChat(host) {
  host.innerHTML = `
  <div class="surface chat-box">
    <div class="chat-head"><div class="chat-head__top"><b>${t('chat.new')}</b><button class="icon-btn" data-newchat aria-label="${t('chat.new')}" title="${t('chat.new')}">${I.menu}</button></div>
      <div class="chat-head__sel"><span>${t('chat.provider')}<b>${t('chat.providerVal')}</b></span><span>${t('chat.model')}<b>${t('chat.modelVal')}</b></span></div></div>
    <div class="chat-body">
      <div class="welcome" data-welcome>
        <div class="welcome__icon"><i class="ic ic--chat"></i></div>
        <h3>${t('chat.hello')}</h3><p>${t('chat.helloSub')}</p>
        <ul>${CHAT.hints.map((h, i) => `<li><button class="welcome__hint" data-hint="${i}">${L(h)}</button></li>`).join('')}</ul>
      </div>
      <div class="thread" data-thread></div>
    </div>
    <form class="composer" data-composer><input type="text" placeholder="${t('chat.placeholder')}" aria-label="${t('chat.placeholder')}" /><button type="submit" aria-label="send">${I.send}</button></form>
  </div>`;
  const thread = host.querySelector('[data-thread]'), welcome = host.querySelector('[data-welcome]');
  let run = 0, played = false;
  const md = (s) => s.split('\n\n').map((blk) => {
    const inline = (x) => esc(x).replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>').replace(/\*\*[^*]*$/, '');
    if (blk.startsWith('- ')) return `<ul>${blk.split('\n').map((l) => `<li>${inline(l.replace(/^- ?/, ''))}</li>`).join('')}</ul>`;
    return `<p>${inline(blk)}</p>`;
  }).join('');
  async function ask(i, text) {
    const me = ++run;
    played = true;
    welcome.classList.add('is-gone');
    thread.innerHTML = '';
    thread.appendChild(el(`<div class="cb cb--user"><p>${esc(text ?? L(CHAT.hints[i]))}</p></div>`));
    await sleep(reduced ? 0 : 380);
    const bubble = el(`<div class="cb cb--assistant"><div class="cb__head"><span class="cb__avatar">${I.sparkle}</span><span class="cb__role">Daygo</span></div><div class="cb__body"><div class="thinking"><i></i><i></i><i></i></div></div></div>`);
    thread.appendChild(bubble);
    await sleep(reduced ? 0 : 900);
    if (me !== run) return;
    const body = bubble.querySelector('.cb__body'), full = L(CHAT.answers[i]);
    body.innerHTML = `<div class="cb__md"><span class="cb__tools"><i></i>${t(i === 1 ? 'chat.read.week' : 'chat.read.day')}</span><div data-md></div></div>`;
    const out = body.querySelector('[data-md]');
    for (let k = 1; k <= full.length; k += 2) { if (me !== run) return; out.innerHTML = md(full.slice(0, k)); if (!reduced) await sleep(16); }
    out.innerHTML = md(full);
    const row = el(`<div class="follow">${CHAT.hints.map((h, k) => [h, k]).filter(([, k]) => k !== i).map(([h, k]) => `<button class="follow__chip" data-hint="${k}">${L(h)}</button>`).join('')}</div>`);
    row.querySelectorAll('button').forEach((b) => b.addEventListener('click', () => ask(+b.dataset.hint)));
    body.appendChild(row);
  }
  host.querySelectorAll('[data-hint]').forEach((b) => b.addEventListener('click', () => ask(+b.dataset.hint)));
  host.querySelector('[data-newchat]').addEventListener('click', () => { run++; thread.innerHTML = ''; welcome.classList.remove('is-gone'); });
  host.querySelector('[data-composer]').addEventListener('submit', (e) => {
    e.preventDefault();
    const input = e.currentTarget.querySelector('input'), q = input.value.trim();
    if (!q) return;
    input.value = '';
    ask(/周|week|分类|categor/i.test(q) ? 1 : /站会|日报|standup/i.test(q) ? 2 : 0, q);
  });
  return { play() { if (!played) setTimeout(() => { if (!played) ask(0); }, 600); } };
}
