export const SETTINGS_SECTIONS = [
  'general',
  'recording',
  'ai',
  'storage',
  'agentAccess',
] as const

export type SettingsSection = (typeof SETTINGS_SECTIONS)[number]

export function settingsSectionFromQuery(value: unknown): SettingsSection {
  return typeof value === 'string' && SETTINGS_SECTIONS.includes(value as SettingsSection)
    ? value as SettingsSection
    : 'general'
}
