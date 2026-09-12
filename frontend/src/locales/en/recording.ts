export default {
  state: {
    loading: 'Reading status',
    unknown: 'Status unknown',
    idle: 'Recording off',
    starting: 'Starting',
    capturing: 'Recording',
    paused: 'Recording paused',
  },
  action: {
    start: 'Start',
    pause: 'Pause',
    resume: 'Resume',
    stop: 'Stop',
  },
  working: 'Working…',
  notOwner: 'Another Daygo instance is managing recording.',
  permissionRequired: 'Screen recording permission is required to start.',
  actionFailed: 'The action did not finish. The last confirmed state is shown.',
}
