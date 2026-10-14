// Manual browser regression: NODE_PATH must resolve Playwright, or install it locally without changing the lockfile.
// node web/tests/performance.browser.cjs http://127.0.0.1:5180 baseline
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs');

(async () => {
  const label = process.argv[3] || 'optimized';
  fs.mkdirSync('.tmp', { recursive: true });
  const browser = await chromium.launch({ headless: true, channel: process.env.PLAYWRIGHT_CHANNEL || (process.platform === 'win32' ? 'msedge' : undefined) });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    await page.route('https://api.github.com/**', route => route.fulfill({ status: 503, body: '{}' }));
    await page.addInitScript(() => {
      window.__perf = { raf: 0, clears: 0, mutations: 0 };
      const raf = window.requestAnimationFrame;
      window.requestAnimationFrame = callback => raf.call(window, time => { window.__perf.raf++; callback(time); });
      const clear = CanvasRenderingContext2D.prototype.clearRect;
      CanvasRenderingContext2D.prototype.clearRect = function (...args) { window.__perf.clears++; return clear.apply(this, args); };
      document.addEventListener('DOMContentLoaded', () => {
        new MutationObserver(entries => { window.__perf.mutations += entries.length; }).observe(document.documentElement, { subtree: true, attributes: true, childList: true, characterData: true });
      });
    });
    await page.goto(process.argv[2] || 'http://127.0.0.1:5180');
    await page.waitForLoadState('networkidle');
    const cdp = await page.context().newCDPSession(page);
    await cdp.send('Performance.enable');
    const results = {};
    async function sample(name) {
      await page.waitForTimeout(2500);
      await page.evaluate(() => { window.__perf = { raf: 0, clears: 0, mutations: 0 }; });
      const before = (await cdp.send('Performance.getMetrics')).metrics;
      await page.waitForTimeout(3000);
      const after = (await cdp.send('Performance.getMetrics')).metrics;
      const delta = key => after.find(m => m.name === key).value - before.find(m => m.name === key).value;
      results[name] = { ...await page.evaluate(() => window.__perf), taskMs: Math.round(delta('TaskDuration') * 1000), scriptMs: Math.round(delta('ScriptDuration') * 1000) };
      if (label !== 'baseline') {
        assert.equal(results[name].raf, 0, `${name}: idle animation frames`);
        assert.equal(results[name].clears, 0, `${name}: idle star redraws`);
      }
    }
    await sample('hero');
    await page.mouse.wheel(0, 1200);
    await page.waitForTimeout(3000);
    assert.ok(await page.evaluate(() => window.scrollY > 500), 'smooth wheel scroll must wake');
    for (const lang of ['zh', 'en', 'zh', 'en', 'zh', 'en']) {
      await page.locator(`.lang [data-lang="${lang}"]`).click();
      await page.waitForTimeout(300);
    }
    for (const width of [700, 1440, 700, 1440]) {
      await page.setViewportSize({ width, height: 900 });
      await page.waitForTimeout(400);
    }
    await page.evaluate(() => window.__daygo.lenis?.scrollTo(0, { immediate: true }));
    await sample('rebuiltHero');
    await page.locator('.nav a[href="#download"]').click();
    await page.waitForTimeout(3000);
    assert.equal(await page.locator('html').getAttribute('data-dg-appearance'), 'dark');
    await sample('night');
    await page.screenshot({ path: `.tmp/web-${label}-night.png` });
    if (label !== 'baseline') {
      assert.ok(await page.locator('.stage').evaluate(el => el.classList.contains('motion-paused')));
      assert.equal(await page.locator('.gen-card__spinner i').first().evaluate(el => getComputedStyle(el).animationPlayState), 'paused');
      // A synthetic visibility event exercises our handler independently of headless tab policies.
      await page.evaluate(() => {
        window.__daygo.lenis.scrollTo(document.querySelector('#how').offsetTop + 250, { immediate: true });
      });
      await page.waitForTimeout(1000);
      const shotCount = await page.locator('.shot').count();
      assert.ok(shotCount > 0, 'visible pipeline starts');
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', { configurable: true, value: true });
        document.dispatchEvent(new Event('visibilitychange'));
        window.__perf = { raf: 0, clears: 0, mutations: 0 };
      });
      await page.waitForTimeout(2500);
      assert.equal(await page.locator('.shot').count(), shotCount, 'hidden pipeline does not spawn');
      assert.equal((await page.evaluate(() => window.__perf)).raf, 0);
      await page.evaluate(() => {
        delete document.hidden;
        document.dispatchEvent(new Event('visibilitychange'));
      });
      await page.waitForTimeout(1200);
      assert.equal(await page.locator('html').evaluate(el => el.classList.contains('page-hidden')), false);
      assert.equal(await page.locator('.pipe__ring').evaluate(el => getComputedStyle(el).animationPlayState), 'running');
      await page.evaluate(() => window.__daygo.lenis.scrollTo(window.__daygo.M.stageEnd, { immediate: true }));
      await page.waitForTimeout(6000);
      assert.equal(await page.locator('.range--processing').count(), 0, 'pending timeline card completes');
      assert.equal(await page.locator('.activity-card.is-waiting').count(), 0);
      await page.evaluate(() => window.__daygo.lenis.scrollTo(window.__daygo.M.chapterStart[0] + window.__daygo.M.chapterLen[0] * 0.4, { immediate: true }));
      await page.waitForTimeout(1500);
      await page.screenshot({ path: `.tmp/web-${label}-timeline.png` });
      await page.locator('.player__big-play').click();
      assert.ok(await page.locator('.player').evaluate(el => el.classList.contains('is-playing')));
      await page.locator('.nav a[href="#download"]').click();
      await page.waitForTimeout(2000);
      assert.equal(await page.locator('.player').evaluate(el => el.classList.contains('is-playing')), false, 'offscreen player stops');
      await page.reload();
      await page.waitForLoadState('networkidle');
      await page.evaluate(() => window.__daygo.lenis.scrollTo(window.__daygo.M.chapterStart[0] + window.__daygo.M.chapterLen[0] * 0.4, { immediate: true }));
      await page.waitForTimeout(2500);
      assert.equal(await page.locator('.player').count(), 1, 'a newly generated card opens the demo even after scrolling stops');
      await page.setViewportSize({ width: 390, height: 844 });
      await page.emulateMedia({ reducedMotion: 'reduce' });
      await page.reload();
      await page.waitForLoadState('networkidle');
      await page.locator('.nav__brand').click();
      await page.waitForTimeout(1000);
      assert.equal(await page.locator('.dg').evaluate(el => el.classList.contains('is-compact')), true);
      assert.equal(await page.evaluate(() => window.__daygo.lenis), null);
      await sample('reducedMobile');
      await page.locator('.nav a[href="#download"]').click();
      await page.waitForTimeout(500);
      assert.equal(await page.locator('html').getAttribute('data-dg-appearance'), 'dark');
      await page.screenshot({ path: `.tmp/web-${label}-mobile.png` });
    }
    assert.deepEqual(errors, []);
    fs.writeFileSync(`.tmp/web-${label}-perf.json`, JSON.stringify(results, null, 2));
    console.log(JSON.stringify({ label, results, errors }, null, 2));
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
