import assert from 'node:assert/strict'
import test from 'node:test'
import { FramePreloader, frameSrc, thumbnailSrc } from '../src/lib/framePreloader'

test('preloader owns at most three images and releases on card changes and dispose', () => {
  const images: { src: string; onload: unknown; onerror: unknown; removeAttribute: (name: string) => void }[] = []
  const loader = new FramePreloader(() => {
    const image = { src: '', onload: null as unknown, onerror: null as unknown,
      removeAttribute(name: string) { if (name === 'src') this.src = '' } }
    images.push(image)
    return image
  })
  loader.update([1, 2, 3, 4])
  assert.equal(loader.size, 3)
  assert.deepEqual(images.map((image) => image.src), [frameSrc(1), frameSrc(2), frameSrc(3)])
  loader.update([2, 3, 5])
  assert.equal(images[0]?.src, '')
  assert.equal(loader.size, 3)
  loader.update([10])
  assert.equal(loader.size, 1)
  assert.ok(images.slice(0, -1).every((image) => image.src === '' && image.onload === null && image.onerror === null))
  loader.clear()
  assert.equal(loader.size, 0)
  assert.ok(images.every((image) => image.src === ''))
  assert.equal(thumbnailSrc(10), '/media/thumbnail?id=10')
})
