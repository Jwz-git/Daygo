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
  journalReminder: {
    title: '今日のジャーナルを書こう',
    body: '数分かけて、今日の進捗と明日の予定を書き留めましょう。',
  },
  plan: {
    startTitle: "開始：{title}",
    startBody: "予定 {start}–{end}",
    distractionTitle: "集中が途切れています",
    distractionBody: "「{title}」の最中に {minutes} 分脱線しています。",
    dayDistractionTitle: "今日の脱線が上限を超えました",
    dayDistractionBody: "今日は {minutes} 分脱線しました（上限 {limit} 分）。",
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
