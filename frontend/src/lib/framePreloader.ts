export const frameSrc = (id: number): string => `/media/frame?id=${id}`
export const thumbnailSrc = (id: number): string => `/media/thumbnail?id=${id}`

interface PreloadImage {
  src: string
  onload: unknown
  onerror: unknown
  removeAttribute: (name: string) => void
}

/** Owns only the next three frames; browser cache lifetime remains WebKit's decision. */
export class FramePreloader {
  private readonly images = new Map<number, PreloadImage>()
  constructor(private readonly create: () => PreloadImage = () => new Image()) {}

  update(ids: readonly number[]): void {
    const wanted = new Set(ids.slice(0, 3))
    for (const [id, image] of this.images) {
      if (!wanted.has(id)) { this.release(image); this.images.delete(id) }
    }
    for (const id of wanted) {
      if (this.images.has(id)) continue
      const image = this.create()
      this.images.set(id, image)
      image.src = frameSrc(id)
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
