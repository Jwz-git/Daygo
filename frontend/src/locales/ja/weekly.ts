export default {
  title: 'ウィークリー',
  developmentFixture: 'サンプルデータ · 開発時のみ',
  navigation: {
    label: '週のナビゲーション',
    current: '今週',
    backendRequired: '現在は週を切り替えられません',
  },
  meta: {
    tracked: '{count} 分を記録',
  },
  intro: {
    eyebrow: 'ウィークリーレビュー',
    title: '今週、時間はどこへ',
    description: '今週の記録時間、集中のリズム、日ごとの配分、カテゴリの内訳を確認できます。',
  },
  state: {
    loading: {
      title: '今週を読み込み中',
      description: '少々お待ちください。',
    },
    unavailable: {
      title: 'ウィークリーは現在表示できません',
      description: 'しばらくしてから再度お試しいただくか、Daygo を再起動してください。',
    },
    failure: {
      title: '今週を読み込めませんでした',
      description: 'データには影響ありません。もう一度お試しください。',
    },
    empty: {
      title: '今週はまだ記録がありません',
      description: 'Daygo が活動を記録して整理すると、ここにウィークリーレビューが表示されます。',
    },
  },
  scopeNote: '毎日の活動カードに基づく集計です。',
  charts: {
    distribution: {
      title: '今週の内訳',
      total: '合計',
      aria: '今週のカテゴリ別の時間の割合',
    },
    context: {
      title: 'コンテキスト切り替えと注意散漫',
      shifts: 'コンテキスト切り替え',
      distractions: '注意散漫',
      distribution: '時間帯の分布（10:00–18:00）',
      comparison: '日別の比較',
      insight: '{day}が最も中断されました：切り替え {shifts} 回、注意散漫 {distracted} 回。',
      insightNone: '今週は目立った切り替えや注意散漫はありませんでした。',
      aria: '日ごとのコンテキスト切り替えと注意散漫の回数',
    },
    workflow: {
      title: '今週のワークフロー',
      total: '今週の合計',
      aria: '各日の時間帯ごとの主なカテゴリ',
    },
    heatmap: {
      title: '集中と注意散漫のヒートマップ',
      focused: '集中作業',
      distracted: '注意散漫',
      aria: '各日の時間帯ごとの集中と注意散漫の度合い',
    },
    treemap: {
      title: 'カテゴリ別のよく使うアプリ',
      empty: '今週のアプリデータはまだありません',
      aria: '各カテゴリで最も長く使ったアプリ',
      // Dayflow's compact units ("6hr 4m", "+ 152m"), kept in English on purpose.
      hoursMinutes: '{hours}hr {minutes}m',
      hours: '{hours}hr',
      minutes: '{minutes}m',
    },
    sankey: {
      title: 'カテゴリとアプリの時間の流れ',
      aria: '今週の時間がカテゴリからアプリへどう流れたか',
    },
    tooltip: {
      shifts: 'コンテキスト切り替え {count} 回',
      distractions: '注意散漫 {count} 回',
      noRecord: '記録なし',
      share: '{name}の {value}',
      change: '先週比 {value}',
      newThisWeek: '先週は未使用',
      pinHint: 'クリックでハイライトを固定、もう一度クリックで解除',
    },
    otherApp: 'その他',
    otherCategory: 'その他',
  },
}
