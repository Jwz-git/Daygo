export const builtInCategoryKeys: Record<string, string> = {
  'Focus Work': 'focusWork',
  Communication: 'communication',
  Learning: 'learning',
  Research: 'research',
  Distraction: 'distraction',
  Personal: 'personal',
}

export function categoryLabel(name: string, translate: (key: string) => string): string {
  const key = builtInCategoryKeys[name]
  return key === undefined ? name : translate(`timeline.category.${key}`)
}
