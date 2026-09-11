export default {
  title: 'Timeline',
  developmentFixture: 'Sample data · development only',
  navigation: {
    label: 'Logical day navigation',
    backendRequired: 'Previous and next day navigation will unlock with the timeline data binding',
  },
  meta: {
    tracked: '{count} minutes tracked',
  },
  filter: {
    label: 'Filter by category',
    all: 'All',
    manage: 'Manage categories',
    manageUnavailable: 'Category management bindings are not available yet',
  },
  track: {
    ariaLabel: 'Activity timeline arranged by hour',
  },
  processing: 'Analysing this range…',
  failure: {
    title: 'Analysis incomplete',
  },
  state: {
    loading: {
      eyebrow: 'Loading',
      title: 'Reading timeline',
      description: 'Reading activity for this day.',
    },
    unavailable: {
      eyebrow: 'Unavailable',
      title: 'Timeline data is not available yet',
      description: 'Timeline data is not connected yet. Production never fills this view with sample activity.',
    },
    empty: {
      eyebrow: 'Today',
      title: 'No activity on this day yet',
      description: 'Once capture and analysis finish, activity will appear here at the time it actually happened.',
    },
    processing: {
      eyebrow: 'Analysing',
      title: 'Organising activity ranges',
      description: 'Ranges in progress appear as skeletons and refresh automatically when ready.',
    },
    failure: {
      eyebrow: 'Needs attention',
      title: 'The timeline could not load completely',
      description: 'Failed ranges stay visible on the track instead of silently looking empty.',
    },
    populated: {
      eyebrow: 'Timeline',
      title: 'Activity is ready',
      description: 'Select an activity card to inspect its details.',
    },
  },
  overview: {
    eyebrow: 'Today',
    title: 'Time breakdown',
    tracked: 'Tracked',
    idle: 'Idle',
    noCategories: 'Category time will appear here once there is activity.',
  },
  inspector: {
    title: 'Activity detail',
    close: 'Close activity detail',
    summary: 'Summary',
    noSummary: 'This activity has no summary.',
    apps: 'Apps and sites',
    distractions: 'Distractions',
    frames: 'Activity frames',
    framesUnavailable: 'Media will load on demand once its binding ships, without blocking the timeline.',
    actionsUnavailable: 'Editing, category changes and deletion will unlock with the card write bindings',
    readOnly: 'Currently read-only',
  },
  duration: {
    minutes: '{count} min',
    hours: '{count} hr',
    hoursMinutes: '{hours} hr {minutes} min',
  },
}
