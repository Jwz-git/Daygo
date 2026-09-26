/** A visibility-aware clock. Playback intention is independent of resource suspension. */
export class PlaybackScheduler {
  private playing = false
  private visible = true
  private disposed = false
  private raf: number | null = null
  private last: number | null = null

  constructor(private readonly advance: (seconds: number) => void,
    private readonly request: (callback: (now: number) => void) => number = (callback) => requestAnimationFrame(callback),
    private readonly cancel: (id: number) => void = (id) => cancelAnimationFrame(id)) {}

  setPlaying(playing: boolean): void { this.playing = playing; this.sync() }
  setVisible(visible: boolean): void { this.visible = visible; this.sync() }
  private sync(): void {
    if (this.disposed || !this.playing || !this.visible) {
      if (this.raf !== null) this.cancel(this.raf)
      this.raf = null
      this.last = null
    } else if (this.raf === null) {
      this.raf = this.request(this.tick)
    }
  }
  private tick = (now: number): void => {
    this.raf = null
    if (this.disposed || !this.playing || !this.visible) return
    if (this.last !== null) this.advance(Math.max(0, (now - this.last) / 1000))
    this.last = now
    this.sync()
  }
  dispose(): void { this.disposed = true; this.sync() }
}
