export const builtInCategoryKeys: Record<string, string> = {
  'focus work': 'focusWork',
  communication: 'communication',
  learning: 'learning',
  research: 'research',
  distraction: 'distraction',
  personal: 'personal',
}

export function categoryKey(name: string): string {
  return name.trim().toLocaleLowerCase()
}

export function categoryLabel(name: string, translate: (key: string) => string): string {
  const key = builtInCategoryKeys[categoryKey(name)]
  return key === undefined ? name : translate(`timeline.category.${key}`)
}
