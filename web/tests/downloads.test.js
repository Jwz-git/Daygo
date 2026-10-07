import test from 'node:test';
import assert from 'node:assert/strict';
import { getReleaseDownloads, loadReleaseDownloads } from '../src/downloads.js';

const page = 'https://github.com/Jwz-git/Daygo/releases/latest';
const fallback = { macos: page, windows: page };
const base = 'https://github.com/Jwz-git/Daygo/releases/download/';
function release(version = '1.2.3') {
  const tag_name = `v${version}`;
  return {
    tag_name, draft: false, prerelease: false,
    assets: [`Daygo-${version}-arm64.dmg`, `Daygo-${version}-amd64-installer.exe`, 'appcast.xml'].map((name) => ({
      name, state: 'uploaded', size: 100,
      browser_download_url: `${base}${tag_name}/${name}`,
    })),
  };
}

test('matches each platform installer and follows a new release without a pinned version', () => {
  for (const version of ['1.2.3', '2.0.0']) {
    assert.deepEqual(getReleaseDownloads(release(version)), {
      macos: `${base}v${version}/Daygo-${version}-arm64.dmg`,
      windows: `${base}v${version}/Daygo-${version}-amd64-installer.exe`,
    });
  }
});

test('a missing or still uploading installer falls back independently', () => {
  const input = release();
  input.assets[1].state = 'starter';
  assert.deepEqual(getReleaseDownloads(input), {
    macos: `${base}v1.2.3/Daygo-1.2.3-arm64.dmg`, windows: page,
  });
  input.assets = [];
  assert.deepEqual(getReleaseDownloads(input), fallback);
});

test('ignores source archives, other architectures, wrong versions and empty files', () => {
  const input = release();
  input.assets[0].name = 'Daygo-1.2.3-amd64.dmg';
  input.assets[1].name = 'Daygo-1.2.2-amd64-installer.exe';
  assert.deepEqual(getReleaseDownloads(input), fallback);
  const empty = release();
  empty.assets.forEach((asset) => { asset.size = 0; });
  assert.deepEqual(getReleaseDownloads(empty), fallback);
});

test('rejects draft, prerelease, malformed metadata and unsupported tags', () => {
  for (const input of [null, {}, { ...release(), assets: null }, { ...release(), draft: true },
    { ...release(), prerelease: true }, { ...release(), tag_name: 'v1.2.3-beta.1' }]) {
    assert.deepEqual(getReleaseDownloads(input), fallback);
  }
});

test('only accepts the exact asset URL in this repository and release', () => {
  for (const url of ['javascript:alert(1)', 'https://example.com/installer.exe',
    `${base}v1.2.2/Daygo-1.2.3-amd64-installer.exe`,
    'https://github.com/other/Daygo/releases/download/v1.2.3/Daygo-1.2.3-amd64-installer.exe']) {
    const input = release();
    input.assets[1].browser_download_url = url;
    assert.equal(getReleaseDownloads(input).windows, page);
  }
});

test('loads latest public release metadata without credentials or stale browser cache', async () => {
  const output = await loadReleaseDownloads(async (url, options) => {
    assert.equal(url, 'https://api.github.com/repos/Jwz-git/Daygo/releases/latest');
    assert.equal(options.credentials, 'omit');
    assert.equal(options.cache, 'no-cache');
    assert.ok(options.signal instanceof AbortSignal);
    return { ok: true, json: async () => release() };
  });
  assert.deepEqual(output, getReleaseDownloads(release()));
});

test('API rate limit, missing release, network failure and invalid JSON preserve working fallback', async () => {
  for (const request of [
    async () => ({ ok: false, status: 403 }),
    async () => ({ ok: false, status: 404 }),
    async () => { throw new TypeError('Network unavailable'); },
    async () => ({ ok: true, json: async () => { throw new SyntaxError('Invalid JSON'); } }),
  ]) {
    assert.deepEqual(await loadReleaseDownloads(request), fallback);
  }
});
