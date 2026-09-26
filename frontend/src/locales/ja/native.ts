export default {
  // Surfaces the native layer renders outside the webview: the frontend pushes
  // this bundle at startup and on every language change (docs/05 §5.5.1).
  applicationPicker: {
    title: 'アプリケーションを選択',
    filterExecutable: 'Windows アプリケーション (*.exe)',
  },
  updater: {
    ownerRequired: '記録中の Daygo ウィンドウでアップデートをインストールしてください。',
  },
  applicationMenu: {
    hide: "Daygo を隠す",
    hideOthers: "ほかのアプリを隠す",
    showAll: "すべて表示",
    background: "バックグラウンドで記録を続ける",
    edit: "編集",
    undo: "取り消す",
    redo: "やり直す",
    cut: "切り取る",
    copy: "コピー",
    paste: "ペースト",
    pasteMatch: "ペーストしてスタイルを合わせる",
    delete: "削除",
    selectAll: "すべて選択",
    speech: "スピーチ",
    startSpeaking: "読み上げを開始",
    stopSpeaking: "読み上げを停止",
    window: "ウインドウ",
    minimize: "しまう",
    zoom: "拡大／縮小",
    fullScreen: "フルスクリーン",
  },
}
