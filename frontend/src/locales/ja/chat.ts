export default {
  actionError: '問題が発生しました。もう一度お試しください。入力した内容は残っています。',
  retry: '再試行',
  working: '考えています…',
  tooLong: 'メッセージが長すぎます。短くしてもう一度お試しください。',

  title: 'チャット',
  newConversation: '新しいチャット',
  showSidebar: 'サイドバーを表示',
  hideSidebar: 'サイドバーを隠す',
  emptyConversations: 'まだチャットがありません。メッセージを送って始めましょう。',
  deleteConversation: 'このチャットを削除',
  renameConversation: 'このチャットの名前を変更',
  rename: '名前を変更',
  renameTitlePlaceholder: 'チャットのタイトルを入力',
  renameTitleRequired: 'タイトルは必須です',
  removeConfirm: 'このチャットを削除しますか？',
  unavailableTitle: 'チャットは現在利用できません',
  unavailableDescription: 'しばらくしてから再度お試しいただくか、Daygo を再起動してください。',
  provider: {
    label: 'AI サービス',
    placeholder: 'AI サービスが選択されていません',
  },
  model: {
    label: 'モデル',
    follow: 'デフォルト（{model}）',
  },
  composer: {
    placeholder: 'メッセージを入力…（Enter で送信）',
    send: '送信',
    cancel: '停止',
  },
  status: {
    failed: '失敗',
    canceled: 'キャンセル済み',
  },
  failure: {
    no_provider: 'AI サービスが設定されていません。設定で追加してください。',
    no_provider_selected: 'この会話では AI サービスが選択されていません。',
    canceled: 'キャンセルしました。',
    authentication: 'API キーが拒否されました。設定を確認してください。',
    rate_limited: 'サービスが混み合っています。しばらくしてからお試しください。',
    timeout: 'サービスが時間内に応答しませんでした。',
    dns: 'サービスのアドレスを解決できませんでした。設定を確認してください。',
    connection: 'サービスに接続できませんでした。起動状態と到達性を確認してください。',
    tls: 'サービスとの安全な接続に失敗しました。アドレスと証明書を確認してください。',
    network: 'リクエストがサービスに到達しませんでした。ネットワークを確認してください。',
    invalid_request: 'サービスがリクエストを拒否しました。アドレスとモデルを確認してください。',
    unsupported_feature: 'このモデルは会話に必要な構造化出力に対応していません。',
    invalid_output: 'サービスの応答を解釈できませんでした。モデルがこの API に合うか確認してください。',
    tool_budget: 'このターンのツール呼び出し上限に達したため、中断しました。',
    internal: 'Daygo 内部でエラーが発生しました。もう一度お試しください。',
  },
  memory: {
    title: 'グローバル指示',
    hint: 'Daygo に毎回のチャットで覚えておいてほしいことを書きます。',
    placeholder: '例：回答は短めに。名前は「佐藤」と呼んで…',
    save: '保存',
    saved: '保存しました',
  },
  loadError: 'チャットを読み込めませんでした',

  drawer: {
    conversations: 'チャット',
    memory: 'グローバル指示',
    newChat: '新しいチャット',
    today: '今日',
    yesterday: '昨日',
    older: 'それ以前',
    untitled: '無題のチャット',
  },

  bubble: {
    copy: 'コピー',
    copied: 'コピーしました',
    roleAssistant: 'Daygo',
  },

  welcome: {
    title: 'こんにちは。何をお手伝いしましょうか？',
    subtitle: 'タイムライン、デイリー、ウィークリーのことは何でも聞いてください',
    hints: [
      '今日は何をしましたか？',
      '今週のカテゴリ別の割合は？',
      '新しいカテゴリを追加：学習',
      '昨日のジャーナルを見せて',
    ],
  },

  dateDivider: {
    today: '今日',
    yesterday: '昨日',
  },
}
