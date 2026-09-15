export default {
  title: 'Weekly',
  developmentFixture: 'Sample data · development only',
  navigation: {
    label: 'Week navigation',
    current: 'This week',
    backendRequired: 'Available when the weekly data binding is connected',
  },
  meta: {
    tracked: '{count} tracked minutes',
  },
  intro: {
    eyebrow: 'Weekly review',
    title: 'Where the week went',
    description: 'Tracked time, focus rhythm, daily distribution, and category mix in one factual view—without inventing insights that do not exist yet.',
  },
  overview: {
    eyebrow: 'Overview',
    title: 'Time and focus',
    description: 'Focus excludes idle categories; tracked time excludes system placeholders. Values come from the backend weekly aggregate.',
    focusAria: 'Focus accounts for {value} of tracked time',
  },
  metric: {
    tracked: 'Tracked time',
    focused: 'Focus time',
    other: 'Other time',
    focusRate: 'Focus share',
  },
  categories: {
    eyebrow: 'Composition',
    title: 'Category distribution',
    count: '{count} categories',
    distributionAria: 'Share of tracked time by category this week',
    total: 'Tracked',
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
    empty: 'No summarizable segments this week',
  },
  insights: {
    eyebrow: 'Insights',
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
      title: 'Loading weekly data',
      description: 'Tracked time, focus, and category composition will appear when the aggregate is ready.',
    },
    unavailable: {
      title: 'Weekly data is not available yet',
      description: 'Production does not inject sample statistics. Real weekly data will appear after GetWeeklyDashboard is connected.',
    },
    failure: {
      title: 'Weekly data could not be loaded',
      description: 'Existing statistics were not replaced. Try reading this week again.',
    },
    empty: {
      title: 'Nothing to summarize this week',
      description: 'Weekly statistics will appear after timeline cards have been created and processed.',
    },
  },
  scopeNote: 'Charts are built from the weekly aggregate\'s per-day detail and segment data. App-level relationship and flow diagrams require separate data contracts.',
}
