import { SetNativeUiLabels } from '../../bindings/github.com/Jwz-git/Daygo/internal/app/backend'
import type * as app from '../../bindings/github.com/Jwz-git/Daygo/internal/app/models'

export type NativeUiLabels = Omit<app.NativeUiLabelsDTO, 'convertValues'> & {
  journalReminderTitle: string
  journalReminderBody: string
}

/**
 * Pushes the localized copy for native surfaces the webview cannot reach — the
 * application menu, picker and the updater's install refusal. Like the menu-bar
 * bundle, it is best-effort: outside a Wails host (standalone preview) the
 * generated binding is absent, so the failure is swallowed rather than
 * surfaced.
 */
export async function setNativeUiLabels(labels: NativeUiLabels): Promise<void> {
  try {
    await SetNativeUiLabels(labels)
  } catch {
    // No native host on this platform.
  }
}
