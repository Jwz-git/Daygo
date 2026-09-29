type Subscribe = (callback: (visible: boolean) => void) => () => void

/** Combines native state with document visibility, rejecting stale snapshots. */
export class VisibilitySource {
  private nativeVisible = true
  private documentVisible = true
  private active = false
  private generation = 0
  private stopEvents: (() => void) | null = null

  constructor(private readonly subscribe: Subscribe,
    private readonly snapshot: () => Promise<boolean>,
    private readonly changed: (visible: boolean) => void) {}

  get visible(): boolean { return this.nativeVisible && this.documentVisible }

  async start(): Promise<void> {
    if (this.active) return
    this.active = true
    this.stopEvents = this.subscribe((visible) => {
      if (!this.active) return
      this.generation += 1
      this.update(() => { this.nativeVisible = visible })
    })
    await this.refresh()
  }

  async refresh(): Promise<void> {
    if (!this.active) return
    const generation = ++this.generation
    try {
      const visible = await this.snapshot()
      if (this.active && this.generation === generation) {
        this.update(() => { this.nativeVisible = visible })
      }
    } catch { /* Bridge unavailable: retain document visibility and current state. */ }
  }

  setDocumentVisible(visible: boolean): void {
    this.update(() => { this.documentVisible = visible })
  }
  private update(mutate: () => void): void {
    const before = this.visible
    mutate()
    if (before !== this.visible) this.changed(this.visible)
  }
  stop(): void {
    this.active = false
    this.generation += 1
    this.stopEvents?.()
    this.stopEvents = null
  }
}
