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
  charts: {
    distribution: {
      title: 'Weekly distribution',
      total: 'Total',
      aria: 'Share of time per category this week',
    },
    context: {
      title: 'Context shifts and distractions',
      shifts: 'Context shifts',
      distractions: 'Distractions',
      distribution: 'Through the day (10:00–18:00)',
      comparison: 'Per day',
      insight: '{day} had the most interruptions: {shifts} context shifts and {distracted} distractions.',
      insightNone: 'No context shift or distraction pattern this week.',
      aria: 'Context shifts and distractions per day',
    },
    workflow: {
      title: 'Your workflow this week',
      total: 'Week total',
      aria: 'Main category for each part of each day',
    },
    heatmap: {
      title: 'Focus and distraction heat map',
      focused: 'Focused work',
      distracted: 'Distracted',
      aria: 'How focused or distracted each part of each day was',
    },
    treemap: {
      title: 'Most used per category',
      empty: 'No app data this week yet',
      aria: 'Apps with the most time in each category',
      // Dayflow's compact units ("6hr 4m", "+ 152m"), kept in English on purpose.
      hoursMinutes: '{hours}hr {minutes}m',
      hours: '{hours}hr',
      minutes: '{minutes}m',
    },
    sankey: {
      title: 'Time between categories and apps',
      aria: 'How this week’s time flowed from categories into apps',
    },
    tooltip: {
      shifts: '{count} context shifts',
      distractions: '{count} distractions',
      noRecord: 'Nothing recorded',
      share: '{value} of {name}',
      change: '{value} vs last week',
      newThisWeek: 'Not used last week',
      pinHint: 'Click to pin the highlight, click again to release',
    },
    otherApp: 'Other',
    otherCategory: 'Other',
  },
}
