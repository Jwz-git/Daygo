/** Give anonymous native fixtures the v3 environment marker. Old go/runtime
 * fields are consumed exclusively by wailsRuntimeFixture, never by the app. */
export function nativeFixtureWindow<T extends object>(value: T): T {
 if ('go' in value || ('runtime' in value && typeof value.runtime === 'object' && value.runtime !== null &&
     ('BrowserOpenURL' in value.runtime || 'Environment' in value.runtime))) {
  Object.assign(value, { _wails: { environment: { OS: 'darwin' } } })
 }
 return value
}
