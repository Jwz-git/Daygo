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
  applicationMenu: {
    hide: "隐藏 Daygo",
    hideOthers: "隐藏其他应用",
    showAll: "显示全部",
    background: "留在后台继续记录",
    edit: "编辑",
    undo: "撤销",
    redo: "重做",
    cut: "剪切",
    copy: "复制",
    paste: "粘贴",
    pasteMatch: "粘贴并匹配样式",
    delete: "删除",
    selectAll: "全选",
    speech: "语音",
    startSpeaking: "开始朗读",
    stopSpeaking: "停止朗读",
    window: "窗口",
    minimize: "最小化",
    zoom: "缩放",
    fullScreen: "全屏",
  },
}
