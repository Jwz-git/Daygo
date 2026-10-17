export default {
  title: 'Test tools',
  badge: 'Test only',
  capture: {
    title: 'Capture test',
  },
  clear: {
    title: 'Data reset',
    action: 'Clear history data',
    confirm: 'This deletes all screenshots and timeline records (AI services and settings are kept). Continue?',
    pending: 'Clearing…',
    unavailable: 'Can’t clear right now',
    note: 'Clears all recordings and timeline history. AI services and other settings are not affected.',
    failed: 'Couldn’t clear. Nothing was changed.',
  },
  onboarding: {
    title: 'First-run guide',
    action: 'Preview the guide',
    note: 'Opens the new-user guide as a preview: categories are not replaced and the completion state does not change; AI service and preference changes are still saved.',
  },
}
