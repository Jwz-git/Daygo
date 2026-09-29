export default {
  title: 'Weekly',
  developmentFixture: 'Sample data · development only',
  navigation: {
    label: 'Week navigation',
    current: 'This week',
    backendRequired: 'Switching weeks is unavailable right now',
  },
  meta: {
    tracked: '{count} min recorded',
  },
  intro: {
    eyebrow: 'Weekly review',
    title: 'Where the week went',
    description: 'Your recorded time, focus rhythm, daily spread, and category mix for the week.',
  },
  overview: {
    eyebrow: 'Overview',
    title: 'Time and focus',
    description: 'Focus time excludes distraction and idle; recorded time excludes system placeholders.',
    focusAria: 'Focus accounts for {value} of recorded time',
  },
  metric: {
    tracked: 'Recorded time',
    focused: 'Focus time',
    other: 'Other time',
    focusRate: 'Focus share',
  },
  categories: {
    eyebrow: 'Composition',
    title: 'Category distribution',
    count: '{count} categories',
    distributionAria: 'Share of recorded time by category this week',
    total: 'Total',
  },
  daily: {
    eyebrow: 'Pattern',
    title: 'Daily timeline',
    hint: 'One row per day; each block is a category segment',
    idleTag: 'idle',
    noActivity: 'No activity',
  },
  rhythm: {
    eyebrow: 'Rhythm',
    title: 'Weekly activity rhythm',
    focus: 'Focus',
    idle: 'Idle',
    chartAria: 'Focus and idle minutes per hour of day',
    empty: 'Nothing to summarize this week yet',
  },
  insights: {
    title: 'This week at a glance',
    activeDays: 'Active days',
    daysCount: '{count} days',
    ofSeven: 'out of 7',
    longestFocus: 'Longest focus',
    peakHour: 'Peak hour',
    avgFocus: 'Avg daily focus',
    perActiveDay: 'per active day',
    busiestDay: 'Busiest day',
    none: 'None',
  },
  state: {
    loading: {
      title: 'Loading this week',
      description: 'Just a moment.',
    },
    unavailable: {
      title: 'Weekly view is unavailable right now',
      description: 'Try again later, or restart Daygo.',
    },
    failure: {
      title: 'Couldn’t load this week',
      description: 'Your data is unaffected. You can try again.',
    },
    empty: {
      title: 'Nothing recorded this week yet',
      description: 'Your weekly review will appear here once Daygo has recorded and organized some activity.',
    },
  },
  scopeNote: 'Based on your daily activity cards.',
}
