export default {
  title: 'Daily',
  developmentFixture: 'Sample data · development only',
  navigation: {
    label: 'Daily review date navigation',
    backendRequired: 'Previous and next day navigation will unlock when date data is fully connected',
  },
  meta: {
    tracked: '{count} minutes tracked',
  },
  overview: {
    eyebrow: 'Daily review',
    title: 'Understand today before planning tomorrow',
    description: 'Activity is grouped from its recorded time. The recap below only shows content already saved by the backend; the frontend does not fill in the gaps.',
  },
  workflow: {
    title: 'Workflow overview',
    description: 'See where the day went and when the context changed.',
    slotNote: '15 minutes per cell · a red corner dot marks an activity with a distraction record',
    empty: 'There is no activity to summarise on this day.',
    total: 'Day total',
  },
  stats: {
    title: 'Day statistics',
    contextSwitched: 'Context switches',
    interrupted: 'Distraction records',
    focusedFor: 'Active time',
    distractedFor: 'Distracted time',
    transitioning: 'Untracked gaps',
    times: '{count}',
  },
  standup: {
    title: 'Daily recap',
    description: 'A copy-ready view of completed work, next steps, and current constraints.',
    highlights: 'Completed work',
    tasks: 'Next steps',
    blockers: 'Current constraints',
    noBlockers: 'No constraints recorded.',
    copy: 'Copy recap',
    copied: 'Copied',
    copyFailed: 'Copy failed',
    generatedAt: 'Generated {date}',
    generateUnavailable: 'The recap generation binding is not available yet',
    unavailableTitle: 'Daily recap is not connected yet',
    unavailableDescription: 'The workflow remains available. Generation, editing, and autosave will unlock with the real bindings.',
    failureTitle: 'Daily recap could not be read',
    failureDescription: 'The activity overview is unaffected. The recap can be loaded again later.',
  },
  state: {
    loading: {
      title: 'Reading the daily overview',
      description: 'Activity and recap data load separately. Missing content is never filled with placeholder data.',
    },
    unavailable: {
      title: 'Daily data is not available yet',
      description: 'Production never injects sample activity. Real data will appear once date and timeline bindings are connected.',
    },
    failure: {
      title: 'The daily overview could not be read',
      description: 'Existing content was not changed. Try again now or return later.',
    },
    empty: {
      title: 'There is nothing to review on this day yet',
      description: 'Activity will appear after analysis finishes or a recap is saved.',
    },
    populated: {
      title: 'The daily overview is ready',
      description: 'Review the workflow and recap.',
    },
  },
  duration: {
    minutes: '{count} min',
    hours: '{count} hr',
    hoursMinutes: '{hours} hr {minutes} min',
  },
}
