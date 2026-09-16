export const builtInCategoryKeys: Record<string, string> = {
  'focus work': 'focusWork',
  communication: 'communication',
  learning: 'learning',
  research: 'research',
  distraction: 'distraction',
  personal: 'personal',
}

/*
 * The English copy the v12 migration seeds for those six categories
 * (internal/storage/migrate.go seedStarterCategories). Category rows are data
 * the model matches activity against, so the UI localizes a default only while
 * the row still holds exactly this text — anything the user rewrote renders
 * verbatim. frontend/tests/categoryDefaults.test.ts fails when the two sides
 * drift apart.
 */
export const builtInCategoryDetails: Record<string, string> = {
  'focus work': 'Focused work: writing, refactoring, or debugging code in an IDE or terminal; deep hands-on building',
  communication: 'Meetings, standups, Slack, email, video calls, messaging, and syncs',
  learning: 'Lectures, reading docs or courses, flashcards, tutorials, and deliberately studying new skills',
  research: 'Exploring tools and APIs, reading papers or Stack Overflow, and writing design docs or technical specs',
  distraction: 'Unfocused browsing and passive content consumption: social media feeds, random videos, idle scrolling, entertainment with no clear intent, and gaming',
  personal: 'Intentional non-work activity with a purpose: messaging friends and family, managing finances, booking travel, errands, life admin, and hobbies',
}

export function categoryKey(name: string): string {
  return name.trim().toLocaleLowerCase()
}

export function categoryLabel(name: string, translate: (key: string) => string): string {
  const key = builtInCategoryKeys[categoryKey(name)]
  return key === undefined ? name : translate(`timeline.category.${key}`)
}

export function categoryDetails(
  name: string,
  details: string,
  translate: (key: string) => string,
): string {
  const normalized = categoryKey(name)
  const key = builtInCategoryKeys[normalized]
  if (key === undefined || builtInCategoryDetails[normalized] !== details) return details
  return translate(`timeline.categoryDetails.${key}`)
}
