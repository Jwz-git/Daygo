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
  overview: {
    eyebrow: '概要',
    title: '時間と集中',
    description: '集中時間には気が散った時間とアイドルを含みません。記録時間にはシステムのプレースホルダーを含みません。',
    focusAria: '記録時間のうち集中が {value}',
  },
  metric: {
    tracked: '記録時間',
    focused: '集中時間',
    other: 'その他の時間',
    focusRate: '集中の割合',
  },
  categories: {
    eyebrow: '構成',
    title: 'カテゴリの分布',
    count: '{count} カテゴリ',
    distributionAria: '今週のカテゴリ別の記録時間の割合',
    total: '合計',
  },
  daily: {
    eyebrow: 'パターン',
    title: '日ごとのタイムライン',
    hint: '1 行が 1 日、各ブロックがカテゴリの区間です',
    idleTag: 'アイドル',
    noActivity: '活動なし',
  },
  rhythm: {
    eyebrow: 'リズム',
    title: '1 週間の活動リズム',
    focus: '集中',
    idle: 'アイドル',
    chartAria: '時間帯ごとの集中とアイドルの分数',
    empty: '今週はまだ集計できる時間帯がありません',
  },
  insights: {
    title: '今週のまとめ',
    activeDays: 'アクティブ日数',
    daysCount: '{count} 日',
    ofSeven: '/ 7 日',
    longestFocus: '最長の集中',
    peakHour: 'ピーク時間帯',
    avgFocus: '1 日あたりの平均集中',
    perActiveDay: 'アクティブ日あたり',
    busiestDay: '最も忙しい日',
    none: 'なし',
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
}
