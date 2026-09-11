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
    description: 'Tracked time, focus, and category mix in one factual view—without inventing insights that do not exist yet.',
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
  },
  duration: {
    minutes: '{count} min',
    hours: '{count} hr',
    hoursMinutes: '{hours} hr {minutes} min',
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
  scopeNote: 'This slice shows only the weekly aggregate in the public contract. Workflow heatmaps, app relationships, and generated insights require separate data contracts.',
}
