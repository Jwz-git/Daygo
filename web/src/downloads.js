export const RELEASE_PAGE = 'https://github.com/Jwz-git/Daygo/releases/latest';
const RELEASE_API = 'https://api.github.com/repos/Jwz-git/Daygo/releases/latest';

export function getReleaseDownloads(release) {
  const downloads = { macos: RELEASE_PAGE, windows: RELEASE_PAGE };
  // Match the vX.Y.Z tags and installer names produced by publish-release.yml.
  if (release?.draft !== false || release?.prerelease !== false ||
      !/^v\d+\.\d+\.\d+$/.test(release?.tag_name) || !Array.isArray(release?.assets)) return downloads;

  const version = release.tag_name.slice(1);
  const names = { macos: `Daygo-${version}-arm64.dmg`, windows: `Daygo-${version}-amd64-installer.exe` };
  for (const [platform, name] of Object.entries(names)) {
    const url = `https://github.com/Jwz-git/Daygo/releases/download/${release.tag_name}/${name}`;
    const asset = release.assets.find((asset) => asset?.name === name && asset.state === 'uploaded' &&
      Number.isFinite(asset.size) && asset.size > 0 && asset.browser_download_url === url);
    if (asset) downloads[platform] = asset.browser_download_url;
  }
  return downloads;
}

export async function loadReleaseDownloads(request = fetch) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 8000);
  try {
    const response = await request(RELEASE_API, {
      headers: { Accept: 'application/vnd.github+json' },
      credentials: 'omit', cache: 'no-cache', signal: controller.signal,
    });
    return getReleaseDownloads(response.ok ? await response.json() : null);
  } catch {
    // Keep the release page usable when GitHub is unavailable or rate-limits the visitor.
    return getReleaseDownloads(null);
  } finally {
    clearTimeout(timeout);
  }
}
