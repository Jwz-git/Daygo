export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: 'Choose an application',
    filterExecutable: 'Windows applications (*.exe)',
  },
  updater: {
    ownerRequired: 'Install updates from the Daygo window that is currently recording.',
  },
}
