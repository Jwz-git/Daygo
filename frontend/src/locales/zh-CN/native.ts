export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: '选择应用',
    filterExecutable: 'Windows 应用 (*.exe)',
  },
  updater: {
    ownerRequired: '请在正在录制的那个 Daygo 窗口中安装更新。',
  },
}
