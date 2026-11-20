/** Test double for the v3 runtime boundary. Existing anonymous backend and
 * event fixtures are reused; application code still consumes v3 envelopes. */
type FixtureRuntime = {
 EventsOnMultiple?: (name: string, callback: (...data: unknown[]) => void, max: number) => (() => void) | undefined
 BrowserOpenURL?: (url: string) => void
 Environment?: () => Promise<unknown>
}
type FixtureWindow = { go?: { app?: { Backend?: Record<string, unknown> } }; runtime?: FixtureRuntime }
function fixture(): FixtureWindow { return globalThis.window as unknown as FixtureWindow }
export const Call = {
 async ByName(name: string, ...args: unknown[]): Promise<unknown> {
  const method = fixture().go?.app?.Backend?.[name.slice(name.lastIndexOf('.') + 1)]
  if (typeof method !== 'function') throw new Error('fixture binding unavailable')
  return method(...args)
 },
}
export const Events = {
 OnMultiple(name: string, callback: (event: { name: string; data: unknown }) => void, max: number): () => void {
  return fixture().runtime?.EventsOnMultiple?.(name, (...data) => callback({ name, data: data[0] }), max) ?? (() => undefined)
 },
}
export const Browser = { OpenURL: async (url: string) => fixture().runtime?.BrowserOpenURL?.(url) }
export const System = { Environment: async () => {
 const value = await fixture().runtime?.Environment?.()
 return { OS: typeof value === 'object' && value !== null && 'platform' in value ? value.platform : 'darwin' }
} }
export const Window = {
 Hide: async () => undefined, Minimise: async () => undefined,
 ToggleMaximise: async () => undefined, IsMaximised: async () => false,
}
