export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: '選擇應用程式',
    filterExecutable: 'Windows 應用程式 (*.exe)',
  },
  updater: {
    ownerRequired: '請在正在錄製的 Daygo 視窗中安裝更新。',
  },
  applicationMenu: {
    hide: "隱藏 Daygo",
    hideOthers: "隱藏其他應用程式",
    showAll: "顯示全部",
    background: "留在背景繼續記錄",
    edit: "編輯",
    undo: "復原",
    redo: "重做",
    cut: "剪下",
    copy: "複製",
    paste: "貼上",
    pasteMatch: "貼上並符合樣式",
    delete: "刪除",
    selectAll: "全選",
    speech: "語音",
    startSpeaking: "開始朗讀",
    stopSpeaking: "停止朗讀",
    window: "視窗",
    minimize: "最小化",
    zoom: "縮放",
    fullScreen: "全螢幕",
  },
}
