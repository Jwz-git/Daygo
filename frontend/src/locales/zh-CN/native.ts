export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: '选择应用',
    filterExecutable: 'Windows 应用 (*.exe)',
  },
  updater: {
    ownerRequired: '只有持有捕获所有权的 Daygo 实例才能安装更新。',
  },
}
