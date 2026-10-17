import type { CategoryArchetype, CategoryPresetDTO } from '@/api/dto'

/*
 * First-run category presets, one per role, after Dayflow's
 * OnboardingCategoryPreset (Models/TimelineCategory.swift): the same seven
 * roles, the same categories, descriptions and starter colors. Each set ends
 * with the shared Distraction and Personal rows. They are offered only by the
 * onboarding guide — the ApplyCategoryPreset binding refuses once onboarding
 * is complete, so a user's set with history is never rewritten wholesale.
 *
 * Names and descriptions are i18n keys; applying writes the copy of the
 * current language (category rows are data the model matches against, stored
 * as plain text). Archetypes are Daygo's own addition.
 */
export const PROFESSION_PRESET_IDS = [
  'software-engineer',
  'founder-executive',
  'designer',
  'student',
  'product-manager',
  'data-scientist',
  'other',
] as const
export type ProfessionPresetId = (typeof PROFESSION_PRESET_IDS)[number]

export interface PresetCategory {
  /** Key under the preset's categories, or under presets.common when shared. */
  key: string
  colorHex: string
  archetype: CategoryArchetype
  shared?: boolean
}

export interface ProfessionPreset {
  id: ProfessionPresetId
  /** Key under presets.profession. */
  nameKey: string
  categories: readonly PresetCategory[]
}

// Dayflow's starter colors, in the order its presets use them.
const BLUE = '#6A7EFF'
const CYAN = '#56CFEE'
const VIOLET = '#C787F7'
const PEACH = '#FFAE8C'

const SHARED_TAIL: readonly PresetCategory[] = [
  { key: 'distraction', colorHex: '#FF4721', archetype: 'camp', shared: true },
  { key: 'personal', colorHex: '#ADE3E3', archetype: 'camp', shared: true },
]

export const PROFESSION_PRESETS: readonly ProfessionPreset[] = [
  {
    id: 'software-engineer',
    nameKey: 'softwareEngineer',
    categories: [
      { key: 'coding', colorHex: BLUE, archetype: 'forge' },
      { key: 'codeReview', colorHex: CYAN, archetype: 'nexus' },
      { key: 'research', colorHex: VIOLET, archetype: 'sanctum' },
      { key: 'communication', colorHex: PEACH, archetype: 'nexus' },
      ...SHARED_TAIL,
    ],
  },
  {
    id: 'founder-executive',
    nameKey: 'founderExecutive',
    categories: [
      { key: 'building', colorHex: BLUE, archetype: 'forge' },
      { key: 'strategy', colorHex: CYAN, archetype: 'sanctum' },
      { key: 'insights', colorHex: VIOLET, archetype: 'sanctum' },
      { key: 'communication', colorHex: PEACH, archetype: 'nexus' },
      ...SHARED_TAIL,
    ],
  },
  {
    id: 'designer',
    nameKey: 'designer',
    categories: [
      { key: 'design', colorHex: BLUE, archetype: 'forge' },
      { key: 'research', colorHex: CYAN, archetype: 'sanctum' },
      { key: 'communication', colorHex: PEACH, archetype: 'nexus' },
      ...SHARED_TAIL,
    ],
  },
  {
    id: 'student',
    nameKey: 'student',
    categories: [
      { key: 'studying', colorHex: BLUE, archetype: 'sanctum' },
      { key: 'assignments', colorHex: CYAN, archetype: 'forge' },
      { key: 'communication', colorHex: PEACH, archetype: 'nexus' },
      ...SHARED_TAIL,
    ],
  },
  {
    id: 'product-manager',
    nameKey: 'productManager',
    categories: [
      { key: 'specs', colorHex: BLUE, archetype: 'forge' },
      { key: 'research', colorHex: CYAN, archetype: 'sanctum' },
      { key: 'communication', colorHex: PEACH, archetype: 'nexus' },
      ...SHARED_TAIL,
    ],
  },
  {
    id: 'data-scientist',
    nameKey: 'dataScientist',
    categories: [
      { key: 'analysis', colorHex: BLUE, archetype: 'forge' },
      { key: 'dataEngineering', colorHex: CYAN, archetype: 'order' },
      { key: 'research', colorHex: VIOLET, archetype: 'sanctum' },
      { key: 'communication', colorHex: PEACH, archetype: 'nexus' },
      ...SHARED_TAIL,
    ],
  },
  {
    id: 'other',
    nameKey: 'other',
    categories: [
      { key: 'work', colorHex: BLUE, archetype: 'forge' },
      { key: 'communication', colorHex: PEACH, archetype: 'nexus' },
      ...SHARED_TAIL,
    ],
  },
]

export function professionPreset(id: ProfessionPresetId): ProfessionPreset {
  const preset = PROFESSION_PRESETS.find((candidate) => candidate.id === id)
  if (preset === undefined) throw new Error(`unknown profession preset: ${id}`)
  return preset
}

export function presetCategoryKey(preset: ProfessionPreset, category: PresetCategory, field: 'name' | 'details'): string {
  return category.shared === true
    ? `presets.common.${category.key}.${field}`
    : `presets.profession.${preset.nameKey}.categories.${category.key}.${field}`
}

/** The rows ApplyCategoryPreset receives, in the current language. */
export function presetRows(preset: ProfessionPreset, translate: (key: string) => string): CategoryPresetDTO[] {
  return preset.categories.map((category) => ({
    name: translate(presetCategoryKey(preset, category, 'name')),
    details: translate(presetCategoryKey(preset, category, 'details')),
    colorHex: category.colorHex,
    archetype: category.archetype,
  }))
}
