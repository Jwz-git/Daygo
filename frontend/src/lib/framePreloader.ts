export const frameSrc = (id: number): string => `/media/frame?id=${id}`
export const previewSrc = (id: number): string => `/media/preview?id=${id}`
export const thumbnailSrc = (id: number): string => `/media/thumbnail?id=${id}`

interface PreloadImage {
  src: string
  onload: unknown
  onerror: unknown
  removeAttribute: (name: string) => void
}

/** Owns only the next three frames; browser cache lifetime remains WebKit's decision. */
export class FramePreloader {
  private readonly images = new Map<string, PreloadImage>()
  constructor(private readonly create: () => PreloadImage = () => new Image()) {}

  update(urls: readonly string[]): void {
    const wanted = new Set(urls.slice(0, 3))
    for (const [url, image] of this.images) {
      if (!wanted.has(url)) { this.release(image); this.images.delete(url) }
    }
    for (const url of wanted) {
      if (this.images.has(url)) continue
      const image = this.create()
      this.images.set(url, image)
      image.src = url
    }
  }

  private release(image: PreloadImage): void {
    image.onload = null
    image.onerror = null
    image.removeAttribute('src')
  }

  clear(): void {
    for (const image of this.images.values()) this.release(image)
    this.images.clear()
  }
  get size(): number { return this.images.size }
}
