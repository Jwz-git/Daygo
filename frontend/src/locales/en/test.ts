export default {
  title: 'Test tools',
  badge: 'Test only',
  capture: {
    title: 'Capture test',
  },
  clear: {
    title: 'Data reset',
    action: 'Clear history data',
    confirm: 'This deletes all screenshots, batches, observations and timeline cards (providers and settings are kept). Continue?',
    pending: 'Clearing…',
    unavailable: 'Clear binding is not connected or this instance is read-only',
    note: 'Test only: wipes recording and analysis history in one click; provider config is untouched.',
    failed: 'The operation did not complete; nothing was rewritten.',
  },
}
