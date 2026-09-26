import { SetNativeUiLabels } from '../../wailsjs/go/app/Backend'
import { app } from '../../wailsjs/go/models'

export type NativeUiLabels = Omit<app.NativeUiLabelsDTO, 'convertValues'>

/**
 * Pushes the localized copy for native surfaces the webview cannot reach — the
 * application menu, picker and the updater's install refusal. Like the menu-bar
 * bundle, it is best-effort: outside a Wails host (standalone preview) the
 * generated binding is absent, so the failure is swallowed rather than
 * surfaced.
 */
export async function setNativeUiLabels(labels: NativeUiLabels): Promise<void> {
  try {
    await SetNativeUiLabels(new app.NativeUiLabelsDTO(labels))
  } catch {
    // No native host on this platform.
  }
}
